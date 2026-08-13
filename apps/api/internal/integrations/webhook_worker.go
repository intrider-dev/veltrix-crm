package integrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

type OutboxWebhookEvent struct {
	ID            ids.UUID
	EventType     string
	SchemaVersion int
	AggregateType string
	AggregateID   ids.UUID
	CorrelationID ids.UUID
	Payload       json.RawMessage
	CreatedAt     time.Time
}

type WebhookFanoutRepository interface {
	Fanout(context.Context, worker.Job, string, string) error
}

const maxWebhookJobPayload = 8 << 10

// webhookDispatchPointer mirrors the canonical pointer emitted by the
// platform outbox dispatcher. Keep strict decoding here: accepting every
// unknown field would make incompatible dispatcher changes fail silently.
type webhookDispatchPointer struct {
	OutboxEventID string `json:"outboxEventId"`
	EventType     string `json:"eventType"`
	SchemaVersion int32  `json:"schemaVersion"`
	AggregateType string `json:"aggregateType"`
	AggregateID   string `json:"aggregateId"`
	CorrelationID string `json:"correlationId"`
}

func NewWebhookDispatchHandler(repository WebhookFanoutRepository) worker.Handler {
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if repository == nil {
			return webhookFailure{code: "webhook_not_configured", err: errors.New("webhook fanout repository is required")}
		}
		pointer, err := decodeWebhookDispatchPointer(job)
		if err != nil {
			return webhookFailure{code: "webhook_payload_invalid", err: errors.New("invalid webhook dispatch payload")}
		}
		if err := repository.Fanout(ctx, job, pointer.OutboxEventID, pointer.EventType); err != nil {
			return webhookFailure{code: "webhook_fanout_failed", err: err}
		}
		return nil
	}
}

func decodeWebhookDispatchPointer(job worker.Job) (webhookDispatchPointer, error) {
	if job.SchemaVersion != 1 || len(job.Payload) == 0 || len(job.Payload) > maxWebhookJobPayload {
		return webhookDispatchPointer{}, errors.New("invalid webhook dispatch payload envelope")
	}
	var pointer webhookDispatchPointer
	if err := decodeStrictJSON(job.Payload, &pointer); err != nil {
		return webhookDispatchPointer{}, err
	}
	if pointer.OutboxEventID == "" || pointer.EventType == "" || pointer.SchemaVersion != 1 ||
		pointer.AggregateType == "" || pointer.AggregateID == "" || pointer.CorrelationID == "" {
		return webhookDispatchPointer{}, errors.New("incomplete webhook dispatch pointer")
	}
	for _, rawID := range []string{pointer.OutboxEventID, pointer.AggregateID, pointer.CorrelationID} {
		if _, err := ids.Parse(rawID); err != nil {
			return webhookDispatchPointer{}, errors.New("invalid webhook dispatch pointer ID")
		}
	}
	return pointer, nil
}

type WebhookDelivery struct {
	WorkspaceID         ids.UUID
	ID                  ids.UUID
	Event               OutboxWebhookEvent
	Subscription        WebhookSubscription
	Secret              identity.SecretEnvelope
	PreviousSecret      *identity.SecretEnvelope
	PreviousSecretUntil *time.Time
	Attempts            int
	MaxAttempts         int
}

type WebhookDeliveryRepository interface {
	BeginDelivery(context.Context, worker.Job, string, time.Time) (WebhookDelivery, bool, error)
	CompleteDelivery(context.Context, WebhookDelivery, WebhookResponse) error
	FailDelivery(context.Context, WebhookDelivery, WebhookResponse, string, bool, time.Time) error
}

type WebhookClient interface {
	Post(context.Context, string, http.Header, []byte, time.Duration) (WebhookResponse, error)
}

func NewWebhookDeliveryHandler(
	repository WebhookDeliveryRepository,
	cipher identity.SecretCipher,
	client WebhookClient,
	now func() time.Time,
) worker.Handler {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return func(ctx context.Context, _ worker.Dependencies, job worker.Job) error {
		if repository == nil || cipher == nil || client == nil {
			return webhookFailure{code: "webhook_not_configured", err: errors.New("webhook delivery is not configured")}
		}
		var payload struct {
			DeliveryID string `json:"deliveryId"`
		}
		if err := decodeStrictJSON(job.Payload, &payload); err != nil || payload.DeliveryID == "" {
			return webhookFailure{code: "webhook_payload_invalid", err: errors.New("invalid webhook delivery payload")}
		}
		timestamp := now().UTC().Truncate(time.Second)
		delivery, claimed, err := repository.BeginDelivery(ctx, job, payload.DeliveryID, timestamp)
		if err != nil {
			return webhookFailure{code: "webhook_begin_failed", err: err}
		}
		if !claimed {
			return nil
		}
		secret, err := cipher.Decrypt(ctx, WebhookSecretPurpose, webhookSubject(delivery.WorkspaceID, delivery.Subscription.ID), delivery.Secret)
		if err != nil {
			_ = repository.FailDelivery(ctx, delivery, WebhookResponse{}, "webhook_secret_unavailable", true, timestamp)
			return webhookFailure{code: "webhook_secret_unavailable", err: err}
		}
		defer clear(secret)
		envelope := struct {
			ID            string          `json:"id"`
			Type          string          `json:"type"`
			SchemaVersion int             `json:"schemaVersion"`
			CreatedAt     time.Time       `json:"createdAt"`
			Data          json.RawMessage `json:"data"`
		}{
			ID: delivery.Event.ID.String(), Type: delivery.Event.EventType,
			SchemaVersion: delivery.Event.SchemaVersion, CreatedAt: delivery.Event.CreatedAt.UTC(),
			Data: delivery.Event.Payload,
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			return webhookFailure{code: "webhook_encode_failed", err: err}
		}
		// The body participates in the signature, so replace the placeholder
		// after encoding. During a bounded rotation overlap both signatures are
		// emitted; receivers may deploy the new secret without a delivery gap.
		signatures := SignWebhook(secret, timestamp, envelope.ID, body)
		if delivery.PreviousSecret != nil && delivery.PreviousSecretUntil != nil && timestamp.Before(*delivery.PreviousSecretUntil) {
			previous, decryptErr := cipher.Decrypt(ctx, WebhookSecretPurpose,
				webhookSubject(delivery.WorkspaceID, delivery.Subscription.ID), *delivery.PreviousSecret)
			if decryptErr != nil {
				return webhookFailure{code: "webhook_previous_secret_unavailable", err: decryptErr}
			}
			signatures += "," + SignWebhook(previous, timestamp, envelope.ID, body)
			clear(previous)
		}
		headers := make(http.Header)
		headers.Set("Webhook-Id", envelope.ID)
		headers.Set("Webhook-Timestamp", strconv.FormatInt(timestamp.Unix(), 10))
		headers.Set("Webhook-Signature", signatures)
		response, sendErr := client.Post(ctx, delivery.Subscription.URL, headers, body, delivery.Subscription.Timeout)
		if sendErr != nil {
			dead := delivery.Attempts >= delivery.MaxAttempts
			next := timestamp.Add(worker.Backoff(int32(delivery.Attempts), time.Second, 5*time.Minute))
			_ = repository.FailDelivery(ctx, delivery, response, "webhook_delivery_failed", dead, next)
			return webhookFailure{code: "webhook_delivery_failed", err: sendErr}
		}
		if err := repository.CompleteDelivery(ctx, delivery, response); err != nil {
			return webhookFailure{code: "webhook_complete_failed", err: err}
		}
		return nil
	}
}

type webhookFailure struct {
	code string
	err  error
}

func (failure webhookFailure) Error() string       { return failure.err.Error() }
func (failure webhookFailure) Unwrap() error       { return failure.err }
func (failure webhookFailure) FailureCode() string { return failure.code }

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON")
		}
		return err
	}
	return nil
}

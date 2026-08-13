package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func TestWebhookDispatchAcceptsCanonicalAutomationPointerWithoutSubscriptions(t *testing.T) {
	t.Parallel()

	workspaceID := mustWebhookWorkerID(t)
	eventID := mustWebhookWorkerID(t)
	repository := &webhookFanoutStub{}
	payload, err := json.Marshal(webhookDispatchPointer{
		OutboxEventID: eventID.String(), EventType: "automation.scheduled", SchemaVersion: 1,
		AggregateType: "workspace", AggregateID: workspaceID.String(), CorrelationID: eventID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	job := worker.Job{
		WorkspaceID: workspaceID, ID: mustWebhookWorkerID(t), Kind: "webhook.dispatch",
		SchemaVersion: 1, Payload: payload,
	}
	if err := NewWebhookDispatchHandler(repository)(context.Background(), worker.Dependencies{}, job); err != nil {
		t.Fatalf("dispatch canonical automation pointer: %v", err)
	}
	if repository.calls != 1 || repository.eventID != eventID.String() || repository.eventType != "automation.scheduled" {
		t.Fatalf("fanout call = (%d, %q, %q)", repository.calls, repository.eventID, repository.eventType)
	}
}

func TestWebhookDispatchRejectsIncompatiblePointer(t *testing.T) {
	t.Parallel()

	repository := &webhookFanoutStub{}
	job := worker.Job{
		WorkspaceID: mustWebhookWorkerID(t), ID: mustWebhookWorkerID(t), Kind: "webhook.dispatch",
		SchemaVersion: 1,
		Payload:       []byte(`{"outboxEventId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b4","eventType":"automation.scheduled","schemaVersion":1,"aggregateType":"workspace","aggregateId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b5","correlationId":"018f47d2-2044-7f25-89b0-85bd4c8ad8b4","unexpected":true}`),
	}
	err := NewWebhookDispatchHandler(repository)(context.Background(), worker.Dependencies{}, job)
	var coded interface{ FailureCode() string }
	if !errors.As(err, &coded) || coded.FailureCode() != "webhook_payload_invalid" {
		t.Fatalf("incompatible pointer error = %v", err)
	}
	if repository.calls != 0 {
		t.Fatal("invalid pointer reached webhook repository")
	}
}

func TestWebhookDeliveryHandlerSendsSignedEvent(t *testing.T) {
	t.Parallel()

	workspaceID := mustWebhookWorkerID(t)
	deliveryID := mustWebhookWorkerID(t)
	eventID := mustWebhookWorkerID(t)
	subscriptionID := mustWebhookWorkerID(t)
	now := time.Date(2026, time.July, 22, 12, 30, 0, 0, time.UTC)
	secret := []byte("unit-test-webhook-secret-32-bytes")
	repository := &webhookDeliveryRepositoryStub{delivery: WebhookDelivery{
		WorkspaceID: workspaceID,
		ID:          deliveryID,
		Event: OutboxWebhookEvent{
			ID: eventID, EventType: "customers.contact.created", SchemaVersion: 1,
			AggregateType: "contact", AggregateID: mustWebhookWorkerID(t),
			CorrelationID: eventID, Payload: json.RawMessage(`{"displayName":"Test Contact"}`), CreatedAt: now.Add(-time.Minute),
		},
		Subscription: WebhookSubscription{
			WorkspaceID: workspaceID, ID: subscriptionID, URL: "https://hooks.example.test/events",
			Timeout: 5 * time.Second, MaxAttempts: 8,
		},
		Secret:   identity.SecretEnvelope{Ciphertext: []byte("encrypted"), Nonce: []byte("nonce"), KeyID: "test"},
		Attempts: 1, MaxAttempts: 8,
	}}
	client := &webhookClientStub{response: WebhookResponse{StatusCode: http.StatusNoContent}}
	cipher := webhookCipherStub{secret: secret}
	payload, err := json.Marshal(map[string]string{"deliveryId": deliveryID.String()})
	if err != nil {
		t.Fatal(err)
	}
	job := worker.Job{
		WorkspaceID: workspaceID, ID: mustWebhookWorkerID(t), Kind: "webhook.deliver",
		SchemaVersion: 1, Payload: payload,
	}
	handler := NewWebhookDeliveryHandler(repository, cipher, client, func() time.Time { return now })
	if err := handler(context.Background(), worker.Dependencies{}, job); err != nil {
		t.Fatalf("deliver webhook: %v", err)
	}
	if client.calls != 1 || repository.completed != 1 || repository.failed != 0 {
		t.Fatalf("delivery calls client=%d completed=%d failed=%d", client.calls, repository.completed, repository.failed)
	}
	if client.rawURL != repository.delivery.Subscription.URL || client.timeout != repository.delivery.Subscription.Timeout {
		t.Fatalf("delivery target = %q timeout=%s", client.rawURL, client.timeout)
	}
	if err := VerifyWebhook(
		context.Background(), secret, now, time.Minute, eventID.String(),
		strconv.FormatInt(now.Unix(), 10), client.headers.Get("Webhook-Signature"), client.body, nil,
	); err != nil {
		t.Fatalf("verify emitted webhook signature: %v", err)
	}
	var envelope struct {
		ID   string          `json:"id"`
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(client.body, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.ID != eventID.String() || envelope.Type != "customers.contact.created" || len(envelope.Data) == 0 {
		t.Fatalf("unexpected webhook envelope: id=%q type=%q data=%s", envelope.ID, envelope.Type, envelope.Data)
	}
}

type webhookFanoutStub struct {
	calls     int
	eventID   string
	eventType string
}

func (repository *webhookFanoutStub) Fanout(_ context.Context, _ worker.Job, eventID, eventType string) error {
	repository.calls++
	repository.eventID = eventID
	repository.eventType = eventType
	return nil
}

type webhookDeliveryRepositoryStub struct {
	delivery  WebhookDelivery
	completed int
	failed    int
}

func (repository *webhookDeliveryRepositoryStub) BeginDelivery(
	context.Context, worker.Job, string, time.Time,
) (WebhookDelivery, bool, error) {
	return repository.delivery, true, nil
}

func (repository *webhookDeliveryRepositoryStub) CompleteDelivery(
	_ context.Context, _ WebhookDelivery, _ WebhookResponse,
) error {
	repository.completed++
	return nil
}

func (repository *webhookDeliveryRepositoryStub) FailDelivery(
	_ context.Context, _ WebhookDelivery, _ WebhookResponse, _ string, _ bool, _ time.Time,
) error {
	repository.failed++
	return nil
}

type webhookClientStub struct {
	response WebhookResponse
	calls    int
	rawURL   string
	headers  http.Header
	body     []byte
	timeout  time.Duration
}

func (client *webhookClientStub) Post(
	_ context.Context, rawURL string, headers http.Header, body []byte, timeout time.Duration,
) (WebhookResponse, error) {
	client.calls++
	client.rawURL = rawURL
	client.headers = headers.Clone()
	client.body = append([]byte(nil), body...)
	client.timeout = timeout
	return client.response, nil
}

type webhookCipherStub struct {
	secret []byte
}

func (cipher webhookCipherStub) Encrypt(
	context.Context, string, string, []byte,
) (identity.SecretEnvelope, error) {
	return identity.SecretEnvelope{}, errors.New("not implemented")
}

func (cipher webhookCipherStub) Decrypt(
	context.Context, string, string, identity.SecretEnvelope,
) ([]byte, error) {
	return append([]byte(nil), cipher.secret...), nil
}

func mustWebhookWorkerID(t *testing.T) ids.UUID {
	t.Helper()
	id, err := ids.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

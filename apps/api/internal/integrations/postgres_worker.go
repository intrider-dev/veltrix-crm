package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/worker"
)

func (repository *PostgresRepository) Fanout(
	ctx context.Context, job worker.Job, rawEventID string, _ string,
) error {
	eventID, err := ids.Parse(rawEventID)
	if err != nil {
		return err
	}
	return repository.withQueries(ctx, job.WorkspaceID, job.ID.String(), func(queries *dbgen.Queries) error {
		event, err := queries.GetOutboxEventForWebhook(ctx, dbgen.GetOutboxEventForWebhookParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: eventID.PG(),
		})
		if err != nil {
			return err
		}
		subscriptions, err := queries.ListWebhookSubscriptionsForEvent(ctx, dbgen.ListWebhookSubscriptionsForEventParams{
			WorkspaceID: job.WorkspaceID.PG(), EventType: event.EventType,
		})
		if err != nil {
			return err
		}
		for _, subscription := range subscriptions {
			deliveryID, err := ids.NewV7()
			if err != nil {
				return err
			}
			_, err = queries.CreateWebhookDelivery(ctx, dbgen.CreateWebhookDeliveryParams{
				WorkspaceID: job.WorkspaceID.PG(), ID: deliveryID.PG(),
				SubscriptionID: subscription.ID, EventID: event.ID,
			})
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			jobID, err := ids.NewV7()
			if err != nil {
				return err
			}
			payload, _ := json.Marshal(map[string]string{"deliveryId": deliveryID.String()})
			subscriptionID, _ := ids.FromPG(subscription.ID)
			if err := queries.EnqueueWebhookDelivery(ctx, dbgen.EnqueueWebhookDeliveryParams{
				WorkspaceID: job.WorkspaceID.PG(), ID: jobID.PG(),
				IdempotencyKey: subscriptionID.String() + ":" + eventID.String(),
				Payload:        payload, MaxAttempts: subscription.MaxAttempts,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository *PostgresRepository) BeginDelivery(
	ctx context.Context, job worker.Job, rawDeliveryID string, timestamp time.Time,
) (WebhookDelivery, bool, error) {
	deliveryID, err := ids.Parse(rawDeliveryID)
	if err != nil {
		return WebhookDelivery{}, false, err
	}
	var delivery WebhookDelivery
	claimed := false
	err = repository.withQueries(ctx, job.WorkspaceID, job.ID.String(), func(queries *dbgen.Queries) error {
		seconds := timestamp.Unix()
		rows, err := queries.StartWebhookDelivery(ctx, dbgen.StartWebhookDeliveryParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: deliveryID.PG(), RequestTimestamp: &seconds,
		})
		if err != nil || rows == 0 {
			return err
		}
		deliveryRow, err := queries.GetWebhookDelivery(ctx, dbgen.GetWebhookDeliveryParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: deliveryID.PG(),
		})
		if err != nil {
			return err
		}
		subscriptionRow, err := queries.GetWebhookSubscriptionForDelivery(ctx, dbgen.GetWebhookSubscriptionForDeliveryParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: deliveryRow.SubscriptionID,
		})
		if err != nil {
			return err
		}
		if !subscriptionRow.Enabled {
			code := "webhook_disabled"
			_, err := queries.FailWebhookDelivery(ctx, dbgen.FailWebhookDeliveryParams{
				WorkspaceID: job.WorkspaceID.PG(), ID: deliveryID.PG(), Dead: true,
				NextAttemptAt: pgTimestamp(timestamp), ErrorCode: &code,
			})
			return err
		}
		eventRow, err := queries.GetOutboxEventForWebhook(ctx, dbgen.GetOutboxEventForWebhookParams{
			WorkspaceID: job.WorkspaceID.PG(), ID: deliveryRow.EventID,
		})
		if err != nil {
			return err
		}
		subscription := webhookFields(
			subscriptionRow.WorkspaceID, subscriptionRow.ID, subscriptionRow.Url, subscriptionRow.EventTypes,
			subscriptionRow.Enabled, subscriptionRow.Version, subscriptionRow.SecretVersion,
			subscriptionRow.TimeoutSeconds, subscriptionRow.MaxAttempts, subscriptionRow.CreatedBy,
			subscriptionRow.CreatedAt, subscriptionRow.UpdatedAt,
		)
		eventID, _ := ids.FromPG(eventRow.ID)
		aggregateID, _ := ids.FromPG(eventRow.AggregateID)
		correlationID, _ := ids.FromPG(eventRow.CorrelationID)
		delivery = WebhookDelivery{
			WorkspaceID: job.WorkspaceID, ID: deliveryID,
			Event: OutboxWebhookEvent{
				ID: eventID, EventType: eventRow.EventType, SchemaVersion: int(eventRow.SchemaVersion),
				AggregateType: eventRow.AggregateType, AggregateID: aggregateID,
				CorrelationID: correlationID, Payload: eventRow.Payload, CreatedAt: eventRow.CreatedAt.Time.UTC(),
			},
			Subscription: subscription,
			Secret: identity.SecretEnvelope{
				Ciphertext: append([]byte(nil), subscriptionRow.SecretCiphertext...),
				Nonce:      append([]byte(nil), subscriptionRow.SecretNonce...), KeyID: subscriptionRow.KeyID,
			},
			Attempts: int(deliveryRow.Attempts), MaxAttempts: int(subscriptionRow.MaxAttempts),
		}
		if len(subscriptionRow.PreviousSecretCiphertext) > 0 && len(subscriptionRow.PreviousSecretNonce) > 0 &&
			subscriptionRow.PreviousKeyID != nil && subscriptionRow.PreviousSecretExpiresAt.Valid {
			delivery.PreviousSecret = &identity.SecretEnvelope{
				Ciphertext: append([]byte(nil), subscriptionRow.PreviousSecretCiphertext...),
				Nonce:      append([]byte(nil), subscriptionRow.PreviousSecretNonce...), KeyID: *subscriptionRow.PreviousKeyID,
			}
			until := subscriptionRow.PreviousSecretExpiresAt.Time.UTC()
			delivery.PreviousSecretUntil = &until
		}
		claimed = true
		return nil
	})
	return delivery, claimed, err
}

func (repository *PostgresRepository) CompleteDelivery(
	ctx context.Context, delivery WebhookDelivery, response WebhookResponse,
) error {
	status := int32(response.StatusCode)
	summary := response.Summary
	return repository.withQueries(ctx, delivery.WorkspaceID, delivery.ID.String(), func(queries *dbgen.Queries) error {
		rows, err := queries.CompleteWebhookDelivery(ctx, dbgen.CompleteWebhookDeliveryParams{
			WorkspaceID: delivery.WorkspaceID.PG(), ID: delivery.ID.PG(),
			ResponseStatus: &status, ResponseSummary: &summary,
		})
		if err == nil && rows != 1 {
			return errors.New("webhook delivery completion lost state")
		}
		return err
	})
}

func (repository *PostgresRepository) FailDelivery(
	ctx context.Context,
	delivery WebhookDelivery,
	response WebhookResponse,
	code string,
	dead bool,
	next time.Time,
) error {
	var status *int32
	if response.StatusCode > 0 {
		value := int32(response.StatusCode)
		status = &value
	}
	var summary *string
	if response.Summary != "" {
		value := response.Summary
		summary = &value
	}
	return repository.withQueries(ctx, delivery.WorkspaceID, delivery.ID.String(), func(queries *dbgen.Queries) error {
		_, err := queries.FailWebhookDelivery(ctx, dbgen.FailWebhookDeliveryParams{
			WorkspaceID: delivery.WorkspaceID.PG(), ID: delivery.ID.PG(),
			ResponseStatus: status, ResponseSummary: summary, Dead: dead,
			NextAttemptAt: pgTimestamp(next), ErrorCode: &code,
		})
		return err
	})
}

func pgTimestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

var _ WebhookFanoutRepository = (*PostgresRepository)(nil)
var _ WebhookDeliveryRepository = (*PostgresRepository)(nil)

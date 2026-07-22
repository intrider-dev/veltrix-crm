package integrations

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type Manager struct {
	cipher    identity.SecretCipher
	validator URLValidator
}

func NewManager(cipher identity.SecretCipher, validator URLValidator) *Manager {
	return &Manager{cipher: cipher, validator: validator}
}

func (manager *Manager) CreateAPIKey(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	name string, scopes []Scope, expiresAt *time.Time,
) (GeneratedAPIKey, error) {
	service := NewAPIKeyService(NewWorkspacePostgresRepository(workspace))
	generated, err := service.Create(ctx, APIKeyCreate{
		WorkspaceID: metadata.WorkspaceID, CreatedBy: metadata.ActorID,
		Name: name, Scopes: scopes, ExpiresAt: expiresAt,
	})
	if err != nil {
		return GeneratedAPIKey{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "api_key.created", EventType: "integrations.api_key.created",
		AggregateType: "api_key", AggregateID: generated.APIKey.ID,
		Summary: map[string]any{"name": generated.APIKey.Name, "scopes": generated.APIKey.Scopes},
		Payload: map[string]any{"apiKeyId": generated.APIKey.ID.String()},
	}); err != nil {
		return GeneratedAPIKey{}, err
	}
	return generated, nil
}

func (manager *Manager) ListAPIKeys(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, limit int,
) ([]APIKey, error) {
	return NewAPIKeyService(NewWorkspacePostgresRepository(workspace)).List(ctx, workspaceID, limit)
}

func (manager *Manager) RevokeAPIKey(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, keyID ids.UUID,
) error {
	if err := NewAPIKeyService(NewWorkspacePostgresRepository(workspace)).Revoke(ctx, metadata.WorkspaceID, keyID); err != nil {
		return err
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "api_key.revoked", EventType: "integrations.api_key.revoked",
		AggregateType: "api_key", AggregateID: keyID,
		Summary: map[string]any{"revoked": true}, Payload: map[string]any{"apiKeyId": keyID.String()},
	})
}

func (manager *Manager) CreateWebhook(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, input WebhookCreate,
) (GeneratedWebhook, error) {
	input.WorkspaceID, input.CreatedBy = metadata.WorkspaceID, metadata.ActorID
	generated, err := NewWebhookService(
		NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator,
	).Create(ctx, input)
	if err != nil {
		return GeneratedWebhook{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "webhook.created", EventType: "integrations.webhook.created",
		AggregateType: "webhook", AggregateID: generated.Subscription.ID,
		Summary: map[string]any{"eventTypes": generated.Subscription.EventTypes, "enabled": generated.Subscription.Enabled},
		Payload: map[string]any{"webhookId": generated.Subscription.ID.String()},
	}); err != nil {
		return GeneratedWebhook{}, err
	}
	return generated, nil
}

func (manager *Manager) ListWebhooks(
	ctx context.Context, workspace *tenancy.WorkspaceTx, workspaceID ids.UUID, limit int,
) ([]WebhookSubscription, error) {
	return NewWebhookService(NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator).
		List(ctx, workspaceID, limit)
}

func (manager *Manager) ListWebhookDeliveries(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	workspaceID ids.UUID,
	subscriptionID *ids.UUID,
	cursor string,
	limit int,
) (WebhookDeliveryPage, error) {
	return NewWebhookService(NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator).
		ListDeliveries(ctx, workspaceID, subscriptionID, cursor, limit)
}

func (manager *Manager) RotateWebhook(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	subscriptionID ids.UUID, version int64, overlap time.Duration,
) (GeneratedWebhook, error) {
	generated, err := NewWebhookService(
		NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator,
	).RotateSecret(ctx, metadata.WorkspaceID, subscriptionID, version, overlap)
	if errors.Is(err, pgx.ErrNoRows) {
		return GeneratedWebhook{}, errx.ErrVersionConflict
	}
	if err != nil {
		return GeneratedWebhook{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "webhook.secret_rotated", EventType: "integrations.webhook.secret_rotated",
		AggregateType: "webhook", AggregateID: subscriptionID,
		Summary: map[string]any{"secretVersion": generated.Subscription.SecretVersion},
		Payload: map[string]any{"webhookId": subscriptionID.String()},
	}); err != nil {
		return GeneratedWebhook{}, err
	}
	return generated, nil
}

func (manager *Manager) SetWebhookEnabled(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata,
	subscriptionID ids.UUID, version int64, enabled bool,
) (WebhookSubscription, error) {
	updated, err := NewWebhookService(
		NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator,
	).SetEnabled(ctx, metadata.WorkspaceID, subscriptionID, version, enabled)
	if err != nil {
		return WebhookSubscription{}, err
	}
	if err := events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "webhook.enabled_changed", EventType: "integrations.webhook.enabled_changed",
		AggregateType: "webhook", AggregateID: subscriptionID,
		Summary: map[string]any{"enabled": enabled}, Payload: map[string]any{"webhookId": subscriptionID.String()},
	}); err != nil {
		return WebhookSubscription{}, err
	}
	return updated, nil
}

func (manager *Manager) RetryWebhookDelivery(
	ctx context.Context, workspace *tenancy.WorkspaceTx, metadata events.Metadata, deliveryID ids.UUID,
) error {
	if err := NewWebhookService(
		NewWorkspacePostgresRepository(workspace), manager.cipher, manager.validator,
	).RetryDelivery(ctx, metadata.WorkspaceID, deliveryID); err != nil {
		return err
	}
	return events.Record(ctx, workspace.Queries, metadata, events.Mutation{
		Action: "webhook.delivery_retried", EventType: "integrations.webhook.delivery_retried",
		AggregateType: "webhook_delivery", AggregateID: deliveryID,
		Summary: map[string]any{"manualRetry": true}, Payload: map[string]any{"deliveryId": deliveryID.String()},
	})
}

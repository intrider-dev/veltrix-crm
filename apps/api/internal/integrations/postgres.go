package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// PostgresRepository is transaction-bound for user management operations and
// pool-backed for API-key authentication and background jobs. In both modes,
// callers get the same sqlc queries and RLS remains active.
type PostgresRepository struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func NewWorkspacePostgresRepository(workspace *tenancy.WorkspaceTx) *PostgresRepository {
	return &PostgresRepository{queries: workspace.Queries}
}

func (repository *PostgresRepository) withQueries(
	ctx context.Context,
	workspaceID ids.UUID,
	requestID string,
	fn func(*dbgen.Queries) error,
) error {
	if repository.queries != nil {
		return fn(repository.queries)
	}
	if repository.pool == nil {
		return errors.New("integrations database pool is required")
	}
	tx, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	if err := queries.SetTenantContext(ctx, dbgen.SetTenantContextParams{
		WorkspaceID: workspaceID.String(), RequestID: requestID,
	}); err != nil {
		return err
	}
	if err := fn(queries); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *PostgresRepository) CreateAPIKey(
	ctx context.Context, key APIKey, hash [32]byte,
) (APIKey, error) {
	var result APIKey
	err := repository.withQueries(ctx, key.WorkspaceID, key.ID.String(), func(queries *dbgen.Queries) error {
		row, err := queries.CreateAPIKey(ctx, dbgen.CreateAPIKeyParams{
			WorkspaceID: key.WorkspaceID.PG(), ID: key.ID.PG(), KeyPrefix: key.Prefix,
			SecretHash: hash[:], Name: key.Name, Scopes: scopesToStrings(key.Scopes),
			CreatedBy: key.CreatedBy.PG(), ExpiresAt: optionalTime(key.ExpiresAt),
		})
		if err != nil {
			return err
		}
		result = apiKeyFromCreate(row)
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) ListAPIKeys(ctx context.Context, workspaceID ids.UUID, limit int) ([]APIKey, error) {
	result := []APIKey{}
	err := repository.withQueries(ctx, workspaceID, "api-key-list", func(queries *dbgen.Queries) error {
		rows, err := queries.ListAPIKeys(ctx, dbgen.ListAPIKeysParams{WorkspaceID: workspaceID.PG(), Limit: int32(limit)})
		if err != nil {
			return err
		}
		result = make([]APIKey, 0, len(rows))
		for _, row := range rows {
			result = append(result, apiKeyFromList(row))
		}
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) LookupAPIKey(
	ctx context.Context, workspaceID ids.UUID, prefix string,
) (APIKeyCredential, bool, error) {
	var credential APIKeyCredential
	found := false
	err := repository.withQueries(ctx, workspaceID, "api-key-auth", func(queries *dbgen.Queries) error {
		row, err := queries.GetAPIKeyForAuthentication(ctx, dbgen.GetAPIKeyForAuthenticationParams{
			WorkspaceID: workspaceID.PG(), KeyPrefix: prefix,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		id, idOK := ids.FromPG(row.ID)
		workspace, workspaceOK := ids.FromPG(row.WorkspaceID)
		createdBy, creatorOK := ids.FromPG(row.CreatedBy)
		if !idOK || !workspaceOK || !creatorOK || len(row.SecretHash) != 32 {
			return errors.New("API key row is invalid")
		}
		var hash [32]byte
		copy(hash[:], row.SecretHash)
		credential = APIKeyCredential{
			WorkspaceID: workspace, ID: id, Prefix: row.KeyPrefix, SecretHash: hash,
			Scopes: scopesFromStrings(row.Scopes), CreatedBy: createdBy,
			ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt),
		}
		found = true
		return nil
	})
	return credential, found, err
}

func (repository *PostgresRepository) TouchAPIKey(ctx context.Context, workspaceID, keyID ids.UUID) error {
	return repository.withQueries(ctx, workspaceID, keyID.String(), func(queries *dbgen.Queries) error {
		return queries.TouchAPIKeyLastUsed(ctx, dbgen.TouchAPIKeyLastUsedParams{WorkspaceID: workspaceID.PG(), ID: keyID.PG()})
	})
}

func (repository *PostgresRepository) RevokeAPIKey(ctx context.Context, workspaceID, keyID ids.UUID) (bool, error) {
	revoked := false
	err := repository.withQueries(ctx, workspaceID, keyID.String(), func(queries *dbgen.Queries) error {
		rows, err := queries.RevokeAPIKey(ctx, dbgen.RevokeAPIKeyParams{WorkspaceID: workspaceID.PG(), ID: keyID.PG()})
		revoked = rows == 1
		return err
	})
	return revoked, err
}

func (repository *PostgresRepository) CreateWebhook(
	ctx context.Context, subscription WebhookSubscription, envelope identity.SecretEnvelope,
) (WebhookSubscription, error) {
	var result WebhookSubscription
	err := repository.withQueries(ctx, subscription.WorkspaceID, subscription.ID.String(), func(queries *dbgen.Queries) error {
		row, err := queries.CreateWebhookSubscription(ctx, dbgen.CreateWebhookSubscriptionParams{
			WorkspaceID: subscription.WorkspaceID.PG(), ID: subscription.ID.PG(), Url: subscription.URL,
			EventTypes: subscription.EventTypes, SecretCiphertext: envelope.Ciphertext,
			SecretNonce: envelope.Nonce, KeyID: envelope.KeyID, Enabled: subscription.Enabled,
			TimeoutSeconds: int32(subscription.Timeout / time.Second), MaxAttempts: int32(subscription.MaxAttempts),
			CreatedBy: subscription.CreatedBy.PG(),
		})
		if err != nil {
			return err
		}
		result = webhookFromCreate(row)
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) ListWebhooks(ctx context.Context, workspaceID ids.UUID, limit int) ([]WebhookSubscription, error) {
	result := []WebhookSubscription{}
	err := repository.withQueries(ctx, workspaceID, "webhook-list", func(queries *dbgen.Queries) error {
		rows, err := queries.ListWebhookSubscriptions(ctx, dbgen.ListWebhookSubscriptionsParams{
			WorkspaceID: workspaceID.PG(), Limit: int32(limit),
		})
		if err != nil {
			return err
		}
		result = make([]WebhookSubscription, 0, len(rows))
		for _, row := range rows {
			result = append(result, webhookFromList(row))
		}
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) ListWebhookDeliveries(
	ctx context.Context,
	workspaceID ids.UUID,
	subscriptionID *ids.UUID,
	cursorTime time.Time,
	cursorID ids.UUID,
	limit int,
) ([]WebhookDeliveryLog, error) {
	result := []WebhookDeliveryLog{}
	err := repository.withQueries(ctx, workspaceID, "webhook-delivery-list", func(queries *dbgen.Queries) error {
		rows, err := queries.ListWebhookDeliveries(ctx, dbgen.ListWebhookDeliveriesParams{
			WorkspaceID: workspaceID.PG(), SubscriptionID: optionalID(subscriptionID),
			CursorCreatedAt: pgTimestamp(cursorTime), CursorID: cursorID.PG(), PageLimit: int32(limit),
		})
		if err != nil {
			return err
		}
		result = make([]WebhookDeliveryLog, 0, len(rows))
		for _, row := range rows {
			result = append(result, webhookDeliveryLogFromRow(row))
		}
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) RotateWebhookSecret(
	ctx context.Context, workspaceID, subscriptionID ids.UUID, version int64,
	envelope identity.SecretEnvelope, overlap time.Duration,
) (WebhookSubscription, error) {
	var result WebhookSubscription
	err := repository.withQueries(ctx, workspaceID, subscriptionID.String(), func(queries *dbgen.Queries) error {
		row, err := queries.RotateWebhookSecret(ctx, dbgen.RotateWebhookSecretParams{
			OverlapSeconds: int64(overlap / time.Second), SecretCiphertext: envelope.Ciphertext,
			SecretNonce: envelope.Nonce, KeyID: envelope.KeyID, WorkspaceID: workspaceID.PG(),
			ID: subscriptionID.PG(), Version: version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("rotate webhook secret: %w", err)
		}
		if err != nil {
			return err
		}
		result = webhookFromRotate(row)
		return nil
	})
	return result, err
}

func (repository *PostgresRepository) SetWebhookEnabled(
	ctx context.Context, workspaceID, subscriptionID ids.UUID, version int64, enabled bool,
) (WebhookSubscription, bool, error) {
	var result WebhookSubscription
	found := false
	err := repository.withQueries(ctx, workspaceID, subscriptionID.String(), func(queries *dbgen.Queries) error {
		row, err := queries.SetWebhookSubscriptionEnabled(ctx, dbgen.SetWebhookSubscriptionEnabledParams{
			WorkspaceID: workspaceID.PG(), ID: subscriptionID.PG(), Enabled: enabled, Version: version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found, result = true, webhookFromEnabled(row)
		return nil
	})
	return result, found, err
}

func (repository *PostgresRepository) RetryWebhookDelivery(ctx context.Context, workspaceID, deliveryID ids.UUID) (bool, error) {
	retried := false
	err := repository.withQueries(ctx, workspaceID, deliveryID.String(), func(queries *dbgen.Queries) error {
		row, err := queries.RetryWebhookDelivery(ctx, dbgen.RetryWebhookDeliveryParams{WorkspaceID: workspaceID.PG(), ID: deliveryID.PG()})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		jobID, err := ids.NewV7()
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]string{"deliveryId": deliveryID.String()})
		if err := queries.EnqueueWebhookDelivery(ctx, dbgen.EnqueueWebhookDeliveryParams{
			WorkspaceID: workspaceID.PG(), ID: jobID.PG(),
			IdempotencyKey: deliveryID.String() + ":manual:" + jobID.String(),
			Payload:        payload, MaxAttempts: int32(max(1, int(row.Attempts)+8)),
		}); err != nil {
			return err
		}
		retried = true
		return nil
	})
	return retried, err
}

func apiKeyFromCreate(row dbgen.CreateAPIKeyRow) APIKey {
	workspaceID, _ := ids.FromPG(row.WorkspaceID)
	id, _ := ids.FromPG(row.ID)
	createdBy, _ := ids.FromPG(row.CreatedBy)
	return APIKey{
		WorkspaceID: workspaceID, ID: id, Prefix: row.KeyPrefix, Name: row.Name,
		Scopes: scopesFromStrings(row.Scopes), CreatedBy: createdBy, CreatedAt: row.CreatedAt.Time.UTC(),
		LastUsedAt: timePointer(row.LastUsedAt), ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt),
	}
}

func apiKeyFromList(row dbgen.ListAPIKeysRow) APIKey {
	workspaceID, _ := ids.FromPG(row.WorkspaceID)
	id, _ := ids.FromPG(row.ID)
	createdBy, _ := ids.FromPG(row.CreatedBy)
	return APIKey{
		WorkspaceID: workspaceID, ID: id, Prefix: row.KeyPrefix, Name: row.Name,
		Scopes: scopesFromStrings(row.Scopes), CreatedBy: createdBy, CreatedAt: row.CreatedAt.Time.UTC(),
		LastUsedAt: timePointer(row.LastUsedAt), ExpiresAt: timePointer(row.ExpiresAt), RevokedAt: timePointer(row.RevokedAt),
	}
}

func webhookFromCreate(row dbgen.CreateWebhookSubscriptionRow) WebhookSubscription {
	return webhookFields(row.WorkspaceID, row.ID, row.Url, row.EventTypes, row.Enabled, row.Version,
		row.SecretVersion, row.TimeoutSeconds, row.MaxAttempts, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
}

func webhookFromList(row dbgen.ListWebhookSubscriptionsRow) WebhookSubscription {
	return webhookFields(row.WorkspaceID, row.ID, row.Url, row.EventTypes, row.Enabled, row.Version,
		row.SecretVersion, row.TimeoutSeconds, row.MaxAttempts, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
}

func webhookFromRotate(row dbgen.RotateWebhookSecretRow) WebhookSubscription {
	return webhookFields(row.WorkspaceID, row.ID, row.Url, row.EventTypes, row.Enabled, row.Version,
		row.SecretVersion, row.TimeoutSeconds, row.MaxAttempts, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
}

func webhookFromEnabled(row dbgen.SetWebhookSubscriptionEnabledRow) WebhookSubscription {
	return webhookFields(row.WorkspaceID, row.ID, row.Url, row.EventTypes, row.Enabled, row.Version,
		row.SecretVersion, row.TimeoutSeconds, row.MaxAttempts, row.CreatedBy, row.CreatedAt, row.UpdatedAt)
}

func webhookFields(
	workspaceRaw, idRaw pgtype.UUID, url string, eventTypes []string, enabled bool, version int64,
	secretVersion, timeoutSeconds, maxAttempts int32, createdByRaw pgtype.UUID,
	createdAt, updatedAt pgtype.Timestamptz,
) WebhookSubscription {
	workspaceID, _ := ids.FromPG(workspaceRaw)
	id, _ := ids.FromPG(idRaw)
	createdBy, _ := ids.FromPG(createdByRaw)
	return WebhookSubscription{
		WorkspaceID: workspaceID, ID: id, URL: url, EventTypes: append([]string(nil), eventTypes...),
		Enabled: enabled, Version: version, SecretVersion: int(secretVersion),
		Timeout: time.Duration(timeoutSeconds) * time.Second, MaxAttempts: int(maxAttempts),
		CreatedBy: createdBy, CreatedAt: createdAt.Time.UTC(), UpdatedAt: updatedAt.Time.UTC(),
	}
}

func scopesToStrings(scopes []Scope) []string {
	result := make([]string, len(scopes))
	for index, scope := range scopes {
		result[index] = string(scope)
	}
	return result
}

func scopesFromStrings(scopes []string) []Scope {
	result := make([]Scope, 0, len(scopes))
	for _, scope := range scopes {
		result = append(result, Scope(scope))
	}
	return result
}

func optionalTime(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func timePointer(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	instant := value.Time.UTC()
	return &instant
}

func optionalID(value *ids.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return value.PG()
}

func webhookDeliveryLogFromRow(row dbgen.ListWebhookDeliveriesRow) WebhookDeliveryLog {
	id, _ := ids.FromPG(row.ID)
	subscriptionID, _ := ids.FromPG(row.SubscriptionID)
	eventID, _ := ids.FromPG(row.EventID)
	return WebhookDeliveryLog{
		ID: id, SubscriptionID: subscriptionID, EventID: eventID,
		Status: row.Status, Attempts: int(row.Attempts), NextAttemptAt: timePointer(row.NextAttemptAt),
		ResponseStatus: row.ResponseStatus, ResponseSummary: row.ResponseSummary,
		DeliveredAt: timePointer(row.DeliveredAt), RequestTimestamp: row.RequestTimestamp,
		SignatureVersion: row.SignatureVersion, LastErrorCode: row.LastErrorCode,
		CreatedAt: row.CreatedAt.Time.UTC(), UpdatedAt: row.UpdatedAt.Time.UTC(),
	}
}

var _ APIKeyRepository = (*PostgresRepository)(nil)
var _ WebhookRepository = (*PostgresRepository)(nil)

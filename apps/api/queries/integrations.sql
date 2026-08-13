-- name: CreateAPIKey :one
INSERT INTO integrations.api_keys (
  workspace_id, id, key_prefix, secret_hash, name, scopes, created_by, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING workspace_id, id, key_prefix, name, scopes, created_by, last_used_at,
          expires_at, revoked_at, created_at;

-- name: ListAPIKeys :many
SELECT workspace_id, id, key_prefix, name, scopes, created_by, last_used_at,
       expires_at, revoked_at, created_at
FROM integrations.api_keys
WHERE workspace_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2;

-- name: GetAPIKeyForAuthentication :one
SELECT workspace_id, id, key_prefix, secret_hash, name, scopes, created_by,
       expires_at, revoked_at
FROM integrations.api_keys
WHERE workspace_id = $1 AND key_prefix = $2;

-- name: TouchAPIKeyLastUsed :exec
UPDATE integrations.api_keys
SET last_used_at = CASE
  WHEN last_used_at IS NULL OR last_used_at < now() - interval '5 minutes' THEN now()
  ELSE last_used_at
END
WHERE workspace_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: RevokeAPIKey :execrows
UPDATE integrations.api_keys
SET revoked_at = COALESCE(revoked_at, now())
WHERE workspace_id = $1 AND id = $2 AND revoked_at IS NULL;

-- name: CreateWebhookSubscription :one
INSERT INTO integrations.webhook_subscriptions (
  workspace_id, id, url, event_types, secret_ciphertext, secret_nonce, key_id,
  enabled, timeout_seconds, max_attempts, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING workspace_id, id, url, event_types, enabled, version,
          secret_version, timeout_seconds, max_attempts, created_by,
          created_at, updated_at;

-- name: ListWebhookSubscriptions :many
SELECT workspace_id, id, url, event_types, enabled, version, secret_version,
       timeout_seconds, max_attempts, created_by, created_at, updated_at
FROM integrations.webhook_subscriptions
WHERE workspace_id = $1
ORDER BY updated_at DESC, id DESC
LIMIT $2;

-- name: GetWebhookSubscriptionForDelivery :one
SELECT workspace_id, id, url, event_types, secret_ciphertext, secret_nonce,
       key_id, previous_secret_ciphertext, previous_secret_nonce,
       previous_key_id, previous_secret_expires_at, enabled, version,
       secret_version, timeout_seconds, max_attempts, created_by,
       created_at, updated_at
FROM integrations.webhook_subscriptions
WHERE workspace_id = $1 AND id = $2;

-- name: SetWebhookSubscriptionEnabled :one
UPDATE integrations.webhook_subscriptions
SET enabled = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $4
RETURNING workspace_id, id, url, event_types, enabled, version,
          secret_version, timeout_seconds, max_attempts, created_by,
          created_at, updated_at;

-- name: RotateWebhookSecret :one
UPDATE integrations.webhook_subscriptions
SET previous_secret_ciphertext = secret_ciphertext,
    previous_secret_nonce = secret_nonce,
    previous_key_id = key_id,
    previous_secret_expires_at = now() + (sqlc.arg(overlap_seconds)::bigint * interval '1 second'),
    secret_ciphertext = sqlc.arg(secret_ciphertext),
    secret_nonce = sqlc.arg(secret_nonce),
    key_id = sqlc.arg(key_id),
    secret_version = secret_version + 1,
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
RETURNING workspace_id, id, url, event_types, enabled, version,
          secret_version, timeout_seconds, max_attempts, created_by,
          created_at, updated_at;

-- name: ListWebhookSubscriptionsForEvent :many
SELECT workspace_id, id, url, event_types, secret_ciphertext, secret_nonce,
       key_id, enabled, secret_version, timeout_seconds, max_attempts
FROM integrations.webhook_subscriptions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND enabled
  AND sqlc.arg(event_type)::text = ANY(event_types)
ORDER BY updated_at, id
LIMIT 500;

-- name: CreateWebhookDelivery :one
INSERT INTO integrations.webhook_deliveries (
  workspace_id, id, subscription_id, event_id, status
) VALUES ($1, $2, $3, $4, 'queued')
ON CONFLICT (workspace_id, subscription_id, event_id) DO NOTHING
RETURNING workspace_id, id, subscription_id, event_id, status, attempts,
          next_attempt_at, response_status, response_summary, delivered_at,
          request_timestamp, signature_version, last_error_code,
          created_at, updated_at;

-- name: EnqueueWebhookDelivery :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  state, attempts, max_attempts, available_at
) VALUES ($1, $2, 'webhook.deliver', 1, $3, $4, 'ready', 0, $5, now())
ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING;

-- name: GetWebhookDelivery :one
SELECT workspace_id, id, subscription_id, event_id, status, attempts,
       next_attempt_at, response_status, response_summary, delivered_at,
       request_timestamp, signature_version, last_error_code,
       created_at, updated_at
FROM integrations.webhook_deliveries
WHERE workspace_id = $1 AND id = $2;

-- name: ListWebhookDeliveries :many
SELECT workspace_id, id, subscription_id, event_id, status, attempts,
       next_attempt_at, response_status, response_summary, delivered_at,
       request_timestamp, signature_version, last_error_code,
       created_at, updated_at
FROM integrations.webhook_deliveries
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (sqlc.narg(subscription_id)::uuid IS NULL
       OR subscription_id = sqlc.narg(subscription_id))
  AND (created_at, id) <
      (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: StartWebhookDelivery :execrows
UPDATE integrations.webhook_deliveries
SET status = 'delivering', attempts = attempts + 1,
    request_timestamp = $3, response_status = NULL, response_summary = NULL,
    last_error_code = NULL, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND status IN ('queued', 'failed');

-- name: CompleteWebhookDelivery :execrows
UPDATE integrations.webhook_deliveries
SET status = 'succeeded', response_status = $3, response_summary = $4,
    delivered_at = now(), updated_at = now(), last_error_code = NULL
WHERE workspace_id = $1 AND id = $2 AND status = 'delivering';

-- name: FailWebhookDelivery :execrows
UPDATE integrations.webhook_deliveries
SET status = CASE WHEN sqlc.arg(dead)::boolean THEN 'dead' ELSE 'failed' END,
    response_status = sqlc.arg(response_status),
    response_summary = sqlc.arg(response_summary),
    next_attempt_at = sqlc.arg(next_attempt_at),
    last_error_code = sqlc.arg(error_code), updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND status = 'delivering';

-- name: RetryWebhookDelivery :one
UPDATE integrations.webhook_deliveries
SET status = 'queued', attempts = 0, next_attempt_at = now(),
    response_status = NULL, response_summary = NULL, delivered_at = NULL,
    last_error_code = NULL, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND status IN ('failed', 'dead')
RETURNING workspace_id, id, subscription_id, event_id, status, attempts,
          next_attempt_at, response_status, response_summary, delivered_at,
          request_timestamp, signature_version, last_error_code,
          created_at, updated_at;

-- name: GetOutboxEventForWebhook :one
SELECT id, event_type, schema_version, aggregate_type, aggregate_id,
       correlation_id, payload, created_at
FROM platform.outbox_events
WHERE workspace_id = $1 AND id = $2;

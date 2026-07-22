SET ROLE veltrix_owner;

DROP INDEX IF EXISTS integrations.webhook_deliveries_retry_idx;
DROP INDEX IF EXISTS integrations.webhook_subscriptions_dispatch_idx;
DROP INDEX IF EXISTS integrations.api_keys_prefix_idx;

ALTER TABLE integrations.webhook_deliveries
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS last_error_code,
  DROP COLUMN IF EXISTS signature_version,
  DROP COLUMN IF EXISTS request_timestamp;

ALTER TABLE integrations.webhook_subscriptions
  DROP CONSTRAINT IF EXISTS webhook_previous_secret_complete,
  DROP COLUMN IF EXISTS max_attempts,
  DROP COLUMN IF EXISTS timeout_seconds,
  DROP COLUMN IF EXISTS secret_version,
  DROP COLUMN IF EXISTS previous_key_id,
  DROP COLUMN IF EXISTS previous_secret_nonce;

DROP TABLE IF EXISTS automation.action_executions;
DROP TABLE IF EXISTS automation.hourly_usage;
DROP INDEX IF EXISTS automation.automation_executions_state_idx;
DROP INDEX IF EXISTS automation.automation_rules_dispatch_idx;

ALTER TABLE automation.executions
  DROP COLUMN IF EXISTS last_error_code,
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS event_payload,
  DROP COLUMN IF EXISTS trigger_type;

ALTER TABLE automation.rules
  DROP COLUMN IF EXISTS entity_type;

ALTER TABLE activities.activities
  DROP CONSTRAINT IF EXISTS activity_system_title_complete,
  DROP COLUMN IF EXISTS system_title_params,
  DROP COLUMN IF EXISTS system_title_key;

RESET ROLE;

SET ROLE veltrix_owner;

-- Automation executions retain only bounded, non-secret event material. The
-- unique rule/event key in the core schema is the execution idempotency fence.
ALTER TABLE automation.executions
  ADD COLUMN trigger_type text NOT NULL DEFAULT 'record_updated'
    CHECK (trigger_type IN (
      'record_created', 'record_updated', 'deal_stage_changed', 'deal_won',
      'deal_lost', 'task_overdue', 'scheduled'
    )),
  ADD COLUMN event_payload jsonb NOT NULL DEFAULT '{}'::jsonb
    CHECK (jsonb_typeof(event_payload) = 'object' AND octet_length(event_payload::text) <= 65536),
  ADD COLUMN started_at timestamptz,
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now(),
  ADD COLUMN last_error_code text
    CHECK (last_error_code IS NULL OR char_length(last_error_code) BETWEEN 1 AND 120);

ALTER TABLE automation.rules
  ADD COLUMN entity_type text NOT NULL DEFAULT 'contact'
    CHECK (entity_type IN ('contact', 'company', 'lead', 'deal', 'activity', 'workspace'));

ALTER TABLE activities.activities
  ADD COLUMN system_title_key text
    CHECK (system_title_key IS NULL OR char_length(system_title_key) BETWEEN 1 AND 160),
  ADD COLUMN system_title_params jsonb
    CHECK (system_title_params IS NULL OR octet_length(system_title_params::text) <= 8192),
  ADD CONSTRAINT activity_system_title_complete CHECK (
    (system_title_key IS NULL AND system_title_params IS NULL)
    OR (system_title_key IS NOT NULL AND system_title_params IS NOT NULL)
  );

CREATE INDEX automation_rules_dispatch_idx
  ON automation.rules (workspace_id, trigger_type, entity_type, updated_at, id)
  WHERE enabled;
CREATE INDEX automation_executions_state_idx
  ON automation.executions (workspace_id, state, created_at, id);

-- A database-side bucket makes the hourly limit atomic across all app/worker
-- processes. It is intentionally small and can be pruned by maintenance work.
CREATE TABLE automation.hourly_usage (
  workspace_id uuid NOT NULL,
  rule_id uuid NOT NULL,
  bucket_start timestamptz NOT NULL,
  used integer NOT NULL DEFAULT 0 CHECK (used BETWEEN 0 AND 100000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, rule_id, bucket_start),
  FOREIGN KEY (workspace_id, rule_id)
    REFERENCES automation.rules(workspace_id, id) ON DELETE CASCADE,
  CHECK (bucket_start = date_trunc('hour', bucket_start))
);

-- Per-action fences make a retry resume after completed actions. External
-- adapters also receive idempotency_key because no local database can promise
-- exactly-once behavior across an HTTP/SMTP boundary.
CREATE TABLE automation.action_executions (
  workspace_id uuid NOT NULL,
  execution_id uuid NOT NULL,
  action_index integer NOT NULL CHECK (action_index BETWEEN 0 AND 19),
  action_type text NOT NULL CHECK (char_length(action_type) BETWEEN 1 AND 80),
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 200),
  state text NOT NULL CHECK (state IN ('running', 'completed', 'failed')),
  result jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(result::text) <= 32768),
  last_error_code text CHECK (last_error_code IS NULL OR char_length(last_error_code) BETWEEN 1 AND 120),
  started_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, execution_id, action_index),
  UNIQUE (workspace_id, idempotency_key),
  FOREIGN KEY (workspace_id, execution_id)
    REFERENCES automation.executions(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX automation_action_executions_state_idx
  ON automation.action_executions (workspace_id, state, updated_at, execution_id);

CREATE INDEX automation_hourly_usage_cleanup_idx
  ON automation.hourly_usage (bucket_start, workspace_id, rule_id);

ALTER TABLE automation.hourly_usage ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation.hourly_usage FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON automation.hourly_usage
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

ALTER TABLE automation.action_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation.action_executions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON automation.action_executions
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

-- A rotated webhook keeps the previous envelope briefly. The core schema
-- already retained its ciphertext but did not retain enough metadata to
-- decrypt it safely.
ALTER TABLE integrations.webhook_subscriptions
  ADD COLUMN previous_secret_nonce bytea,
  ADD COLUMN previous_key_id text,
  ADD COLUMN secret_version integer NOT NULL DEFAULT 1 CHECK (secret_version > 0),
  ADD COLUMN timeout_seconds integer NOT NULL DEFAULT 10 CHECK (timeout_seconds BETWEEN 1 AND 30),
  ADD COLUMN max_attempts integer NOT NULL DEFAULT 8 CHECK (max_attempts BETWEEN 1 AND 20),
  ADD CONSTRAINT webhook_previous_secret_complete CHECK (
    (previous_secret_ciphertext IS NULL AND previous_secret_nonce IS NULL
      AND previous_key_id IS NULL AND previous_secret_expires_at IS NULL)
    OR
    (previous_secret_ciphertext IS NOT NULL AND previous_secret_nonce IS NOT NULL
      AND previous_key_id IS NOT NULL AND previous_secret_expires_at IS NOT NULL)
  );

ALTER TABLE integrations.webhook_deliveries
  ADD COLUMN request_timestamp bigint,
  ADD COLUMN signature_version integer NOT NULL DEFAULT 1 CHECK (signature_version = 1),
  ADD COLUMN last_error_code text
    CHECK (last_error_code IS NULL OR char_length(last_error_code) BETWEEN 1 AND 120),
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

CREATE INDEX api_keys_prefix_idx
  ON integrations.api_keys (workspace_id, key_prefix)
  WHERE revoked_at IS NULL;
CREATE INDEX webhook_subscriptions_dispatch_idx
  ON integrations.webhook_subscriptions (workspace_id, updated_at, id)
  WHERE enabled;
CREATE INDEX webhook_deliveries_retry_idx
  ON integrations.webhook_deliveries (workspace_id, next_attempt_at, created_at, id)
  WHERE status IN ('queued', 'failed');

GRANT SELECT, INSERT, UPDATE, DELETE ON automation.hourly_usage,
  automation.action_executions TO veltrix_app;

RESET ROLE;

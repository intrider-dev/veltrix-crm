-- name: CreateAutomationRule :one
INSERT INTO automation.rules (
  workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
  rate_limit_per_hour, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
          rate_limit_per_hour, version, created_by, created_at, updated_at;

-- name: GetAutomationRule :one
SELECT workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
       rate_limit_per_hour, version, created_by, created_at, updated_at
FROM automation.rules
WHERE workspace_id = $1 AND id = $2;

-- name: ListAutomationRules :many
SELECT workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
       rate_limit_per_hour, version, created_by, created_at, updated_at
FROM automation.rules
WHERE workspace_id = $1
ORDER BY updated_at DESC, id DESC
LIMIT $2;

-- name: ListEnabledAutomationRulesForTrigger :many
SELECT workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
       rate_limit_per_hour, version, created_by, created_at, updated_at
FROM automation.rules
WHERE workspace_id = $1 AND trigger_type = $2 AND entity_type = $3 AND enabled
ORDER BY updated_at, id
LIMIT 500;

-- name: GetAutomationOutboxEvent :one
SELECT id, event_type, aggregate_type, aggregate_id, correlation_id, payload
FROM platform.outbox_events
WHERE workspace_id = $1 AND id = $2;

-- name: GetContactAutomationSnapshot :one
SELECT jsonb_build_object(
  'first_name', first_name, 'last_name', last_name, 'display_name', display_name,
  'email', email, 'phone', phone, 'job_title', job_title, 'company_id', company_id,
  'owner_user_id', owner_user_id, 'team_id', team_id, 'status', status,
  'source', source, 'last_contacted_at', last_contacted_at,
  'next_activity_at', next_activity_at, 'custom_fields', custom_fields,
  'tags', COALESCE((
    SELECT jsonb_agg(relation.tag_id ORDER BY relation.tag_id)
    FROM customers.contact_tags relation
    WHERE relation.workspace_id = contacts.workspace_id AND relation.contact_id = contacts.id
  ), '[]'::jsonb)
) AS snapshot
FROM customers.contacts contacts
WHERE contacts.workspace_id = $1 AND contacts.id = $2;

-- name: GetCompanyAutomationSnapshot :one
SELECT jsonb_build_object(
  'name', name, 'domain', domain, 'industry', industry,
  'owner_user_id', owner_user_id, 'team_id', team_id,
  'status', status, 'address', address, 'custom_fields', custom_fields
) AS snapshot
FROM customers.companies
WHERE workspace_id = $1 AND id = $2;

-- name: GetLeadAutomationSnapshot :one
SELECT jsonb_build_object(
  'name', name, 'email', email, 'company_name', company_name,
  'status', status, 'source', source, 'owner_user_id', owner_user_id,
  'converted_contact_id', converted_contact_id,
  'converted_company_id', converted_company_id, 'converted_deal_id', converted_deal_id,
  'custom_fields', custom_fields
) AS snapshot
FROM sales.leads
WHERE workspace_id = $1 AND id = $2;

-- name: GetDealAutomationSnapshot :one
SELECT jsonb_build_object(
  'name', name, 'pipeline_id', pipeline_id, 'stage_id', stage_id,
  'contact_id', contact_id, 'company_id', company_id,
  'owner_user_id', owner_user_id,
  'amount_minor', amount_minor, 'currency', currency,
  'expected_close_date', expected_close_date, 'status', status,
  'lost_reason', lost_reason,
  'custom_fields', custom_fields
) AS snapshot
FROM sales.deals
WHERE workspace_id = $1 AND id = $2;

-- name: GetActivityAutomationSnapshot :one
SELECT jsonb_build_object(
  'activity_type', activity_type, 'title', title, 'related_type', related_type,
  'related_id', related_id, 'assignee_user_id', assignee_user_id,
  'due_at', due_at, 'priority', priority, 'status', status,
  'recurrence_rule', recurrence_rule, 'completed_at', completed_at,
  'occurred_at', occurred_at
) AS snapshot
FROM activities.activities
WHERE workspace_id = $1 AND id = $2;

-- name: UpdateAutomationRule :one
UPDATE automation.rules
SET name = $3,
    trigger_type = $4,
    entity_type = $5,
    conditions = $6,
    actions = $7,
    rate_limit_per_hour = $8,
    version = version + 1,
    updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $9
RETURNING workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
          rate_limit_per_hour, version, created_by, created_at, updated_at;

-- name: SetAutomationRuleEnabled :one
UPDATE automation.rules
SET enabled = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $4
RETURNING workspace_id, id, name, trigger_type, entity_type, conditions, actions, enabled,
          rate_limit_per_hour, version, created_by, created_at, updated_at;

-- name: TryConsumeAutomationRateLimit :one
INSERT INTO automation.hourly_usage (
  workspace_id, rule_id, bucket_start, used
) VALUES ($1, $2, date_trunc('hour', now()), 1)
ON CONFLICT (workspace_id, rule_id, bucket_start) DO UPDATE
SET used = automation.hourly_usage.used + 1,
    updated_at = now()
WHERE automation.hourly_usage.used < sqlc.arg(rate_limit_per_hour)
RETURNING used;

-- name: ReserveAutomationExecution :one
INSERT INTO automation.executions (
  workspace_id, id, rule_id, event_id, correlation_id, depth, state,
  trigger_type, event_payload
) VALUES ($1, $2, $3, $4, $5, $6, 'queued', $7, $8)
ON CONFLICT (workspace_id, rule_id, event_id) DO NOTHING
RETURNING workspace_id, id, rule_id, event_id, correlation_id, depth, state,
          attempts, result, trigger_type, event_payload, started_at,
          completed_at, updated_at, last_error_code, created_at;

-- name: EnqueueAutomationExecution :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  state, attempts, max_attempts, available_at
) VALUES ($1, $2, 'automation.execute', 1, $3, $4, 'ready', 0, $5, now())
ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING;

-- name: CancelRateLimitedAutomationExecution :execrows
DELETE FROM automation.executions
WHERE workspace_id = $1 AND id = $2 AND state = 'queued';

-- name: GetAutomationExecution :one
SELECT workspace_id, id, rule_id, event_id, correlation_id, depth, state,
       attempts, result, trigger_type, event_payload, started_at,
       completed_at, updated_at, last_error_code, created_at
FROM automation.executions
WHERE workspace_id = $1 AND id = $2;

-- name: StartAutomationAction :one
INSERT INTO automation.action_executions (
  workspace_id, execution_id, action_index, action_type, idempotency_key, state
) VALUES ($1, $2, $3, $4, $5, 'running')
ON CONFLICT (workspace_id, execution_id, action_index) DO UPDATE
SET state = 'running', action_type = EXCLUDED.action_type,
    idempotency_key = EXCLUDED.idempotency_key,
    started_at = now(), updated_at = now(), last_error_code = NULL
WHERE automation.action_executions.state = 'failed'
   OR (automation.action_executions.state = 'running'
       AND automation.action_executions.updated_at < now() - interval '2 minutes')
RETURNING state, result, idempotency_key;

-- name: GetAutomationAction :one
SELECT state, result, idempotency_key
FROM automation.action_executions
WHERE workspace_id = $1 AND execution_id = $2 AND action_index = $3;

-- name: CompleteAutomationAction :execrows
UPDATE automation.action_executions
SET state = 'completed', result = $4, completed_at = now(), updated_at = now(),
    last_error_code = NULL
WHERE workspace_id = $1 AND execution_id = $2 AND action_index = $3
  AND state = 'running';

-- name: FailAutomationAction :execrows
UPDATE automation.action_executions
SET state = 'failed', last_error_code = $4, updated_at = now()
WHERE workspace_id = $1 AND execution_id = $2 AND action_index = $3
  AND state = 'running';

-- name: StartAutomationExecution :execrows
UPDATE automation.executions
SET state = 'running', attempts = attempts + 1,
    started_at = COALESCE(started_at, now()), updated_at = now(),
    last_error_code = NULL
WHERE workspace_id = $1 AND id = $2
  AND (
    state IN ('queued', 'failed')
    OR (state = 'running' AND updated_at < now() - interval '2 minutes')
  );

-- name: CompleteAutomationExecution :execrows
UPDATE automation.executions
SET state = 'completed', result = $3, completed_at = now(), updated_at = now(),
    last_error_code = NULL
WHERE workspace_id = $1 AND id = $2 AND state = 'running';

-- name: FailAutomationExecution :execrows
UPDATE automation.executions
SET state = CASE WHEN sqlc.arg(dead)::boolean THEN 'dead' ELSE 'failed' END,
    result = sqlc.arg(result),
    completed_at = CASE WHEN sqlc.arg(dead)::boolean THEN now() ELSE NULL END,
    updated_at = now(), last_error_code = sqlc.arg(error_code)
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND state = 'running';

-- name: PruneAutomationHourlyUsage :execrows
DELETE FROM automation.hourly_usage
WHERE bucket_start < now() - interval '48 hours';

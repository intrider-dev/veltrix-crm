-- name: InsertAuditEvent :exec
INSERT INTO audit.events (
  workspace_id, id, actor_user_id, action, entity_type, entity_id, request_id,
  summary, ip_address, user_agent
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAuditEvents :many
SELECT id, actor_user_id, action, entity_type, entity_id, request_id, summary,
       occurred_at
FROM audit.events
WHERE workspace_id = $1
ORDER BY occurred_at DESC, id DESC
LIMIT $2;

-- name: InsertOutboxEvent :exec
INSERT INTO platform.outbox_events (
  workspace_id, id, event_type, schema_version, aggregate_type, aggregate_id,
  causation_id, correlation_id, payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: InsertSSEEvent :exec
INSERT INTO notifications.sse_events (
  workspace_id, id, event_type, data, expires_at
) VALUES ($1, $2, $3, $4, now() + interval '24 hours');

-- name: InsertUserSSEEvent :exec
INSERT INTO notifications.sse_events (
  workspace_id, id, event_type, data, recipient_user_id, expires_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(event_id), sqlc.arg(event_type),
  sqlc.arg(data), sqlc.arg(recipient_user_id), now() + interval '24 hours'
);

-- name: ListSSEEventsAfter :many
WITH anchor AS (
  SELECT anchor_event.created_at, anchor_event.id
  FROM notifications.sse_events AS anchor_event
  WHERE anchor_event.workspace_id = $1 AND anchor_event.id = $2
)
SELECT event.id, event.event_type, event.data, event.created_at
FROM notifications.sse_events event, anchor
WHERE event.workspace_id = $1
  AND (event.created_at, event.id) > (anchor.created_at, anchor.id)
  AND event.expires_at > now()
ORDER BY event.created_at, event.id
LIMIT 100;

-- name: GetSSEEventForDispatch :one
SELECT id, event_type, data, recipient_user_id
FROM notifications.sse_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(event_id)
  AND expires_at > now();

-- name: ListSSEEventsAfterForRecipient :many
WITH anchor AS (
  SELECT anchor_event.created_at, anchor_event.id
  FROM notifications.sse_events AS anchor_event
  WHERE anchor_event.workspace_id = sqlc.arg(workspace_id)
    AND anchor_event.id = sqlc.arg(anchor_id)
    AND (anchor_event.recipient_user_id IS NULL
         OR anchor_event.recipient_user_id = sqlc.arg(recipient_user_id))
)
SELECT event.id, event.event_type, event.data, event.recipient_user_id, event.created_at
FROM notifications.sse_events AS event, anchor
WHERE event.workspace_id = sqlc.arg(workspace_id)
  AND (event.created_at, event.id) > (anchor.created_at, anchor.id)
  AND event.expires_at > now()
  AND (event.recipient_user_id IS NULL
       OR event.recipient_user_id = sqlc.arg(recipient_user_id))
ORDER BY event.created_at, event.id
LIMIT 100;

-- name: UpsertSearchDocument :exec
INSERT INTO search.documents (
  workspace_id, entity_type, entity_id, title, subtitle, searchable_text,
  rank_boost, version
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (workspace_id, entity_type, entity_id) DO UPDATE
SET title = EXCLUDED.title,
    subtitle = EXCLUDED.subtitle,
    searchable_text = EXCLUDED.searchable_text,
    rank_boost = EXCLUDED.rank_boost,
    version = EXCLUDED.version,
    updated_at = now();

-- name: DeleteSearchDocument :exec
DELETE FROM search.documents
WHERE workspace_id = $1 AND entity_type = $2 AND entity_id = $3;

-- name: GlobalSearch :many
SELECT (query_result).entity_type::text AS entity_type,
       (query_result).entity_id::uuid AS entity_id,
       (query_result).title::text AS title,
       (query_result).subtitle::text AS subtitle,
       (query_result).snippet::text AS snippet,
       (query_result).rank::real AS rank
FROM (
  SELECT search.query_documents(
    sqlc.arg(workspace_id), sqlc.arg(search_query)::text, 50
  ) AS query_result
) AS results;

-- name: GetDashboardSummary :one
SELECT currency, open_pipeline_minor, weighted_forecast_minor, won_count,
       lost_count, overdue_tasks, deals_by_stage, computed_at, source_version
FROM reporting.dashboard_summaries
WHERE workspace_id = $1;

-- name: HasPipelineStageAccessRules :one
SELECT EXISTS (
  SELECT 1
  FROM sales.pipeline_stage_role_access access
  WHERE access.workspace_id = $1
);

-- name: GetDashboardSummaryWithStageAccess :one
WITH visible_deals AS MATERIALIZED (
  SELECT deal.id, deal.stage_id, deal.amount_minor, deal.status, deal.version
  FROM sales.deals deal
  WHERE deal.workspace_id = sqlc.arg(workspace_id)
    AND deal.deleted_at IS NULL
    AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
)
SELECT workspace.default_currency AS currency,
       COALESCE((SELECT sum(deal.amount_minor) FROM visible_deals deal WHERE deal.status = 'open'), 0)::bigint AS open_pipeline_minor,
       COALESCE((
         SELECT sum((deal.amount_minor * stage.probability) / 100)
         FROM visible_deals deal
         JOIN sales.pipeline_stages stage
           ON stage.workspace_id = workspace.id AND stage.id = deal.stage_id
         WHERE deal.status = 'open'
       ), 0)::bigint AS weighted_forecast_minor,
       (SELECT count(*) FROM visible_deals deal WHERE deal.status = 'won')::bigint AS won_count,
       (SELECT count(*) FROM visible_deals deal WHERE deal.status = 'lost')::bigint AS lost_count,
       (SELECT count(*) FROM activities.activities activity
        WHERE activity.workspace_id = workspace.id
          AND activity.activity_type = 'task'
          AND activity.status = 'open'
          AND activity.due_at < now()
          AND activity.deleted_at IS NULL)::bigint AS overdue_tasks,
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'stageId', stage.id,
           'stageName', stage.name,
           'count', (SELECT count(*) FROM visible_deals deal WHERE deal.stage_id = stage.id AND deal.status = 'open'),
           'amountMinor', COALESCE((SELECT sum(deal.amount_minor) FROM visible_deals deal WHERE deal.stage_id = stage.id AND deal.status = 'open'), 0)
         ) ORDER BY stage.position)
         FROM sales.pipeline_stages stage
         WHERE stage.workspace_id = workspace.id
           AND sales.pipeline_stage_access_allowed(stage.workspace_id, stage.id, 'view')
       ), '[]'::jsonb)::jsonb AS deals_by_stage,
       now()::timestamptz AS computed_at,
       COALESCE((SELECT max(deal.version) FROM visible_deals deal), 1)::bigint AS source_version
FROM tenancy.workspaces workspace
WHERE workspace.id = sqlc.arg(workspace_id);

-- name: RefreshDashboardSummary :exec
INSERT INTO reporting.dashboard_summaries (
  workspace_id, currency, open_pipeline_minor, weighted_forecast_minor,
  won_count, lost_count, overdue_tasks, deals_by_stage, computed_at, source_version
)
SELECT w.id,
       w.default_currency,
       COALESCE(sum(d.amount_minor) FILTER (WHERE d.status = 'open'), 0)::bigint,
       COALESCE(sum((d.amount_minor * s.probability) / 100) FILTER (WHERE d.status = 'open'), 0)::bigint,
       count(d.id) FILTER (WHERE d.status = 'won')::bigint,
       count(d.id) FILTER (WHERE d.status = 'lost')::bigint,
       (SELECT count(*) FROM activities.activities a
         WHERE a.workspace_id = w.id AND a.activity_type = 'task'
           AND a.status = 'open' AND a.due_at < now() AND a.deleted_at IS NULL)::bigint,
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'stageId', stage_totals.stage_id,
           'stageName', stage_totals.stage_name,
           'count', stage_totals.deal_count,
           'amountMinor', stage_totals.amount_minor
         ) ORDER BY stage_totals.position)
         FROM (
           SELECT ps.id AS stage_id, ps.name AS stage_name, ps.position,
                  count(sd.id)::bigint AS deal_count,
                  COALESCE(sum(sd.amount_minor), 0)::bigint AS amount_minor
           FROM sales.pipeline_stages ps
           LEFT JOIN sales.deals sd
             ON sd.workspace_id = ps.workspace_id AND sd.stage_id = ps.id
             AND sd.status = 'open' AND sd.deleted_at IS NULL
           WHERE ps.workspace_id = w.id
           GROUP BY ps.id, ps.name, ps.position
         ) stage_totals
       ), '[]'::jsonb),
       now(),
       COALESCE((SELECT max(version) FROM sales.deals vd WHERE vd.workspace_id = w.id), 1)
FROM tenancy.workspaces w
LEFT JOIN sales.deals d ON d.workspace_id = w.id AND d.deleted_at IS NULL
LEFT JOIN sales.pipeline_stages s ON s.workspace_id = d.workspace_id AND s.id = d.stage_id
WHERE w.id = $1
GROUP BY w.id, w.default_currency
ON CONFLICT (workspace_id) DO UPDATE
SET currency = EXCLUDED.currency,
    open_pipeline_minor = EXCLUDED.open_pipeline_minor,
    weighted_forecast_minor = EXCLUDED.weighted_forecast_minor,
    won_count = EXCLUDED.won_count,
    lost_count = EXCLUDED.lost_count,
    overdue_tasks = EXCLUDED.overdue_tasks,
    deals_by_stage = EXCLUDED.deals_by_stage,
    computed_at = EXCLUDED.computed_at,
    source_version = EXCLUDED.source_version;

-- name: ReserveIdempotencyKey :one
INSERT INTO platform.idempotency_keys (
  workspace_id, key, actor_id, operation, request_hash, expires_at
) VALUES ($1, $2, $3, $4, $5, now() + interval '24 hours')
ON CONFLICT (workspace_id, key) DO NOTHING
RETURNING key;

-- name: GetIdempotencyKey :one
SELECT key, actor_id, operation, request_hash, response_status, response_body,
       expires_at, created_at
FROM platform.idempotency_keys
WHERE workspace_id = $1 AND key = $2 AND expires_at > now();

-- name: CompleteIdempotencyKey :exec
UPDATE platform.idempotency_keys
SET response_status = $3, response_body = $4
WHERE workspace_id = $1 AND key = $2;

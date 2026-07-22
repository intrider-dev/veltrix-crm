-- name: GetDashboardPreferences :one
SELECT preference.preferences, preference.period_days, preference.timezone,
       preference.version, preference.updated_at,
       workspace.timezone AS workspace_timezone
FROM tenancy.workspaces workspace
LEFT JOIN reporting.dashboard_preferences preference
  ON preference.workspace_id = workspace.id
 AND preference.user_id = sqlc.arg(user_id)
WHERE workspace.id = sqlc.arg(workspace_id);

-- name: SaveDashboardPreferences :one
INSERT INTO reporting.dashboard_preferences (
  workspace_id, user_id, preferences, period_days, timezone
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(preferences),
  sqlc.arg(period_days), sqlc.narg(timezone)
)
ON CONFLICT (workspace_id, user_id) DO UPDATE
SET preferences = EXCLUDED.preferences,
    period_days = EXCLUDED.period_days,
    timezone = EXCLUDED.timezone,
    version = reporting.dashboard_preferences.version + 1,
    updated_at = now()
WHERE reporting.dashboard_preferences.version = sqlc.arg(expected_version)
RETURNING preferences, period_days, timezone, version, updated_at;

-- name: GetPeriodReportOverview :one
SELECT
  (SELECT count(*)::bigint
   FROM sales.deals deal
   WHERE deal.workspace_id = sqlc.arg(workspace_id)
     AND deal.deleted_at IS NULL
     AND deal.updated_at >= sqlc.arg(period_start)::timestamptz
     AND deal.updated_at < sqlc.arg(period_end)::timestamptz
     AND deal.status = 'won') AS won_count,
  (SELECT count(*)::bigint
   FROM sales.deals deal
   WHERE deal.workspace_id = sqlc.arg(workspace_id)
     AND deal.deleted_at IS NULL
     AND deal.updated_at >= sqlc.arg(period_start)::timestamptz
     AND deal.updated_at < sqlc.arg(period_end)::timestamptz
     AND deal.status = 'lost') AS lost_count,
  (SELECT COALESCE(sum(deal.amount_minor), 0)::bigint
   FROM sales.deals deal
   WHERE deal.workspace_id = sqlc.arg(workspace_id)
     AND deal.deleted_at IS NULL
     AND deal.updated_at >= sqlc.arg(period_start)::timestamptz
     AND deal.updated_at < sqlc.arg(period_end)::timestamptz
     AND deal.status = 'won') AS won_value_minor,
  (SELECT count(*)::bigint
   FROM sales.leads lead
   WHERE lead.workspace_id = sqlc.arg(workspace_id)
     AND lead.deleted_at IS NULL
     AND lead.created_at >= sqlc.arg(period_start)::timestamptz
     AND lead.created_at < sqlc.arg(period_end)::timestamptz) AS lead_count,
  (SELECT count(*)::bigint
   FROM sales.leads lead
   WHERE lead.workspace_id = sqlc.arg(workspace_id)
     AND lead.deleted_at IS NULL
     AND lead.created_at >= sqlc.arg(period_start)::timestamptz
     AND lead.created_at < sqlc.arg(period_end)::timestamptz
     AND lead.status = 'converted') AS converted_lead_count,
  (SELECT count(*)::bigint
   FROM activities.activities activity
   WHERE activity.workspace_id = sqlc.arg(workspace_id)
     AND activity.deleted_at IS NULL
     AND activity.occurred_at >= sqlc.arg(period_start)::timestamptz
     AND activity.occurred_at < sqlc.arg(period_end)::timestamptz) AS activity_count;

-- name: ReportDealsByStage :many
SELECT stage.id AS stage_id, stage.name AS stage_name, stage.position,
       count(deal.id)::bigint AS deal_count,
       COALESCE(sum(deal.amount_minor), 0)::bigint AS amount_minor,
       COALESCE(sum((deal.amount_minor * stage.probability) / 100), 0)::bigint
         AS weighted_amount_minor
FROM sales.pipeline_stages stage
LEFT JOIN sales.deals deal
  ON deal.workspace_id = stage.workspace_id
 AND deal.stage_id = stage.id
 AND deal.deleted_at IS NULL
 AND deal.updated_at >= sqlc.arg(period_start)::timestamptz
 AND deal.updated_at < sqlc.arg(period_end)::timestamptz
WHERE stage.workspace_id = sqlc.arg(workspace_id)
GROUP BY stage.id, stage.name, stage.position
ORDER BY stage.position, stage.id
LIMIT 200;

-- name: ReportDealsByOwner :many
SELECT deal.owner_user_id,
       COALESCE(users.display_name, '') AS owner_name,
       count(deal.id)::bigint AS deal_count,
       count(deal.id) FILTER (WHERE deal.status = 'won')::bigint AS won_count,
       count(deal.id) FILTER (WHERE deal.status = 'lost')::bigint AS lost_count,
       COALESCE(sum(deal.amount_minor), 0)::bigint AS amount_minor
FROM sales.deals deal
LEFT JOIN identity.users users ON users.id = deal.owner_user_id
WHERE deal.workspace_id = sqlc.arg(workspace_id)
  AND deal.deleted_at IS NULL
  AND deal.updated_at >= sqlc.arg(period_start)::timestamptz
  AND deal.updated_at < sqlc.arg(period_end)::timestamptz
GROUP BY deal.owner_user_id, users.display_name
ORDER BY amount_minor DESC, owner_name, deal.owner_user_id
LIMIT 500;

-- name: ReportActivitiesByDay :many
SELECT (date_trunc(
          'day', activity.occurred_at AT TIME ZONE sqlc.arg(timezone)::text
       ))::date AS activity_date,
       count(*)::bigint AS activity_count,
       count(*) FILTER (WHERE activity.activity_type = 'task')::bigint AS task_count,
       count(*) FILTER (WHERE activity.activity_type = 'call')::bigint AS call_count,
       count(*) FILTER (WHERE activity.activity_type = 'meeting')::bigint AS meeting_count,
       count(*) FILTER (WHERE activity.activity_type = 'note')::bigint AS note_count
FROM activities.activities activity
WHERE activity.workspace_id = sqlc.arg(workspace_id)
  AND activity.deleted_at IS NULL
  AND activity.occurred_at >= sqlc.arg(period_start)::timestamptz
  AND activity.occurred_at < sqlc.arg(period_end)::timestamptz
GROUP BY activity_date
ORDER BY activity_date
LIMIT 366;

-- name: ReportLeadSources :many
SELECT COALESCE(NULLIF(trim(lead.source), ''), 'unspecified')::text AS source,
       count(*)::bigint AS lead_count,
       count(*) FILTER (WHERE lead.status = 'converted')::bigint AS converted_count
FROM sales.leads lead
WHERE lead.workspace_id = sqlc.arg(workspace_id)
  AND lead.deleted_at IS NULL
  AND lead.created_at >= sqlc.arg(period_start)::timestamptz
  AND lead.created_at < sqlc.arg(period_end)::timestamptz
GROUP BY COALESCE(NULLIF(trim(lead.source), ''), 'unspecified')
ORDER BY lead_count DESC, source
LIMIT 100;

-- name: ListDashboardRecentActivity :many
SELECT id, activity_type, title, related_type, related_id, status, priority,
       occurred_at, due_at, assignee_user_id
FROM activities.activities
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND occurred_at >= sqlc.arg(period_start)::timestamptz
  AND occurred_at < sqlc.arg(period_end)::timestamptz
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(result_limit);

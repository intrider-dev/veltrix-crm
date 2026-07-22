-- name: ListPipelines :many
SELECT id, name, is_default, version, created_at, updated_at
FROM sales.pipelines
WHERE workspace_id = $1
ORDER BY is_default DESC, name, id;

-- name: ListPipelineStages :many
SELECT id, pipeline_id, name, probability, forecast_category, position,
       created_at, updated_at
FROM sales.pipeline_stages stage
WHERE stage.workspace_id = $1 AND stage.pipeline_id = $2
  AND sales.pipeline_stage_access_allowed(stage.workspace_id, stage.id, 'view')
ORDER BY position, id;

-- name: GetPipelineStage :one
SELECT id, pipeline_id, name, probability, forecast_category, position,
       created_at, updated_at
FROM sales.pipeline_stages
WHERE workspace_id = $1 AND pipeline_id = $2 AND id = $3;

-- name: CreatePipeline :one
INSERT INTO sales.pipelines (workspace_id, id, name, is_default)
VALUES ($1, $2, $3, $4)
RETURNING id, name, is_default, version, created_at, updated_at;

-- name: CreatePipelineStage :one
INSERT INTO sales.pipeline_stages (
  workspace_id, id, pipeline_id, name, probability, forecast_category, position
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, pipeline_id, name, probability, forecast_category, position,
          created_at, updated_at;

-- name: ListDeals :many
SELECT id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
       amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
       version, created_at, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = sqlc.arg(workspace_id)
  AND deal.deleted_at IS NULL
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
  AND (sqlc.arg(pipeline_id)::uuid IS NULL OR deal.pipeline_id = sqlc.arg(pipeline_id))
  AND (sqlc.arg(stage_id)::uuid IS NULL OR deal.stage_id = sqlc.arg(stage_id))
  AND (deal.updated_at, deal.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY deal.updated_at DESC, deal.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetDeal :one
SELECT id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
       amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
       custom_fields, version, created_at, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = $1 AND deal.id = $2 AND deal.deleted_at IS NULL
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view');

-- name: CreateDeal :one
INSERT INTO sales.deals (
  workspace_id, id, pipeline_id, stage_id, name, contact_id, company_id,
  owner_user_id, amount_minor, currency, planned_start_date, expected_close_date, position
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
          amount_minor, currency, planned_start_date, expected_close_date, position, status,
          lost_reason, version, created_at, updated_at;

-- name: MoveDeal :one
UPDATE sales.deals
SET stage_id = $3, position = $4, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $5 AND deleted_at IS NULL
RETURNING id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
          amount_minor, currency, planned_start_date, expected_close_date, position, status,
          lost_reason, version, created_at, updated_at;

-- name: AddDealStageHistory :exec
INSERT INTO sales.deal_stage_history (
  workspace_id, id, deal_id, from_stage_id, to_stage_id, changed_by
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: NextDealPosition :one
SELECT COALESCE(max(position), -1)::integer + 1
FROM sales.deals
WHERE workspace_id = $1 AND pipeline_id = $2 AND stage_id = $3
  AND status = 'open' AND deleted_at IS NULL;

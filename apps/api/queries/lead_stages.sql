-- name: ListLeadStages :many
SELECT id, name, category, color, position, system_key, is_default, version,
       created_at, updated_at
FROM sales.lead_stages
WHERE workspace_id = $1 AND archived_at IS NULL
ORDER BY position, id;

-- name: GetLeadStage :one
SELECT id, name, category, color, position, system_key, is_default, version,
       created_at, updated_at
FROM sales.lead_stages
WHERE workspace_id = $1 AND id = $2 AND archived_at IS NULL;

-- name: GetDefaultLeadStageByCategory :one
SELECT id, name, category, color, position, system_key, is_default, version,
       created_at, updated_at
FROM sales.lead_stages
WHERE workspace_id = $1 AND category = $2 AND is_default AND archived_at IS NULL;

-- name: NextLeadStagePosition :one
SELECT COALESCE(max(position), -1)::integer + 1
FROM sales.lead_stages
WHERE workspace_id = $1 AND archived_at IS NULL;

-- name: CreateLeadStage :one
INSERT INTO sales.lead_stages (
  workspace_id, id, name, category, color, position, system_key, is_default
) VALUES ($1, $2, $3, $4, $5, $6, NULL, false)
RETURNING id, name, category, color, position, system_key, is_default, version,
          created_at, updated_at;

-- name: UpdateLeadStage :one
UPDATE sales.lead_stages
SET name = CASE WHEN system_key IS NULL THEN $3 ELSE name END,
    color = $4, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $5 AND archived_at IS NULL
RETURNING id, name, category, color, position, system_key, is_default, version,
          created_at, updated_at;

-- name: DeleteLeadStage :execrows
UPDATE sales.lead_stages stage
SET archived_at = now(), version = version + 1, updated_at = now()
WHERE stage.workspace_id = $1 AND stage.id = $2 AND stage.version = $3
  AND stage.system_key IS NULL
  AND stage.archived_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sales.leads lead
    WHERE lead.workspace_id = stage.workspace_id AND lead.stage_id = stage.id
  );

-- name: MoveLeadToStage :one
UPDATE sales.leads lead
SET stage_id = stage.id, status = stage.category,
    version = lead.version + 1, updated_at = now()
FROM sales.lead_stages stage
WHERE lead.workspace_id = sqlc.arg(workspace_id)
  AND lead.id = sqlc.arg(lead_id)
  AND lead.version = sqlc.arg(version)
  AND lead.deleted_at IS NULL
  AND lead.status <> 'converted'
  AND stage.workspace_id = lead.workspace_id
  AND stage.id = sqlc.arg(stage_id)
  AND stage.category <> 'converted'
  AND stage.archived_at IS NULL
RETURNING lead.id, lead.name, lead.email, lead.phone, lead.company_name,
          lead.job_title, lead.source, lead.status, lead.stage_id,
          lead.owner_user_id, lead.team_id, lead.converted_contact_id,
          lead.converted_company_id, lead.converted_deal_id, lead.custom_fields,
          lead.version, lead.created_at, lead.updated_at;

-- name: InsertLeadStageHistory :exec
INSERT INTO sales.lead_stage_history (
  workspace_id, id, lead_id, from_stage_id, to_stage_id, changed_by
) VALUES ($1, $2, $3, $4, $5, $6);

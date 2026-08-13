-- name: GetPipelineAdvanced :one
SELECT id, name, is_default, version, created_at, updated_at
FROM sales.pipelines
WHERE workspace_id = $1 AND id = $2;

-- name: CountPipelines :one
SELECT count(*) FROM sales.pipelines WHERE workspace_id = $1;

-- name: ListPipelineStagesForWorkspace :many
SELECT id, pipeline_id, name, probability, forecast_category, position, version,
       created_at, updated_at
FROM sales.pipeline_stages stage
WHERE stage.workspace_id = $1
  AND sales.pipeline_stage_access_allowed(stage.workspace_id, stage.id, 'view')
ORDER BY pipeline_id, position, id;

-- name: UnsetPipelineDefaults :exec
UPDATE sales.pipelines
SET is_default = false, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id <> $2 AND is_default;

-- name: UpdatePipelineAdvanced :one
UPDATE sales.pipelines
SET name = $3, is_default = $4, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $5
RETURNING id, name, is_default, version, created_at, updated_at;

-- name: DeletePipelineAdvanced :one
DELETE FROM sales.pipelines pipeline
WHERE pipeline.workspace_id = $1
  AND pipeline.id = $2
  AND pipeline.version = $3
  AND NOT pipeline.is_default
  AND (SELECT count(*) FROM sales.pipelines sibling WHERE sibling.workspace_id = $1) > 1
  AND NOT EXISTS (
    SELECT 1 FROM sales.deals deal
    WHERE deal.workspace_id = pipeline.workspace_id AND deal.pipeline_id = pipeline.id
  )
RETURNING pipeline.id;

-- name: NextPipelineStagePositionAdvanced :one
SELECT COALESCE(max(position), -1)::integer + 1
FROM sales.pipeline_stages
WHERE workspace_id = $1 AND pipeline_id = $2;

-- name: GetPipelineStageAdvanced :one
SELECT id, pipeline_id, name, probability, forecast_category, position, version,
       created_at, updated_at
FROM sales.pipeline_stages
WHERE workspace_id = $1 AND id = $2;

-- name: UpdatePipelineStageAdvanced :one
UPDATE sales.pipeline_stages
SET name = $3, probability = $4, forecast_category = $5,
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $6
RETURNING id, pipeline_id, name, probability, forecast_category, position, version,
          created_at, updated_at;

-- name: OffsetPipelineStagePositions :exec
UPDATE sales.pipeline_stages
SET position = position + 1000000
WHERE workspace_id = $1 AND pipeline_id = $2;

-- name: ApplyPipelineStagePosition :one
UPDATE sales.pipeline_stages
SET position = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2
RETURNING id, pipeline_id, name, probability, forecast_category, position, version,
          created_at, updated_at;

-- name: CountPipelineStages :one
SELECT count(*) FROM sales.pipeline_stages
WHERE workspace_id = $1 AND pipeline_id = $2;

-- name: DeletePipelineStageAdvanced :one
DELETE FROM sales.pipeline_stages stage
WHERE stage.workspace_id = $1
  AND stage.id = $2
  AND stage.version = $3
  AND (SELECT count(*) FROM sales.pipeline_stages sibling
       WHERE sibling.workspace_id = stage.workspace_id AND sibling.pipeline_id = stage.pipeline_id) > 1
  AND NOT EXISTS (
    SELECT 1 FROM sales.deals deal
    WHERE deal.workspace_id = stage.workspace_id AND deal.stage_id = stage.id
  )
RETURNING stage.id, stage.pipeline_id;

-- name: ListLeadsAdvanced :many
SELECT id, name, email, phone, company_name, job_title, source, status, stage_id,
       owner_user_id, team_id, converted_contact_id, converted_company_id,
       converted_deal_id, planned_start_date, expected_close_date,
       custom_fields, version, created_at, updated_at
FROM sales.leads lead
WHERE lead.workspace_id = sqlc.arg(workspace_id)
  AND lead.deleted_at IS NULL
  AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
  AND (sqlc.arg(search_query)::text = '' OR lead.name ILIKE '%' || sqlc.arg(search_query) || '%'
       OR COALESCE(lead.email_normalized, '') ILIKE '%' || lower(sqlc.arg(search_query)) || '%'
       OR COALESCE(lead.company_name, '') ILIKE '%' || sqlc.arg(search_query) || '%')
  AND (sqlc.arg(status_filter)::text = '' OR lead.status = sqlc.arg(status_filter))
  AND (sqlc.arg(owner_id)::uuid IS NULL OR lead.owner_user_id = sqlc.arg(owner_id))
  AND (sqlc.arg(stage_filter)::uuid IS NULL OR lead.stage_id = sqlc.arg(stage_filter))
  AND (lead.updated_at, lead.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY lead.updated_at DESC, lead.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetLeadAdvanced :one
SELECT id, name, email, phone, company_name, job_title, source, status, stage_id,
       owner_user_id, team_id, converted_contact_id, converted_company_id,
       converted_deal_id, planned_start_date, expected_close_date,
       custom_fields, version, deleted_at, deleted_by,
       created_at, updated_at
FROM sales.leads lead
WHERE lead.workspace_id = $1 AND lead.id = $2 AND lead.deleted_at IS NULL
  AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view');

-- name: CreateLeadAdvanced :one
INSERT INTO sales.leads (
  workspace_id, id, name, email, email_normalized, phone, phone_normalized,
  company_name, job_title, source, status, stage_id, owner_user_id, team_id,
  planned_start_date, expected_close_date, custom_fields
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
RETURNING id, name, email, phone, company_name, job_title, source, status, stage_id,
          owner_user_id, team_id, converted_contact_id, converted_company_id,
          converted_deal_id, planned_start_date, expected_close_date,
          custom_fields, version, created_at, updated_at;

-- name: UpdateLeadAdvanced :one
UPDATE sales.leads
SET name = $3, email = $4, email_normalized = $5, phone = $6,
    phone_normalized = $7, company_name = $8, job_title = $9, source = $10,
    status = $11, stage_id = $12, owner_user_id = $13, team_id = $14,
    planned_start_date = $15, expected_close_date = $16, custom_fields = $17,
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $18 AND deleted_at IS NULL
  AND status <> 'converted'
RETURNING id, name, email, phone, company_name, job_title, source, status, stage_id,
          owner_user_id, team_id, converted_contact_id, converted_company_id,
          converted_deal_id, planned_start_date, expected_close_date,
          custom_fields, version, created_at, updated_at;

-- name: SoftDeleteLead :one
UPDATE sales.leads
SET deleted_at = now(), deleted_by = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $4 AND deleted_at IS NULL
RETURNING version;

-- name: RestoreLead :one
UPDATE sales.leads
SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NOT NULL
RETURNING id, name, email, phone, company_name, job_title, source, status, stage_id,
          owner_user_id, team_id, converted_contact_id, converted_company_id,
          converted_deal_id, planned_start_date, expected_close_date,
          custom_fields, version, created_at, updated_at;

-- name: ListLeadTrash :many
SELECT id, name, email, company_name, status, owner_user_id, version, deleted_at,
       deleted_by, created_at, updated_at
FROM sales.leads lead
WHERE lead.workspace_id = sqlc.arg(workspace_id) AND lead.deleted_at IS NOT NULL
  AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
  AND (lead.deleted_at, lead.id) < (sqlc.arg(cursor_deleted_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY lead.deleted_at DESC, lead.id DESC
LIMIT sqlc.arg(page_limit);

-- name: MarkLeadConverted :one
UPDATE sales.leads
SET status = 'converted', converted_contact_id = $3, converted_company_id = $4,
    converted_deal_id = $5,
    stage_id = (SELECT stage.id FROM sales.lead_stages stage
                WHERE stage.workspace_id = sales.leads.workspace_id
                  AND stage.category = 'converted' AND stage.is_default),
    version = version + 1, updated_at = now()
WHERE sales.leads.workspace_id = $1 AND sales.leads.id = $2
  AND sales.leads.version = $6 AND sales.leads.deleted_at IS NULL
  AND sales.leads.status <> 'converted'
RETURNING id, name, email, phone, company_name, job_title, source, status, stage_id,
          owner_user_id, team_id, converted_contact_id, converted_company_id,
          converted_deal_id, planned_start_date, expected_close_date,
          custom_fields, version, created_at, updated_at;

-- name: ListDealsAdvanced :many
SELECT id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
       amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
       forecast_category, won_at, lost_at, custom_fields, version, created_at, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = sqlc.arg(workspace_id)
  AND deal.deleted_at IS NULL
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
  AND (sqlc.arg(search_query)::text = '' OR deal.name ILIKE '%' || sqlc.arg(search_query) || '%')
  AND (sqlc.arg(pipeline_id)::uuid IS NULL OR deal.pipeline_id = sqlc.arg(pipeline_id))
  AND (sqlc.arg(stage_id)::uuid IS NULL OR deal.stage_id = sqlc.arg(stage_id))
  AND (sqlc.arg(owner_id)::uuid IS NULL OR deal.owner_user_id = sqlc.arg(owner_id))
  AND (sqlc.arg(status_filter)::text = '' OR deal.status = sqlc.arg(status_filter))
  AND (deal.updated_at, deal.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY deal.updated_at DESC, deal.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetDealAdvanced :one
SELECT id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
       amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
       forecast_category, won_at, lost_at, custom_fields, version, deleted_at,
       deleted_by, created_at, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = $1 AND deal.id = $2 AND deal.deleted_at IS NULL
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view');

-- name: UpdateDealAdvanced :one
UPDATE sales.deals
SET pipeline_id = $3, stage_id = $4, name = $5, contact_id = $6,
    company_id = $7, owner_user_id = $8, amount_minor = $9, currency = $10,
    planned_start_date = $11, expected_close_date = $12, forecast_category = $13, custom_fields = $14,
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $15 AND deleted_at IS NULL
RETURNING id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
          amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
          forecast_category, won_at, lost_at, custom_fields, version, created_at, updated_at;

-- name: SetDealOutcome :one
UPDATE sales.deals
SET status = $3,
    lost_reason = CASE WHEN $3 = 'lost' THEN $4::text ELSE NULL END,
    won_at = CASE WHEN $3 = 'won' THEN now() ELSE NULL END,
    lost_at = CASE WHEN $3 = 'lost' THEN now() ELSE NULL END,
    forecast_category = CASE WHEN $3 IN ('won', 'lost') THEN 'closed' ELSE $5 END,
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $6 AND deleted_at IS NULL
RETURNING id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
          amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
          forecast_category, won_at, lost_at, custom_fields, version, created_at, updated_at;

-- name: SoftDeleteDeal :one
UPDATE sales.deals
SET deleted_at = now(), deleted_by = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $4 AND deleted_at IS NULL
RETURNING version;

-- name: RestoreDeal :one
UPDATE sales.deals
SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NOT NULL
RETURNING id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
          amount_minor, currency, planned_start_date, expected_close_date, position, status, lost_reason,
          forecast_category, won_at, lost_at, custom_fields, version, created_at, updated_at;

-- name: ListDealTrash :many
SELECT id, pipeline_id, stage_id, name, status, amount_minor, currency,
       owner_user_id, version, deleted_at, deleted_by, created_at, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = sqlc.arg(workspace_id) AND deal.deleted_at IS NOT NULL
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
  AND (deal.deleted_at, deal.id) < (sqlc.arg(cursor_deleted_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY deal.deleted_at DESC, deal.id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListKanbanStageDeals :many
SELECT id, pipeline_id, stage_id, name, contact_id, company_id, owner_user_id,
       amount_minor, currency, planned_start_date, expected_close_date, position, forecast_category,
       version, updated_at
FROM sales.deals deal
WHERE deal.workspace_id = sqlc.arg(workspace_id)
  AND deal.pipeline_id = sqlc.arg(pipeline_id)
  AND deal.stage_id = sqlc.arg(stage_id)
  AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
  AND deal.status = 'open' AND deal.deleted_at IS NULL
  AND (deal.position, deal.id) > (sqlc.arg(after_position)::integer, sqlc.arg(after_id)::uuid)
ORDER BY deal.position, deal.id
LIMIT sqlc.arg(page_limit);

-- name: ListDealStageHistoryAdvanced :many
SELECT id, deal_id, from_stage_id, to_stage_id, changed_by, changed_at
FROM sales.deal_stage_history
WHERE workspace_id = sqlc.arg(workspace_id) AND deal_id = sqlc.arg(deal_id)
  AND (changed_at, id) < (sqlc.arg(cursor_changed_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY changed_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListDealLineItems :many
SELECT id, deal_id, name, quantity, unit_price_minor, currency, position, version
FROM sales.deal_line_items
WHERE workspace_id = $1 AND deal_id = $2
ORDER BY position, id;

-- name: CreateDealLineItem :one
INSERT INTO sales.deal_line_items (
  workspace_id, id, deal_id, name, quantity, unit_price_minor, currency, position
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, deal_id, name, quantity, unit_price_minor, currency, position, version;

-- name: UpdateDealLineItem :one
UPDATE sales.deal_line_items
SET name = $4, quantity = $5, unit_price_minor = $6, currency = $7,
    position = $8, version = version + 1
WHERE workspace_id = $1 AND deal_id = $2 AND id = $3 AND version = $9
RETURNING id, deal_id, name, quantity, unit_price_minor, currency, position, version;

-- name: DeleteDealLineItem :one
DELETE FROM sales.deal_line_items
WHERE workspace_id = $1 AND deal_id = $2 AND id = $3 AND version = $4
RETURNING id;

-- name: BumpDealVersion :one
UPDATE sales.deals
SET version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NULL
RETURNING version;

-- name: ListDealParticipants :many
SELECT participant.contact_id, participant.role, participant.version,
       participant.created_at, participant.updated_at,
       contact.display_name, contact.email
FROM sales.deal_participants participant
JOIN customers.contacts contact
  ON contact.workspace_id = participant.workspace_id AND contact.id = participant.contact_id
WHERE participant.workspace_id = $1 AND participant.deal_id = $2
  AND contact.deleted_at IS NULL
ORDER BY contact.display_name, participant.contact_id;

-- name: UpsertDealParticipant :one
INSERT INTO sales.deal_participants (workspace_id, deal_id, contact_id, role)
VALUES ($1, $2, $3, $4)
ON CONFLICT (workspace_id, deal_id, contact_id) DO UPDATE
SET role = EXCLUDED.role, version = sales.deal_participants.version + 1, updated_at = now()
RETURNING contact_id, role, version, created_at, updated_at;

-- name: DeleteDealParticipant :one
DELETE FROM sales.deal_participants
WHERE workspace_id = $1 AND deal_id = $2 AND contact_id = $3 AND version = $4
RETURNING contact_id;

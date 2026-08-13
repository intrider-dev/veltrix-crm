-- name: GetSearchOutboxEvent :one
SELECT schema_version, aggregate_type, aggregate_id
FROM platform.outbox_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(event_id);

-- name: GetContactSearchSource :one
SELECT display_name AS title, email AS subtitle,
       concat_ws(' ', display_name, email, phone) AS searchable_text,
       version
FROM customers.contacts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(entity_id)
  AND deleted_at IS NULL;

-- name: GetCompanySearchSource :one
SELECT name AS title, domain AS subtitle,
       concat_ws(' ', name, domain, industry) AS searchable_text,
       version
FROM customers.companies
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(entity_id)
  AND deleted_at IS NULL;

-- name: GetLeadSearchSource :one
SELECT name AS title, company_name AS subtitle,
       concat_ws(' ', name, email, company_name) AS searchable_text,
       version
FROM sales.leads
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(entity_id)
  AND deleted_at IS NULL;

-- name: GetDealSearchSource :one
SELECT name AS title, NULL::text AS subtitle, name AS searchable_text, version
FROM sales.deals
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(entity_id)
  AND deleted_at IS NULL;

-- name: GetNoteSearchSource :one
SELECT title, related_type AS subtitle,
       concat_ws(' ', title, body) AS searchable_text,
       version
FROM activities.activities
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(entity_id)
  AND activity_type = 'note'
  AND deleted_at IS NULL;

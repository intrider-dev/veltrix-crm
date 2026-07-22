-- name: ListContacts :many
SELECT c.id, c.first_name, c.last_name, c.display_name, c.email, c.phone,
       c.company_id, co.name AS company_name, c.owner_user_id, c.status, c.source,
       c.custom_fields, c.version, c.created_at, c.updated_at
FROM customers.contacts c
LEFT JOIN customers.companies co
  ON co.workspace_id = c.workspace_id AND co.id = c.company_id AND co.deleted_at IS NULL
WHERE c.workspace_id = sqlc.arg(workspace_id)
  AND c.deleted_at IS NULL
  AND (sqlc.arg(search_query)::text = '' OR c.display_name ILIKE '%' || sqlc.arg(search_query) || '%'
       OR c.email_normalized LIKE '%' || lower(sqlc.arg(search_query)) || '%')
  AND (sqlc.arg(status_filter)::text = '' OR c.status = sqlc.arg(status_filter))
  AND (c.updated_at, c.id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY c.updated_at DESC, c.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetContact :one
SELECT c.id, c.first_name, c.last_name, c.display_name, c.email, c.phone,
       c.job_title, c.company_id, co.name AS company_name, c.owner_user_id,
       c.status, c.source, c.address, c.custom_fields, c.last_contacted_at,
       c.next_activity_at, c.version, c.created_at, c.updated_at
FROM customers.contacts c
LEFT JOIN customers.companies co
  ON co.workspace_id = c.workspace_id AND co.id = c.company_id AND co.deleted_at IS NULL
WHERE c.workspace_id = $1 AND c.id = $2 AND c.deleted_at IS NULL;

-- name: CreateContact :one
INSERT INTO customers.contacts (
  workspace_id, id, first_name, last_name, display_name, email, email_normalized,
  phone, phone_normalized, job_title, company_id, owner_user_id, team_id, status, source,
  address, custom_fields
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING id, first_name, last_name, display_name, email, phone, job_title,
          company_id, owner_user_id, team_id, status, source, address, custom_fields,
          last_contacted_at, next_activity_at, version, created_at, updated_at;

-- name: UpdateContact :one
UPDATE customers.contacts
SET first_name = $3,
    last_name = $4,
    display_name = $5,
    email = $6,
    email_normalized = $7,
    phone = $8,
    phone_normalized = $9,
    job_title = $10,
    company_id = $11,
    owner_user_id = $12,
    team_id = $13,
    status = $14,
    source = $15,
    address = $16,
    custom_fields = $17,
    version = version + 1,
    updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $18 AND deleted_at IS NULL
RETURNING id, first_name, last_name, display_name, email, phone, job_title,
          company_id, owner_user_id, team_id, status, source, address, custom_fields,
          last_contacted_at, next_activity_at, version, created_at, updated_at;

-- name: SoftDeleteContact :one
UPDATE customers.contacts
SET deleted_at = now(), deleted_by = $3, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $4 AND deleted_at IS NULL
RETURNING version;

-- name: RestoreContact :one
UPDATE customers.contacts
SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NOT NULL
RETURNING version;

-- name: ListContactTags :many
SELECT t.id, t.name, t.color
FROM customers.contact_tags ct
JOIN customers.tags t ON t.workspace_id = ct.workspace_id AND t.id = ct.tag_id
WHERE ct.workspace_id = $1 AND ct.contact_id = $2
ORDER BY t.name, t.id;

-- name: CountActiveContacts :one
SELECT count(*) FROM customers.contacts
WHERE workspace_id = $1 AND deleted_at IS NULL;

-- name: ListDeletedContacts :many
SELECT id, first_name, last_name, display_name, email, phone, company_id,
       owner_user_id, status, source, version, deleted_at, deleted_by, created_at, updated_at
FROM customers.contacts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NOT NULL
  AND (deleted_at, id) < (sqlc.arg(cursor_deleted_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY deleted_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetDeletedContact :one
SELECT id, first_name, last_name, display_name, email, phone, job_title,
       company_id, owner_user_id, team_id, status, source, address, custom_fields,
       last_contacted_at, next_activity_at, version, deleted_at, deleted_by,
       created_at, updated_at
FROM customers.contacts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND deleted_at IS NOT NULL;

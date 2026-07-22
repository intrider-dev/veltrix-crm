-- Advanced customer-domain SQL lives here even where package-local typed row
-- scanners are used for JSON-heavy payloads. Keeping the statements centralized
-- makes tenant predicates and query plans reviewable.

-- name: ListCustomerTags :many
SELECT id, name, color, version, created_at, updated_at
FROM customers.tags
WHERE workspace_id = sqlc.arg(workspace_id)
ORDER BY lower(name), id;

-- name: CreateCustomerTag :one
INSERT INTO customers.tags (workspace_id, id, name, color)
VALUES (sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(name), sqlc.arg(color))
RETURNING id, name, color, version, created_at, updated_at;

-- name: UpdateCustomerTag :one
UPDATE customers.tags
SET name = sqlc.arg(name), color = sqlc.arg(color), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
RETURNING id, name, color, version, created_at, updated_at;

-- name: DeleteCustomerTag :one
DELETE FROM customers.tags
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
RETURNING id;

-- name: ListCustomFieldDefinitions :many
SELECT id, entity_type, field_key, label, value_type, validation, options,
       schema_version, version, created_at, updated_at
FROM customers.custom_field_definitions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND (sqlc.arg(entity_type)::text = '' OR entity_type = sqlc.arg(entity_type))
ORDER BY entity_type, field_key, id;

-- name: ListSavedViews :many
SELECT id, owner_user_id, entity_type, name, definition, is_shared, version,
       created_at, updated_at
FROM customers.saved_views
WHERE workspace_id = sqlc.arg(workspace_id)
  AND entity_type = sqlc.arg(entity_type)
  AND (owner_user_id = sqlc.arg(actor_user_id) OR is_shared)
ORDER BY is_shared DESC, lower(name), id;

-- name: GetImportSession :one
SELECT id, actor_user_id, entity_type, status, mapping, source_headers,
       total_rows, processed_rows, created_rows, error_rows,
       created_at, updated_at, started_at, completed_at
FROM customers.import_sessions
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(id);

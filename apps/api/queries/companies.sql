-- name: ListCompanies :many
SELECT id, name, domain, industry, status, owner_user_id, version, created_at, updated_at
FROM customers.companies
WHERE workspace_id = $1 AND deleted_at IS NULL
ORDER BY updated_at DESC, id DESC
LIMIT $2;

-- name: GetCompany :one
SELECT id, name, domain, industry, status, owner_user_id, team_id, address, custom_fields,
       version, created_at, updated_at
FROM customers.companies
WHERE workspace_id = $1 AND id = $2 AND deleted_at IS NULL;

-- name: CreateCompany :one
INSERT INTO customers.companies (
  workspace_id, id, name, domain, domain_normalized, industry, owner_user_id,
  team_id, status, address, custom_fields
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, name, domain, industry, status, owner_user_id, team_id, address,
          custom_fields, version, created_at, updated_at;

-- name: CountActiveCompanies :one
SELECT count(*) FROM customers.companies
WHERE workspace_id = $1 AND deleted_at IS NULL;

-- name: ListCompaniesPage :many
SELECT id, name, domain, industry, status, owner_user_id, team_id, address,
       custom_fields, version, created_at, updated_at
FROM customers.companies
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND (sqlc.arg(search_query)::text = '' OR name ILIKE '%' || sqlc.arg(search_query) || '%'
       OR domain_normalized LIKE '%' || lower(sqlc.arg(search_query)) || '%')
  AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter))
  AND (updated_at, id) < (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: UpdateCompany :one
UPDATE customers.companies
SET name = sqlc.arg(name),
    domain = sqlc.narg(domain),
    domain_normalized = sqlc.narg(domain_normalized),
    industry = sqlc.narg(industry),
    status = sqlc.arg(status),
    owner_user_id = sqlc.narg(owner_user_id),
    team_id = sqlc.narg(team_id),
    address = sqlc.arg(address),
    custom_fields = sqlc.arg(custom_fields),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
  AND deleted_at IS NULL
RETURNING id, name, domain, industry, status, owner_user_id, team_id, address,
          custom_fields, version, created_at, updated_at;

-- name: SoftDeleteCompany :one
UPDATE customers.companies
SET deleted_at = now(), deleted_by = sqlc.arg(deleted_by), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
  AND deleted_at IS NULL
RETURNING version;

-- name: RestoreCompany :one
UPDATE customers.companies
SET deleted_at = NULL, deleted_by = NULL, version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(version)
  AND deleted_at IS NOT NULL
RETURNING id, name, domain, industry, status, owner_user_id, team_id, address,
          custom_fields, version, created_at, updated_at;

-- name: ListDeletedCompanies :many
SELECT id, name, domain, industry, status, owner_user_id, team_id, version,
       deleted_at, deleted_by, created_at, updated_at
FROM customers.companies
WHERE workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NOT NULL
  AND (deleted_at, id) < (sqlc.arg(cursor_deleted_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY deleted_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: SetActorContext :exec
SELECT set_config('app.actor_id', $1, true);

-- name: SetTenantContext :exec
SELECT set_config('app.workspace_id', sqlc.arg(workspace_id)::text, true),
       set_config('app.request_id', sqlc.arg(request_id)::text, true);

-- name: ListUserWorkspaces :many
SELECT w.id, w.name, w.slug, w.default_locale, w.timezone, w.default_currency,
       m.role, m.role_id, role.name AS role_name, m.status, m.locale_override, m.timezone_override,
       COALESCE(ARRAY(
         SELECT permission.permission
         FROM tenancy.role_permissions permission
         WHERE permission.workspace_id = m.workspace_id AND permission.role_id = m.role_id
         ORDER BY permission.permission
       ), ARRAY[]::text[])::text[] AS permissions
FROM tenancy.memberships m
JOIN tenancy.workspaces w ON w.id = m.workspace_id
JOIN tenancy.workspace_roles role ON role.workspace_id = m.workspace_id AND role.id = m.role_id
WHERE m.user_id = $1 AND m.status = 'active'
ORDER BY w.name, w.id;

-- name: GetActiveMembership :one
SELECT workspace_id, id, user_id, role, status, locale_override, timezone_override,
       created_at, updated_at, role_id
FROM tenancy.memberships
WHERE workspace_id = $1 AND user_id = $2 AND status = 'active';

-- name: CreateWorkspace :one
INSERT INTO tenancy.workspaces (
  id, name, slug, default_locale, timezone, default_currency, supported_locales
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, name, slug, default_locale, timezone, default_currency,
          supported_locales, version, created_at, updated_at;

-- name: CreateMembership :one
INSERT INTO tenancy.memberships (workspace_id, id, user_id, role, role_id, status)
SELECT $1, $2, $3, role.base_role, role.id, 'active'
FROM tenancy.workspace_roles role
WHERE role.workspace_id = $1 AND role.role_key = $4 AND role.is_system
RETURNING workspace_id, id, user_id, role, status, locale_override,
          timezone_override, created_at, updated_at, role_id;

-- name: CountActiveOwners :one
SELECT count(*)
FROM tenancy.memberships
WHERE workspace_id = $1 AND role = 'owner' AND status = 'active';

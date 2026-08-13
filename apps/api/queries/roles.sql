-- name: ListWorkspaceRoles :many
SELECT role.id, role.role_key, role.name, role.base_role, role.is_system, role.version,
       role.created_at, role.updated_at,
       COALESCE(array_agg(permission.permission ORDER BY permission.permission)
         FILTER (WHERE permission.permission IS NOT NULL), ARRAY[]::text[])::text[] AS permissions
FROM tenancy.workspace_roles role
LEFT JOIN tenancy.role_permissions permission
  ON permission.workspace_id = role.workspace_id AND permission.role_id = role.id
WHERE role.workspace_id = $1
GROUP BY role.workspace_id, role.id
ORDER BY role.is_system DESC, role.name, role.id;

-- name: ListMembershipPermissions :many
SELECT permission.permission
FROM tenancy.memberships membership
JOIN tenancy.role_permissions permission
  ON permission.workspace_id = membership.workspace_id
 AND permission.role_id = membership.role_id
WHERE membership.workspace_id = sqlc.arg(workspace_id)
  AND membership.id = sqlc.arg(membership_id)
  AND membership.status = 'active'
ORDER BY permission.permission;

-- name: CreateWorkspaceRole :one
INSERT INTO tenancy.workspace_roles (
  workspace_id, id, role_key, name, base_role, is_system
) VALUES ($1, $2, $3, $4, $5, false)
RETURNING id, role_key, name, base_role, is_system, version, created_at, updated_at;

-- name: GetWorkspaceRoleForUpdate :one
SELECT id, role_key, name, base_role, is_system, version, created_at, updated_at
FROM tenancy.workspace_roles
WHERE workspace_id = $1 AND id = $2
FOR UPDATE;

-- name: UpdateWorkspaceRole :one
UPDATE tenancy.workspace_roles
SET name = $3, base_role = $4, version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $5 AND NOT is_system
RETURNING id, role_key, name, base_role, is_system, version, created_at, updated_at;

-- name: DeleteWorkspaceRole :execrows
DELETE FROM tenancy.workspace_roles
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND NOT is_system;

-- name: DeleteWorkspaceRolePermissions :exec
DELETE FROM tenancy.role_permissions
WHERE workspace_id = $1 AND role_id = $2;

-- name: InsertWorkspaceRolePermissions :exec
INSERT INTO tenancy.role_permissions (workspace_id, role_id, permission)
SELECT $1, $2, permission
FROM unnest(sqlc.arg(permissions)::text[]) AS permission;

-- name: AssignMembershipWorkspaceRole :one
UPDATE tenancy.memberships membership
SET role_id = role.id, role = role.base_role, updated_at = now()
FROM tenancy.workspace_roles role
WHERE membership.workspace_id = sqlc.arg(workspace_id)
  AND membership.id = sqlc.arg(membership_id)
  AND role.workspace_id = membership.workspace_id
  AND role.id = sqlc.arg(role_id)
  AND membership.role <> 'owner'
  AND role.base_role <> 'owner'
RETURNING membership.workspace_id, membership.id, membership.user_id, membership.role,
          membership.status, membership.locale_override, membership.timezone_override,
          membership.created_at, membership.updated_at, membership.role_id;

SET ROLE veltrix_owner;

DROP TRIGGER IF EXISTS workspace_roles_bootstrap ON tenancy.workspaces;
DROP FUNCTION IF EXISTS tenancy.bootstrap_workspace_roles_trigger();
ALTER TABLE tenancy.memberships DROP CONSTRAINT IF EXISTS memberships_role_fk;
DROP INDEX IF EXISTS tenancy.memberships_role_idx;
ALTER TABLE tenancy.memberships DROP COLUMN IF EXISTS role_id;
DROP TABLE IF EXISTS tenancy.role_permissions;
DROP TABLE IF EXISTS tenancy.workspace_roles;
DROP FUNCTION IF EXISTS tenancy.bootstrap_workspace_roles(uuid);

RESET ROLE;

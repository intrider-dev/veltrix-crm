SET ROLE veltrix_owner;
DROP POLICY IF EXISTS role_permissions_migrator_all ON tenancy.role_permissions;
DROP POLICY IF EXISTS workspace_roles_migrator_all ON tenancy.workspace_roles;
RESET ROLE;

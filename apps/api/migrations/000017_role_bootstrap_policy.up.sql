SET ROLE veltrix_owner;

-- SECURITY DEFINER workspace bootstrap runs as this NOLOGIN owner role. FORCE
-- RLS therefore needs an explicit owner-only policy; application roles are not
-- included and continue through their actor-aware policies.
CREATE POLICY workspace_roles_migrator_all ON tenancy.workspace_roles
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);
CREATE POLICY role_permissions_migrator_all ON tenancy.role_permissions
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);

RESET ROLE;

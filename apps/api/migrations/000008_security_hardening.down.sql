SET ROLE veltrix_owner;

-- Keep the corrected policy on rollback: reintroducing the ambiguous
-- correlated identifier would knowingly restore a tenant-access defect.
DROP POLICY IF EXISTS workspace_read ON tenancy.workspaces;
CREATE POLICY workspace_read ON tenancy.workspaces
  FOR SELECT TO veltrix_app
  USING (
    id = security.current_workspace_id()
    OR EXISTS (
      SELECT 1
      FROM tenancy.memberships AS membership
      WHERE membership.workspace_id = tenancy.workspaces.id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
    )
  );

RESET ROLE;

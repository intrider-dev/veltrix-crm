SET ROLE veltrix_owner;

-- Qualify the outer workspace identifier. The unqualified `id` in the
-- original correlated subquery resolved to memberships.id, preventing a user
-- from discovering workspaces they legitimately belong to before selecting
-- an active workspace.
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

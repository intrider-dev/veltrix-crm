SET ROLE veltrix_owner;

ALTER POLICY activity_visible_select ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    visibility_scope = 'workspace' OR created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR (
      visibility_scope = 'department'
      AND EXISTS (
        SELECT 1 FROM tenancy.memberships membership
        JOIN tenancy.team_memberships department_member
          ON department_member.workspace_id = membership.workspace_id
         AND department_member.membership_id = membership.id
        WHERE membership.workspace_id = activities.workspace_id
          AND membership.user_id = security.current_actor_id()
          AND membership.status = 'active'
          AND department_member.team_id = activities.scope_department_id
      )
    )
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner', 'admin')
    )
  )
);

ALTER POLICY activity_visible_update ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner', 'admin')
    )
  )
);

DROP FUNCTION IF EXISTS security.activity_responsible_allows(uuid, uuid, uuid);
DROP FUNCTION IF EXISTS security.activity_assignment_allows(uuid, uuid, uuid);

DROP POLICY IF EXISTS task_assignment_owner_team_memberships_select ON tenancy.team_memberships;
DROP POLICY IF EXISTS task_assignment_owner_memberships_select ON tenancy.memberships;

DROP TABLE IF EXISTS activities.activity_assignments;
DROP TABLE IF EXISTS sales.deal_assignments;
DROP TABLE IF EXISTS sales.lead_assignments;

RESET ROLE;

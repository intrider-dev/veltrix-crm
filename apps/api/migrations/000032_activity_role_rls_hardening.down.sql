SET ROLE veltrix_owner;

DROP TRIGGER IF EXISTS activities_guard_assignee_update ON activities.activities;
DROP FUNCTION IF EXISTS security.guard_activity_update();

ALTER POLICY activity_visible_select ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    visibility_scope = 'workspace'
    OR created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR security.activity_assignment_allows(workspace_id, id, security.current_actor_id())
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
        AND membership.role IN ('owner','admin')
    )
  )
);

ALTER POLICY activity_visible_update ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR security.activity_responsible_allows(workspace_id, id, security.current_actor_id())
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner','admin')
    )
  )
);

ALTER POLICY activity_visible_delete ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner','admin')
    )
  )
);

ALTER POLICY comment_author_update ON activities.comments USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = comments.workspace_id AND activity.id = comments.activity_id AND activity.deleted_at IS NULL)
  AND (
    author_user_id = security.current_actor_id()
    OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = comments.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))
  )
);
ALTER POLICY comment_author_delete ON activities.comments USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = comments.workspace_id AND activity.id = comments.activity_id AND activity.deleted_at IS NULL)
  AND (
    author_user_id = security.current_actor_id()
    OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = comments.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))
  )
);

ALTER POLICY reminder_visible_select ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = reminders.workspace_id AND activity.id = reminders.activity_id AND activity.deleted_at IS NULL AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))))
);
ALTER POLICY reminder_visible_insert ON activities.reminders WITH CHECK (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = reminders.workspace_id AND activity.id = reminders.activity_id AND activity.deleted_at IS NULL AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))))
);
ALTER POLICY reminder_visible_update ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = reminders.workspace_id AND activity.id = reminders.activity_id AND activity.deleted_at IS NULL AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))))
);
ALTER POLICY reminder_visible_delete ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = reminders.workspace_id AND activity.id = reminders.activity_id AND activity.deleted_at IS NULL AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner','admin'))))
);

DROP FUNCTION IF EXISTS security.current_actor_has_system_role(uuid, text[]);

RESET ROLE;

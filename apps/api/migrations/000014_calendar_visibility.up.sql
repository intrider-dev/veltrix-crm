SET ROLE veltrix_owner;

ALTER TABLE notifications.sse_events DROP CONSTRAINT sse_events_targeted_type_check;
ALTER TABLE notifications.sse_events ADD CONSTRAINT sse_events_targeted_type_check
  CHECK (
    recipient_user_id IS NOT NULL
    OR NOT (
      event_type = 'notification.created'
      OR event_type LIKE 'chat.%'
      OR event_type LIKE 'call.%'
      OR event_type LIKE 'mail.%'
      OR event_type LIKE 'activities.private.%'
    )
  );

ALTER TABLE activities.activities
  ADD COLUMN visibility_scope text NOT NULL DEFAULT 'workspace'
    CHECK (visibility_scope IN ('workspace', 'department', 'user')),
  ADD COLUMN scope_department_id uuid,
  ADD COLUMN scope_user_id uuid,
  ADD CONSTRAINT activities_scope_shape_check CHECK (
    (visibility_scope = 'workspace' AND scope_department_id IS NULL AND scope_user_id IS NULL)
    OR (visibility_scope = 'department' AND scope_department_id IS NOT NULL AND scope_user_id IS NULL)
    OR (visibility_scope = 'user' AND scope_department_id IS NULL AND scope_user_id IS NOT NULL)
  ),
  ADD CONSTRAINT activities_scope_department_fk
    FOREIGN KEY (workspace_id, scope_department_id)
    REFERENCES tenancy.teams(workspace_id, id),
  ADD CONSTRAINT activities_scope_user_fk
    FOREIGN KEY (workspace_id, scope_user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id);

CREATE INDEX activities_visibility_idx
  ON activities.activities (workspace_id, visibility_scope, scope_department_id, scope_user_id, occurred_at, id)
  WHERE deleted_at IS NULL;

DROP POLICY tenant_scope ON activities.activities;

CREATE POLICY activity_visible_select ON activities.activities
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND (
      visibility_scope = 'workspace'
      OR created_by = security.current_actor_id()
      OR assignee_user_id = security.current_actor_id()
      OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
      OR (
        visibility_scope = 'department'
        AND EXISTS (
          SELECT 1
          FROM tenancy.memberships membership
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

CREATE POLICY activity_visible_insert ON activities.activities
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND created_by = security.current_actor_id()
    AND (
      visibility_scope = 'workspace'
      OR (
        visibility_scope = 'user'
        AND EXISTS (
          SELECT 1 FROM tenancy.memberships membership
          WHERE membership.workspace_id = activities.workspace_id
            AND membership.user_id = activities.scope_user_id
            AND membership.status = 'active'
        )
      )
      OR (
        visibility_scope = 'department'
        AND EXISTS (
          SELECT 1 FROM tenancy.teams department
          WHERE department.workspace_id = activities.workspace_id
            AND department.id = activities.scope_department_id
        )
      )
    )
  );

CREATE POLICY activity_visible_update ON activities.activities
  FOR UPDATE TO veltrix_app
  USING (
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
  )
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND (
      visibility_scope = 'workspace'
      OR (
        visibility_scope = 'user'
        AND EXISTS (
          SELECT 1 FROM tenancy.memberships membership
          WHERE membership.workspace_id = activities.workspace_id
            AND membership.user_id = activities.scope_user_id
            AND membership.status = 'active'
        )
      )
      OR (
        visibility_scope = 'department'
        AND EXISTS (
          SELECT 1 FROM tenancy.teams department
          WHERE department.workspace_id = activities.workspace_id
            AND department.id = activities.scope_department_id
        )
      )
    )
  );

CREATE POLICY activity_visible_delete ON activities.activities
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND (
      created_by = security.current_actor_id()
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

DROP POLICY tenant_scope ON activities.comments;
CREATE POLICY comment_visible_select ON activities.comments
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = comments.workspace_id
        AND activity.id = comments.activity_id
        AND activity.deleted_at IS NULL
    )
  );
CREATE POLICY comment_visible_insert ON activities.comments
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND author_user_id = security.current_actor_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = comments.workspace_id
        AND activity.id = comments.activity_id
        AND activity.deleted_at IS NULL
    )
  );
CREATE POLICY comment_author_update ON activities.comments
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = comments.workspace_id AND activity.id = comments.activity_id AND activity.deleted_at IS NULL)
    AND (
      author_user_id = security.current_actor_id()
      OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = comments.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin'))
    )
  )
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY comment_author_delete ON activities.comments
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (SELECT 1 FROM activities.activities activity WHERE activity.workspace_id = comments.workspace_id AND activity.id = comments.activity_id AND activity.deleted_at IS NULL)
    AND (
      author_user_id = security.current_actor_id()
      OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = comments.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin'))
    )
  );

DROP POLICY tenant_scope ON activities.reminders;
CREATE POLICY reminder_visible_select ON activities.reminders
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = reminders.workspace_id
        AND activity.id = reminders.activity_id
        AND activity.deleted_at IS NULL
        AND (
          reminders.recipient_user_id = security.current_actor_id()
          OR activity.created_by = security.current_actor_id()
          OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin'))
        )
    )
  );
CREATE POLICY reminder_visible_insert ON activities.reminders
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = reminders.workspace_id
        AND activity.id = reminders.activity_id
        AND activity.deleted_at IS NULL
        AND (
          reminders.recipient_user_id = security.current_actor_id()
          OR activity.created_by = security.current_actor_id()
          OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin'))
        )
    )
  );
CREATE POLICY reminder_visible_update ON activities.reminders
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = reminders.workspace_id
        AND activity.id = reminders.activity_id
        AND activity.deleted_at IS NULL
        AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin')))
    )
  )
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY reminder_visible_delete ON activities.reminders
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = reminders.workspace_id
        AND activity.id = reminders.activity_id
        AND activity.deleted_at IS NULL
        AND (reminders.recipient_user_id = security.current_actor_id() OR activity.created_by = security.current_actor_id() OR EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = reminders.workspace_id AND membership.user_id = security.current_actor_id() AND membership.status = 'active' AND membership.role IN ('owner', 'admin')))
    )
  );

RESET ROLE;

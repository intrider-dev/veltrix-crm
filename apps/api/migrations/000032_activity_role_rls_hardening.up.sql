SET ROLE veltrix_owner;

-- Compatibility base_role values are not authorization identities. Only the
-- immutable system owner/admin role rows may receive administrative RLS bypass.
CREATE OR REPLACE FUNCTION security.current_actor_has_system_role(
  target_workspace_id uuid,
  allowed_role_keys text[]
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT target_workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1
      FROM tenancy.memberships membership
      JOIN tenancy.workspace_roles role
        ON role.workspace_id = membership.workspace_id
       AND role.id = membership.role_id
      WHERE membership.workspace_id = target_workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND role.is_system
        AND role.role_key = ANY(allowed_role_keys)
    )
$function$;
REVOKE ALL ON FUNCTION security.current_actor_has_system_role(uuid, text[]) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.current_actor_has_system_role(uuid, text[]) TO veltrix_app;

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
    OR security.current_actor_has_system_role(workspace_id, ARRAY['owner','admin'])
  )
);

ALTER POLICY activity_visible_update ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR security.activity_responsible_allows(workspace_id, id, security.current_actor_id())
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR security.current_actor_has_system_role(workspace_id, ARRAY['owner','admin'])
  )
);

ALTER POLICY activity_visible_delete ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR security.current_actor_has_system_role(workspace_id, ARRAY['owner','admin'])
  )
);

ALTER POLICY comment_author_update ON activities.comments USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = comments.workspace_id
      AND activity.id = comments.activity_id
      AND activity.deleted_at IS NULL
  )
  AND (
    author_user_id = security.current_actor_id()
    OR security.current_actor_has_system_role(workspace_id, ARRAY['owner','admin'])
  )
);

ALTER POLICY comment_author_delete ON activities.comments USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = comments.workspace_id
      AND activity.id = comments.activity_id
      AND activity.deleted_at IS NULL
  )
  AND (
    author_user_id = security.current_actor_id()
    OR security.current_actor_has_system_role(workspace_id, ARRAY['owner','admin'])
  )
);

ALTER POLICY reminder_visible_select ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = reminders.workspace_id
      AND activity.id = reminders.activity_id
      AND activity.deleted_at IS NULL
      AND (
        reminders.recipient_user_id = security.current_actor_id()
        OR activity.created_by = security.current_actor_id()
        OR security.current_actor_has_system_role(reminders.workspace_id, ARRAY['owner','admin'])
      )
  )
);

ALTER POLICY reminder_visible_insert ON activities.reminders WITH CHECK (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = reminders.workspace_id
      AND activity.id = reminders.activity_id
      AND activity.deleted_at IS NULL
      AND (
        reminders.recipient_user_id = security.current_actor_id()
        OR activity.created_by = security.current_actor_id()
        OR security.current_actor_has_system_role(reminders.workspace_id, ARRAY['owner','admin'])
      )
  )
);

ALTER POLICY reminder_visible_update ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = reminders.workspace_id
      AND activity.id = reminders.activity_id
      AND activity.deleted_at IS NULL
      AND (
        reminders.recipient_user_id = security.current_actor_id()
        OR activity.created_by = security.current_actor_id()
        OR security.current_actor_has_system_role(reminders.workspace_id, ARRAY['owner','admin'])
      )
  )
);

ALTER POLICY reminder_visible_delete ON activities.reminders USING (
  workspace_id = security.current_workspace_id()
  AND EXISTS (
    SELECT 1 FROM activities.activities activity
    WHERE activity.workspace_id = reminders.workspace_id
      AND activity.id = reminders.activity_id
      AND activity.deleted_at IS NULL
      AND (
        reminders.recipient_user_id = security.current_actor_id()
        OR activity.created_by = security.current_actor_id()
        OR security.current_actor_has_system_role(reminders.workspace_id, ARRAY['owner','admin'])
      )
  )
);

-- Assignees may complete/cancel work but must not use the broad UPDATE RLS
-- visibility policy to rewrite ownership, audience, content, or relationships.
CREATE OR REPLACE FUNCTION security.guard_activity_update()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
  IF session_user = 'veltrix_app'
     AND OLD.created_by <> security.current_actor_id()
     AND NOT security.current_actor_has_system_role(OLD.workspace_id, ARRAY['owner','admin'])
     AND (
       to_jsonb(NEW) - ARRAY['status','completed_at','version','updated_at']::text[]
       IS DISTINCT FROM
       to_jsonb(OLD) - ARRAY['status','completed_at','version','updated_at']::text[]
     ) THEN
    RAISE EXCEPTION 'activity assignee may only change completion state'
      USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION security.guard_activity_update() FROM PUBLIC;

DROP TRIGGER IF EXISTS activities_guard_assignee_update ON activities.activities;
CREATE TRIGGER activities_guard_assignee_update
  BEFORE UPDATE ON activities.activities
  FOR EACH ROW EXECUTE FUNCTION security.guard_activity_update();

RESET ROLE;

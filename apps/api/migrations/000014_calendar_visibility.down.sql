SET ROLE veltrix_owner;

ALTER TABLE notifications.sse_events DROP CONSTRAINT sse_events_targeted_type_check;
ALTER TABLE notifications.sse_events ADD CONSTRAINT sse_events_targeted_type_check
  CHECK (
    recipient_user_id IS NOT NULL
    OR NOT (event_type = 'notification.created' OR event_type LIKE 'chat.%' OR event_type LIKE 'call.%' OR event_type LIKE 'mail.%')
  );

DROP POLICY reminder_visible_delete ON activities.reminders;
DROP POLICY reminder_visible_update ON activities.reminders;
DROP POLICY reminder_visible_insert ON activities.reminders;
DROP POLICY reminder_visible_select ON activities.reminders;
CREATE POLICY tenant_scope ON activities.reminders
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

DROP POLICY comment_author_delete ON activities.comments;
DROP POLICY comment_author_update ON activities.comments;
DROP POLICY comment_visible_insert ON activities.comments;
DROP POLICY comment_visible_select ON activities.comments;
CREATE POLICY tenant_scope ON activities.comments
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

DROP POLICY activity_visible_delete ON activities.activities;
DROP POLICY activity_visible_update ON activities.activities;
DROP POLICY activity_visible_insert ON activities.activities;
DROP POLICY activity_visible_select ON activities.activities;
CREATE POLICY tenant_scope ON activities.activities
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

DROP INDEX activities.activities_visibility_idx;
ALTER TABLE activities.activities
  DROP CONSTRAINT activities_scope_user_fk,
  DROP CONSTRAINT activities_scope_department_fk,
  DROP CONSTRAINT activities_scope_shape_check,
  DROP COLUMN scope_user_id,
  DROP COLUMN scope_department_id,
  DROP COLUMN visibility_scope;

RESET ROLE;

SET ROLE veltrix_owner;

ALTER TABLE files.attachments DROP CONSTRAINT IF EXISTS attachments_entity_type_check;
ALTER TABLE files.attachments ADD CONSTRAINT attachments_entity_type_check
  CHECK (entity_type IN ('contact', 'company', 'deal', 'activity', 'project', 'import'));
DROP SCHEMA IF EXISTS collaboration CASCADE;
ALTER TABLE notifications.sse_events DROP CONSTRAINT IF EXISTS sse_events_targeted_type_check;
ALTER TABLE notifications.sse_events ADD CONSTRAINT sse_events_targeted_type_check
  CHECK (event_type <> 'notification.created' OR recipient_user_id IS NOT NULL);
DROP POLICY tenant_scope ON notifications.sse_events;
CREATE POLICY tenant_scope ON notifications.sse_events
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

RESET ROLE;

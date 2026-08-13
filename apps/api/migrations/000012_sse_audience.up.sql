SET ROLE veltrix_owner;

ALTER TABLE notifications.sse_events
  ADD COLUMN recipient_user_id uuid;

UPDATE notifications.sse_events
SET recipient_user_id = (data->>'recipientUserId')::uuid
WHERE event_type = 'notification.created'
  AND data->>'recipientUserId' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

UPDATE notifications.sse_events
SET recipient_user_id = (data->>'recipientId')::uuid
WHERE event_type = 'notification.created'
  AND recipient_user_id IS NULL
  AND data->>'recipientId' ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

DELETE FROM notifications.sse_events
WHERE event_type = 'notification.created' AND recipient_user_id IS NULL;

ALTER TABLE notifications.sse_events
  ADD CONSTRAINT sse_events_recipient_fk
    FOREIGN KEY (workspace_id, recipient_user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE,
  ADD CONSTRAINT sse_events_targeted_type_check
    CHECK (event_type <> 'notification.created' OR recipient_user_id IS NOT NULL);

CREATE INDEX sse_recipient_replay_idx
  ON notifications.sse_events (workspace_id, recipient_user_id, created_at, id)
  WHERE recipient_user_id IS NOT NULL;

RESET ROLE;

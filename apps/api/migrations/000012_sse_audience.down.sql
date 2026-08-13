SET ROLE veltrix_owner;

DROP INDEX IF EXISTS notifications.sse_recipient_replay_idx;
ALTER TABLE notifications.sse_events
  DROP CONSTRAINT IF EXISTS sse_events_targeted_type_check,
  DROP CONSTRAINT IF EXISTS sse_events_recipient_fk,
  DROP COLUMN IF EXISTS recipient_user_id;

RESET ROLE;

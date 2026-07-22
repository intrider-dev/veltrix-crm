SET ROLE veltrix_owner;

DROP INDEX IF EXISTS sales.leads_reporting_period_idx;
DROP INDEX IF EXISTS sales.deals_reporting_period_idx;
DROP INDEX IF EXISTS notifications.notifications_user_list_idx;
DROP INDEX IF EXISTS activities.reminders_recipient_idx;
DROP INDEX IF EXISTS activities.comments_activity_timeline_idx;
DROP INDEX IF EXISTS activities.activities_task_calendar_idx;
DROP INDEX IF EXISTS activities.activities_calendar_idx;
DROP INDEX IF EXISTS activities.activities_feed_idx;

ALTER TABLE reporting.dashboard_preferences
  DROP COLUMN IF EXISTS timezone,
  DROP COLUMN IF EXISTS period_days;

ALTER TABLE notifications.notifications
  DROP COLUMN IF EXISTS email_updated_at,
  DROP COLUMN IF EXISTS email_state,
  DROP COLUMN IF EXISTS version;

ALTER TABLE activities.reminders
  DROP CONSTRAINT IF EXISTS reminder_terminal_state,
  DROP COLUMN IF EXISTS version,
  DROP COLUMN IF EXISTS cancelled_at,
  DROP COLUMN IF EXISTS channel;

ALTER TABLE activities.comments
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS version;

ALTER TABLE activities.activities
  DROP CONSTRAINT IF EXISTS activities_location_length,
  DROP CONSTRAINT IF EXISTS activities_ends_after_start,
  DROP COLUMN IF EXISTS location,
  DROP COLUMN IF EXISTS ends_at;

RESET ROLE;

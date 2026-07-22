SET ROLE veltrix_owner;

-- Calls and meetings use occurred_at as their UTC start. Keeping a single
-- start column avoids divergent ordering between feeds, timelines, and the
-- calendar while these optional columns capture the remaining calendar data.
ALTER TABLE activities.activities
  ADD COLUMN ends_at timestamptz,
  ADD COLUMN location text,
  ADD CONSTRAINT activities_ends_after_start CHECK (
    ends_at IS NULL OR ends_at >= occurred_at
  ),
  ADD CONSTRAINT activities_location_length CHECK (
    location IS NULL OR char_length(location) <= 500
  );

ALTER TABLE activities.comments
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  ADD COLUMN deleted_at timestamptz;

ALTER TABLE activities.reminders
  ADD COLUMN channel text NOT NULL DEFAULT 'in_app'
    CHECK (channel IN ('in_app', 'email', 'both')),
  ADD COLUMN cancelled_at timestamptz,
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  ADD CONSTRAINT reminder_terminal_state CHECK (
    delivered_at IS NULL OR cancelled_at IS NULL
  );

ALTER TABLE notifications.notifications
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  ADD COLUMN email_state text NOT NULL DEFAULT 'not_requested'
    CHECK (email_state IN ('not_requested', 'queued', 'sent', 'failed')),
  ADD COLUMN email_updated_at timestamptz;

ALTER TABLE reporting.dashboard_preferences
  ADD COLUMN period_days smallint NOT NULL DEFAULT 30
    CHECK (period_days IN (7, 30, 90, 365)),
  ADD COLUMN timezone text
    CHECK (timezone IS NULL OR char_length(timezone) BETWEEN 1 AND 80);

CREATE INDEX activities_feed_idx
  ON activities.activities (workspace_id, occurred_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX activities_calendar_idx
  ON activities.activities (workspace_id, occurred_at, id)
  WHERE deleted_at IS NULL AND activity_type IN ('call', 'meeting');
CREATE INDEX activities_task_calendar_idx
  ON activities.activities (workspace_id, due_at, id)
  WHERE deleted_at IS NULL AND activity_type = 'task' AND due_at IS NOT NULL;
CREATE INDEX comments_activity_timeline_idx
  ON activities.comments (workspace_id, activity_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX reminders_recipient_idx
  ON activities.reminders (workspace_id, recipient_user_id, remind_at, id)
  WHERE delivered_at IS NULL AND cancelled_at IS NULL;
CREATE INDEX notifications_user_list_idx
  ON notifications.notifications
    (workspace_id, recipient_user_id, created_at DESC, id DESC);
CREATE INDEX deals_reporting_period_idx
  ON sales.deals (workspace_id, updated_at, status, stage_id, owner_user_id)
  WHERE deleted_at IS NULL;
CREATE INDEX leads_reporting_period_idx
  ON sales.leads (workspace_id, created_at, source, status)
  WHERE deleted_at IS NULL;

RESET ROLE;

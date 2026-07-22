SET ROLE veltrix_owner;

-- These ledgers are the cross-process idempotency fences for the two
-- database-driven trigger producers. They contain no customer payload.
CREATE TABLE automation.schedule_ticks (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  bucket_start timestamptz NOT NULL,
  event_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, bucket_start),
  UNIQUE (workspace_id, event_id),
  CHECK (bucket_start = date_trunc('hour', bucket_start))
);

CREATE TABLE automation.overdue_task_ticks (
  workspace_id uuid NOT NULL,
  activity_id uuid NOT NULL,
  activity_version bigint NOT NULL CHECK (activity_version > 0),
  event_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, activity_id, activity_version),
  UNIQUE (workspace_id, event_id),
  FOREIGN KEY (workspace_id, activity_id)
    REFERENCES activities.activities(workspace_id, id) ON DELETE CASCADE
);

ALTER TABLE automation.schedule_ticks ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation.schedule_ticks FORCE ROW LEVEL SECURITY;
ALTER TABLE automation.overdue_task_ticks ENABLE ROW LEVEL SECURITY;
ALTER TABLE automation.overdue_task_ticks FORCE ROW LEVEL SECURITY;

CREATE POLICY producer_owner_schedule_ticks ON automation.schedule_ticks
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);
CREATE POLICY producer_owner_overdue_ticks ON automation.overdue_task_ticks
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);
CREATE POLICY producer_owner_activities ON activities.activities
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY producer_owner_workspaces ON tenancy.workspaces
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY producer_owner_outbox ON platform.outbox_events
  FOR INSERT TO veltrix_owner WITH CHECK (true);

-- Runtime receives EXECUTE only. The SECURITY DEFINER boundary exposes one
-- bounded operation instead of granting the dispatcher cross-tenant access to
-- CRM tables. Concurrent integrated/standalone workers are deduplicated by the
-- ledger primary keys.
CREATE FUNCTION automation.enqueue_due_trigger_events()
RETURNS TABLE (scheduled_count integer, overdue_count integer)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, automation, activities, tenancy, platform
AS $function$
DECLARE
  scheduled_events integer := 0;
  overdue_events integer := 0;
BEGIN
  WITH inserted AS (
    INSERT INTO automation.schedule_ticks (workspace_id, bucket_start, event_id)
    SELECT workspace.id, date_trunc('hour', clock_timestamp()), uuidv7()
    FROM tenancy.workspaces AS workspace
    ON CONFLICT (workspace_id, bucket_start) DO NOTHING
    RETURNING workspace_id, bucket_start, event_id
  ), published AS (
    INSERT INTO platform.outbox_events (
      workspace_id, id, event_type, aggregate_type, aggregate_id,
      correlation_id, payload
    )
    SELECT inserted.workspace_id, inserted.event_id, 'automation.scheduled',
           'workspace', inserted.workspace_id, inserted.event_id,
           jsonb_build_object('scheduled_at', inserted.bucket_start)
    FROM inserted
    RETURNING 1
  )
  SELECT count(*)::integer INTO scheduled_events FROM published;

  WITH due AS (
    SELECT activity.workspace_id, activity.id, activity.version
    FROM activities.activities AS activity
    WHERE activity.activity_type = 'task'
      AND activity.status = 'open'
      AND activity.due_at IS NOT NULL
      AND activity.due_at <= clock_timestamp()
      AND NOT EXISTS (
        SELECT 1
        FROM automation.overdue_task_ticks AS tick
        WHERE tick.workspace_id = activity.workspace_id
          AND tick.activity_id = activity.id
          AND tick.activity_version = activity.version
      )
    ORDER BY activity.due_at, activity.workspace_id, activity.id
    LIMIT 1000
  ), inserted AS (
    INSERT INTO automation.overdue_task_ticks (
      workspace_id, activity_id, activity_version, event_id
    )
    SELECT due.workspace_id, due.id, due.version, uuidv7()
    FROM due
    ON CONFLICT (workspace_id, activity_id, activity_version) DO NOTHING
    RETURNING workspace_id, activity_id, activity_version, event_id
  ), published AS (
    INSERT INTO platform.outbox_events (
      workspace_id, id, event_type, aggregate_type, aggregate_id,
      correlation_id, payload
    )
    SELECT inserted.workspace_id, inserted.event_id, 'activities.task.overdue',
           'activity', inserted.activity_id, inserted.event_id,
           jsonb_build_object(
             'activityId', inserted.activity_id,
             'type', 'task',
             'version', inserted.activity_version
           )
    FROM inserted
    RETURNING 1
  )
  SELECT count(*)::integer INTO overdue_events FROM published;

  RETURN QUERY SELECT scheduled_events, overdue_events;
END
$function$;

REVOKE ALL ON FUNCTION automation.enqueue_due_trigger_events() FROM PUBLIC;
GRANT USAGE ON SCHEMA automation TO veltrix_dispatcher;
GRANT EXECUTE ON FUNCTION automation.enqueue_due_trigger_events() TO veltrix_dispatcher;

RESET ROLE;

SET ROLE veltrix_owner;

REVOKE EXECUTE ON FUNCTION automation.enqueue_due_trigger_events() FROM veltrix_dispatcher;
DROP FUNCTION IF EXISTS automation.enqueue_due_trigger_events();
DROP POLICY IF EXISTS producer_owner_outbox ON platform.outbox_events;
DROP POLICY IF EXISTS producer_owner_workspaces ON tenancy.workspaces;
DROP POLICY IF EXISTS producer_owner_activities ON activities.activities;
DROP TABLE IF EXISTS automation.overdue_task_ticks;
DROP TABLE IF EXISTS automation.schedule_ticks;

RESET ROLE;

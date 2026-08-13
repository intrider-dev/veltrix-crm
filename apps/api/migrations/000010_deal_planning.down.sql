SET ROLE veltrix_owner;

DROP INDEX IF EXISTS sales.deals_planning_window_idx;
ALTER TABLE sales.deals DROP COLUMN IF EXISTS planned_start_date;

RESET ROLE;

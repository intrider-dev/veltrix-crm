SET ROLE veltrix_owner;

ALTER TABLE sales.deals
  ADD COLUMN planned_start_date date;

CREATE INDEX deals_planning_window_idx
  ON sales.deals (workspace_id, planned_start_date, expected_close_date, id)
  WHERE deleted_at IS NULL;

RESET ROLE;

SET ROLE veltrix_owner;

DROP TRIGGER IF EXISTS workspace_lead_stages_bootstrap ON tenancy.workspaces;
DROP FUNCTION IF EXISTS sales.bootstrap_lead_stages_trigger();
DROP FUNCTION IF EXISTS sales.bootstrap_lead_stages(uuid);
DROP TRIGGER IF EXISTS leads_legacy_stage_projection ON sales.leads;
DROP FUNCTION IF EXISTS sales.resolve_legacy_lead_stage_id();
DROP TABLE IF EXISTS sales.lead_stage_history;
DROP INDEX IF EXISTS sales.leads_stage_list_idx;
ALTER TABLE sales.leads DROP CONSTRAINT IF EXISTS leads_stage_fk;
ALTER TABLE sales.leads DROP COLUMN IF EXISTS stage_id;
DROP TABLE IF EXISTS sales.lead_stages;

RESET ROLE;

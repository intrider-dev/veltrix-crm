SET ROLE veltrix_owner;

REVOKE ALL ON FUNCTION sales.pipeline_stage_access_allowed(uuid, uuid, text) FROM veltrix_app;
REVOKE ALL ON FUNCTION sales.lead_stage_access_allowed(uuid, uuid, text) FROM veltrix_app;
DROP FUNCTION IF EXISTS sales.pipeline_stage_access_allowed(uuid, uuid, text);
DROP FUNCTION IF EXISTS sales.lead_stage_access_allowed(uuid, uuid, text);
DROP TABLE IF EXISTS sales.pipeline_stage_role_access;
DROP TABLE IF EXISTS sales.lead_stage_role_access;

RESET ROLE;

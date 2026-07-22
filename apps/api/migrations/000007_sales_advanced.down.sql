SET ROLE veltrix_owner;

DROP TABLE IF EXISTS sales.deal_participants;

DROP INDEX IF EXISTS sales.deal_line_items_order_idx;
DROP INDEX IF EXISTS sales.deals_name_trgm_idx;
DROP INDEX IF EXISTS sales.deals_trash_idx;
DROP INDEX IF EXISTS sales.deals_filter_idx;
DROP INDEX IF EXISTS sales.leads_email_trgm_idx;
DROP INDEX IF EXISTS sales.leads_name_trgm_idx;
DROP INDEX IF EXISTS sales.leads_trash_idx;
DROP INDEX IF EXISTS sales.leads_filter_idx;

ALTER TABLE sales.deal_line_items DROP COLUMN IF EXISTS version;

ALTER TABLE sales.deals
  DROP CONSTRAINT IF EXISTS deals_outcome_shape,
  DROP CONSTRAINT IF EXISTS deals_deleted_by_fk,
  DROP COLUMN IF EXISTS deleted_by,
  DROP COLUMN IF EXISTS lost_at,
  DROP COLUMN IF EXISTS won_at,
  DROP COLUMN IF EXISTS forecast_category;

ALTER TABLE sales.leads
  DROP CONSTRAINT IF EXISTS leads_converted_deal_fk,
  DROP CONSTRAINT IF EXISTS leads_converted_company_fk,
  DROP CONSTRAINT IF EXISTS leads_converted_contact_fk,
  DROP CONSTRAINT IF EXISTS leads_deleted_by_fk,
  DROP CONSTRAINT IF EXISTS leads_team_fk,
  DROP COLUMN IF EXISTS deleted_by,
  DROP COLUMN IF EXISTS team_id,
  DROP COLUMN IF EXISTS job_title,
  DROP COLUMN IF EXISTS phone_normalized,
  DROP COLUMN IF EXISTS email_normalized,
  DROP COLUMN IF EXISTS phone;

ALTER TABLE sales.pipeline_stages DROP COLUMN IF EXISTS version;

RESET ROLE;

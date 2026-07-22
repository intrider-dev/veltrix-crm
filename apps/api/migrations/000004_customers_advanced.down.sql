SET ROLE veltrix_owner;

DROP TRIGGER IF EXISTS custom_field_value_shape ON customers.custom_field_values;
DROP FUNCTION IF EXISTS customers.validate_custom_field_value();
DROP TABLE IF EXISTS customers.import_errors;
DROP TABLE IF EXISTS customers.import_rows;
DROP TABLE IF EXISTS customers.record_merges;

DROP INDEX IF EXISTS customers.saved_views_owner_name_idx;
DROP INDEX IF EXISTS customers.saved_views_visible_idx;
DROP INDEX IF EXISTS customers.custom_field_values_value_gin_idx;
DROP INDEX IF EXISTS customers.custom_field_values_entity_idx;
DROP INDEX IF EXISTS customers.custom_field_definitions_list_idx;
DROP INDEX IF EXISTS customers.contact_tags_reverse_idx;
DROP INDEX IF EXISTS customers.contacts_trash_idx;
DROP INDEX IF EXISTS customers.companies_active_owner_idx;
DROP INDEX IF EXISTS customers.companies_trash_idx;

ALTER TABLE customers.import_sessions
  DROP COLUMN IF EXISTS completed_at,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS created_rows,
  DROP COLUMN IF EXISTS source_headers;

ALTER TABLE customers.tags
  DROP COLUMN IF EXISTS updated_at,
  DROP COLUMN IF EXISTS version;

RESET ROLE;

SET ROLE veltrix_owner;

ALTER TABLE files.attachments DROP CONSTRAINT IF EXISTS attachments_entity_type_check;
ALTER TABLE files.attachments ADD CONSTRAINT attachments_entity_type_check
  CHECK (entity_type IN ('contact', 'company', 'deal', 'activity', 'import'));

ALTER TABLE activities.activities DROP CONSTRAINT IF EXISTS activities_related_type_check;
ALTER TABLE activities.activities ADD CONSTRAINT activities_related_type_check
  CHECK (related_type IS NULL OR related_type IN ('contact', 'company', 'deal'));

DROP SCHEMA IF EXISTS projects CASCADE;

RESET ROLE;

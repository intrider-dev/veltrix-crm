SET ROLE veltrix_owner;

ALTER TABLE customers.tags
  ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

ALTER TABLE customers.import_sessions
  ADD COLUMN source_headers jsonb NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(source_headers) = 'array' AND octet_length(source_headers::text) <= 32768),
  ADD COLUMN created_rows integer NOT NULL DEFAULT 0 CHECK (created_rows >= 0),
  ADD COLUMN started_at timestamptz,
  ADD COLUMN completed_at timestamptz;

CREATE INDEX companies_trash_idx
  ON customers.companies (workspace_id, deleted_at DESC, id DESC)
  WHERE deleted_at IS NOT NULL;
CREATE INDEX companies_active_owner_idx
  ON customers.companies (workspace_id, owner_user_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;
CREATE INDEX contacts_trash_idx
  ON customers.contacts (workspace_id, deleted_at DESC, id DESC)
  WHERE deleted_at IS NOT NULL;
CREATE INDEX contact_tags_reverse_idx
  ON customers.contact_tags (workspace_id, tag_id, contact_id);
CREATE INDEX custom_field_definitions_list_idx
  ON customers.custom_field_definitions (workspace_id, entity_type, updated_at DESC, id DESC);
CREATE INDEX custom_field_values_entity_idx
  ON customers.custom_field_values (workspace_id, entity_type, entity_id, definition_id);
CREATE INDEX custom_field_values_value_gin_idx
  ON customers.custom_field_values USING gin (value jsonb_path_ops);
CREATE INDEX saved_views_visible_idx
  ON customers.saved_views (workspace_id, entity_type, is_shared, owner_user_id, updated_at DESC, id DESC);
CREATE UNIQUE INDEX saved_views_owner_name_idx
  ON customers.saved_views (workspace_id, owner_user_id, entity_type, lower(name));

CREATE TABLE customers.record_merges (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  entity_type text NOT NULL CHECK (entity_type IN ('contact', 'company')),
  source_id uuid NOT NULL,
  target_id uuid NOT NULL,
  actor_user_id uuid NOT NULL,
  source_version bigint NOT NULL CHECK (source_version > 0),
  target_version bigint NOT NULL CHECK (target_version > 0),
  merged_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, entity_type, source_id),
  CHECK (source_id <> target_id),
  FOREIGN KEY (workspace_id, actor_user_id) REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX record_merges_target_idx
  ON customers.record_merges (workspace_id, entity_type, target_id, merged_at DESC);

CREATE TABLE customers.import_rows (
  workspace_id uuid NOT NULL,
  import_session_id uuid NOT NULL,
  row_number integer NOT NULL CHECK (row_number > 1),
  source_values jsonb NOT NULL
    CHECK (jsonb_typeof(source_values) = 'object' AND octet_length(source_values::text) <= 65536),
  state text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'created', 'failed')),
  created_entity_id uuid,
  processed_at timestamptz,
  PRIMARY KEY (workspace_id, import_session_id, row_number),
  FOREIGN KEY (workspace_id, import_session_id)
    REFERENCES customers.import_sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX import_rows_pending_idx
  ON customers.import_rows (workspace_id, import_session_id, row_number)
  WHERE state = 'pending';

CREATE TABLE customers.import_errors (
  workspace_id uuid NOT NULL,
  import_session_id uuid NOT NULL,
  row_number integer NOT NULL CHECK (row_number > 1),
  error_code text NOT NULL CHECK (char_length(error_code) BETWEEN 1 AND 160),
  field_key text CHECK (field_key IS NULL OR char_length(field_key) <= 160),
  safe_value text CHECK (safe_value IS NULL OR char_length(safe_value) <= 512),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, import_session_id, row_number, error_code),
  FOREIGN KEY (workspace_id, import_session_id)
    REFERENCES customers.import_sessions(workspace_id, id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION customers.validate_custom_field_value()
RETURNS trigger
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog, customers
AS $function$
DECLARE
  definition customers.custom_field_definitions%ROWTYPE;
BEGIN
  SELECT * INTO definition
  FROM customers.custom_field_definitions
  WHERE workspace_id = NEW.workspace_id AND id = NEW.definition_id;

  IF NOT FOUND THEN
    RAISE EXCEPTION 'custom field definition does not exist' USING ERRCODE = '23503';
  END IF;
  IF NEW.entity_type <> definition.entity_type OR NEW.schema_version <> definition.schema_version THEN
    RAISE EXCEPTION 'custom field schema mismatch' USING ERRCODE = '23514';
  END IF;
  IF (CASE definition.value_type
    WHEN 'text' THEN jsonb_typeof(NEW.value) <> 'string'
    WHEN 'number' THEN jsonb_typeof(NEW.value) <> 'number'
    WHEN 'money' THEN NOT (
      jsonb_typeof(NEW.value) = 'object'
      AND jsonb_typeof(NEW.value -> 'minor') = 'number'
      AND jsonb_typeof(NEW.value -> 'currency') = 'string'
    )
    WHEN 'date' THEN jsonb_typeof(NEW.value) <> 'string'
    WHEN 'boolean' THEN jsonb_typeof(NEW.value) <> 'boolean'
    WHEN 'single_select' THEN jsonb_typeof(NEW.value) <> 'string'
    WHEN 'multi_select' THEN jsonb_typeof(NEW.value) <> 'array'
    WHEN 'user_reference' THEN jsonb_typeof(NEW.value) <> 'string'
    ELSE true
  END) THEN
    RAISE EXCEPTION 'custom field value type mismatch' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END
$function$;

CREATE TRIGGER custom_field_value_shape
BEFORE INSERT OR UPDATE ON customers.custom_field_values
FOR EACH ROW EXECUTE FUNCTION customers.validate_custom_field_value();

ALTER TABLE customers.record_merges ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.record_merges FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON customers.record_merges
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

ALTER TABLE customers.import_rows ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.import_rows FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON customers.import_rows
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

ALTER TABLE customers.import_errors ENABLE ROW LEVEL SECURITY;
ALTER TABLE customers.import_errors FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON customers.import_errors
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

GRANT SELECT, INSERT ON customers.record_merges TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON customers.import_rows, customers.import_errors TO veltrix_app;

RESET ROLE;

SET ROLE veltrix_owner;

ALTER TABLE sales.leads
  ADD COLUMN planned_start_date date,
  ADD COLUMN expected_close_date date;

ALTER TABLE customers.custom_field_definitions
  DROP CONSTRAINT IF EXISTS custom_field_definitions_value_type_check;
ALTER TABLE customers.custom_field_definitions
  ADD CONSTRAINT custom_field_definitions_value_type_check
  CHECK (value_type IN (
    'text', 'multiline_text', 'number', 'money', 'date', 'boolean',
    'single_select', 'multi_select', 'user_reference'
  ));

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
    WHEN 'multiline_text' THEN jsonb_typeof(NEW.value) <> 'string'
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

CREATE TABLE collaboration.entity_conversations (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  entity_type text NOT NULL CHECK (entity_type IN ('lead', 'deal')),
  entity_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, entity_type, entity_id),
  UNIQUE (workspace_id, conversation_id),
  FOREIGN KEY (workspace_id, conversation_id)
    REFERENCES collaboration.conversations(workspace_id, id) ON DELETE CASCADE
);

ALTER TABLE collaboration.messages
  DROP CONSTRAINT IF EXISTS messages_message_kind_check;
ALTER TABLE collaboration.messages
  ADD CONSTRAINT messages_message_kind_check
  CHECK (message_kind IN ('text', 'system', 'file', 'voice', 'video'));

ALTER TABLE collaboration.entity_conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.entity_conversations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON collaboration.entity_conversations
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration.entity_conversations TO veltrix_app;

RESET ROLE;

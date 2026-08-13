SET ROLE veltrix_owner;

DROP TABLE IF EXISTS collaboration.entity_conversations;

ALTER TABLE collaboration.messages
  DROP CONSTRAINT IF EXISTS messages_message_kind_check;
ALTER TABLE collaboration.messages
  ADD CONSTRAINT messages_message_kind_check
  CHECK (message_kind IN ('text', 'system', 'file', 'voice'));

ALTER TABLE customers.custom_field_definitions
  DROP CONSTRAINT IF EXISTS custom_field_definitions_value_type_check;
UPDATE customers.custom_field_definitions
SET value_type = 'text', schema_version = schema_version + 1, updated_at = now()
WHERE value_type = 'multiline_text';
ALTER TABLE customers.custom_field_definitions
  ADD CONSTRAINT custom_field_definitions_value_type_check
  CHECK (value_type IN (
    'text', 'number', 'money', 'date', 'boolean',
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

ALTER TABLE sales.leads
  DROP COLUMN IF EXISTS expected_close_date,
  DROP COLUMN IF EXISTS planned_start_date;

RESET ROLE;

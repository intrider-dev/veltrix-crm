SET ROLE veltrix_owner;

-- The tables use FORCE RLS. These narrow owner-only policies exist only for
-- this transaction so the migration can repair values that predate normalized
-- lead/deal custom-field persistence without giving the runtime role a bypass.
CREATE POLICY sales_custom_field_backfill_definitions_select
  ON customers.custom_field_definitions
  FOR SELECT TO veltrix_owner USING (entity_type IN ('lead', 'deal'));
CREATE POLICY sales_custom_field_backfill_values_all
  ON customers.custom_field_values
  FOR ALL TO veltrix_owner
  USING (entity_type IN ('lead', 'deal'))
  WITH CHECK (entity_type IN ('lead', 'deal'));

INSERT INTO customers.custom_field_values (
  workspace_id, definition_id, entity_type, entity_id, value, schema_version
)
SELECT definition.workspace_id, definition.id, definition.entity_type,
       record.id, record.custom_fields -> definition.field_key, definition.schema_version
FROM customers.custom_field_definitions definition
JOIN LATERAL (
  SELECT lead.id, lead.custom_fields
  FROM sales.leads lead
  WHERE definition.entity_type = 'lead'
    AND lead.workspace_id = definition.workspace_id
  UNION ALL
  SELECT deal.id, deal.custom_fields
  FROM sales.deals deal
  WHERE definition.entity_type = 'deal'
    AND deal.workspace_id = definition.workspace_id
) record ON record.custom_fields ? definition.field_key
WHERE definition.entity_type IN ('lead', 'deal')
ON CONFLICT (workspace_id, definition_id, entity_id) DO UPDATE
SET value = EXCLUDED.value,
    entity_type = EXCLUDED.entity_type,
    schema_version = EXCLUDED.schema_version,
    updated_at = now();

DROP POLICY sales_custom_field_backfill_values_all ON customers.custom_field_values;
DROP POLICY sales_custom_field_backfill_definitions_select ON customers.custom_field_definitions;

RESET ROLE;

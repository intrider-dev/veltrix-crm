SET ROLE veltrix_owner;

CREATE TABLE sales.lead_stages (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  category text NOT NULL CHECK (category IN ('new', 'qualified', 'disqualified', 'converted')),
  color text NOT NULL DEFAULT '#64748b' CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
  position integer NOT NULL CHECK (position >= 0),
  system_key text CHECK (system_key IS NULL OR system_key IN ('new', 'qualified', 'disqualified', 'converted')),
  is_default boolean NOT NULL DEFAULT false,
  archived_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, position),
  UNIQUE (workspace_id, name),
  UNIQUE (workspace_id, system_key)
);

CREATE UNIQUE INDEX lead_stages_default_category_idx
  ON sales.lead_stages (workspace_id, category) WHERE is_default;

CREATE OR REPLACE FUNCTION sales.bootstrap_lead_stages(target_workspace_id uuid)
RETURNS void
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  INSERT INTO sales.lead_stages (
    workspace_id, id, name, category, color, position, system_key, is_default
  ) VALUES
    (target_workspace_id, pg_catalog.uuidv7(), 'New', 'new', '#64748b', 0, 'new', true),
    (target_workspace_id, pg_catalog.uuidv7(), 'Qualified', 'qualified', '#2563eb', 1, 'qualified', true),
    (target_workspace_id, pg_catalog.uuidv7(), 'Disqualified', 'disqualified', '#dc2626', 2, 'disqualified', true),
    (target_workspace_id, pg_catalog.uuidv7(), 'Converted', 'converted', '#16a34a', 3, 'converted', true)
  ON CONFLICT (workspace_id, system_key) DO NOTHING;
$function$;
REVOKE ALL ON FUNCTION sales.bootstrap_lead_stages(uuid) FROM PUBLIC;

DO $backfill$
DECLARE workspace_record record;
BEGIN
  FOR workspace_record IN SELECT id FROM tenancy.workspaces LOOP
    PERFORM sales.bootstrap_lead_stages(workspace_record.id);
  END LOOP;
END
$backfill$;

CREATE OR REPLACE FUNCTION sales.bootstrap_lead_stages_trigger()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
  PERFORM sales.bootstrap_lead_stages(NEW.id);
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION sales.bootstrap_lead_stages_trigger() FROM PUBLIC;
CREATE TRIGGER workspace_lead_stages_bootstrap
  AFTER INSERT ON tenancy.workspaces
  FOR EACH ROW EXECUTE FUNCTION sales.bootstrap_lead_stages_trigger();

ALTER TABLE sales.leads ADD COLUMN stage_id uuid;
UPDATE sales.leads lead
SET stage_id = stage.id
FROM sales.lead_stages stage
WHERE stage.workspace_id = lead.workspace_id
  AND stage.category = lead.status
  AND stage.is_default;
ALTER TABLE sales.leads
  ALTER COLUMN stage_id SET NOT NULL,
  ADD CONSTRAINT leads_stage_fk FOREIGN KEY (workspace_id, stage_id)
    REFERENCES sales.lead_stages(workspace_id, id);

CREATE INDEX leads_stage_list_idx
  ON sales.leads (workspace_id, stage_id, updated_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE sales.lead_stage_history (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  lead_id uuid NOT NULL,
  from_stage_id uuid,
  to_stage_id uuid NOT NULL,
  changed_by uuid NOT NULL,
  changed_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, lead_id) REFERENCES sales.leads(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, from_stage_id) REFERENCES sales.lead_stages(workspace_id, id),
  FOREIGN KEY (workspace_id, to_stage_id) REFERENCES sales.lead_stages(workspace_id, id),
  FOREIGN KEY (workspace_id, changed_by) REFERENCES tenancy.memberships(workspace_id, user_id)
);
CREATE INDEX lead_stage_history_timeline_idx
  ON sales.lead_stage_history (workspace_id, lead_id, changed_at DESC, id DESC);

ALTER TABLE sales.lead_stages ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.lead_stages FORCE ROW LEVEL SECURITY;
CREATE POLICY lead_stages_tenant_scope ON sales.lead_stages
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY lead_stages_migrator_all ON sales.lead_stages
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);

CREATE OR REPLACE FUNCTION sales.resolve_legacy_lead_stage_id()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE resolved_category text;
BEGIN
  IF NEW.stage_id IS NULL
     OR (TG_OP = 'UPDATE'
         AND NEW.status IS DISTINCT FROM OLD.status
         AND NEW.stage_id IS NOT DISTINCT FROM OLD.stage_id) THEN
    SELECT stage.id INTO NEW.stage_id
    FROM sales.lead_stages stage
    WHERE stage.workspace_id = NEW.workspace_id
      AND stage.category = NEW.status
      AND stage.is_default;
  ELSE
    SELECT stage.category INTO resolved_category
    FROM sales.lead_stages stage
    WHERE stage.workspace_id = NEW.workspace_id
      AND stage.id = NEW.stage_id;
    IF resolved_category IS NOT NULL THEN
      NEW.status := resolved_category;
    END IF;
  END IF;
  IF NEW.status = 'converted' AND NEW.converted_contact_id IS NULL THEN
    RAISE EXCEPTION 'lead conversion requires converted_contact_id'
      USING ERRCODE = '23514', CONSTRAINT = 'leads_conversion_reference_required';
  END IF;
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION sales.resolve_legacy_lead_stage_id() FROM PUBLIC;
CREATE TRIGGER leads_legacy_stage_projection
  BEFORE INSERT OR UPDATE OF status, stage_id ON sales.leads
  FOR EACH ROW EXECUTE FUNCTION sales.resolve_legacy_lead_stage_id();

ALTER TABLE sales.lead_stage_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.lead_stage_history FORCE ROW LEVEL SECURITY;
CREATE POLICY lead_stage_history_tenant_scope ON sales.lead_stage_history
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

GRANT SELECT, INSERT, UPDATE, DELETE ON sales.lead_stages, sales.lead_stage_history TO veltrix_app;

RESET ROLE;

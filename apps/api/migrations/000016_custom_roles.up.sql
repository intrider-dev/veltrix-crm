SET ROLE veltrix_owner;

-- The migrator assumes the NOLOGIN table-owner role. Existing tenant tables
-- use FORCE RLS, so temporarily let their owner perform this bounded backfill.
ALTER TABLE tenancy.workspaces NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenancy.memberships NO FORCE ROW LEVEL SECURITY;

CREATE TABLE tenancy.workspace_roles (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  role_key text NOT NULL CHECK (role_key ~ '^[a-z][a-z0-9_-]{1,63}$'),
  name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 120),
  base_role text NOT NULL CHECK (base_role IN ('owner', 'admin', 'manager', 'sales', 'viewer')),
  is_system boolean NOT NULL DEFAULT false,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, role_key),
  UNIQUE (workspace_id, name)
);

CREATE TABLE tenancy.role_permissions (
  workspace_id uuid NOT NULL,
  role_id uuid NOT NULL,
  permission text NOT NULL CHECK (permission ~ '^[a-z][a-z0-9_.-]{2,63}$'),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, role_id, permission),
  FOREIGN KEY (workspace_id, role_id)
    REFERENCES tenancy.workspace_roles(workspace_id, id) ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION tenancy.bootstrap_workspace_roles(target_workspace_id uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
DECLARE
  role_record record;
  granted_permissions text[];
BEGIN
  FOR role_record IN
    INSERT INTO tenancy.workspace_roles (workspace_id, id, role_key, name, base_role, is_system)
    VALUES
      (target_workspace_id, pg_catalog.uuidv7(), 'owner', 'Owner', 'owner', true),
      (target_workspace_id, pg_catalog.uuidv7(), 'admin', 'Administrator', 'admin', true),
      (target_workspace_id, pg_catalog.uuidv7(), 'manager', 'Manager', 'manager', true),
      (target_workspace_id, pg_catalog.uuidv7(), 'sales', 'Sales', 'sales', true),
      (target_workspace_id, pg_catalog.uuidv7(), 'viewer', 'Viewer', 'viewer', true)
    ON CONFLICT (workspace_id, role_key) DO NOTHING
    RETURNING id, role_key
  LOOP
    granted_permissions := CASE role_record.role_key
      WHEN 'owner' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','settings.write','members.read','members.write','roles.write']
      WHEN 'admin' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','settings.write','members.read','members.write']
      WHEN 'manager' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','members.read']
      WHEN 'sales' THEN ARRAY['records.read','records.create','records.update']
      ELSE ARRAY['records.read','reports.read']
    END;
    INSERT INTO tenancy.role_permissions (workspace_id, role_id, permission)
    SELECT target_workspace_id, role_record.id, permission
    FROM unnest(granted_permissions) AS permission;
  END LOOP;
END
$function$;

REVOKE ALL ON FUNCTION tenancy.bootstrap_workspace_roles(uuid) FROM PUBLIC;

DO $backfill$
DECLARE workspace_record record;
BEGIN
  FOR workspace_record IN SELECT id FROM tenancy.workspaces LOOP
    PERFORM tenancy.bootstrap_workspace_roles(workspace_record.id);
  END LOOP;
END
$backfill$;

-- Keep the migration backfill set-based as well as the reusable trigger path.
-- This is intentionally idempotent and makes a partially initialized workspace
-- fail closed only after every system role has been materialized.
INSERT INTO tenancy.workspace_roles (workspace_id, id, role_key, name, base_role, is_system)
SELECT workspace.id, pg_catalog.uuidv7(), definition.role_key, definition.name,
       definition.base_role, true
FROM tenancy.workspaces workspace
CROSS JOIN (VALUES
  ('owner', 'Owner', 'owner'),
  ('admin', 'Administrator', 'admin'),
  ('manager', 'Manager', 'manager'),
  ('sales', 'Sales', 'sales'),
  ('viewer', 'Viewer', 'viewer')
) AS definition(role_key, name, base_role)
ON CONFLICT (workspace_id, role_key) DO NOTHING;

INSERT INTO tenancy.role_permissions (workspace_id, role_id, permission)
SELECT role.workspace_id, role.id, permission
FROM tenancy.workspace_roles role
CROSS JOIN LATERAL unnest(CASE role.role_key
  WHEN 'owner' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','settings.write','members.read','members.write','roles.write']
  WHEN 'admin' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','settings.write','members.read','members.write']
  WHEN 'manager' THEN ARRAY['records.read','records.create','records.update','records.delete','data.export','reports.read','audit.read','members.read']
  WHEN 'sales' THEN ARRAY['records.read','records.create','records.update']
  ELSE ARRAY['records.read','reports.read']
END) AS permission
WHERE role.is_system
ON CONFLICT DO NOTHING;

ALTER TABLE tenancy.memberships ADD COLUMN role_id uuid;
UPDATE tenancy.memberships membership
SET role_id = role.id
FROM tenancy.workspace_roles role
WHERE role.workspace_id = membership.workspace_id
  AND role.role_key = membership.role;
ALTER TABLE tenancy.memberships
  ALTER COLUMN role_id SET NOT NULL,
  ADD CONSTRAINT memberships_role_fk FOREIGN KEY (workspace_id, role_id)
    REFERENCES tenancy.workspace_roles(workspace_id, id);
CREATE INDEX memberships_role_idx ON tenancy.memberships (workspace_id, role_id, status, user_id);

CREATE OR REPLACE FUNCTION tenancy.bootstrap_workspace_roles_trigger()
RETURNS trigger
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
BEGIN
  PERFORM tenancy.bootstrap_workspace_roles(NEW.id);
  RETURN NEW;
END
$function$;
REVOKE ALL ON FUNCTION tenancy.bootstrap_workspace_roles_trigger() FROM PUBLIC;
CREATE TRIGGER workspace_roles_bootstrap
  AFTER INSERT ON tenancy.workspaces
  FOR EACH ROW EXECUTE FUNCTION tenancy.bootstrap_workspace_roles_trigger();

ALTER TABLE tenancy.workspace_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.workspace_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_roles_member_select ON tenancy.workspace_roles
  FOR SELECT TO veltrix_app
  USING (
    EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = workspace_roles.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
    )
  );
CREATE POLICY workspace_roles_owner_insert ON tenancy.workspace_roles
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND NOT is_system
    AND base_role <> 'owner'
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = workspace_roles.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role = 'owner'
    )
  );
CREATE POLICY workspace_roles_owner_update ON tenancy.workspace_roles
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND NOT is_system
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = workspace_roles.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role = 'owner'
    )
  )
  WITH CHECK (workspace_id = security.current_workspace_id() AND NOT is_system AND base_role <> 'owner');
CREATE POLICY workspace_roles_owner_delete ON tenancy.workspace_roles
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND NOT is_system
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = workspace_roles.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role = 'owner'
    )
  );

ALTER TABLE tenancy.role_permissions ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenancy.role_permissions FORCE ROW LEVEL SECURITY;
CREATE POLICY role_permissions_member_select ON tenancy.role_permissions
  FOR SELECT TO veltrix_app
  USING (
    EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = role_permissions.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
    )
  );
CREATE POLICY role_permissions_owner_insert ON tenancy.role_permissions
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND permission <> 'roles.write'
    AND EXISTS (
      SELECT 1 FROM tenancy.workspace_roles role
      JOIN tenancy.memberships membership ON membership.workspace_id = role.workspace_id
      WHERE role.workspace_id = role_permissions.workspace_id
        AND role.id = role_permissions.role_id
        AND NOT role.is_system
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role = 'owner'
    )
  );
CREATE POLICY role_permissions_owner_delete ON tenancy.role_permissions
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM tenancy.workspace_roles role
      JOIN tenancy.memberships membership ON membership.workspace_id = role.workspace_id
      WHERE role.workspace_id = role_permissions.workspace_id
        AND role.id = role_permissions.role_id
        AND NOT role.is_system
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role = 'owner'
    )
  );

GRANT SELECT, INSERT, UPDATE, DELETE ON tenancy.workspace_roles,
  tenancy.role_permissions TO veltrix_app;

ALTER TABLE tenancy.workspaces FORCE ROW LEVEL SECURITY;
ALTER TABLE tenancy.memberships FORCE ROW LEVEL SECURITY;

RESET ROLE;

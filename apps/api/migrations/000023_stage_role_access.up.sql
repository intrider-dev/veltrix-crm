SET ROLE veltrix_owner;

CREATE TABLE sales.lead_stage_role_access (
  workspace_id uuid NOT NULL,
  stage_id uuid NOT NULL,
  role_id uuid NOT NULL,
  can_view boolean NOT NULL DEFAULT false,
  can_enter boolean NOT NULL DEFAULT false,
  can_leave boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, stage_id, role_id),
  FOREIGN KEY (workspace_id, stage_id)
    REFERENCES sales.lead_stages(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, role_id)
    REFERENCES tenancy.workspace_roles(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX lead_stage_role_access_role_idx
  ON sales.lead_stage_role_access (workspace_id, role_id, stage_id);

CREATE TABLE sales.pipeline_stage_role_access (
  workspace_id uuid NOT NULL,
  stage_id uuid NOT NULL,
  role_id uuid NOT NULL,
  can_view boolean NOT NULL DEFAULT false,
  can_enter boolean NOT NULL DEFAULT false,
  can_leave boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, stage_id, role_id),
  FOREIGN KEY (workspace_id, stage_id)
    REFERENCES sales.pipeline_stages(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, role_id)
    REFERENCES tenancy.workspace_roles(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX pipeline_stage_role_access_role_idx
  ON sales.pipeline_stage_role_access (workspace_id, role_id, stage_id);

ALTER TABLE sales.lead_stage_role_access ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.lead_stage_role_access FORCE ROW LEVEL SECURITY;
CREATE POLICY lead_stage_role_access_tenant_scope ON sales.lead_stage_role_access
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

ALTER TABLE sales.pipeline_stage_role_access ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.pipeline_stage_role_access FORCE ROW LEVEL SECURITY;
CREATE POLICY pipeline_stage_role_access_tenant_scope ON sales.pipeline_stage_role_access
  FOR ALL TO veltrix_app
  USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

-- Stage access is deliberately evaluated against the active membership instead
-- of a caller-supplied role. A stage without rules inherits the resource-level
-- permission checked by the application; once any rule exists, a missing role
-- row denies access. Workspace owners and administrators always bypass rules.
CREATE FUNCTION sales.lead_stage_access_allowed(
  target_workspace_id uuid,
  target_stage_id uuid,
  target_action text
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY INVOKER
SET search_path = pg_catalog
AS $function$
  SELECT COALESCE((
    SELECT CASE
      WHEN membership.role IN ('owner', 'admin') THEN true
      WHEN NOT EXISTS (
        SELECT 1
        FROM sales.lead_stage_role_access any_rule
        WHERE any_rule.workspace_id = stage.workspace_id
          AND any_rule.stage_id = stage.id
      ) THEN true
      WHEN target_action = 'view' THEN COALESCE(matching_rule.can_view, false)
      WHEN target_action = 'enter' THEN COALESCE(matching_rule.can_enter, false)
      WHEN target_action = 'leave' THEN COALESCE(matching_rule.can_leave, false)
      ELSE false
    END
    FROM sales.lead_stages stage
    JOIN tenancy.memberships membership
      ON membership.workspace_id = stage.workspace_id
     AND membership.user_id = security.current_actor_id()
     AND membership.status = 'active'
    LEFT JOIN sales.lead_stage_role_access matching_rule
      ON matching_rule.workspace_id = stage.workspace_id
     AND matching_rule.stage_id = stage.id
     AND matching_rule.role_id = membership.role_id
    WHERE stage.workspace_id = target_workspace_id
      AND stage.id = target_stage_id
  ), false);
$function$;

CREATE FUNCTION sales.pipeline_stage_access_allowed(
  target_workspace_id uuid,
  target_stage_id uuid,
  target_action text
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY INVOKER
SET search_path = pg_catalog
AS $function$
  SELECT COALESCE((
    SELECT CASE
      WHEN membership.role IN ('owner', 'admin') THEN true
      WHEN NOT EXISTS (
        SELECT 1
        FROM sales.pipeline_stage_role_access any_rule
        WHERE any_rule.workspace_id = stage.workspace_id
          AND any_rule.stage_id = stage.id
      ) THEN true
      WHEN target_action = 'view' THEN COALESCE(matching_rule.can_view, false)
      WHEN target_action = 'enter' THEN COALESCE(matching_rule.can_enter, false)
      WHEN target_action = 'leave' THEN COALESCE(matching_rule.can_leave, false)
      ELSE false
    END
    FROM sales.pipeline_stages stage
    JOIN tenancy.memberships membership
      ON membership.workspace_id = stage.workspace_id
     AND membership.user_id = security.current_actor_id()
     AND membership.status = 'active'
    LEFT JOIN sales.pipeline_stage_role_access matching_rule
      ON matching_rule.workspace_id = stage.workspace_id
     AND matching_rule.stage_id = stage.id
     AND matching_rule.role_id = membership.role_id
    WHERE stage.workspace_id = target_workspace_id
      AND stage.id = target_stage_id
  ), false);
$function$;

REVOKE ALL ON FUNCTION sales.lead_stage_access_allowed(uuid, uuid, text) FROM PUBLIC;
REVOKE ALL ON FUNCTION sales.pipeline_stage_access_allowed(uuid, uuid, text) FROM PUBLIC;

GRANT SELECT, INSERT, UPDATE, DELETE ON
  sales.lead_stage_role_access,
  sales.pipeline_stage_role_access
TO veltrix_app;
GRANT EXECUTE ON FUNCTION sales.lead_stage_access_allowed(uuid, uuid, text) TO veltrix_app;
GRANT EXECUTE ON FUNCTION sales.pipeline_stage_access_allowed(uuid, uuid, text) TO veltrix_app;

RESET ROLE;

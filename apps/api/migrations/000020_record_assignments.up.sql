SET ROLE veltrix_owner;

CREATE TABLE sales.lead_assignments (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  lead_id uuid NOT NULL,
  assignment_kind text NOT NULL CHECK (assignment_kind IN ('responsible', 'watcher')),
  user_id uuid,
  department_id uuid,
  is_primary boolean NOT NULL DEFAULT false,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, lead_id) REFERENCES sales.leads(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id) REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, department_id) REFERENCES tenancy.teams(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK ((user_id IS NOT NULL)::integer + (department_id IS NOT NULL)::integer = 1),
  CHECK (NOT is_primary OR assignment_kind = 'responsible')
);

CREATE TABLE sales.deal_assignments (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  deal_id uuid NOT NULL,
  assignment_kind text NOT NULL CHECK (assignment_kind IN ('responsible', 'watcher')),
  user_id uuid,
  department_id uuid,
  is_primary boolean NOT NULL DEFAULT false,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, deal_id) REFERENCES sales.deals(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id) REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, department_id) REFERENCES tenancy.teams(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK ((user_id IS NOT NULL)::integer + (department_id IS NOT NULL)::integer = 1),
  CHECK (NOT is_primary OR assignment_kind = 'responsible')
);

CREATE TABLE activities.activity_assignments (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  activity_id uuid NOT NULL,
  assignment_kind text NOT NULL CHECK (assignment_kind IN ('responsible', 'watcher')),
  user_id uuid,
  department_id uuid,
  is_primary boolean NOT NULL DEFAULT false,
  created_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, activity_id) REFERENCES activities.activities(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id) REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, department_id) REFERENCES tenancy.teams(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, created_by) REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK ((user_id IS NOT NULL)::integer + (department_id IS NOT NULL)::integer = 1),
  CHECK (NOT is_primary OR assignment_kind = 'responsible')
);

CREATE UNIQUE INDEX lead_assignment_user_unique_idx
  ON sales.lead_assignments (workspace_id, lead_id, assignment_kind, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX lead_assignment_department_unique_idx
  ON sales.lead_assignments (workspace_id, lead_id, assignment_kind, department_id) WHERE department_id IS NOT NULL;
CREATE UNIQUE INDEX lead_assignment_primary_idx
  ON sales.lead_assignments (workspace_id, lead_id) WHERE is_primary;
CREATE INDEX lead_assignment_user_access_idx
  ON sales.lead_assignments (workspace_id, user_id, lead_id, assignment_kind) WHERE user_id IS NOT NULL;
CREATE INDEX lead_assignment_department_access_idx
  ON sales.lead_assignments (workspace_id, department_id, lead_id, assignment_kind) WHERE department_id IS NOT NULL;

CREATE UNIQUE INDEX deal_assignment_user_unique_idx
  ON sales.deal_assignments (workspace_id, deal_id, assignment_kind, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX deal_assignment_department_unique_idx
  ON sales.deal_assignments (workspace_id, deal_id, assignment_kind, department_id) WHERE department_id IS NOT NULL;
CREATE UNIQUE INDEX deal_assignment_primary_idx
  ON sales.deal_assignments (workspace_id, deal_id) WHERE is_primary;
CREATE INDEX deal_assignment_user_access_idx
  ON sales.deal_assignments (workspace_id, user_id, deal_id, assignment_kind) WHERE user_id IS NOT NULL;
CREATE INDEX deal_assignment_department_access_idx
  ON sales.deal_assignments (workspace_id, department_id, deal_id, assignment_kind) WHERE department_id IS NOT NULL;

CREATE UNIQUE INDEX activity_assignment_user_unique_idx
  ON activities.activity_assignments (workspace_id, activity_id, assignment_kind, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX activity_assignment_department_unique_idx
  ON activities.activity_assignments (workspace_id, activity_id, assignment_kind, department_id) WHERE department_id IS NOT NULL;
CREATE UNIQUE INDEX activity_assignment_primary_idx
  ON activities.activity_assignments (workspace_id, activity_id) WHERE is_primary;
CREATE INDEX activity_assignment_user_access_idx
  ON activities.activity_assignments (workspace_id, user_id, activity_id, assignment_kind) WHERE user_id IS NOT NULL;
CREATE INDEX activity_assignment_department_access_idx
  ON activities.activity_assignments (workspace_id, department_id, activity_id, assignment_kind) WHERE department_id IS NOT NULL;

INSERT INTO sales.lead_assignments (
  workspace_id, id, lead_id, assignment_kind, user_id, department_id, is_primary, created_by
)
SELECT lead.workspace_id, pg_catalog.uuidv7(), lead.id, 'responsible', lead.owner_user_id, NULL, true, lead.owner_user_id
FROM sales.leads lead WHERE lead.owner_user_id IS NOT NULL;

INSERT INTO sales.lead_assignments (
  workspace_id, id, lead_id, assignment_kind, user_id, department_id, is_primary, created_by
)
SELECT lead.workspace_id, pg_catalog.uuidv7(), lead.id, 'responsible', NULL, lead.team_id,
       lead.owner_user_id IS NULL,
       COALESCE(lead.owner_user_id, membership.user_id)
FROM sales.leads lead
JOIN LATERAL (
  SELECT member.user_id FROM tenancy.memberships member
  WHERE member.workspace_id = lead.workspace_id AND member.status = 'active'
  ORDER BY (member.role = 'owner') DESC, member.created_at, member.id LIMIT 1
) membership ON true
WHERE lead.team_id IS NOT NULL;

INSERT INTO sales.deal_assignments (
  workspace_id, id, deal_id, assignment_kind, user_id, department_id, is_primary, created_by
)
SELECT deal.workspace_id, pg_catalog.uuidv7(), deal.id, 'responsible', deal.owner_user_id, NULL, true, deal.owner_user_id
FROM sales.deals deal WHERE deal.owner_user_id IS NOT NULL;

INSERT INTO activities.activity_assignments (
  workspace_id, id, activity_id, assignment_kind, user_id, department_id, is_primary, created_by
)
SELECT activity.workspace_id, pg_catalog.uuidv7(), activity.id, 'responsible', activity.assignee_user_id, NULL, true, activity.created_by
FROM activities.activities activity WHERE activity.assignee_user_id IS NOT NULL;

ALTER TABLE sales.lead_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.lead_assignments FORCE ROW LEVEL SECURITY;
ALTER TABLE sales.deal_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE sales.deal_assignments FORCE ROW LEVEL SECURITY;
ALTER TABLE activities.activity_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE activities.activity_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY lead_assignments_tenant_scope ON sales.lead_assignments
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY deal_assignments_tenant_scope ON sales.deal_assignments
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY activity_assignments_visible_select ON activities.activity_assignments
  FOR SELECT TO veltrix_app USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM activities.activities activity
      WHERE activity.workspace_id = activity_assignments.workspace_id
        AND activity.id = activity_assignments.activity_id
    )
  );
CREATE POLICY activity_assignments_tenant_insert ON activities.activity_assignments
  FOR INSERT TO veltrix_app WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY activity_assignments_tenant_update ON activities.activity_assignments
  FOR UPDATE TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY activity_assignments_tenant_delete ON activities.activity_assignments
  FOR DELETE TO veltrix_app USING (workspace_id = security.current_workspace_id());
CREATE POLICY activity_assignments_owner_all ON activities.activity_assignments
  FOR ALL TO veltrix_owner USING (true) WITH CHECK (true);

-- The two bounded SECURITY DEFINER predicates below run as the NOLOGIN table
-- owner. FORCE RLS therefore needs read-only policies for their membership
-- lookup; application and dispatcher roles are intentionally not included.
CREATE POLICY task_assignment_owner_memberships_select ON tenancy.memberships
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY task_assignment_owner_team_memberships_select ON tenancy.team_memberships
  FOR SELECT TO veltrix_owner USING (true);

CREATE OR REPLACE FUNCTION security.activity_assignment_allows(
  target_workspace_id uuid, target_activity_id uuid, target_user_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1 FROM activities.activity_assignments assignment
    WHERE assignment.workspace_id = target_workspace_id
      AND assignment.activity_id = target_activity_id
      AND (
        assignment.user_id = target_user_id
        OR EXISTS (
          SELECT 1 FROM tenancy.memberships membership
          JOIN tenancy.team_memberships department_member
            ON department_member.workspace_id = membership.workspace_id
           AND department_member.membership_id = membership.id
          WHERE membership.workspace_id = target_workspace_id
            AND membership.user_id = target_user_id
            AND membership.status = 'active'
            AND department_member.team_id = assignment.department_id
        )
      )
  )
$function$;
REVOKE ALL ON FUNCTION security.activity_assignment_allows(uuid, uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.activity_assignment_allows(uuid, uuid, uuid) TO veltrix_app;

CREATE OR REPLACE FUNCTION security.activity_responsible_allows(
  target_workspace_id uuid, target_activity_id uuid, target_user_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1 FROM activities.activity_assignments assignment
    WHERE assignment.workspace_id = target_workspace_id
      AND assignment.activity_id = target_activity_id
      AND assignment.assignment_kind = 'responsible'
      AND (
        assignment.user_id = target_user_id
        OR EXISTS (
          SELECT 1 FROM tenancy.memberships membership
          JOIN tenancy.team_memberships department_member
            ON department_member.workspace_id = membership.workspace_id
           AND department_member.membership_id = membership.id
          WHERE membership.workspace_id = target_workspace_id
            AND membership.user_id = target_user_id
            AND membership.status = 'active'
            AND department_member.team_id = assignment.department_id
        )
      )
  )
$function$;
REVOKE ALL ON FUNCTION security.activity_responsible_allows(uuid, uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.activity_responsible_allows(uuid, uuid, uuid) TO veltrix_app;

ALTER POLICY activity_visible_select ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    visibility_scope = 'workspace'
    OR created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR security.activity_assignment_allows(workspace_id, id, security.current_actor_id())
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR (
      visibility_scope = 'department'
      AND EXISTS (
        SELECT 1 FROM tenancy.memberships membership
        JOIN tenancy.team_memberships department_member
          ON department_member.workspace_id = membership.workspace_id
         AND department_member.membership_id = membership.id
        WHERE membership.workspace_id = activities.workspace_id
          AND membership.user_id = security.current_actor_id()
          AND membership.status = 'active'
          AND department_member.team_id = activities.scope_department_id
      )
    )
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner', 'admin')
    )
  )
);

ALTER POLICY activity_visible_update ON activities.activities USING (
  workspace_id = security.current_workspace_id()
  AND (
    created_by = security.current_actor_id()
    OR assignee_user_id = security.current_actor_id()
    OR security.activity_responsible_allows(workspace_id, id, security.current_actor_id())
    OR (visibility_scope = 'user' AND scope_user_id = security.current_actor_id())
    OR EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = activities.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
        AND membership.role IN ('owner', 'admin')
    )
  )
);

GRANT SELECT, INSERT, UPDATE, DELETE ON sales.lead_assignments, sales.deal_assignments TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON activities.activity_assignments TO veltrix_app;

RESET ROLE;

-- Keep the indexed SECURITY DEFINER search path narrowly privileged. This
-- NOLOGIN role can bypass RLS only for the read-only tables needed by the
-- explicitly actor/workspace/permission-checked function below.
RESET ROLE;

DO $roles$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'veltrix_search') THEN
    CREATE ROLE veltrix_search NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT BYPASSRLS;
  END IF;
END
$roles$;

GRANT USAGE ON SCHEMA security, tenancy, sales, activities, search TO veltrix_search;
GRANT USAGE ON TYPE search.document_query_result TO veltrix_search;
GRANT SELECT ON
  tenancy.memberships,
  tenancy.workspace_roles,
  tenancy.role_permissions,
  tenancy.team_memberships,
  sales.leads,
  sales.deals,
  sales.lead_stages,
  sales.pipeline_stages,
  sales.lead_stage_role_access,
  sales.pipeline_stage_role_access,
  activities.activities,
  activities.activity_assignments,
  search.documents
TO veltrix_search;
GRANT EXECUTE ON FUNCTION security.current_workspace_id() TO veltrix_search;
GRANT EXECUTE ON FUNCTION security.current_actor_id() TO veltrix_search;
GRANT EXECUTE ON FUNCTION security.activity_assignment_allows(uuid, uuid, uuid) TO veltrix_search;

SET ROLE veltrix_owner;

-- Only immutable system owner/admin roles bypass explicit stage rules. A
-- custom role may use an admin capability envelope, but remains constrained by
-- its own role_id and configured stage grants.
CREATE OR REPLACE FUNCTION sales.lead_stage_access_allowed(
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
      WHEN role.is_system AND role.role_key IN ('owner', 'admin') THEN true
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
    JOIN tenancy.workspace_roles role
      ON role.workspace_id = membership.workspace_id
     AND role.id = membership.role_id
    LEFT JOIN sales.lead_stage_role_access matching_rule
      ON matching_rule.workspace_id = stage.workspace_id
     AND matching_rule.stage_id = stage.id
     AND matching_rule.role_id = membership.role_id
    WHERE stage.workspace_id = target_workspace_id
      AND stage.id = target_stage_id
  ), false);
$function$;

CREATE OR REPLACE FUNCTION sales.pipeline_stage_access_allowed(
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
      WHEN role.is_system AND role.role_key IN ('owner', 'admin') THEN true
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
    JOIN tenancy.workspace_roles role
      ON role.workspace_id = membership.workspace_id
     AND role.id = membership.role_id
    LEFT JOIN sales.pipeline_stage_role_access matching_rule
      ON matching_rule.workspace_id = stage.workspace_id
     AND matching_rule.stage_id = stage.id
     AND matching_rule.role_id = membership.role_id
    WHERE stage.workspace_id = target_workspace_id
      AND stage.id = target_stage_id
  ), false);
$function$;

RESET ROLE;
GRANT EXECUTE ON FUNCTION sales.lead_stage_access_allowed(uuid, uuid, text) TO veltrix_search;
GRANT EXECUTE ON FUNCTION sales.pipeline_stage_access_allowed(uuid, uuid, text) TO veltrix_search;

CREATE OR REPLACE FUNCTION search.query_documents(
  target_workspace_id uuid,
  raw_query text,
  result_limit integer DEFAULT 50
) RETURNS SETOF search.document_query_result
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  WITH actor_membership AS MATERIALIZED (
    SELECT membership.id, membership.role_id, role.role_key, role.is_system
    FROM tenancy.memberships membership
    JOIN tenancy.workspace_roles role
      ON role.workspace_id = membership.workspace_id
     AND role.id = membership.role_id
    WHERE membership.workspace_id = target_workspace_id
      AND membership.user_id = security.current_actor_id()
      AND membership.status = 'active'
      AND target_workspace_id = security.current_workspace_id()
  ), request_context AS MATERIALIZED (
    SELECT EXISTS (SELECT 1 FROM actor_membership) AS allowed,
           websearch_to_tsquery('simple', raw_query) AS tsq
  ), full_text AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           (ts_rank(document.search_vector, context.tsq) * document.rank_boost)::real AS rank
    FROM search.documents document
    CROSS JOIN request_context context
    WHERE context.allowed
      AND document.workspace_id = target_workspace_id
      AND document.search_vector @@ context.tsq
    ORDER BY rank DESC, document.title, document.entity_id
    LIMIT 100
  ), fuzzy AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           public.similarity(document.searchable_text, raw_query)::real AS rank
    FROM search.documents document
    CROSS JOIN request_context context
    WHERE context.allowed
      AND document.workspace_id = target_workspace_id
      AND NOT EXISTS (SELECT 1 FROM full_text)
      AND document.searchable_text OPERATOR(public.%) raw_query
    ORDER BY rank DESC, document.title, document.entity_id
    LIMIT 100
  ), candidates AS (
    SELECT * FROM full_text
    UNION ALL
    SELECT * FROM fuzzy
  )
  SELECT document.entity_type, document.entity_id, document.title,
         COALESCE(document.subtitle, ''), left(document.searchable_text, 240), document.rank
  FROM candidates document
  WHERE CASE document.entity_type
    WHEN 'contact' THEN EXISTS (
      SELECT 1 FROM actor_membership membership
      JOIN tenancy.role_permissions permission
        ON permission.workspace_id = target_workspace_id
       AND permission.role_id = membership.role_id
       AND permission.permission = 'records.read'
    )
    WHEN 'company' THEN EXISTS (
      SELECT 1 FROM actor_membership membership
      JOIN tenancy.role_permissions permission
        ON permission.workspace_id = target_workspace_id
       AND permission.role_id = membership.role_id
       AND permission.permission = 'records.read'
    )
    WHEN 'lead' THEN EXISTS (
      SELECT 1 FROM actor_membership membership
      JOIN tenancy.role_permissions permission
        ON permission.workspace_id = target_workspace_id
       AND permission.role_id = membership.role_id
       AND permission.permission = 'leads.read'
    ) AND EXISTS (
      SELECT 1 FROM sales.leads lead
      WHERE lead.workspace_id = document.workspace_id
        AND lead.id = document.entity_id
        AND lead.deleted_at IS NULL
        AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
    )
    WHEN 'deal' THEN EXISTS (
      SELECT 1 FROM actor_membership membership
      JOIN tenancy.role_permissions permission
        ON permission.workspace_id = target_workspace_id
       AND permission.role_id = membership.role_id
       AND permission.permission = 'deals.read'
    ) AND EXISTS (
      SELECT 1 FROM sales.deals deal
      WHERE deal.workspace_id = document.workspace_id
        AND deal.id = document.entity_id
        AND deal.deleted_at IS NULL
        AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
    )
    WHEN 'note' THEN EXISTS (
      SELECT 1 FROM actor_membership membership
      JOIN tenancy.role_permissions permission
        ON permission.workspace_id = target_workspace_id
       AND permission.role_id = membership.role_id
       AND permission.permission = 'records.read'
    ) AND EXISTS (
      SELECT 1
      FROM activities.activities activity
      WHERE activity.workspace_id = document.workspace_id
        AND activity.id = document.entity_id
        AND activity.activity_type = 'note'
        AND activity.deleted_at IS NULL
        AND (
          activity.visibility_scope = 'workspace'
          OR activity.created_by = security.current_actor_id()
          OR activity.assignee_user_id = security.current_actor_id()
          OR security.activity_assignment_allows(activity.workspace_id, activity.id, security.current_actor_id())
          OR (activity.visibility_scope = 'user' AND activity.scope_user_id = security.current_actor_id())
          OR (
            activity.visibility_scope = 'department'
            AND EXISTS (
              SELECT 1
              FROM actor_membership membership
              JOIN tenancy.team_memberships department_member
                ON department_member.workspace_id = target_workspace_id
               AND department_member.membership_id = membership.id
               AND department_member.team_id = activity.scope_department_id
            )
          )
          OR EXISTS (
            SELECT 1 FROM actor_membership membership
            WHERE membership.is_system
              AND membership.role_key IN ('owner', 'admin')
          )
        )
    )
    ELSE false
  END
  ORDER BY document.rank DESC, document.title, document.entity_id
  LIMIT LEAST(GREATEST(result_limit, 1), 100);
$function$;

GRANT CREATE ON SCHEMA search TO veltrix_search;
ALTER FUNCTION search.query_documents(uuid, text, integer) OWNER TO veltrix_search;
REVOKE CREATE ON SCHEMA search FROM veltrix_search;
REVOKE ALL ON FUNCTION search.query_documents(uuid, text, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION search.query_documents(uuid, text, integer) TO veltrix_app;

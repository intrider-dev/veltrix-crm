-- Precompute the actor permission envelope once per search. The prior
-- hardening function repeated permission joins for every candidate; it was
-- correct, but amplified PostgreSQL CPU contention in the constrained
-- benchmark profile.
RESET ROLE;
GRANT CREATE ON SCHEMA search TO veltrix_search;
SET ROLE veltrix_search;

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
  WITH actor_access AS MATERIALIZED (
    SELECT membership.id,
           membership.role_id,
           role.role_key,
           role.is_system,
           (role.is_system AND role.role_key IN ('owner', 'admin')) AS system_administrator,
           COALESCE(bool_or(permission.permission = 'records.read'), false) AS records_read,
           COALESCE(bool_or(permission.permission = 'leads.read'), false) AS leads_read,
           COALESCE(bool_or(permission.permission = 'deals.read'), false) AS deals_read
    FROM tenancy.memberships membership
    JOIN tenancy.workspace_roles role
      ON role.workspace_id = membership.workspace_id
     AND role.id = membership.role_id
    LEFT JOIN tenancy.role_permissions permission
      ON permission.workspace_id = membership.workspace_id
     AND permission.role_id = membership.role_id
    WHERE membership.workspace_id = target_workspace_id
      AND membership.user_id = security.current_actor_id()
      AND membership.status = 'active'
      AND target_workspace_id = security.current_workspace_id()
    GROUP BY membership.id, membership.role_id, role.role_key, role.is_system
  ), request_context AS MATERIALIZED (
    SELECT websearch_to_tsquery('simple', raw_query) AS tsq
    FROM actor_access
  ), full_text AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           (ts_rank(document.search_vector, context.tsq) * document.rank_boost)::real AS rank
    FROM search.documents document
    CROSS JOIN request_context context
    WHERE document.workspace_id = target_workspace_id
      AND document.search_vector @@ context.tsq
    ORDER BY rank DESC, document.title, document.entity_id
    LIMIT LEAST(GREATEST(result_limit, 1), 100) * 2
  ), fuzzy AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           public.similarity(document.searchable_text, raw_query)::real AS rank
    FROM search.documents document
    CROSS JOIN request_context context
    WHERE document.workspace_id = target_workspace_id
      AND NOT EXISTS (SELECT 1 FROM full_text)
      AND document.searchable_text OPERATOR(public.%) raw_query
    ORDER BY rank DESC, document.title, document.entity_id
    LIMIT LEAST(GREATEST(result_limit, 1), 100) * 2
  ), candidates AS (
    SELECT * FROM full_text
    UNION ALL
    SELECT * FROM fuzzy
  )
  SELECT document.entity_type, document.entity_id, document.title,
         COALESCE(document.subtitle, ''), left(document.searchable_text, 240), document.rank
  FROM candidates document
  CROSS JOIN actor_access access
  WHERE CASE document.entity_type
    WHEN 'contact' THEN access.records_read
    WHEN 'company' THEN access.records_read
    WHEN 'lead' THEN access.leads_read AND (
      access.system_administrator OR EXISTS (
        SELECT 1 FROM sales.leads lead
        WHERE lead.workspace_id = document.workspace_id
          AND lead.id = document.entity_id
          AND lead.deleted_at IS NULL
          AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
      )
    )
    WHEN 'deal' THEN access.deals_read AND (
      access.system_administrator OR EXISTS (
        SELECT 1 FROM sales.deals deal
        WHERE deal.workspace_id = document.workspace_id
          AND deal.id = document.entity_id
          AND deal.deleted_at IS NULL
          AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
      )
    )
    WHEN 'note' THEN access.records_read AND EXISTS (
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
              FROM tenancy.team_memberships department_member
              WHERE department_member.workspace_id = target_workspace_id
                AND department_member.membership_id = access.id
                AND department_member.team_id = activity.scope_department_id
            )
          )
          OR access.system_administrator
        )
    )
    ELSE false
  END
  ORDER BY document.rank DESC, document.title, document.entity_id
  LIMIT LEAST(GREATEST(result_limit, 1), 100);
$function$;

RESET ROLE;
REVOKE CREATE ON SCHEMA search FROM veltrix_search;

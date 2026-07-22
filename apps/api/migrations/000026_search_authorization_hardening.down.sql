RESET ROLE;

-- Restore the migration-25 function owner before revoking the narrowly scoped
-- search-role grants. The role itself is cluster-wide and is intentionally not
-- dropped because another database in the same PostgreSQL cluster may use it.
DO $owner$
BEGIN
  EXECUTE format(
    'ALTER FUNCTION search.query_documents(uuid, text, integer) OWNER TO %I',
    session_user
  );
END
$owner$;

REVOKE ALL ON FUNCTION search.query_documents(uuid, text, integer) FROM veltrix_app;
REVOKE EXECUTE ON FUNCTION security.current_workspace_id() FROM veltrix_search;
REVOKE EXECUTE ON FUNCTION security.current_actor_id() FROM veltrix_search;
REVOKE EXECUTE ON FUNCTION security.activity_assignment_allows(uuid, uuid, uuid) FROM veltrix_search;
REVOKE EXECUTE ON FUNCTION sales.lead_stage_access_allowed(uuid, uuid, text) FROM veltrix_search;
REVOKE EXECUTE ON FUNCTION sales.pipeline_stage_access_allowed(uuid, uuid, text) FROM veltrix_search;
REVOKE SELECT ON
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
FROM veltrix_search;
REVOKE USAGE ON TYPE search.document_query_result FROM veltrix_search;
REVOKE USAGE ON SCHEMA security, tenancy, sales, activities, search FROM veltrix_search;

SET ROLE veltrix_owner;

CREATE OR REPLACE FUNCTION sales.lead_stage_access_allowed(
  target_workspace_id uuid, target_stage_id uuid, target_action text
) RETURNS boolean
LANGUAGE sql STABLE SECURITY INVOKER SET search_path = pg_catalog
AS $function$
  SELECT COALESCE((
    SELECT CASE
      WHEN membership.role IN ('owner', 'admin') THEN true
      WHEN NOT EXISTS (
        SELECT 1 FROM sales.lead_stage_role_access any_rule
        WHERE any_rule.workspace_id = stage.workspace_id AND any_rule.stage_id = stage.id
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
    WHERE stage.workspace_id = target_workspace_id AND stage.id = target_stage_id
  ), false);
$function$;

CREATE OR REPLACE FUNCTION sales.pipeline_stage_access_allowed(
  target_workspace_id uuid, target_stage_id uuid, target_action text
) RETURNS boolean
LANGUAGE sql STABLE SECURITY INVOKER SET search_path = pg_catalog
AS $function$
  SELECT COALESCE((
    SELECT CASE
      WHEN membership.role IN ('owner', 'admin') THEN true
      WHEN NOT EXISTS (
        SELECT 1 FROM sales.pipeline_stage_role_access any_rule
        WHERE any_rule.workspace_id = stage.workspace_id AND any_rule.stage_id = stage.id
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
    WHERE stage.workspace_id = target_workspace_id AND stage.id = target_stage_id
  ), false);
$function$;

RESET ROLE;

CREATE OR REPLACE FUNCTION search.query_documents(
  target_workspace_id uuid, raw_query text, result_limit integer DEFAULT 50
) RETURNS SETOF search.document_query_result
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $function$
  WITH request_context AS MATERIALIZED (
    SELECT target_workspace_id = security.current_workspace_id()
      AND EXISTS (
        SELECT 1 FROM tenancy.memberships membership
        WHERE membership.workspace_id = target_workspace_id
          AND membership.user_id = security.current_actor_id()
          AND membership.status = 'active'
      ) AS allowed,
      websearch_to_tsquery('simple', raw_query) AS tsq
  ), full_text AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           (ts_rank(document.search_vector, context.tsq) * document.rank_boost)::real AS rank
    FROM search.documents document CROSS JOIN request_context context
    WHERE context.allowed AND document.workspace_id = target_workspace_id
      AND document.search_vector @@ context.tsq
    ORDER BY rank DESC, document.title, document.entity_id LIMIT 100
  ), fuzzy AS MATERIALIZED (
    SELECT document.workspace_id, document.entity_type, document.entity_id,
           document.title, document.subtitle, document.searchable_text,
           public.similarity(document.searchable_text, raw_query)::real AS rank
    FROM search.documents document CROSS JOIN request_context context
    WHERE context.allowed AND document.workspace_id = target_workspace_id
      AND NOT EXISTS (SELECT 1 FROM full_text)
      AND document.searchable_text OPERATOR(public.%) raw_query
    ORDER BY rank DESC, document.title, document.entity_id LIMIT 100
  ), candidates AS (
    SELECT * FROM full_text UNION ALL SELECT * FROM fuzzy
  )
  SELECT document.entity_type, document.entity_id, document.title,
         document.subtitle, left(document.searchable_text, 240), document.rank
  FROM candidates document
  WHERE (document.entity_type <> 'lead' OR EXISTS (
    SELECT 1 FROM sales.leads lead
    WHERE lead.workspace_id = document.workspace_id AND lead.id = document.entity_id
      AND lead.deleted_at IS NULL
      AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
  )) AND (document.entity_type <> 'deal' OR EXISTS (
    SELECT 1 FROM sales.deals deal
    WHERE deal.workspace_id = document.workspace_id AND deal.id = document.entity_id
      AND deal.deleted_at IS NULL
      AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
  ))
  ORDER BY document.rank DESC, document.title, document.entity_id
  LIMIT LEAST(GREATEST(result_limit, 1), 100);
$function$;

REVOKE ALL ON FUNCTION search.query_documents(uuid, text, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION search.query_documents(uuid, text, integer) TO veltrix_app;

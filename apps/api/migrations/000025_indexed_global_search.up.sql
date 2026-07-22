-- PostgreSQL cannot push non-leakproof full-text and trigram operators below an
-- RLS security barrier. Keep the public database role unprivileged and expose
-- one narrowly scoped, tenant-checked read function so the GIN indexes remain
-- usable for the global-search hot path.
CREATE TYPE search.document_query_result AS (
  entity_type text,
  entity_id uuid,
  title text,
  subtitle text,
  snippet text,
  rank real
);

CREATE FUNCTION search.query_documents(
  target_workspace_id uuid,
  raw_query text,
  result_limit integer DEFAULT 50
) RETURNS SETOF search.document_query_result
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  WITH request_context AS MATERIALIZED (
    SELECT
      target_workspace_id = security.current_workspace_id()
      AND EXISTS (
        SELECT 1
        FROM tenancy.memberships membership
        WHERE membership.workspace_id = target_workspace_id
          AND membership.user_id = security.current_actor_id()
          AND membership.status = 'active'
      ) AS allowed,
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
         document.subtitle, left(document.searchable_text, 240), document.rank
  FROM candidates document
  WHERE (document.entity_type <> 'lead' OR EXISTS (
    SELECT 1
    FROM sales.leads lead
    WHERE lead.workspace_id = document.workspace_id
      AND lead.id = document.entity_id
      AND lead.deleted_at IS NULL
      AND sales.lead_stage_access_allowed(lead.workspace_id, lead.stage_id, 'view')
  ))
    AND (document.entity_type <> 'deal' OR EXISTS (
      SELECT 1
      FROM sales.deals deal
      WHERE deal.workspace_id = document.workspace_id
        AND deal.id = document.entity_id
        AND deal.deleted_at IS NULL
        AND sales.pipeline_stage_access_allowed(deal.workspace_id, deal.stage_id, 'view')
    ))
  ORDER BY document.rank DESC, document.title, document.entity_id
  LIMIT LEAST(GREATEST(result_limit, 1), 100);
$function$;

REVOKE ALL ON FUNCTION search.query_documents(uuid, text, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION search.query_documents(uuid, text, integer) TO veltrix_app;

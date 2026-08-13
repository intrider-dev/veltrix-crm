SET ROLE veltrix_owner;

-- The entity-chat authorization predicates run as the NOLOGIN owner so they
-- can evaluate a target recipient without changing the request actor context.
-- FORCE RLS therefore needs narrowly scoped, read-only owner policies.
CREATE POLICY entity_chat_owner_links_select ON collaboration.entity_conversations
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY entity_chat_owner_leads_select ON sales.leads
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY entity_chat_owner_deals_select ON sales.deals
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY entity_chat_owner_pipeline_stages_select ON sales.pipeline_stages
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY entity_chat_owner_lead_stage_access_select ON sales.lead_stage_role_access
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY entity_chat_owner_pipeline_stage_access_select ON sales.pipeline_stage_role_access
  FOR SELECT TO veltrix_owner USING (true);

CREATE OR REPLACE FUNCTION security.chat_entity_conversation_user_allowed(
  target_workspace_id uuid,
  target_conversation_id uuid,
  target_user_id uuid
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  WITH linked AS MATERIALIZED (
    SELECT link.entity_type, link.entity_id
    FROM collaboration.entity_conversations link
    WHERE link.workspace_id = target_workspace_id
      AND link.conversation_id = target_conversation_id
  ), actor_role AS MATERIALIZED (
    SELECT membership.role_id, role.is_system, role.role_key
    FROM tenancy.memberships membership
    JOIN tenancy.workspace_roles role
      ON role.workspace_id = membership.workspace_id
     AND role.id = membership.role_id
    WHERE membership.workspace_id = target_workspace_id
      AND membership.user_id = target_user_id
      AND membership.status = 'active'
  )
  SELECT target_workspace_id = security.current_workspace_id()
    AND EXISTS (SELECT 1 FROM actor_role)
    AND CASE
      WHEN NOT EXISTS (SELECT 1 FROM linked) THEN true
      WHEN EXISTS (
        SELECT 1
        FROM linked link
        JOIN sales.leads lead
          ON link.entity_type = 'lead'
         AND lead.workspace_id = target_workspace_id
         AND lead.id = link.entity_id
         AND lead.deleted_at IS NULL
        CROSS JOIN actor_role actor
        WHERE (
          (actor.is_system AND actor.role_key IN ('owner', 'admin'))
          OR EXISTS (
            SELECT 1 FROM tenancy.role_permissions permission
            WHERE permission.workspace_id = target_workspace_id
              AND permission.role_id = actor.role_id
              AND permission.permission = 'leads.read'
          )
        )
        AND (
          (actor.is_system AND actor.role_key IN ('owner', 'admin'))
          OR NOT EXISTS (
            SELECT 1 FROM sales.lead_stage_role_access any_rule
            WHERE any_rule.workspace_id = target_workspace_id
              AND any_rule.stage_id = lead.stage_id
          )
          OR EXISTS (
            SELECT 1 FROM sales.lead_stage_role_access matching_rule
            WHERE matching_rule.workspace_id = target_workspace_id
              AND matching_rule.stage_id = lead.stage_id
              AND matching_rule.role_id = actor.role_id
              AND matching_rule.can_view
          )
        )
      ) THEN true
      WHEN EXISTS (
        SELECT 1
        FROM linked link
        JOIN sales.deals deal
          ON link.entity_type = 'deal'
         AND deal.workspace_id = target_workspace_id
         AND deal.id = link.entity_id
         AND deal.deleted_at IS NULL
        CROSS JOIN actor_role actor
        WHERE (
          (actor.is_system AND actor.role_key IN ('owner', 'admin'))
          OR EXISTS (
            SELECT 1 FROM tenancy.role_permissions permission
            WHERE permission.workspace_id = target_workspace_id
              AND permission.role_id = actor.role_id
              AND permission.permission = 'deals.read'
          )
        )
        AND (
          (actor.is_system AND actor.role_key IN ('owner', 'admin'))
          OR NOT EXISTS (
            SELECT 1 FROM sales.pipeline_stage_role_access any_rule
            WHERE any_rule.workspace_id = target_workspace_id
              AND any_rule.stage_id = deal.stage_id
          )
          OR EXISTS (
            SELECT 1 FROM sales.pipeline_stage_role_access matching_rule
            WHERE matching_rule.workspace_id = target_workspace_id
              AND matching_rule.stage_id = deal.stage_id
              AND matching_rule.role_id = actor.role_id
              AND matching_rule.can_view
          )
        )
      ) THEN true
      ELSE false
    END
$function$;

CREATE OR REPLACE FUNCTION security.chat_conversation_visible(
  target_workspace_id uuid, target_conversation_id uuid
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.conversation_members member
    JOIN tenancy.memberships active_membership
      ON active_membership.workspace_id = member.workspace_id
     AND active_membership.user_id = member.user_id
     AND active_membership.status = 'active'
    WHERE member.workspace_id = target_workspace_id
      AND member.conversation_id = target_conversation_id
      AND member.user_id = security.current_actor_id()
      AND target_workspace_id = security.current_workspace_id()
      AND security.chat_entity_conversation_user_allowed(
        target_workspace_id, target_conversation_id, member.user_id
      )
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_conversation_manage_allowed(
  target_workspace_id uuid, target_conversation_id uuid
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT target_workspace_id = security.current_workspace_id()
     AND security.chat_entity_conversation_user_allowed(
       target_workspace_id, target_conversation_id, security.current_actor_id()
     )
     AND EXISTS (
       SELECT 1
       FROM collaboration.conversations conversation
       JOIN tenancy.memberships active_membership
         ON active_membership.workspace_id = conversation.workspace_id
        AND active_membership.user_id = security.current_actor_id()
        AND active_membership.status = 'active'
       WHERE conversation.workspace_id = target_workspace_id
         AND conversation.id = target_conversation_id
         AND (
           EXISTS (
             SELECT 1 FROM collaboration.conversation_members owner_member
             WHERE owner_member.workspace_id = conversation.workspace_id
               AND owner_member.conversation_id = conversation.id
               AND owner_member.user_id = security.current_actor_id()
               AND owner_member.member_role = 'owner'
           )
           OR (
             conversation.created_by = security.current_actor_id()
             AND NOT EXISTS (
               SELECT 1 FROM collaboration.conversation_members existing_owner
               WHERE existing_owner.workspace_id = conversation.workspace_id
                 AND existing_owner.conversation_id = conversation.id
                 AND existing_owner.member_role = 'owner'
             )
           )
         )
     )
$function$;

CREATE OR REPLACE FUNCTION security.chat_call_user_allowed(
  target_workspace_id uuid, target_call_id uuid, target_user_id uuid
) RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.calls call
    JOIN collaboration.conversation_members member
      ON member.workspace_id = call.workspace_id
     AND member.conversation_id = call.conversation_id
     AND member.user_id = target_user_id
    JOIN tenancy.memberships active_membership
      ON active_membership.workspace_id = member.workspace_id
     AND active_membership.user_id = member.user_id
     AND active_membership.status = 'active'
    WHERE call.workspace_id = target_workspace_id
      AND call.id = target_call_id
      AND target_workspace_id = security.current_workspace_id()
      AND security.chat_entity_conversation_user_allowed(
        target_workspace_id, call.conversation_id, target_user_id
      )
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_authorized_member_count(
  target_workspace_id uuid, target_conversation_id uuid
) RETURNS integer
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT count(*)::integer
  FROM collaboration.conversation_members member
  WHERE member.workspace_id = target_workspace_id
    AND member.conversation_id = target_conversation_id
    AND target_workspace_id = security.current_workspace_id()
    AND security.chat_entity_conversation_user_allowed(
      target_workspace_id, target_conversation_id, member.user_id
    )
$function$;

DROP POLICY conversation_members_owner_insert ON collaboration.conversation_members;
CREATE POLICY conversation_members_owner_insert ON collaboration.conversation_members
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = conversation_members.workspace_id
        AND membership.user_id = conversation_members.user_id
        AND membership.status = 'active'
    )
    AND (
      security.chat_conversation_manage_allowed(workspace_id, conversation_id)
      OR (
        user_id = security.current_actor_id()
        AND EXISTS (
          SELECT 1 FROM collaboration.entity_conversations link
          WHERE link.workspace_id = conversation_members.workspace_id
            AND link.conversation_id = conversation_members.conversation_id
        )
        AND security.chat_entity_conversation_user_allowed(
          workspace_id, conversation_id, user_id
        )
      )
    )
  );

REVOKE ALL ON FUNCTION security.chat_entity_conversation_user_allowed(uuid, uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_authorized_member_count(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.chat_entity_conversation_user_allowed(uuid, uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_authorized_member_count(uuid, uuid) TO veltrix_app;

-- Normalize values already present in the sales JSON aggregates so schema
-- migrations and deletion safety see the same usage as contact/company fields.
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
    AND lead.deleted_at IS NULL
  UNION ALL
  SELECT deal.id, deal.custom_fields
  FROM sales.deals deal
  WHERE definition.entity_type = 'deal'
    AND deal.workspace_id = definition.workspace_id
    AND deal.deleted_at IS NULL
) record ON record.custom_fields ? definition.field_key
ON CONFLICT (workspace_id, definition_id, entity_id) DO UPDATE
SET value = EXCLUDED.value,
    entity_type = EXCLUDED.entity_type,
    schema_version = EXCLUDED.schema_version,
    updated_at = now();

RESET ROLE;

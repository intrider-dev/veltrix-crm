SET ROLE veltrix_owner;

DELETE FROM customers.custom_field_values WHERE entity_type IN ('lead', 'deal');

DROP POLICY conversation_members_owner_insert ON collaboration.conversation_members;
CREATE POLICY conversation_members_owner_insert ON collaboration.conversation_members
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_conversation_manage_allowed(workspace_id, conversation_id)
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = conversation_members.workspace_id
        AND membership.user_id = conversation_members.user_id
        AND membership.status = 'active'
    )
  );

CREATE OR REPLACE FUNCTION security.chat_conversation_visible(
  target_workspace_id uuid, target_conversation_id uuid
) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
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
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_conversation_manage_allowed(
  target_workspace_id uuid, target_conversation_id uuid
) RETURNS boolean
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
AS $function$
  SELECT target_workspace_id = security.current_workspace_id()
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
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
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
  )
$function$;

DROP FUNCTION security.chat_authorized_member_count(uuid, uuid);
DROP FUNCTION security.chat_entity_conversation_user_allowed(uuid, uuid, uuid);

DROP POLICY entity_chat_owner_links_select ON collaboration.entity_conversations;
DROP POLICY entity_chat_owner_leads_select ON sales.leads;
DROP POLICY entity_chat_owner_deals_select ON sales.deals;
DROP POLICY entity_chat_owner_pipeline_stages_select ON sales.pipeline_stages;
DROP POLICY entity_chat_owner_lead_stage_access_select ON sales.lead_stage_role_access;
DROP POLICY entity_chat_owner_pipeline_stage_access_select ON sales.pipeline_stage_role_access;

RESET ROLE;

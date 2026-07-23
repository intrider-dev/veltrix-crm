SET ROLE veltrix_owner;

CREATE POLICY entity_chat_owner_members_delete ON collaboration.conversation_members
  FOR DELETE TO veltrix_owner
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1
      FROM collaboration.entity_conversations link
      WHERE link.workspace_id = conversation_members.workspace_id
        AND link.conversation_id = conversation_members.conversation_id
    )
    AND NOT security.chat_entity_conversation_user_allowed(
      workspace_id, conversation_id, user_id
    )
  );

CREATE OR REPLACE FUNCTION security.prune_unauthorized_entity_conversation_members(
  target_workspace_id uuid,
  target_conversation_id uuid
) RETURNS integer
LANGUAGE sql
VOLATILE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  WITH removed AS (
    DELETE FROM collaboration.conversation_members member
    WHERE member.workspace_id = target_workspace_id
      AND member.conversation_id = target_conversation_id
      AND target_workspace_id = security.current_workspace_id()
      AND EXISTS (
        SELECT 1
        FROM collaboration.entity_conversations link
        WHERE link.workspace_id = member.workspace_id
          AND link.conversation_id = member.conversation_id
      )
      AND NOT security.chat_entity_conversation_user_allowed(
        member.workspace_id, member.conversation_id, member.user_id
      )
    RETURNING 1
  )
  SELECT count(*)::integer FROM removed
$function$;

REVOKE ALL ON FUNCTION security.prune_unauthorized_entity_conversation_members(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.prune_unauthorized_entity_conversation_members(uuid, uuid) TO veltrix_app;

RESET ROLE;

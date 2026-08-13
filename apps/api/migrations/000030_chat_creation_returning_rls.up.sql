SET ROLE veltrix_owner;

-- A SQL SECURITY DEFINER lookup of the just-inserted conversation cannot see
-- that row through the current statement snapshot. Evaluate creator identity
-- from the INSERT row itself and use only the helper for pre-existing owners.
CREATE OR REPLACE FUNCTION security.chat_conversation_has_owner(
  target_workspace_id uuid, target_conversation_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.conversation_members owner_member
    WHERE owner_member.workspace_id = target_workspace_id
      AND owner_member.conversation_id = target_conversation_id
      AND owner_member.member_role = 'owner'
  )
$function$;
REVOKE ALL ON FUNCTION security.chat_conversation_has_owner(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.chat_conversation_has_owner(uuid, uuid) TO veltrix_app;

DROP POLICY conversations_member_select ON collaboration.conversations;
CREATE POLICY conversations_member_select ON collaboration.conversations
  FOR SELECT TO veltrix_app
  USING (
    security.chat_conversation_visible(workspace_id, id)
    OR (
      workspace_id = security.current_workspace_id()
      AND created_by = security.current_actor_id()
      AND NOT security.chat_conversation_has_owner(workspace_id, id)
    )
  );

RESET ROLE;

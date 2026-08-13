SET ROLE veltrix_owner;

CREATE OR REPLACE FUNCTION security.chat_conversation_has_owner(
  target_workspace_id uuid, target_conversation_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT target_workspace_id = security.current_workspace_id()
     AND EXISTS (
       SELECT 1
       FROM collaboration.conversation_members owner_member
       WHERE owner_member.workspace_id = target_workspace_id
         AND owner_member.conversation_id = target_conversation_id
         AND owner_member.member_role = 'owner'
     )
$function$;

RESET ROLE;

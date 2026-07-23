SET ROLE veltrix_owner;

REVOKE ALL ON FUNCTION security.prune_unauthorized_entity_conversation_members(uuid, uuid) FROM veltrix_app;
DROP FUNCTION IF EXISTS security.prune_unauthorized_entity_conversation_members(uuid, uuid);
DROP POLICY IF EXISTS entity_chat_owner_members_delete ON collaboration.conversation_members;

RESET ROLE;

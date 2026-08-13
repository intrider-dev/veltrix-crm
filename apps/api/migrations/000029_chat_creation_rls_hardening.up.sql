SET ROLE veltrix_owner;

-- INSERT ... RETURNING requires the newly created conversation to satisfy its
-- SELECT policy before the creator membership row exists. The bounded manage
-- predicate exposes only the actor's own owner-less bootstrap conversation.
DROP POLICY conversations_member_select ON collaboration.conversations;
CREATE POLICY conversations_member_select ON collaboration.conversations
  FOR SELECT TO veltrix_app
  USING (
    security.chat_conversation_visible(workspace_id, id)
    OR security.chat_conversation_manage_allowed(workspace_id, id)
  );

RESET ROLE;

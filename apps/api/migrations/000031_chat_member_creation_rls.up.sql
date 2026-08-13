SET ROLE veltrix_owner;

-- INSERT ... ON CONFLICT performs visibility checks before the new member row
-- is visible to the stable membership helper. During conversation bootstrap,
-- the creator's bounded manage permission supplies that visibility.
DROP POLICY conversation_members_member_select ON collaboration.conversation_members;
CREATE POLICY conversation_members_member_select ON collaboration.conversation_members
  FOR SELECT TO veltrix_app
  USING (
    security.chat_conversation_visible(workspace_id, conversation_id)
    OR security.chat_conversation_manage_allowed(workspace_id, conversation_id)
  );

RESET ROLE;

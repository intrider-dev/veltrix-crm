SET ROLE veltrix_owner;

-- INSERT ... RETURNING must authorize the new call before a stable helper can
-- observe it. The row-local branch is limited to the current conversation
-- member creating their own call.
DROP POLICY calls_member_select ON collaboration.calls;
CREATE POLICY calls_member_select ON collaboration.calls
  FOR SELECT TO veltrix_app
  USING (
    security.chat_call_visible(workspace_id, id)
    OR (
      workspace_id = security.current_workspace_id()
      AND created_by = security.current_actor_id()
      AND security.chat_conversation_visible(workspace_id, conversation_id)
    )
  );

RESET ROLE;

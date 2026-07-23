SET ROLE veltrix_owner;

DROP POLICY calls_member_update ON collaboration.calls;
CREATE POLICY calls_member_update ON collaboration.calls
  FOR UPDATE TO veltrix_app
  USING (security.chat_call_visible(workspace_id, id))
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_call_visible(workspace_id, id)
  );

RESET ROLE;

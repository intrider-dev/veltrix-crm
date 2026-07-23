SET ROLE veltrix_owner;

DROP POLICY calls_member_update ON collaboration.calls;
CREATE POLICY calls_member_update ON collaboration.calls
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND (
      created_by = security.current_actor_id()
      OR security.chat_call_visible(workspace_id, id)
    )
  )
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND (
      created_by = security.current_actor_id()
      OR security.chat_call_visible(workspace_id, id)
    )
  );

RESET ROLE;

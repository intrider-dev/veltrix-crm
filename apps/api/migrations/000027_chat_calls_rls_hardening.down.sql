SET ROLE veltrix_owner;

DROP TRIGGER IF EXISTS conversations_immutable_identity ON collaboration.conversations;
DROP TRIGGER IF EXISTS conversation_members_immutable_identity ON collaboration.conversation_members;
DROP TRIGGER IF EXISTS messages_immutable_identity ON collaboration.messages;
DROP TRIGGER IF EXISTS calls_immutable_identity ON collaboration.calls;
DROP TRIGGER IF EXISTS call_participants_immutable_identity ON collaboration.call_participants;

DROP POLICY IF EXISTS conversations_member_select ON collaboration.conversations;
DROP POLICY IF EXISTS conversations_actor_insert ON collaboration.conversations;
DROP POLICY IF EXISTS conversations_member_update ON collaboration.conversations;
DROP POLICY IF EXISTS conversations_owner_delete ON collaboration.conversations;
DROP POLICY IF EXISTS conversation_members_member_select ON collaboration.conversation_members;
DROP POLICY IF EXISTS conversation_members_owner_insert ON collaboration.conversation_members;
DROP POLICY IF EXISTS conversation_members_self_or_owner_update ON collaboration.conversation_members;
DROP POLICY IF EXISTS conversation_members_self_or_owner_delete ON collaboration.conversation_members;
DROP POLICY IF EXISTS messages_member_select ON collaboration.messages;
DROP POLICY IF EXISTS messages_sender_insert ON collaboration.messages;
DROP POLICY IF EXISTS messages_sender_update ON collaboration.messages;
DROP POLICY IF EXISTS messages_sender_delete ON collaboration.messages;
DROP POLICY IF EXISTS message_reactions_member_select ON collaboration.message_reactions;
DROP POLICY IF EXISTS message_reactions_self_insert ON collaboration.message_reactions;
DROP POLICY IF EXISTS message_reactions_self_delete ON collaboration.message_reactions;
DROP POLICY IF EXISTS pinned_messages_member_select ON collaboration.pinned_messages;
DROP POLICY IF EXISTS pinned_messages_member_insert ON collaboration.pinned_messages;
DROP POLICY IF EXISTS pinned_messages_member_delete ON collaboration.pinned_messages;

DROP POLICY IF EXISTS calls_member_select ON collaboration.calls;
DROP POLICY IF EXISTS calls_member_insert ON collaboration.calls;
DROP POLICY IF EXISTS calls_member_update ON collaboration.calls;
DROP POLICY IF EXISTS calls_creator_delete ON collaboration.calls;
DROP POLICY IF EXISTS call_participants_member_select ON collaboration.call_participants;
DROP POLICY IF EXISTS call_participants_creator_insert ON collaboration.call_participants;
DROP POLICY IF EXISTS call_participants_self_or_creator_update ON collaboration.call_participants;
DROP POLICY IF EXISTS call_participants_creator_delete ON collaboration.call_participants;

DROP POLICY IF EXISTS chat_owner_conversations_select ON collaboration.conversations;
DROP POLICY IF EXISTS chat_owner_members_select ON collaboration.conversation_members;
DROP POLICY IF EXISTS chat_owner_messages_select ON collaboration.messages;
DROP POLICY IF EXISTS chat_owner_calls_select ON collaboration.calls;
DROP POLICY IF EXISTS chat_owner_call_participants_select ON collaboration.call_participants;

DROP FUNCTION IF EXISTS security.enforce_chat_conversation_identity();
DROP FUNCTION IF EXISTS security.enforce_chat_member_identity();
DROP FUNCTION IF EXISTS security.enforce_chat_message_identity();
DROP FUNCTION IF EXISTS security.enforce_call_identity();
DROP FUNCTION IF EXISTS security.enforce_call_participant_identity();
DROP FUNCTION IF EXISTS security.chat_conversation_has_owner(uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_call_user_allowed(uuid, uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_call_created_by_actor(uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_call_visible(uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_message_visible(uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_conversation_manage_allowed(uuid, uuid);
DROP FUNCTION IF EXISTS security.chat_conversation_visible(uuid, uuid);

CREATE POLICY tenant_scope ON collaboration.conversations
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY tenant_scope ON collaboration.conversation_members
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY tenant_scope ON collaboration.messages
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY tenant_scope ON collaboration.message_reactions
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY tenant_scope ON collaboration.pinned_messages
  FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id())
  WITH CHECK (workspace_id = security.current_workspace_id());

CREATE POLICY calls_member_select ON collaboration.calls
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.conversation_members conversation_member
      WHERE conversation_member.workspace_id = calls.workspace_id
        AND conversation_member.conversation_id = calls.conversation_id
        AND conversation_member.user_id = security.current_actor_id()
    )
  );
CREATE POLICY calls_member_insert ON collaboration.calls
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND created_by = security.current_actor_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.conversation_members conversation_member
      WHERE conversation_member.workspace_id = calls.workspace_id
        AND conversation_member.conversation_id = calls.conversation_id
        AND conversation_member.user_id = security.current_actor_id()
    )
  );
CREATE POLICY calls_member_update ON collaboration.calls
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.conversation_members conversation_member
      WHERE conversation_member.workspace_id = calls.workspace_id
        AND conversation_member.conversation_id = calls.conversation_id
        AND conversation_member.user_id = security.current_actor_id()
    )
  )
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY calls_creator_delete ON collaboration.calls
  FOR DELETE TO veltrix_app
  USING (workspace_id = security.current_workspace_id() AND created_by = security.current_actor_id());

CREATE POLICY call_participants_member_select ON collaboration.call_participants
  FOR SELECT TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.calls call
      WHERE call.workspace_id = call_participants.workspace_id
        AND call.id = call_participants.call_id
    )
  );
CREATE POLICY call_participants_creator_insert ON collaboration.call_participants
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.calls call
      WHERE call.workspace_id = call_participants.workspace_id
        AND call.id = call_participants.call_id
        AND call.created_by = security.current_actor_id()
    )
  );
CREATE POLICY call_participants_self_or_creator_update ON collaboration.call_participants
  FOR UPDATE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND (
      user_id = security.current_actor_id()
      OR EXISTS (
        SELECT 1 FROM collaboration.calls call
        WHERE call.workspace_id = call_participants.workspace_id
          AND call.id = call_participants.call_id
          AND call.created_by = security.current_actor_id()
      )
    )
  )
  WITH CHECK (workspace_id = security.current_workspace_id());
CREATE POLICY call_participants_creator_delete ON collaboration.call_participants
  FOR DELETE TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND EXISTS (
      SELECT 1 FROM collaboration.calls call
      WHERE call.workspace_id = call_participants.workspace_id
        AND call.id = call_participants.call_id
        AND call.created_by = security.current_actor_id()
    )
  );

RESET ROLE;

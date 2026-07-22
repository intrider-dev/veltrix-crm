SET ROLE veltrix_owner;

-- SECURITY DEFINER predicates below are deliberately bounded to boolean
-- membership decisions. FORCE RLS requires explicit read-only owner policies;
-- the NOLOGIN owner role is not available to the request-serving process.
CREATE POLICY chat_owner_conversations_select ON collaboration.conversations
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY chat_owner_members_select ON collaboration.conversation_members
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY chat_owner_messages_select ON collaboration.messages
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY chat_owner_calls_select ON collaboration.calls
  FOR SELECT TO veltrix_owner USING (true);
CREATE POLICY chat_owner_call_participants_select ON collaboration.call_participants
  FOR SELECT TO veltrix_owner USING (true);

CREATE OR REPLACE FUNCTION security.chat_conversation_visible(
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
    FROM collaboration.conversation_members member
    JOIN tenancy.memberships active_membership
      ON active_membership.workspace_id = member.workspace_id
     AND active_membership.user_id = member.user_id
     AND active_membership.status = 'active'
    WHERE member.workspace_id = target_workspace_id
      AND member.conversation_id = target_conversation_id
      AND member.user_id = security.current_actor_id()
      AND target_workspace_id = security.current_workspace_id()
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_conversation_manage_allowed(
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
       FROM collaboration.conversations conversation
       JOIN tenancy.memberships active_membership
         ON active_membership.workspace_id = conversation.workspace_id
        AND active_membership.user_id = security.current_actor_id()
        AND active_membership.status = 'active'
       WHERE conversation.workspace_id = target_workspace_id
         AND conversation.id = target_conversation_id
         AND (
           EXISTS (
             SELECT 1
             FROM collaboration.conversation_members owner_member
             WHERE owner_member.workspace_id = conversation.workspace_id
               AND owner_member.conversation_id = conversation.id
               AND owner_member.user_id = security.current_actor_id()
               AND owner_member.member_role = 'owner'
           )
           OR (
             conversation.created_by = security.current_actor_id()
             AND NOT EXISTS (
               SELECT 1
               FROM collaboration.conversation_members existing_owner
               WHERE existing_owner.workspace_id = conversation.workspace_id
                 AND existing_owner.conversation_id = conversation.id
                 AND existing_owner.member_role = 'owner'
             )
           )
         )
     )
$function$;

CREATE OR REPLACE FUNCTION security.chat_message_visible(
  target_workspace_id uuid, target_message_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.messages message
    WHERE message.workspace_id = target_workspace_id
      AND message.id = target_message_id
      AND security.chat_conversation_visible(message.workspace_id, message.conversation_id)
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_call_visible(
  target_workspace_id uuid, target_call_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.calls call
    WHERE call.workspace_id = target_workspace_id
      AND call.id = target_call_id
      AND security.chat_conversation_visible(call.workspace_id, call.conversation_id)
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_call_created_by_actor(
  target_workspace_id uuid, target_call_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.calls call
    WHERE call.workspace_id = target_workspace_id
      AND call.id = target_call_id
      AND call.created_by = security.current_actor_id()
      AND security.chat_conversation_visible(call.workspace_id, call.conversation_id)
  )
$function$;

CREATE OR REPLACE FUNCTION security.chat_call_user_allowed(
  target_workspace_id uuid, target_call_id uuid, target_user_id uuid
)
RETURNS boolean
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog
AS $function$
  SELECT EXISTS (
    SELECT 1
    FROM collaboration.calls call
    JOIN collaboration.conversation_members member
      ON member.workspace_id = call.workspace_id
     AND member.conversation_id = call.conversation_id
     AND member.user_id = target_user_id
    JOIN tenancy.memberships active_membership
      ON active_membership.workspace_id = member.workspace_id
     AND active_membership.user_id = member.user_id
     AND active_membership.status = 'active'
    WHERE call.workspace_id = target_workspace_id
      AND call.id = target_call_id
      AND target_workspace_id = security.current_workspace_id()
  )
$function$;

REVOKE ALL ON FUNCTION security.chat_conversation_visible(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_conversation_manage_allowed(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_message_visible(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_call_visible(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_call_created_by_actor(uuid, uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION security.chat_call_user_allowed(uuid, uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION security.chat_conversation_visible(uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_conversation_manage_allowed(uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_message_visible(uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_call_visible(uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_call_created_by_actor(uuid, uuid) TO veltrix_app;
GRANT EXECUTE ON FUNCTION security.chat_call_user_allowed(uuid, uuid, uuid) TO veltrix_app;

DROP POLICY tenant_scope ON collaboration.conversations;
CREATE POLICY conversations_member_select ON collaboration.conversations
  FOR SELECT TO veltrix_app
  USING (security.chat_conversation_visible(workspace_id, id));
CREATE POLICY conversations_actor_insert ON collaboration.conversations
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND created_by = security.current_actor_id()
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = conversations.workspace_id
        AND membership.user_id = security.current_actor_id()
        AND membership.status = 'active'
    )
  );
CREATE POLICY conversations_member_update ON collaboration.conversations
  FOR UPDATE TO veltrix_app
  USING (security.chat_conversation_visible(workspace_id, id))
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_conversation_visible(workspace_id, id)
  );
CREATE POLICY conversations_owner_delete ON collaboration.conversations
  FOR DELETE TO veltrix_app
  USING (security.chat_conversation_manage_allowed(workspace_id, id));

DROP POLICY tenant_scope ON collaboration.conversation_members;
CREATE POLICY conversation_members_member_select ON collaboration.conversation_members
  FOR SELECT TO veltrix_app
  USING (security.chat_conversation_visible(workspace_id, conversation_id));
CREATE POLICY conversation_members_owner_insert ON collaboration.conversation_members
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_conversation_manage_allowed(workspace_id, conversation_id)
    AND EXISTS (
      SELECT 1 FROM tenancy.memberships membership
      WHERE membership.workspace_id = conversation_members.workspace_id
        AND membership.user_id = conversation_members.user_id
        AND membership.status = 'active'
    )
  );
CREATE POLICY conversation_members_self_or_owner_update ON collaboration.conversation_members
  FOR UPDATE TO veltrix_app
  USING (
    security.chat_conversation_visible(workspace_id, conversation_id)
    AND (user_id = security.current_actor_id()
      OR security.chat_conversation_manage_allowed(workspace_id, conversation_id))
  )
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
    AND (user_id = security.current_actor_id()
      OR security.chat_conversation_manage_allowed(workspace_id, conversation_id))
  );
CREATE POLICY conversation_members_self_or_owner_delete ON collaboration.conversation_members
  FOR DELETE TO veltrix_app
  USING (
    security.chat_conversation_visible(workspace_id, conversation_id)
    AND (user_id = security.current_actor_id()
      OR security.chat_conversation_manage_allowed(workspace_id, conversation_id))
  );

DROP POLICY tenant_scope ON collaboration.messages;
CREATE POLICY messages_member_select ON collaboration.messages
  FOR SELECT TO veltrix_app
  USING (security.chat_conversation_visible(workspace_id, conversation_id));
CREATE POLICY messages_sender_insert ON collaboration.messages
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND sender_user_id = security.current_actor_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
  );
CREATE POLICY messages_sender_update ON collaboration.messages
  FOR UPDATE TO veltrix_app
  USING (
    sender_user_id = security.current_actor_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
  )
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND sender_user_id = security.current_actor_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
  );
CREATE POLICY messages_sender_delete ON collaboration.messages
  FOR DELETE TO veltrix_app
  USING (
    sender_user_id = security.current_actor_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
  );

DROP POLICY tenant_scope ON collaboration.message_reactions;
CREATE POLICY message_reactions_member_select ON collaboration.message_reactions
  FOR SELECT TO veltrix_app
  USING (security.chat_message_visible(workspace_id, message_id));
CREATE POLICY message_reactions_self_insert ON collaboration.message_reactions
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND user_id = security.current_actor_id()
    AND security.chat_message_visible(workspace_id, message_id)
  );
CREATE POLICY message_reactions_self_delete ON collaboration.message_reactions
  FOR DELETE TO veltrix_app
  USING (
    user_id = security.current_actor_id()
    AND security.chat_message_visible(workspace_id, message_id)
  );

DROP POLICY tenant_scope ON collaboration.pinned_messages;
CREATE POLICY pinned_messages_member_select ON collaboration.pinned_messages
  FOR SELECT TO veltrix_app
  USING (security.chat_message_visible(workspace_id, message_id));
CREATE POLICY pinned_messages_member_insert ON collaboration.pinned_messages
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND pinned_by = security.current_actor_id()
    AND security.chat_message_visible(workspace_id, message_id)
  );
CREATE POLICY pinned_messages_member_delete ON collaboration.pinned_messages
  FOR DELETE TO veltrix_app
  USING (security.chat_message_visible(workspace_id, message_id));

DROP POLICY calls_member_select ON collaboration.calls;
DROP POLICY calls_member_insert ON collaboration.calls;
DROP POLICY calls_member_update ON collaboration.calls;
DROP POLICY calls_creator_delete ON collaboration.calls;
CREATE POLICY calls_member_select ON collaboration.calls
  FOR SELECT TO veltrix_app
  USING (security.chat_call_visible(workspace_id, id));
CREATE POLICY calls_member_insert ON collaboration.calls
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND created_by = security.current_actor_id()
    AND security.chat_conversation_visible(workspace_id, conversation_id)
  );
CREATE POLICY calls_member_update ON collaboration.calls
  FOR UPDATE TO veltrix_app
  USING (security.chat_call_visible(workspace_id, id))
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_call_visible(workspace_id, id)
  );
CREATE POLICY calls_creator_delete ON collaboration.calls
  FOR DELETE TO veltrix_app
  USING (
    created_by = security.current_actor_id()
    AND security.chat_call_visible(workspace_id, id)
  );

DROP POLICY call_participants_member_select ON collaboration.call_participants;
DROP POLICY call_participants_creator_insert ON collaboration.call_participants;
DROP POLICY call_participants_self_or_creator_update ON collaboration.call_participants;
DROP POLICY call_participants_creator_delete ON collaboration.call_participants;
CREATE POLICY call_participants_member_select ON collaboration.call_participants
  FOR SELECT TO veltrix_app
  USING (security.chat_call_visible(workspace_id, call_id));
CREATE POLICY call_participants_creator_insert ON collaboration.call_participants
  FOR INSERT TO veltrix_app
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_call_created_by_actor(workspace_id, call_id)
    AND security.chat_call_user_allowed(workspace_id, call_id, user_id)
  );
CREATE POLICY call_participants_self_or_creator_update ON collaboration.call_participants
  FOR UPDATE TO veltrix_app
  USING (
    security.chat_call_visible(workspace_id, call_id)
    AND (user_id = security.current_actor_id()
      OR security.chat_call_created_by_actor(workspace_id, call_id))
  )
  WITH CHECK (
    workspace_id = security.current_workspace_id()
    AND security.chat_call_visible(workspace_id, call_id)
    AND security.chat_call_user_allowed(workspace_id, call_id, user_id)
    AND (user_id = security.current_actor_id()
      OR security.chat_call_created_by_actor(workspace_id, call_id))
  );
CREATE POLICY call_participants_creator_delete ON collaboration.call_participants
  FOR DELETE TO veltrix_app
  USING (security.chat_call_created_by_actor(workspace_id, call_id));

CREATE OR REPLACE FUNCTION security.enforce_chat_conversation_identity()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
  IF ROW(NEW.workspace_id, NEW.id, NEW.conversation_type, NEW.direct_key, NEW.created_by, NEW.created_at)
     IS DISTINCT FROM
     ROW(OLD.workspace_id, OLD.id, OLD.conversation_type, OLD.direct_key, OLD.created_by, OLD.created_at) THEN
    RAISE EXCEPTION 'conversation identity fields are immutable' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION security.enforce_chat_member_identity()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
  IF ROW(NEW.workspace_id, NEW.conversation_id, NEW.user_id, NEW.member_role, NEW.joined_at)
     IS DISTINCT FROM
     ROW(OLD.workspace_id, OLD.conversation_id, OLD.user_id, OLD.member_role, OLD.joined_at) THEN
    RAISE EXCEPTION 'conversation member identity and role fields are immutable' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION security.enforce_chat_message_identity()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
  IF ROW(NEW.workspace_id, NEW.id, NEW.conversation_id, NEW.sender_user_id,
         NEW.message_kind, NEW.reply_to_message_id, NEW.created_at)
     IS DISTINCT FROM
     ROW(OLD.workspace_id, OLD.id, OLD.conversation_id, OLD.sender_user_id,
         OLD.message_kind, OLD.reply_to_message_id, OLD.created_at) THEN
    RAISE EXCEPTION 'message identity fields are immutable' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION security.enforce_call_identity()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
  IF ROW(NEW.workspace_id, NEW.id, NEW.conversation_id, NEW.room_name,
         NEW.call_kind, NEW.created_by, NEW.created_at)
     IS DISTINCT FROM
     ROW(OLD.workspace_id, OLD.id, OLD.conversation_id, OLD.room_name,
         OLD.call_kind, OLD.created_by, OLD.created_at) THEN
    RAISE EXCEPTION 'call identity fields are immutable' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;

CREATE OR REPLACE FUNCTION security.enforce_call_participant_identity()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog
AS $function$
BEGIN
  IF ROW(NEW.workspace_id, NEW.call_id, NEW.user_id, NEW.invited_at)
     IS DISTINCT FROM
     ROW(OLD.workspace_id, OLD.call_id, OLD.user_id, OLD.invited_at) THEN
    RAISE EXCEPTION 'call participant identity fields are immutable' USING ERRCODE = '42501';
  END IF;
  RETURN NEW;
END
$function$;

REVOKE ALL ON FUNCTION security.enforce_chat_conversation_identity() FROM PUBLIC;
REVOKE ALL ON FUNCTION security.enforce_chat_member_identity() FROM PUBLIC;
REVOKE ALL ON FUNCTION security.enforce_chat_message_identity() FROM PUBLIC;
REVOKE ALL ON FUNCTION security.enforce_call_identity() FROM PUBLIC;
REVOKE ALL ON FUNCTION security.enforce_call_participant_identity() FROM PUBLIC;

CREATE TRIGGER conversations_immutable_identity
  BEFORE UPDATE ON collaboration.conversations
  FOR EACH ROW EXECUTE FUNCTION security.enforce_chat_conversation_identity();
CREATE TRIGGER conversation_members_immutable_identity
  BEFORE UPDATE ON collaboration.conversation_members
  FOR EACH ROW EXECUTE FUNCTION security.enforce_chat_member_identity();
CREATE TRIGGER messages_immutable_identity
  BEFORE UPDATE ON collaboration.messages
  FOR EACH ROW EXECUTE FUNCTION security.enforce_chat_message_identity();
CREATE TRIGGER calls_immutable_identity
  BEFORE UPDATE ON collaboration.calls
  FOR EACH ROW EXECUTE FUNCTION security.enforce_call_identity();
CREATE TRIGGER call_participants_immutable_identity
  BEFORE UPDATE ON collaboration.call_participants
  FOR EACH ROW EXECUTE FUNCTION security.enforce_call_participant_identity();

RESET ROLE;

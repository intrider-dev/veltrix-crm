SET ROLE veltrix_owner;

CREATE TABLE collaboration.calls (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  room_name text NOT NULL CHECK (char_length(room_name) BETWEEN 1 AND 120),
  call_kind text NOT NULL CHECK (call_kind IN ('audio', 'video')),
  state text NOT NULL DEFAULT 'ringing' CHECK (state IN ('ringing', 'active', 'ended')),
  created_by uuid NOT NULL,
  started_at timestamptz,
  ended_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, room_name),
  FOREIGN KEY (workspace_id, conversation_id)
    REFERENCES collaboration.conversations(workspace_id, id),
  FOREIGN KEY (workspace_id, created_by)
    REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE UNIQUE INDEX calls_one_live_conversation_idx
  ON collaboration.calls (workspace_id, conversation_id)
  WHERE state IN ('ringing', 'active');
CREATE INDEX calls_recent_idx
  ON collaboration.calls (workspace_id, conversation_id, created_at DESC, id DESC);

CREATE TABLE collaboration.call_participants (
  workspace_id uuid NOT NULL,
  call_id uuid NOT NULL,
  user_id uuid NOT NULL,
  state text NOT NULL DEFAULT 'invited' CHECK (state IN ('invited', 'joined', 'declined', 'left')),
  invited_at timestamptz NOT NULL DEFAULT now(),
  joined_at timestamptz,
  left_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, call_id, user_id),
  FOREIGN KEY (workspace_id, call_id)
    REFERENCES collaboration.calls(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id)
);

CREATE INDEX call_participants_user_idx
  ON collaboration.call_participants (workspace_id, user_id, state, call_id);

ALTER TABLE collaboration.calls ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.calls FORCE ROW LEVEL SECURITY;
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

ALTER TABLE collaboration.call_participants ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.call_participants FORCE ROW LEVEL SECURITY;
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

GRANT SELECT, INSERT, UPDATE, DELETE ON collaboration.calls,
  collaboration.call_participants TO veltrix_app;

RESET ROLE;

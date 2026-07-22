SET ROLE veltrix_owner;

CREATE SCHEMA collaboration;

ALTER TABLE notifications.sse_events DROP CONSTRAINT sse_events_targeted_type_check;
ALTER TABLE notifications.sse_events ADD CONSTRAINT sse_events_targeted_type_check
  CHECK (
    recipient_user_id IS NOT NULL
    OR NOT (event_type = 'notification.created' OR event_type LIKE 'chat.%' OR event_type LIKE 'call.%' OR event_type LIKE 'mail.%')
  );

DROP POLICY tenant_scope ON notifications.sse_events;
CREATE POLICY tenant_scope ON notifications.sse_events
  FOR ALL TO veltrix_app
  USING (
    workspace_id = security.current_workspace_id()
    AND (recipient_user_id IS NULL OR recipient_user_id = security.current_actor_id())
  )
  WITH CHECK (workspace_id = security.current_workspace_id());

CREATE TABLE collaboration.conversations (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  conversation_type text NOT NULL CHECK (conversation_type IN ('direct', 'group')),
  title text NOT NULL DEFAULT '' CHECK (char_length(title) <= 160),
  direct_key text,
  created_by uuid NOT NULL,
  last_message_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  FOREIGN KEY (workspace_id, created_by)
    REFERENCES tenancy.memberships(workspace_id, user_id),
  CHECK ((conversation_type = 'direct' AND direct_key IS NOT NULL)
      OR (conversation_type = 'group' AND direct_key IS NULL))
);

CREATE UNIQUE INDEX conversations_direct_unique_idx
  ON collaboration.conversations (workspace_id, direct_key)
  WHERE conversation_type = 'direct' AND archived_at IS NULL;
CREATE INDEX conversations_recent_idx
  ON collaboration.conversations (workspace_id, COALESCE(last_message_at, created_at) DESC, id DESC)
  WHERE archived_at IS NULL;

CREATE TABLE collaboration.conversation_members (
  workspace_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  user_id uuid NOT NULL,
  member_role text NOT NULL DEFAULT 'member' CHECK (member_role IN ('owner', 'member')),
  muted boolean NOT NULL DEFAULT false,
  last_read_at timestamptz NOT NULL DEFAULT now(),
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, conversation_id, user_id),
  FOREIGN KEY (workspace_id, conversation_id)
    REFERENCES collaboration.conversations(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE
);

CREATE INDEX conversation_members_user_idx
  ON collaboration.conversation_members (workspace_id, user_id, conversation_id);

CREATE TABLE collaboration.messages (
  workspace_id uuid NOT NULL,
  id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  sender_user_id uuid NOT NULL,
  message_kind text NOT NULL DEFAULT 'text' CHECK (message_kind IN ('text', 'system', 'file', 'voice')),
  body text NOT NULL DEFAULT '' CHECK (char_length(body) <= 10000),
  reply_to_message_id uuid,
  edited_at timestamptz,
  deleted_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, conversation_id, id),
  FOREIGN KEY (workspace_id, conversation_id)
    REFERENCES collaboration.conversations(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, sender_user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id),
  FOREIGN KEY (workspace_id, conversation_id, reply_to_message_id)
    REFERENCES collaboration.messages(workspace_id, conversation_id, id)
);

CREATE INDEX messages_conversation_cursor_idx
  ON collaboration.messages (workspace_id, conversation_id, created_at DESC, id DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE collaboration.message_reactions (
  workspace_id uuid NOT NULL,
  message_id uuid NOT NULL,
  user_id uuid NOT NULL,
  emoji text NOT NULL CHECK (char_length(emoji) BETWEEN 1 AND 32),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, message_id, user_id, emoji),
  FOREIGN KEY (workspace_id, message_id)
    REFERENCES collaboration.messages(workspace_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE
);

CREATE TABLE collaboration.pinned_messages (
  workspace_id uuid NOT NULL,
  conversation_id uuid NOT NULL,
  message_id uuid NOT NULL,
  pinned_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, conversation_id, message_id),
  FOREIGN KEY (workspace_id, conversation_id, message_id)
    REFERENCES collaboration.messages(workspace_id, conversation_id, id) ON DELETE CASCADE,
  FOREIGN KEY (workspace_id, pinned_by)
    REFERENCES tenancy.memberships(workspace_id, user_id)
);

ALTER TABLE files.attachments DROP CONSTRAINT attachments_entity_type_check;
ALTER TABLE files.attachments ADD CONSTRAINT attachments_entity_type_check
  CHECK (entity_type IN ('contact', 'company', 'deal', 'activity', 'project', 'chat_message', 'import'));

ALTER TABLE collaboration.conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.conversations FORCE ROW LEVEL SECURITY;
ALTER TABLE collaboration.conversation_members ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.conversation_members FORCE ROW LEVEL SECURITY;
ALTER TABLE collaboration.messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.messages FORCE ROW LEVEL SECURITY;
ALTER TABLE collaboration.message_reactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.message_reactions FORCE ROW LEVEL SECURITY;
ALTER TABLE collaboration.pinned_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE collaboration.pinned_messages FORCE ROW LEVEL SECURITY;

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

GRANT USAGE ON SCHEMA collaboration TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA collaboration TO veltrix_app;

RESET ROLE;

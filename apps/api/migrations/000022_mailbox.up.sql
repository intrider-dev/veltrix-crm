SET ROLE veltrix_owner;

CREATE SCHEMA mailbox AUTHORIZATION veltrix_owner;

CREATE TABLE mailbox.accounts (
  workspace_id uuid NOT NULL REFERENCES tenancy.workspaces(id) ON DELETE CASCADE,
  id uuid NOT NULL,
  user_id uuid NOT NULL,
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 160),
  email text NOT NULL CHECK (char_length(email) BETWEEN 3 AND 254),
  username text NOT NULL CHECK (char_length(username) BETWEEN 1 AND 320),
  imap_host text NOT NULL CHECK (char_length(imap_host) BETWEEN 1 AND 253),
  imap_port integer NOT NULL CHECK (imap_port BETWEEN 1 AND 65535),
  imap_security text NOT NULL CHECK (imap_security IN ('tls', 'starttls')),
  smtp_host text NOT NULL CHECK (char_length(smtp_host) BETWEEN 1 AND 253),
  smtp_port integer NOT NULL CHECK (smtp_port BETWEEN 1 AND 65535),
  smtp_security text NOT NULL CHECK (smtp_security IN ('tls', 'starttls')),
  credential_ciphertext bytea NOT NULL CHECK (octet_length(credential_ciphertext) BETWEEN 17 AND 16384),
  credential_nonce bytea NOT NULL CHECK (octet_length(credential_nonce) = 12),
  key_id text NOT NULL CHECK (char_length(key_id) BETWEEN 1 AND 64),
  sync_enabled boolean NOT NULL DEFAULT true,
  sync_state text NOT NULL DEFAULT 'pending' CHECK (sync_state IN ('pending', 'syncing', 'ready', 'error', 'disabled')),
  last_sync_at timestamptz,
  next_sync_at timestamptz,
  last_error_code text CHECK (last_error_code IS NULL OR char_length(last_error_code) <= 120),
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, user_id, id),
  UNIQUE (workspace_id, user_id, email),
  FOREIGN KEY (workspace_id, user_id)
    REFERENCES tenancy.memberships(workspace_id, user_id) ON DELETE CASCADE
);

CREATE INDEX mailbox_accounts_user_idx
  ON mailbox.accounts (workspace_id, user_id, updated_at DESC, id DESC);
CREATE INDEX mailbox_accounts_sync_idx
  ON mailbox.accounts (workspace_id, next_sync_at, id)
  WHERE sync_enabled AND sync_state <> 'disabled';

CREATE TABLE mailbox.folders (
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  account_id uuid NOT NULL,
  id uuid NOT NULL,
  remote_name text NOT NULL CHECK (char_length(remote_name) BETWEEN 1 AND 1000),
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 1000),
  delimiter text CHECK (delimiter IS NULL OR char_length(delimiter) <= 4),
  special_use text CHECK (special_use IS NULL OR special_use IN ('inbox', 'sent', 'drafts', 'trash', 'archive', 'junk')),
  sync_enabled boolean NOT NULL DEFAULT true,
  uid_validity bigint CHECK (uid_validity IS NULL OR uid_validity BETWEEN 1 AND 4294967295),
  uid_next bigint CHECK (uid_next IS NULL OR uid_next BETWEEN 1 AND 4294967296),
  highest_uid bigint NOT NULL DEFAULT 0 CHECK (highest_uid BETWEEN 0 AND 4294967295),
  total_count integer NOT NULL DEFAULT 0 CHECK (total_count >= 0),
  unread_count integer NOT NULL DEFAULT 0 CHECK (unread_count >= 0),
  last_sync_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, user_id, account_id, id),
  UNIQUE (workspace_id, user_id, account_id, remote_name),
  FOREIGN KEY (workspace_id, user_id, account_id)
    REFERENCES mailbox.accounts(workspace_id, user_id, id) ON DELETE CASCADE
);

CREATE INDEX mailbox_folders_account_idx
  ON mailbox.folders (workspace_id, user_id, account_id, special_use, display_name, id);

CREATE TABLE mailbox.messages (
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  account_id uuid NOT NULL,
  folder_id uuid NOT NULL,
  id uuid NOT NULL,
  uid_validity bigint NOT NULL CHECK (uid_validity BETWEEN 1 AND 4294967295),
  remote_uid bigint NOT NULL CHECK (remote_uid BETWEEN 1 AND 4294967295),
  internet_message_id text CHECK (internet_message_id IS NULL OR char_length(internet_message_id) <= 998),
  subject text NOT NULL DEFAULT '' CHECK (char_length(subject) <= 2000),
  sender_name text NOT NULL DEFAULT '' CHECK (char_length(sender_name) <= 320),
  sender_address text NOT NULL DEFAULT '' CHECK (char_length(sender_address) <= 320),
  recipients jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(recipients) = 'array' AND octet_length(recipients::text) <= 32768),
  sent_at timestamptz,
  received_at timestamptz NOT NULL,
  flags text[] NOT NULL DEFAULT '{}',
  size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes BETWEEN 0 AND 104857600),
  snippet text NOT NULL DEFAULT '' CHECK (char_length(snippet) <= 500),
  has_attachments boolean NOT NULL DEFAULT false,
  body_state text NOT NULL DEFAULT 'missing' CHECK (body_state IN ('missing', 'queued', 'ready', 'error')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, user_id, account_id, id),
  UNIQUE (workspace_id, user_id, account_id, folder_id, uid_validity, remote_uid),
  FOREIGN KEY (workspace_id, user_id, account_id, folder_id)
    REFERENCES mailbox.folders(workspace_id, user_id, account_id, id) ON DELETE CASCADE
);

CREATE INDEX mailbox_messages_folder_cursor_idx
  ON mailbox.messages (workspace_id, user_id, folder_id, received_at DESC, id DESC);

CREATE TABLE mailbox.message_bodies (
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  account_id uuid NOT NULL,
  message_id uuid NOT NULL,
  plain_text text NOT NULL CHECK (octet_length(plain_text) <= 2097152),
  fetched_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, message_id),
  FOREIGN KEY (workspace_id, user_id, account_id, message_id)
    REFERENCES mailbox.messages(workspace_id, user_id, account_id, id) ON DELETE CASCADE
);

CREATE INDEX mailbox_message_bodies_owner_idx
  ON mailbox.message_bodies (workspace_id, user_id, account_id, fetched_at DESC);

CREATE TABLE mailbox.message_parts (
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  account_id uuid NOT NULL,
  message_id uuid NOT NULL,
  part_spec text NOT NULL CHECK (part_spec ~ '^[0-9]+(\\.[0-9]+)*$'),
  display_name text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 255),
  media_type text NOT NULL CHECK (char_length(media_type) BETWEEN 1 AND 120),
  size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 0 AND 104857600),
  content_id text CHECK (content_id IS NULL OR char_length(content_id) <= 998),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, message_id, part_spec),
  FOREIGN KEY (workspace_id, user_id, account_id, message_id)
    REFERENCES mailbox.messages(workspace_id, user_id, account_id, id) ON DELETE CASCADE
);

CREATE INDEX mailbox_message_parts_owner_idx
  ON mailbox.message_parts (workspace_id, user_id, account_id, message_id);

CREATE TABLE mailbox.outgoing_messages (
  workspace_id uuid NOT NULL,
  user_id uuid NOT NULL,
  account_id uuid NOT NULL,
  id uuid NOT NULL,
  internet_message_id text NOT NULL CHECK (char_length(internet_message_id) BETWEEN 3 AND 998),
  recipients jsonb NOT NULL CHECK (jsonb_typeof(recipients) = 'object' AND octet_length(recipients::text) <= 32768),
  subject text NOT NULL DEFAULT '' CHECK (char_length(subject) <= 2000),
  plain_text text NOT NULL CHECK (octet_length(plain_text) <= 2097152),
  state text NOT NULL DEFAULT 'queued' CHECK (state IN ('queued', 'sending', 'sent', 'failed', 'dead')),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 20),
  last_error_code text CHECK (last_error_code IS NULL OR char_length(last_error_code) <= 120),
  sent_at timestamptz,
  version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, id),
  UNIQUE (workspace_id, user_id, id),
  UNIQUE (workspace_id, user_id, internet_message_id),
  FOREIGN KEY (workspace_id, user_id, account_id)
    REFERENCES mailbox.accounts(workspace_id, user_id, id) ON DELETE CASCADE
);

CREATE INDEX mailbox_outgoing_owner_idx
  ON mailbox.outgoing_messages (workspace_id, user_id, created_at DESC, id DESC);

DO $rls$
DECLARE target regclass;
BEGIN
  FOREACH target IN ARRAY ARRAY[
    'mailbox.accounts'::regclass,
    'mailbox.folders'::regclass,
    'mailbox.messages'::regclass,
    'mailbox.message_bodies'::regclass,
    'mailbox.message_parts'::regclass,
    'mailbox.outgoing_messages'::regclass
  ] LOOP
    EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY', target);
    EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY', target);
    EXECUTE format(
      'CREATE POLICY actor_scope ON %s FOR ALL TO veltrix_app USING (workspace_id = security.current_workspace_id() AND user_id = security.current_actor_id()) WITH CHECK (workspace_id = security.current_workspace_id() AND user_id = security.current_actor_id() AND EXISTS (SELECT 1 FROM tenancy.memberships membership WHERE membership.workspace_id = security.current_workspace_id() AND membership.user_id = security.current_actor_id() AND membership.status = ''active''))',
      target
    );
  END LOOP;
END
$rls$;

GRANT USAGE ON SCHEMA mailbox TO veltrix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mailbox TO veltrix_app;

RESET ROLE;

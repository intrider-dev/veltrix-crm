-- name: CreateMailboxAccount :one
INSERT INTO mailbox.accounts (
  workspace_id, id, user_id, display_name, email, username,
  imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
  credential_ciphertext, credential_nonce, key_id, sync_enabled, next_sync_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(display_name),
  sqlc.arg(email), sqlc.arg(username), sqlc.arg(imap_host), sqlc.arg(imap_port),
  sqlc.arg(imap_security), sqlc.arg(smtp_host), sqlc.arg(smtp_port),
  sqlc.arg(smtp_security), sqlc.arg(credential_ciphertext), sqlc.arg(credential_nonce),
  sqlc.arg(key_id), sqlc.arg(sync_enabled), now()
)
RETURNING workspace_id, id, user_id, display_name, email, username,
          imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
          sync_enabled, sync_state, last_sync_at, next_sync_at, last_error_code,
          version, created_at, updated_at;

-- name: ListMailboxAccounts :many
SELECT workspace_id, id, user_id, display_name, email, username,
       imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
       sync_enabled, sync_state, last_sync_at, next_sync_at, last_error_code,
       version, created_at, updated_at
FROM mailbox.accounts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
ORDER BY updated_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetMailboxAccount :one
SELECT workspace_id, id, user_id, display_name, email, username,
       imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
       sync_enabled, sync_state, last_sync_at, next_sync_at, last_error_code,
       version, created_at, updated_at
FROM mailbox.accounts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id);

-- name: GetMailboxAccountSecret :one
SELECT workspace_id, id, user_id, display_name, email, username,
       imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
       credential_ciphertext, credential_nonce, key_id, sync_enabled, sync_state,
       last_sync_at, next_sync_at, last_error_code, version, created_at, updated_at
FROM mailbox.accounts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id);

-- name: UpdateMailboxAccount :one
UPDATE mailbox.accounts
SET display_name = sqlc.arg(display_name),
    email = sqlc.arg(email),
    username = sqlc.arg(username),
    imap_host = sqlc.arg(imap_host),
    imap_port = sqlc.arg(imap_port),
    imap_security = sqlc.arg(imap_security),
    smtp_host = sqlc.arg(smtp_host),
    smtp_port = sqlc.arg(smtp_port),
    smtp_security = sqlc.arg(smtp_security),
    sync_enabled = sqlc.arg(sync_enabled),
    sync_state = CASE WHEN sqlc.arg(sync_enabled)::boolean THEN 'pending' ELSE 'disabled' END,
    next_sync_at = CASE WHEN sqlc.arg(sync_enabled)::boolean THEN now() ELSE NULL END,
    last_error_code = NULL,
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
RETURNING workspace_id, id, user_id, display_name, email, username,
          imap_host, imap_port, imap_security, smtp_host, smtp_port, smtp_security,
          sync_enabled, sync_state, last_sync_at, next_sync_at, last_error_code,
          version, created_at, updated_at;

-- name: ReplaceMailboxCredential :execrows
UPDATE mailbox.accounts
SET credential_ciphertext = sqlc.arg(credential_ciphertext),
    credential_nonce = sqlc.arg(credential_nonce),
    key_id = sqlc.arg(key_id),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version);

-- name: DeleteMailboxAccount :execrows
DELETE FROM mailbox.accounts
WHERE workspace_id = sqlc.arg(workspace_id)
  AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version);

-- name: MarkMailboxSyncStarted :execrows
UPDATE mailbox.accounts
SET sync_state = 'syncing', last_error_code = NULL, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id) AND sync_enabled
  AND (sync_state <> 'syncing' OR updated_at < now() - interval '2 minutes');

-- name: MarkMailboxSyncFinished :exec
UPDATE mailbox.accounts
SET sync_state = 'ready', last_sync_at = now(), next_sync_at = now() + interval '2 minutes',
    last_error_code = NULL, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id) AND id = sqlc.arg(id);

-- name: MarkMailboxSyncFailed :exec
UPDATE mailbox.accounts
SET sync_state = 'error', next_sync_at = now() + interval '5 minutes',
    last_error_code = sqlc.arg(error_code), updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id) AND id = sqlc.arg(id);

-- name: UpsertMailboxFolder :one
INSERT INTO mailbox.folders (
  workspace_id, user_id, account_id, id, remote_name, display_name, delimiter,
  special_use, uid_validity, uid_next, total_count, unread_count, last_sync_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(account_id), sqlc.arg(id),
  sqlc.arg(remote_name), sqlc.arg(display_name), sqlc.narg(delimiter),
  sqlc.narg(special_use), sqlc.narg(uid_validity), sqlc.narg(uid_next),
  sqlc.arg(total_count), sqlc.arg(unread_count), now()
)
ON CONFLICT (workspace_id, user_id, account_id, remote_name) DO UPDATE
SET display_name = EXCLUDED.display_name, delimiter = EXCLUDED.delimiter,
    special_use = EXCLUDED.special_use, uid_validity = EXCLUDED.uid_validity,
    uid_next = EXCLUDED.uid_next, total_count = EXCLUDED.total_count,
    unread_count = EXCLUDED.unread_count, last_sync_at = now(), updated_at = now()
RETURNING workspace_id, user_id, account_id, id, remote_name, display_name,
          delimiter, special_use, sync_enabled, uid_validity, uid_next, highest_uid,
          total_count, unread_count, last_sync_at, created_at, updated_at;

-- name: ListMailboxFolders :many
SELECT workspace_id, user_id, account_id, id, remote_name, display_name,
       delimiter, special_use, sync_enabled, uid_validity, uid_next, highest_uid,
       total_count, unread_count, last_sync_at, created_at, updated_at
FROM mailbox.folders
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND account_id = sqlc.arg(account_id)
ORDER BY CASE special_use WHEN 'inbox' THEN 0 WHEN 'sent' THEN 1 WHEN 'drafts' THEN 2
  WHEN 'archive' THEN 3 WHEN 'trash' THEN 4 WHEN 'junk' THEN 5 ELSE 6 END,
  display_name, id;

-- name: GetMailboxFolder :one
SELECT workspace_id, user_id, account_id, id, remote_name, display_name,
       delimiter, special_use, sync_enabled, uid_validity, uid_next, highest_uid,
       total_count, unread_count, last_sync_at, created_at, updated_at
FROM mailbox.folders
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id);

-- name: UpsertMailboxMessage :one
INSERT INTO mailbox.messages (
  workspace_id, user_id, account_id, folder_id, id, uid_validity, remote_uid,
  internet_message_id, subject, sender_name, sender_address, recipients,
  sent_at, received_at, flags, size_bytes, snippet, has_attachments
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(account_id), sqlc.arg(folder_id),
  sqlc.arg(id), sqlc.arg(uid_validity), sqlc.arg(remote_uid), sqlc.narg(internet_message_id),
  sqlc.arg(subject), sqlc.arg(sender_name), sqlc.arg(sender_address), sqlc.arg(recipients),
  sqlc.narg(sent_at), sqlc.arg(received_at), sqlc.arg(flags), sqlc.arg(size_bytes),
  sqlc.arg(snippet), sqlc.arg(has_attachments)
)
ON CONFLICT (workspace_id, user_id, account_id, folder_id, uid_validity, remote_uid) DO UPDATE
SET internet_message_id = EXCLUDED.internet_message_id, subject = EXCLUDED.subject,
    sender_name = EXCLUDED.sender_name, sender_address = EXCLUDED.sender_address,
    recipients = EXCLUDED.recipients, sent_at = EXCLUDED.sent_at,
    received_at = EXCLUDED.received_at, flags = EXCLUDED.flags,
    size_bytes = EXCLUDED.size_bytes, snippet = EXCLUDED.snippet,
    has_attachments = EXCLUDED.has_attachments, updated_at = now()
RETURNING workspace_id, user_id, account_id, folder_id, id, uid_validity,
          remote_uid, internet_message_id, subject, sender_name, sender_address,
          recipients, sent_at, received_at, flags, size_bytes, snippet,
          has_attachments, body_state, created_at, updated_at;

-- name: UpdateMailboxFolderHighWater :exec
UPDATE mailbox.folders
SET highest_uid = GREATEST(highest_uid, sqlc.arg(highest_uid)),
    uid_validity = sqlc.arg(uid_validity), uid_next = sqlc.narg(uid_next),
    last_sync_at = now(), updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND account_id = sqlc.arg(account_id) AND id = sqlc.arg(id);

-- name: ResetMailboxFolderForUIDValidity :exec
DELETE FROM mailbox.messages
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND account_id = sqlc.arg(account_id) AND folder_id = sqlc.arg(folder_id)
  AND uid_validity <> sqlc.arg(uid_validity);

-- name: ListMailboxMessages :many
SELECT workspace_id, user_id, account_id, folder_id, id, uid_validity,
       remote_uid, internet_message_id, subject, sender_name, sender_address,
       recipients, sent_at, received_at, flags, size_bytes, snippet,
       has_attachments, body_state, created_at, updated_at
FROM mailbox.messages
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND folder_id = sqlc.arg(folder_id)
  AND (sqlc.narg(cursor_time)::timestamptz IS NULL
       OR (received_at, id) < (sqlc.narg(cursor_time)::timestamptz, sqlc.narg(cursor_id)::uuid))
ORDER BY received_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetMailboxMessage :one
SELECT workspace_id, user_id, account_id, folder_id, id, uid_validity,
       remote_uid, internet_message_id, subject, sender_name, sender_address,
       recipients, sent_at, received_at, flags, size_bytes, snippet,
       has_attachments, body_state, created_at, updated_at
FROM mailbox.messages
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id);

-- name: StoreMailboxMessageBody :exec
WITH stored AS (
  INSERT INTO mailbox.message_bodies (workspace_id, user_id, account_id, message_id, plain_text)
  VALUES (sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(account_id), sqlc.arg(message_id), sqlc.arg(plain_text))
  ON CONFLICT (workspace_id, message_id) DO UPDATE
    SET plain_text = EXCLUDED.plain_text, fetched_at = now()
)
UPDATE mailbox.messages AS message SET body_state = 'ready', updated_at = now()
WHERE message.workspace_id = sqlc.arg(workspace_id) AND message.user_id = sqlc.arg(user_id)
  AND message.id = sqlc.arg(message_id);

-- name: GetMailboxMessageBody :one
SELECT plain_text, fetched_at
FROM mailbox.message_bodies
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND message_id = sqlc.arg(message_id);

-- name: CreateMailboxOutgoing :one
INSERT INTO mailbox.outgoing_messages (
  workspace_id, user_id, account_id, id, internet_message_id, recipients, subject, plain_text
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(user_id), sqlc.arg(account_id), sqlc.arg(id),
  sqlc.arg(internet_message_id), sqlc.arg(recipients), sqlc.arg(subject), sqlc.arg(plain_text)
)
RETURNING workspace_id, user_id, account_id, id, internet_message_id, recipients,
          subject, plain_text, state, attempts, last_error_code, sent_at,
          version, created_at, updated_at;

-- name: EnqueueMailboxOutgoingDelivery :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  state, attempts, max_attempts, available_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(job_id), 'mailbox.outgoing.deliver', 1,
  sqlc.arg(outgoing_id)::text,
  jsonb_build_object(
    'outgoingId', sqlc.arg(outgoing_id)::text,
    'actorUserId', sqlc.arg(user_id)::text
  ),
  'ready', 0, 5, now()
)
ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING;

-- name: GetMailboxOutgoing :one
SELECT workspace_id, user_id, account_id, id, internet_message_id, recipients,
       subject, plain_text, state, attempts, last_error_code, sent_at,
       version, created_at, updated_at
FROM mailbox.outgoing_messages
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id);

-- name: ClaimMailboxOutgoingDelivery :one
UPDATE mailbox.outgoing_messages
SET state = 'sending', attempts = attempts + 1, last_error_code = NULL,
    version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id) AND state IN ('queued', 'failed') AND attempts < 5
RETURNING workspace_id, user_id, account_id, id, internet_message_id, recipients,
          subject, plain_text, state, attempts, last_error_code, sent_at,
          version, created_at, updated_at;

-- name: MarkMailboxOutgoingSent :execrows
UPDATE mailbox.outgoing_messages
SET state = 'sent', sent_at = now(), last_error_code = NULL,
    version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id) AND state = 'sending';

-- name: MarkMailboxOutgoingFailed :execrows
UPDATE mailbox.outgoing_messages
SET state = CASE WHEN sqlc.arg(terminal)::boolean THEN 'dead' ELSE 'failed' END,
    last_error_code = sqlc.arg(error_code),
    version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id) AND user_id = sqlc.arg(user_id)
  AND id = sqlc.arg(id) AND state = 'sending';

-- name: FindDirectConversation :one
SELECT id
FROM collaboration.conversations
WHERE workspace_id = sqlc.arg(workspace_id)
  AND conversation_type = 'direct'
  AND direct_key = sqlc.arg(direct_key)
  AND archived_at IS NULL;

-- name: CreateConversation :one
INSERT INTO collaboration.conversations (
  workspace_id, id, conversation_type, title, direct_key, created_by
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(conversation_id), sqlc.arg(conversation_type),
  sqlc.arg(title), sqlc.narg(direct_key), sqlc.arg(created_by)
)
RETURNING id, conversation_type, title, last_message_at, version, created_at, updated_at;

-- name: AddConversationMember :exec
INSERT INTO collaboration.conversation_members (
  workspace_id, conversation_id, user_id, member_role
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(conversation_id), sqlc.arg(user_id), sqlc.arg(member_role)
)
ON CONFLICT (workspace_id, conversation_id, user_id) DO NOTHING;

-- name: ActiveWorkspaceUserExists :one
SELECT EXISTS (
  SELECT 1 FROM tenancy.memberships
  WHERE workspace_id = sqlc.arg(workspace_id)
    AND user_id = sqlc.arg(user_id)
    AND status = 'active'
);

-- name: ConversationMemberExists :one
SELECT EXISTS (
  SELECT 1 FROM collaboration.conversation_members
  WHERE workspace_id = sqlc.arg(workspace_id)
    AND conversation_id = sqlc.arg(conversation_id)
    AND user_id = sqlc.arg(user_id)
);

-- name: ListUserConversations :many
SELECT conversation.id, conversation.conversation_type, conversation.title,
       conversation.last_message_at, conversation.version,
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'userId', member.user_id,
           'displayName', users.display_name,
           'role', member.member_role
         ) ORDER BY users.display_name, member.user_id)
         FROM collaboration.conversation_members member
         JOIN identity.users users ON users.id = member.user_id
         WHERE member.workspace_id = conversation.workspace_id
           AND member.conversation_id = conversation.id
       ), '[]'::jsonb)::text AS members,
       (SELECT count(*)
        FROM collaboration.messages message
        WHERE message.workspace_id = conversation.workspace_id
          AND message.conversation_id = conversation.id
          AND message.deleted_at IS NULL
          AND message.created_at > own_member.last_read_at
          AND message.sender_user_id <> sqlc.arg(user_id))::bigint AS unread_count,
       conversation.created_at, conversation.updated_at
FROM collaboration.conversations conversation
JOIN collaboration.conversation_members own_member
  ON own_member.workspace_id = conversation.workspace_id
 AND own_member.conversation_id = conversation.id
 AND own_member.user_id = sqlc.arg(user_id)
WHERE conversation.workspace_id = sqlc.arg(workspace_id)
  AND conversation.archived_at IS NULL
ORDER BY COALESCE(conversation.last_message_at, conversation.created_at) DESC, conversation.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetUserConversation :one
SELECT conversation.id, conversation.conversation_type, conversation.title,
       conversation.last_message_at, conversation.version,
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'userId', member.user_id,
           'displayName', users.display_name,
           'role', member.member_role
         ) ORDER BY users.display_name, member.user_id)
         FROM collaboration.conversation_members member
         JOIN identity.users users ON users.id = member.user_id
         WHERE member.workspace_id = conversation.workspace_id
           AND member.conversation_id = conversation.id
       ), '[]'::jsonb)::text AS members,
       (SELECT count(*)
        FROM collaboration.messages message
        WHERE message.workspace_id = conversation.workspace_id
          AND message.conversation_id = conversation.id
          AND message.deleted_at IS NULL
          AND message.created_at > own_member.last_read_at
          AND message.sender_user_id <> sqlc.arg(user_id))::bigint AS unread_count,
       conversation.created_at, conversation.updated_at
FROM collaboration.conversations conversation
JOIN collaboration.conversation_members own_member
  ON own_member.workspace_id = conversation.workspace_id
 AND own_member.conversation_id = conversation.id
 AND own_member.user_id = sqlc.arg(user_id)
WHERE conversation.workspace_id = sqlc.arg(workspace_id)
  AND conversation.id = sqlc.arg(conversation_id)
  AND conversation.archived_at IS NULL;

-- name: ListConversationMessages :many
SELECT message.id, message.conversation_id, message.sender_user_id,
       users.display_name AS sender_display_name, message.message_kind,
       message.body, message.reply_to_message_id, message.edited_at,
       message.version, message.created_at,
       EXISTS (
         SELECT 1 FROM collaboration.pinned_messages pin
         WHERE pin.workspace_id = message.workspace_id
           AND pin.conversation_id = message.conversation_id
           AND pin.message_id = message.id
       ) AS pinned,
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'emoji', reaction.emoji,
           'userId', reaction.user_id
         ) ORDER BY reaction.created_at, reaction.user_id)
         FROM collaboration.message_reactions reaction
         WHERE reaction.workspace_id = message.workspace_id
           AND reaction.message_id = message.id
       ), '[]'::jsonb)::text AS reactions
FROM collaboration.messages message
JOIN identity.users users ON users.id = message.sender_user_id
JOIN collaboration.conversation_members own_member
  ON own_member.workspace_id = message.workspace_id
 AND own_member.conversation_id = message.conversation_id
 AND own_member.user_id = sqlc.arg(user_id)
WHERE message.workspace_id = sqlc.arg(workspace_id)
  AND message.conversation_id = sqlc.arg(conversation_id)
  AND message.deleted_at IS NULL
  AND (message.created_at, message.id) <
      (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY message.created_at DESC, message.id DESC
LIMIT sqlc.arg(page_limit);

-- name: CreateMessage :one
WITH created AS (
  INSERT INTO collaboration.messages (
    workspace_id, id, conversation_id, sender_user_id,
    message_kind, body, reply_to_message_id
  )
  SELECT sqlc.arg(workspace_id), sqlc.arg(message_id), sqlc.arg(conversation_id),
         sqlc.arg(sender_user_id), sqlc.arg(message_kind), sqlc.arg(body),
         sqlc.narg(reply_to_message_id)
  WHERE EXISTS (
    SELECT 1 FROM collaboration.conversation_members
    WHERE workspace_id = sqlc.arg(workspace_id)
      AND conversation_id = sqlc.arg(conversation_id)
      AND user_id = sqlc.arg(sender_user_id)
  )
  RETURNING *
), touched AS (
  UPDATE collaboration.conversations
  SET last_message_at = created.created_at, updated_at = now(), version = conversations.version + 1
  FROM created
  WHERE conversations.workspace_id = created.workspace_id
    AND conversations.id = created.conversation_id
)
SELECT created.id, created.conversation_id, created.sender_user_id,
       users.display_name AS sender_display_name, created.message_kind,
       created.body, created.reply_to_message_id, created.edited_at,
       created.version, created.created_at
FROM created JOIN identity.users users ON users.id = created.sender_user_id;

-- name: ListConversationRecipientIDs :many
SELECT user_id
FROM collaboration.conversation_members
WHERE workspace_id = sqlc.arg(workspace_id)
  AND conversation_id = sqlc.arg(conversation_id)
ORDER BY user_id;

-- name: MarkConversationRead :execrows
UPDATE collaboration.conversation_members
SET last_read_at = GREATEST(last_read_at, now())
WHERE workspace_id = sqlc.arg(workspace_id)
  AND conversation_id = sqlc.arg(conversation_id)
  AND user_id = sqlc.arg(user_id);

-- name: ChatMessageVisible :one
SELECT EXISTS (
  SELECT 1
  FROM collaboration.messages message
  JOIN collaboration.conversation_members member
    ON member.workspace_id = message.workspace_id
   AND member.conversation_id = message.conversation_id
   AND member.user_id = sqlc.arg(user_id)
  WHERE message.workspace_id = sqlc.arg(workspace_id)
    AND message.id = sqlc.arg(message_id)
    AND message.deleted_at IS NULL
);

-- name: ListConversationAttachments :many
SELECT attachment.id, attachment.entity_id AS message_id, attachment.display_name,
       attachment.media_type, attachment.size_bytes, attachment.scan_state,
       attachment.created_at
FROM files.attachments attachment
JOIN collaboration.messages message
  ON message.workspace_id = attachment.workspace_id
 AND message.id = attachment.entity_id
 AND message.conversation_id = sqlc.arg(conversation_id)
 AND message.deleted_at IS NULL
JOIN collaboration.conversation_members member
  ON member.workspace_id = message.workspace_id
 AND member.conversation_id = message.conversation_id
 AND member.user_id = sqlc.arg(user_id)
WHERE attachment.workspace_id = sqlc.arg(workspace_id)
  AND attachment.entity_type = 'chat_message'
  AND attachment.deleted_at IS NULL
ORDER BY attachment.created_at, attachment.id
LIMIT 500;

-- name: AddMessageReaction :exec
INSERT INTO collaboration.message_reactions (workspace_id, message_id, user_id, emoji)
SELECT sqlc.arg(workspace_id), sqlc.arg(message_id), sqlc.arg(user_id), sqlc.arg(emoji)
WHERE EXISTS (
  SELECT 1 FROM collaboration.messages message
  JOIN collaboration.conversation_members member
    ON member.workspace_id = message.workspace_id
   AND member.conversation_id = message.conversation_id
   AND member.user_id = sqlc.arg(user_id)
  WHERE message.workspace_id = sqlc.arg(workspace_id)
    AND message.id = sqlc.arg(message_id)
)
ON CONFLICT DO NOTHING;

-- name: RemoveMessageReaction :execrows
DELETE FROM collaboration.message_reactions
WHERE workspace_id = sqlc.arg(workspace_id)
  AND message_id = sqlc.arg(message_id)
  AND user_id = sqlc.arg(user_id)
  AND emoji = sqlc.arg(emoji);

-- name: PinMessage :exec
INSERT INTO collaboration.pinned_messages (
  workspace_id, conversation_id, message_id, pinned_by
)
SELECT message.workspace_id, message.conversation_id, message.id, sqlc.arg(user_id)
FROM collaboration.messages message
JOIN collaboration.conversation_members member
  ON member.workspace_id = message.workspace_id
 AND member.conversation_id = message.conversation_id
 AND member.user_id = sqlc.arg(user_id)
WHERE message.workspace_id = sqlc.arg(workspace_id)
  AND message.id = sqlc.arg(message_id)
ON CONFLICT DO NOTHING;

-- name: UnpinMessage :execrows
DELETE FROM collaboration.pinned_messages pin
USING collaboration.conversation_members member
WHERE pin.workspace_id = sqlc.arg(workspace_id)
  AND pin.message_id = sqlc.arg(message_id)
  AND member.workspace_id = pin.workspace_id
  AND member.conversation_id = pin.conversation_id
  AND member.user_id = sqlc.arg(user_id);

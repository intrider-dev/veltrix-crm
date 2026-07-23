-- name: CreateCall :one
INSERT INTO collaboration.calls (
  workspace_id, id, conversation_id, room_name, call_kind, created_by
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, conversation_id, call_kind, state, created_by, started_at,
          ended_at, version, created_at, updated_at;

-- name: ExpireStaleConversationCalls :many
UPDATE collaboration.calls
SET state = 'ended', ended_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND conversation_id = sqlc.arg(conversation_id)
  AND (
    (state = 'ringing' AND created_at < sqlc.arg(ringing_cutoff))
    OR (state = 'active' AND COALESCE(started_at, created_at) < sqlc.arg(active_cutoff))
  )
RETURNING id;

-- name: AddConversationCallParticipants :execrows
INSERT INTO collaboration.call_participants (workspace_id, call_id, user_id)
SELECT member.workspace_id, sqlc.arg(call_id), member.user_id
FROM collaboration.conversation_members member
JOIN tenancy.memberships membership
  ON membership.workspace_id = member.workspace_id
 AND membership.user_id = member.user_id
 AND membership.status = 'active'
WHERE member.workspace_id = sqlc.arg(workspace_id)
  AND member.conversation_id = sqlc.arg(conversation_id)
  AND security.chat_entity_conversation_user_allowed(
    member.workspace_id, member.conversation_id, member.user_id
  )
ON CONFLICT DO NOTHING;

-- name: GetCallForParticipant :one
SELECT call.id, call.conversation_id, call.call_kind, call.state, call.created_by,
       call.started_at, call.ended_at, call.version, call.created_at, call.updated_at,
       participant.state AS participant_state
FROM collaboration.calls call
JOIN collaboration.call_participants participant
  ON participant.workspace_id = call.workspace_id
 AND participant.call_id = call.id
 AND participant.user_id = sqlc.arg(actor_user_id)
WHERE call.workspace_id = sqlc.arg(workspace_id)
  AND call.id = sqlc.arg(call_id);

-- name: ListCallParticipantUserIDs :many
SELECT participant.user_id
FROM collaboration.call_participants participant
JOIN collaboration.calls call
  ON call.workspace_id = participant.workspace_id AND call.id = participant.call_id
WHERE participant.workspace_id = $1 AND participant.call_id = $2
  AND security.chat_entity_conversation_user_allowed(
    participant.workspace_id, call.conversation_id, participant.user_id
  )
ORDER BY participant.user_id;

-- name: JoinCallParticipant :one
UPDATE collaboration.call_participants
SET state = 'joined', joined_at = COALESCE(joined_at, now()), left_at = NULL,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND call_id = sqlc.arg(call_id)
  AND user_id = sqlc.arg(actor_user_id)
  AND state IN ('invited', 'joined')
RETURNING state, joined_at, left_at;

-- name: ActivateCall :exec
UPDATE collaboration.calls
SET state = 'active', started_at = COALESCE(started_at, now()),
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND state = 'ringing';

-- name: DeclineCallParticipant :execrows
UPDATE collaboration.call_participants
SET state = 'declined', left_at = now(), updated_at = now()
WHERE workspace_id = $1 AND call_id = $2 AND user_id = $3 AND state = 'invited';

-- name: LeaveCallParticipant :execrows
UPDATE collaboration.call_participants
SET state = 'left', left_at = now(), updated_at = now()
WHERE workspace_id = $1 AND call_id = $2 AND user_id = $3 AND state = 'joined';

-- name: EndCall :one
UPDATE collaboration.calls
SET state = 'ended', ended_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(call_id)
  AND created_by = sqlc.arg(actor_user_id)
  AND state <> 'ended'
RETURNING id, conversation_id, call_kind, state, created_by, started_at,
          ended_at, version, created_at, updated_at;

-- name: EndCallParticipants :exec
UPDATE collaboration.call_participants
SET state = CASE WHEN state = 'joined' THEN 'left' ELSE state END,
    left_at = CASE WHEN state = 'joined' THEN now() ELSE left_at END,
    updated_at = now()
WHERE workspace_id = $1 AND call_id = $2
  AND security.chat_call_user_allowed(workspace_id, call_id, user_id);

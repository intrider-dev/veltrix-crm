-- name: ListUserNotifications :many
SELECT id, recipient_user_id, message_key, message_params, template_version,
       entity_type, entity_id, read_at, version, email_state, created_at
FROM notifications.notifications
WHERE workspace_id = sqlc.arg(workspace_id)
  AND recipient_user_id = sqlc.arg(recipient_user_id)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL)
  AND (created_at, id) <
    (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: CreateNotification :one
INSERT INTO notifications.notifications (
  workspace_id, id, recipient_user_id, message_key, message_params,
  template_version, entity_type, entity_id, email_state, email_updated_at
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(notification_id),
  sqlc.arg(recipient_user_id), sqlc.arg(message_key),
  sqlc.arg(message_params), sqlc.arg(template_version),
  sqlc.narg(entity_type), sqlc.narg(entity_id), sqlc.arg(email_state),
  CASE WHEN sqlc.arg(email_state)::text = 'queued' THEN now() ELSE NULL END
)
RETURNING id, recipient_user_id, message_key, message_params, template_version,
          entity_type, entity_id, read_at, version, email_state, created_at;

-- name: MarkNotificationRead :one
UPDATE notifications.notifications
SET read_at = COALESCE(read_at, now()),
    version = CASE WHEN read_at IS NULL THEN version + 1 ELSE version END
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(notification_id)
  AND recipient_user_id = sqlc.arg(recipient_user_id)
  AND version = sqlc.arg(expected_version)
RETURNING id, recipient_user_id, message_key, message_params, template_version,
          entity_type, entity_id, read_at, version, email_state, created_at;

-- name: MarkAllNotificationsRead :execrows
UPDATE notifications.notifications
SET read_at = now(), version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND recipient_user_id = sqlc.arg(recipient_user_id)
  AND read_at IS NULL;

-- name: EnqueueNotificationEmailJob :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  available_at, max_attempts
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(job_id), 'notification.email', 1,
  'notification-email:' || sqlc.arg(notification_id)::text,
  jsonb_build_object(
    'notificationId', sqlc.arg(notification_id)::text,
    'recipientId', sqlc.arg(recipient_user_id)::text
  ),
  now(), 8
)
ON CONFLICT (workspace_id, kind, idempotency_key) DO NOTHING;

-- name: LockNotificationEmailDelivery :one
SELECT notification.id, notification.recipient_user_id,
       notification.message_key, notification.message_params,
       notification.template_version, notification.entity_type,
       notification.entity_id, notification.email_state,
       users.email,
       COALESCE(membership.locale_override, users.preferred_locale,
                workspace.default_locale) AS recipient_locale,
       workspace.name AS workspace_name
FROM notifications.notifications notification
JOIN identity.users users
  ON users.id = notification.recipient_user_id
 AND users.status = 'active'
JOIN tenancy.memberships membership
  ON membership.workspace_id = notification.workspace_id
 AND membership.user_id = notification.recipient_user_id
 AND membership.status = 'active'
JOIN tenancy.workspaces workspace
  ON workspace.id = notification.workspace_id
WHERE notification.workspace_id = sqlc.arg(workspace_id)
  AND notification.id = sqlc.arg(notification_id)
FOR UPDATE OF notification;

-- name: MarkNotificationEmailSent :execrows
UPDATE notifications.notifications
SET email_state = 'sent', email_updated_at = now(), version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(notification_id)
  AND email_state = 'queued';

-- name: MarkNotificationEmailFailed :execrows
UPDATE notifications.notifications
SET email_state = 'failed', email_updated_at = now(), version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(notification_id)
  AND email_state = 'queued';

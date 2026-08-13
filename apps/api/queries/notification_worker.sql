-- name: GetNotificationOutboxEvent :one
SELECT schema_version, event_type, aggregate_type, aggregate_id, payload
FROM platform.outbox_events
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(event_id);

-- name: GetDealNotificationTarget :one
SELECT deal.name AS title, stage.name AS stage_name,
       deal.owner_user_id AS recipient_user_id
FROM sales.deals AS deal
JOIN sales.pipeline_stages AS stage
  ON stage.workspace_id = deal.workspace_id
 AND stage.pipeline_id = deal.pipeline_id
JOIN tenancy.memberships AS membership
  ON membership.workspace_id = deal.workspace_id
 AND membership.user_id = deal.owner_user_id
 AND membership.status = 'active'
JOIN identity.users AS recipient
  ON recipient.id = membership.user_id
 AND recipient.status = 'active'
WHERE deal.workspace_id = sqlc.arg(workspace_id)
  AND deal.id = sqlc.arg(entity_id)
  AND stage.id = sqlc.arg(stage_id)
  AND deal.deleted_at IS NULL;

-- name: GetActivityNotificationTarget :one
SELECT activity.title, activity.assignee_user_id AS recipient_user_id
FROM activities.activities AS activity
JOIN tenancy.memberships AS membership
  ON membership.workspace_id = activity.workspace_id
 AND membership.user_id = activity.assignee_user_id
 AND membership.status = 'active'
JOIN identity.users AS recipient
  ON recipient.id = membership.user_id
 AND recipient.status = 'active'
WHERE activity.workspace_id = sqlc.arg(workspace_id)
  AND activity.id = sqlc.arg(entity_id);

-- name: CreateDispatchedNotification :one
INSERT INTO notifications.notifications (
  workspace_id, id, recipient_user_id, message_key, message_params,
  template_version, entity_type, entity_id
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(notification_id),
  sqlc.arg(recipient_user_id), sqlc.arg(message_key),
  sqlc.arg(message_params), 1, sqlc.arg(entity_type), sqlc.arg(entity_id)
)
ON CONFLICT (workspace_id, id) DO NOTHING
RETURNING id;

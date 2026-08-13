-- name: GetActivity :one
SELECT id, activity_type, title, body, related_type, related_id,
       assignee_user_id, status, priority, due_at, occurred_at, ends_at,
       location, recurrence_rule, visibility_scope, scope_department_id,
       scope_user_id, completed_at, created_by, version,
       created_at, updated_at
FROM activities.activities
WHERE activities.activities.workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(activity_id)
  AND (
    visibility_scope = 'workspace' OR created_by = sqlc.arg(actor_user_id)
    OR assignee_user_id = sqlc.arg(actor_user_id)
    OR security.activity_assignment_allows(workspace_id, id, sqlc.arg(actor_user_id))
    OR (visibility_scope = 'user' AND scope_user_id = sqlc.arg(actor_user_id))
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (visibility_scope = 'department' AND EXISTS (
      SELECT 1 FROM tenancy.team_memberships department_member
      WHERE department_member.workspace_id = activities.workspace_id
        AND department_member.team_id = activities.scope_department_id
        AND department_member.membership_id = sqlc.arg(actor_membership_id)
    ))
  )
  AND deleted_at IS NULL;

-- name: UpdateActivity :one
UPDATE activities.activities
SET activity_type = sqlc.arg(activity_type),
    title = sqlc.arg(title),
    body = sqlc.narg(body),
    related_type = sqlc.narg(related_type),
    related_id = sqlc.narg(related_id),
    assignee_user_id = sqlc.narg(assignee_user_id),
    status = sqlc.arg(status),
    priority = sqlc.arg(priority),
    due_at = sqlc.narg(due_at),
    occurred_at = sqlc.arg(occurred_at),
    ends_at = sqlc.narg(ends_at),
    location = sqlc.narg(location),
    recurrence_rule = sqlc.narg(recurrence_rule),
    visibility_scope = sqlc.arg(visibility_scope),
    scope_department_id = sqlc.narg(scope_department_id),
    scope_user_id = sqlc.narg(scope_user_id),
    completed_at = CASE
      WHEN sqlc.arg(status)::text = 'completed' THEN COALESCE(completed_at, now())
      ELSE NULL
    END,
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(activity_id)
  AND (created_by = sqlc.arg(actor_user_id)
       OR security.activity_responsible_allows(workspace_id, id, sqlc.arg(actor_user_id))
       OR sqlc.arg(actor_role)::text IN ('owner', 'admin'))
  AND version = sqlc.arg(expected_version)
  AND deleted_at IS NULL
RETURNING id, activity_type, title, body, related_type, related_id,
          assignee_user_id, status, priority, due_at, occurred_at, ends_at,
          location, recurrence_rule, visibility_scope, scope_department_id,
          scope_user_id, completed_at, created_by, version,
          created_at, updated_at;

-- name: SoftDeleteActivity :execrows
UPDATE activities.activities
SET deleted_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(activity_id)
  AND (created_by = sqlc.arg(actor_user_id) OR sqlc.arg(actor_role)::text IN ('owner', 'admin'))
  AND version = sqlc.arg(expected_version)
  AND deleted_at IS NULL;

-- name: ListEntityTimeline :many
SELECT id, activity_type, title, body, related_type, related_id,
       assignee_user_id, status, priority, due_at, occurred_at, ends_at,
       location, recurrence_rule, visibility_scope, scope_department_id,
       scope_user_id, completed_at, created_by, version,
       created_at, updated_at
FROM activities.activities
WHERE activities.activities.workspace_id = sqlc.arg(workspace_id)
  AND related_type = sqlc.arg(entity_type)
  AND related_id = sqlc.arg(entity_id)
  AND (
    visibility_scope = 'workspace' OR created_by = sqlc.arg(actor_user_id)
    OR assignee_user_id = sqlc.arg(actor_user_id)
    OR security.activity_assignment_allows(workspace_id, id, sqlc.arg(actor_user_id))
    OR (visibility_scope = 'user' AND scope_user_id = sqlc.arg(actor_user_id))
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (visibility_scope = 'department' AND EXISTS (
      SELECT 1 FROM tenancy.team_memberships department_member
      WHERE department_member.workspace_id = activities.workspace_id
        AND department_member.team_id = activities.scope_department_id
        AND department_member.membership_id = sqlc.arg(actor_membership_id)
    ))
  )
  AND deleted_at IS NULL
  AND (occurred_at, id) <
    (sqlc.arg(cursor_occurred_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListActivityFeed :many
SELECT id, activity_type, title, body, related_type, related_id,
       assignee_user_id, status, priority, due_at, occurred_at, ends_at,
       location, recurrence_rule, visibility_scope, scope_department_id,
       scope_user_id, completed_at, created_by, version,
       created_at, updated_at
FROM activities.activities
WHERE activities.activities.workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND (
    visibility_scope = 'workspace' OR created_by = sqlc.arg(actor_user_id)
    OR assignee_user_id = sqlc.arg(actor_user_id)
    OR security.activity_assignment_allows(workspace_id, id, sqlc.arg(actor_user_id))
    OR (visibility_scope = 'user' AND scope_user_id = sqlc.arg(actor_user_id))
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (visibility_scope = 'department' AND EXISTS (
      SELECT 1 FROM tenancy.team_memberships department_member
      WHERE department_member.workspace_id = activities.workspace_id
        AND department_member.team_id = activities.scope_department_id
        AND department_member.membership_id = sqlc.arg(actor_membership_id)
    ))
  )
  AND (sqlc.arg(activity_type)::text = '' OR activity_type = sqlc.arg(activity_type))
  AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter))
  AND (sqlc.narg(assignee_user_id)::uuid IS NULL
       OR assignee_user_id = sqlc.narg(assignee_user_id))
  AND (occurred_at, id) <
    (sqlc.arg(cursor_occurred_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: ListCalendarActivities :many
SELECT id, activity_type, title, body, related_type, related_id,
       assignee_user_id, status, priority, due_at, occurred_at, ends_at,
       location, recurrence_rule, visibility_scope, scope_department_id,
       scope_user_id, completed_at, created_by, version,
       created_at, updated_at,
       (CASE WHEN activity_type = 'task' THEN due_at ELSE occurred_at END)::timestamptz
         AS calendar_start,
       (CASE
         WHEN activity_type = 'task' THEN due_at
         ELSE COALESCE(ends_at, occurred_at)
       END)::timestamptz AS calendar_end
FROM activities.activities
WHERE activities.activities.workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND (
    visibility_scope = 'workspace' OR created_by = sqlc.arg(actor_user_id)
    OR assignee_user_id = sqlc.arg(actor_user_id)
    OR security.activity_assignment_allows(workspace_id, id, sqlc.arg(actor_user_id))
    OR (visibility_scope = 'user' AND scope_user_id = sqlc.arg(actor_user_id))
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR (visibility_scope = 'department' AND EXISTS (
      SELECT 1 FROM tenancy.team_memberships department_member
      WHERE department_member.workspace_id = activities.workspace_id
        AND department_member.team_id = activities.scope_department_id
        AND department_member.membership_id = sqlc.arg(actor_membership_id)
    ))
  )
  AND (
    (activity_type = 'task' AND due_at >= sqlc.arg(range_start)::timestamptz
      AND due_at < sqlc.arg(range_end)::timestamptz)
    OR
    (activity_type IN ('call', 'meeting')
      AND occurred_at < sqlc.arg(range_end)::timestamptz
      AND COALESCE(ends_at, occurred_at) >= sqlc.arg(range_start)::timestamptz)
  )
ORDER BY calendar_start, id
LIMIT sqlc.arg(result_limit);

-- name: CountActiveMentionedUsers :one
SELECT count(*)::integer
FROM (
  SELECT DISTINCT mentioned_id
  FROM unnest(sqlc.arg(mentioned_user_ids)::uuid[]) AS mentioned_id
) mentioned
JOIN tenancy.memberships membership
  ON membership.workspace_id = sqlc.arg(workspace_id)
 AND membership.user_id = mentioned.mentioned_id
 AND membership.status = 'active';

-- name: ListActivityAudienceUserIDs :many
SELECT DISTINCT membership.user_id
FROM tenancy.memberships membership
WHERE membership.workspace_id = sqlc.arg(workspace_id)
  AND membership.status = 'active'
  AND (
    membership.user_id = sqlc.arg(created_by)
    OR membership.user_id = sqlc.narg(scope_user_id)
    OR membership.user_id = sqlc.narg(assignee_user_id)
    OR EXISTS (
      SELECT 1 FROM tenancy.workspace_roles role
      WHERE role.workspace_id = membership.workspace_id
        AND role.id = membership.role_id
        AND role.is_system
        AND role.role_key IN ('owner', 'admin')
    )
    OR (
      sqlc.arg(visibility_scope)::text = 'department'
      AND EXISTS (
        SELECT 1 FROM tenancy.team_memberships department_member
        WHERE department_member.workspace_id = membership.workspace_id
          AND department_member.membership_id = membership.id
          AND department_member.team_id = sqlc.narg(scope_department_id)
      )
    )
  )
ORDER BY membership.user_id;

-- name: CreateActivityComment :one
INSERT INTO activities.comments (
  workspace_id, id, activity_id, author_user_id, body, mentioned_user_ids
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(comment_id), sqlc.arg(activity_id),
  sqlc.arg(author_user_id), sqlc.arg(body), sqlc.arg(mentioned_user_ids)
)
RETURNING id, activity_id, author_user_id, body, mentioned_user_ids,
          version, created_at, updated_at;

-- name: ListActivityComments :many
SELECT id, activity_id, author_user_id, body, mentioned_user_ids,
       version, created_at, updated_at
FROM activities.comments
WHERE workspace_id = sqlc.arg(workspace_id)
  AND activity_id = sqlc.arg(activity_id)
  AND deleted_at IS NULL
  AND (created_at, id) <
    (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: UpdateActivityComment :one
UPDATE activities.comments
SET body = sqlc.arg(body),
    mentioned_user_ids = sqlc.arg(mentioned_user_ids),
    version = version + 1,
    updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(comment_id)
  AND author_user_id = sqlc.arg(author_user_id)
  AND version = sqlc.arg(expected_version)
  AND deleted_at IS NULL
RETURNING id, activity_id, author_user_id, body, mentioned_user_ids,
          version, created_at, updated_at;

-- name: SoftDeleteActivityComment :execrows
UPDATE activities.comments
SET deleted_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(comment_id)
  AND author_user_id = sqlc.arg(author_user_id)
  AND version = sqlc.arg(expected_version)
  AND deleted_at IS NULL;

-- name: CreateActivityReminder :one
INSERT INTO activities.reminders (
  workspace_id, id, activity_id, recipient_user_id, remind_at, channel
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(reminder_id), sqlc.arg(activity_id),
  sqlc.arg(recipient_user_id), sqlc.arg(remind_at), sqlc.arg(channel)
)
RETURNING id, activity_id, recipient_user_id, remind_at, channel,
          delivered_at, cancelled_at, version, created_at;

-- name: ListActivityReminders :many
SELECT id, activity_id, recipient_user_id, remind_at, channel,
       delivered_at, cancelled_at, version, created_at
FROM activities.reminders
WHERE workspace_id = sqlc.arg(workspace_id)
  AND activity_id = sqlc.arg(activity_id)
  AND cancelled_at IS NULL
ORDER BY remind_at, id
LIMIT 100;

-- name: UpdateActivityReminder :one
UPDATE activities.reminders
SET remind_at = sqlc.arg(remind_at),
    channel = sqlc.arg(channel),
    version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(reminder_id)
  AND version = sqlc.arg(expected_version)
  AND delivered_at IS NULL
  AND cancelled_at IS NULL
RETURNING id, activity_id, recipient_user_id, remind_at, channel,
          delivered_at, cancelled_at, version, created_at;

-- name: CancelActivityReminder :execrows
UPDATE activities.reminders
SET cancelled_at = now(), version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(reminder_id)
  AND version = sqlc.arg(expected_version)
  AND delivered_at IS NULL
  AND cancelled_at IS NULL;

-- name: EnqueueActivityReminderJob :exec
INSERT INTO platform.jobs (
  workspace_id, id, kind, schema_version, idempotency_key, payload,
  available_at, max_attempts
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(job_id), 'activity.reminder', 1,
  'reminder:' || sqlc.arg(reminder_id)::text,
  jsonb_build_object(
    'reminderId', sqlc.arg(reminder_id)::text,
    'recipientId', sqlc.arg(recipient_user_id)::text
  ),
  sqlc.arg(remind_at), 8
)
ON CONFLICT (workspace_id, kind, idempotency_key) DO UPDATE
SET available_at = EXCLUDED.available_at,
    payload = EXCLUDED.payload,
    state = CASE
      WHEN platform.jobs.state = 'completed' THEN platform.jobs.state
      ELSE 'ready'
    END,
    locked_at = NULL,
    locked_until = NULL,
    worker_id = NULL,
    updated_at = now()
WHERE platform.jobs.state <> 'completed';

-- name: LockActivityReminderForDelivery :one
SELECT reminder.id, reminder.activity_id, reminder.recipient_user_id,
       reminder.remind_at, reminder.channel, reminder.delivered_at,
       reminder.cancelled_at, reminder.version,
       activity.title AS activity_title, activity.due_at,
       activity.related_type, activity.related_id
FROM activities.reminders reminder
JOIN activities.activities activity
  ON activity.workspace_id = reminder.workspace_id
 AND activity.id = reminder.activity_id
 AND activity.deleted_at IS NULL
WHERE reminder.workspace_id = sqlc.arg(workspace_id)
  AND reminder.id = sqlc.arg(reminder_id)
FOR UPDATE OF reminder;

-- name: MarkActivityReminderDelivered :execrows
UPDATE activities.reminders
SET delivered_at = now(), version = version + 1
WHERE workspace_id = sqlc.arg(workspace_id)
  AND id = sqlc.arg(reminder_id)
  AND delivered_at IS NULL
  AND cancelled_at IS NULL;

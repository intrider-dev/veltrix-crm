-- name: ListActivities :many
SELECT id, activity_type, title, body, related_type, related_id, assignee_user_id,
       status, priority, due_at, occurred_at, recurrence_rule, completed_at,
       visibility_scope, scope_department_id, scope_user_id,
       created_by, version, created_at, updated_at
FROM activities.activities
WHERE activities.activities.workspace_id = sqlc.arg(workspace_id)
  AND deleted_at IS NULL
  AND (
    visibility_scope = 'workspace'
    OR created_by = sqlc.arg(actor_user_id)
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
  AND (sqlc.arg(entity_type)::text = '' OR related_type = sqlc.arg(entity_type))
  AND (sqlc.arg(entity_id)::uuid IS NULL OR related_id = sqlc.arg(entity_id))
ORDER BY occurred_at DESC, id DESC
LIMIT sqlc.arg(page_limit);

-- name: CreateActivity :one
INSERT INTO activities.activities (
  workspace_id, id, activity_type, title, body, related_type, related_id,
  assignee_user_id, priority, due_at, occurred_at, ends_at, location,
  recurrence_rule, visibility_scope, scope_department_id, scope_user_id, created_by
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(id), sqlc.arg(activity_type),
  sqlc.arg(title), sqlc.narg(body), sqlc.narg(related_type),
  sqlc.narg(related_id), sqlc.narg(assignee_user_id), sqlc.arg(priority),
  sqlc.narg(due_at), sqlc.arg(occurred_at), sqlc.narg(ends_at),
  sqlc.narg(location), sqlc.narg(recurrence_rule), sqlc.arg(visibility_scope),
  sqlc.narg(scope_department_id), sqlc.narg(scope_user_id), sqlc.arg(created_by)
)
RETURNING id, activity_type, title, body, related_type, related_id,
          assignee_user_id, status, priority, due_at, occurred_at,
          ends_at, location, recurrence_rule, visibility_scope,
          scope_department_id, scope_user_id, completed_at, created_by,
          version, created_at, updated_at;

-- name: CompleteActivity :one
UPDATE activities.activities
SET status = 'completed', completed_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3
  AND deleted_at IS NULL AND status = 'open'
  AND (created_by = sqlc.arg(actor_user_id) OR assignee_user_id = sqlc.arg(actor_user_id)
       OR security.activity_responsible_allows(workspace_id, id, sqlc.arg(actor_user_id))
       OR sqlc.arg(actor_role)::text IN ('owner', 'admin'))
RETURNING id, activity_type, title, body, related_type, related_id,
          assignee_user_id, status, priority, due_at, occurred_at,
          recurrence_rule, visibility_scope, scope_department_id, scope_user_id,
          completed_at, created_by, version, created_at, updated_at;

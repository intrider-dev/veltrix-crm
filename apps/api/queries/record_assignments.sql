-- name: ListLeadAssignments :many
SELECT assignment.id, assignment.assignment_kind, assignment.user_id,
       assignment.department_id, assignment.is_primary,
       user_account.display_name AS user_name,
       department.name AS department_name, assignment.created_at
FROM sales.lead_assignments assignment
LEFT JOIN identity.users user_account ON user_account.id = assignment.user_id
LEFT JOIN tenancy.teams department
  ON department.workspace_id = assignment.workspace_id AND department.id = assignment.department_id
WHERE assignment.workspace_id = $1 AND assignment.lead_id = $2
ORDER BY assignment.assignment_kind, assignment.is_primary DESC,
         COALESCE(user_account.display_name, department.name), assignment.id;

-- name: ListAssignmentUserOptions :many
SELECT membership.user_id AS id, user_account.display_name AS name
FROM tenancy.memberships membership
JOIN identity.users user_account ON user_account.id = membership.user_id
WHERE membership.workspace_id = $1 AND membership.status = 'active'
ORDER BY user_account.display_name, membership.user_id
LIMIT 500;

-- name: ListAssignmentDepartmentOptions :many
SELECT department.id, department.name
FROM tenancy.teams department
WHERE department.workspace_id = $1
ORDER BY department.name, department.id
LIMIT 500;

-- name: DeleteLeadAssignments :exec
DELETE FROM sales.lead_assignments WHERE workspace_id = $1 AND lead_id = $2;

-- name: CreateLeadAssignment :exec
INSERT INTO sales.lead_assignments (
  workspace_id, id, lead_id, assignment_kind, user_id, department_id, is_primary, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: BumpLeadAssignmentsVersion :one
UPDATE sales.leads lead
SET owner_user_id = (
      SELECT assignment.user_id FROM sales.lead_assignments assignment
      WHERE assignment.workspace_id = lead.workspace_id AND assignment.lead_id = lead.id
        AND assignment.assignment_kind = 'responsible' AND assignment.user_id IS NOT NULL
      ORDER BY assignment.is_primary DESC, assignment.created_at, assignment.id LIMIT 1
    ),
    team_id = (
      SELECT assignment.department_id FROM sales.lead_assignments assignment
      WHERE assignment.workspace_id = lead.workspace_id AND assignment.lead_id = lead.id
        AND assignment.assignment_kind = 'responsible' AND assignment.department_id IS NOT NULL
      ORDER BY assignment.is_primary DESC, assignment.created_at, assignment.id LIMIT 1
    ),
    version = lead.version + 1, updated_at = now()
WHERE lead.workspace_id = $1 AND lead.id = $2 AND lead.version = $3 AND lead.deleted_at IS NULL
RETURNING lead.version;

-- name: ListDealAssignments :many
SELECT assignment.id, assignment.assignment_kind, assignment.user_id,
       assignment.department_id, assignment.is_primary,
       user_account.display_name AS user_name,
       department.name AS department_name, assignment.created_at
FROM sales.deal_assignments assignment
LEFT JOIN identity.users user_account ON user_account.id = assignment.user_id
LEFT JOIN tenancy.teams department
  ON department.workspace_id = assignment.workspace_id AND department.id = assignment.department_id
WHERE assignment.workspace_id = $1 AND assignment.deal_id = $2
ORDER BY assignment.assignment_kind, assignment.is_primary DESC,
         COALESCE(user_account.display_name, department.name), assignment.id;

-- name: DeleteDealAssignments :exec
DELETE FROM sales.deal_assignments WHERE workspace_id = $1 AND deal_id = $2;

-- name: CreateDealAssignment :exec
INSERT INTO sales.deal_assignments (
  workspace_id, id, deal_id, assignment_kind, user_id, department_id, is_primary, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: BumpDealAssignmentsVersion :one
UPDATE sales.deals deal
SET owner_user_id = (
      SELECT assignment.user_id FROM sales.deal_assignments assignment
      WHERE assignment.workspace_id = deal.workspace_id AND assignment.deal_id = deal.id
        AND assignment.assignment_kind = 'responsible' AND assignment.user_id IS NOT NULL
      ORDER BY assignment.is_primary DESC, assignment.created_at, assignment.id LIMIT 1
    ),
    version = deal.version + 1, updated_at = now()
WHERE deal.workspace_id = $1 AND deal.id = $2 AND deal.version = $3 AND deal.deleted_at IS NULL
RETURNING deal.version;

-- name: ListActivityAssignments :many
SELECT assignment.id, assignment.assignment_kind, assignment.user_id,
       assignment.department_id, assignment.is_primary,
       user_account.display_name AS user_name,
       department.name AS department_name, assignment.created_at
FROM activities.activity_assignments assignment
LEFT JOIN identity.users user_account ON user_account.id = assignment.user_id
LEFT JOIN tenancy.teams department
  ON department.workspace_id = assignment.workspace_id AND department.id = assignment.department_id
WHERE assignment.workspace_id = $1 AND assignment.activity_id = $2
ORDER BY assignment.assignment_kind, assignment.is_primary DESC,
         COALESCE(user_account.display_name, department.name), assignment.id;

-- name: LockTaskForAssignmentUpdate :one
SELECT activity.version
FROM activities.activities activity
WHERE activity.workspace_id = sqlc.arg(workspace_id)
  AND activity.id = sqlc.arg(activity_id)
  AND activity.activity_type = 'task'
  AND activity.deleted_at IS NULL
  AND (
    activity.created_by = sqlc.arg(actor_user_id)
    OR activity.assignee_user_id = sqlc.arg(actor_user_id)
    OR security.activity_responsible_allows(activity.workspace_id, activity.id, sqlc.arg(actor_user_id))
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
  )
FOR UPDATE;

-- name: DeleteActivityAssignments :exec
DELETE FROM activities.activity_assignments WHERE workspace_id = $1 AND activity_id = $2;

-- name: ListActivityAssignmentUserIDs :many
SELECT DISTINCT membership.user_id
FROM tenancy.memberships membership
WHERE membership.workspace_id = sqlc.arg(workspace_id)
  AND membership.status = 'active'
  AND EXISTS (
    SELECT 1 FROM activities.activity_assignments assignment
    WHERE assignment.workspace_id = membership.workspace_id
      AND assignment.activity_id = sqlc.arg(activity_id)
      AND (
        assignment.user_id = membership.user_id
        OR EXISTS (
          SELECT 1 FROM tenancy.team_memberships department_member
          WHERE department_member.workspace_id = membership.workspace_id
            AND department_member.membership_id = membership.id
            AND department_member.team_id = assignment.department_id
        )
      )
  )
ORDER BY membership.user_id;

-- name: CreateActivityAssignment :exec
INSERT INTO activities.activity_assignments (
  workspace_id, id, activity_id, assignment_kind, user_id, department_id, is_primary, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: BumpActivityAssignmentsVersion :one
UPDATE activities.activities activity
SET assignee_user_id = (
      SELECT assignment.user_id FROM activities.activity_assignments assignment
      WHERE assignment.workspace_id = activity.workspace_id AND assignment.activity_id = activity.id
        AND assignment.assignment_kind = 'responsible' AND assignment.user_id IS NOT NULL
      ORDER BY assignment.is_primary DESC, assignment.created_at, assignment.id LIMIT 1
    ),
    version = activity.version + 1, updated_at = now()
WHERE activity.workspace_id = $1 AND activity.id = $2 AND activity.version = $3
  AND activity.deleted_at IS NULL AND activity.activity_type = 'task'
RETURNING activity.version;

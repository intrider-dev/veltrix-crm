-- name: ListVisibleProjects :many
SELECT project.id, project.name, project.description, project.status, project.visibility,
       project.planned_start_date, project.target_end_date, project.owner_user_id,
       project.version, project.created_at, project.updated_at,
       (sqlc.arg(actor_role)::text IN ('owner', 'admin')
         OR project.owner_user_id = sqlc.arg(actor_user_id)
         OR EXISTS (
           SELECT 1 FROM projects.project_assignments assignment
           WHERE assignment.workspace_id = project.workspace_id
             AND assignment.project_id = project.id
             AND assignment.assignment_kind = 'responsible'
             AND (
               assignment.user_id = sqlc.arg(actor_user_id)
               OR EXISTS (
                 SELECT 1 FROM tenancy.team_memberships department_member
                 WHERE department_member.workspace_id = project.workspace_id
                   AND department_member.team_id = assignment.department_id
                   AND department_member.membership_id = sqlc.arg(membership_id)
               )
             )
         )) AS can_edit,
       (sqlc.arg(actor_role)::text IN ('owner', 'admin')
         OR project.owner_user_id = sqlc.arg(actor_user_id)) AS can_manage
FROM projects.projects project
WHERE project.workspace_id = sqlc.arg(workspace_id)
  AND project.deleted_at IS NULL
  AND (sqlc.arg(status_filter)::text = '' OR project.status = sqlc.arg(status_filter))
  AND (project.updated_at, project.id) <
      (sqlc.arg(cursor_updated_at)::timestamptz, sqlc.arg(cursor_id)::uuid)
  AND (
    project.visibility = 'workspace'
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR project.owner_user_id = sqlc.arg(actor_user_id)
    OR EXISTS (
      SELECT 1 FROM projects.project_assignments assignment
      WHERE assignment.workspace_id = project.workspace_id
        AND assignment.project_id = project.id
        AND (
          assignment.user_id = sqlc.arg(actor_user_id)
          OR EXISTS (
            SELECT 1 FROM tenancy.team_memberships department_member
            WHERE department_member.workspace_id = project.workspace_id
              AND department_member.team_id = assignment.department_id
              AND department_member.membership_id = sqlc.arg(membership_id)
          )
        )
    )
  )
ORDER BY project.updated_at DESC, project.id DESC
LIMIT sqlc.arg(page_limit);

-- name: GetVisibleProject :one
SELECT project.id, project.name, project.description, project.status, project.visibility,
       project.planned_start_date, project.target_end_date, project.owner_user_id,
       project.version, project.created_at, project.updated_at,
       (sqlc.arg(actor_role)::text IN ('owner', 'admin')
         OR project.owner_user_id = sqlc.arg(actor_user_id)
         OR EXISTS (
           SELECT 1 FROM projects.project_assignments assignment
           WHERE assignment.workspace_id = project.workspace_id
             AND assignment.project_id = project.id
             AND assignment.assignment_kind = 'responsible'
             AND (
               assignment.user_id = sqlc.arg(actor_user_id)
               OR EXISTS (
                 SELECT 1 FROM tenancy.team_memberships department_member
                 WHERE department_member.workspace_id = project.workspace_id
                   AND department_member.team_id = assignment.department_id
                   AND department_member.membership_id = sqlc.arg(membership_id)
               )
             )
         )) AS can_edit,
       (sqlc.arg(actor_role)::text IN ('owner', 'admin')
         OR project.owner_user_id = sqlc.arg(actor_user_id)) AS can_manage
FROM projects.projects project
WHERE project.workspace_id = sqlc.arg(workspace_id)
  AND project.id = sqlc.arg(project_id)
  AND project.deleted_at IS NULL
  AND (
    project.visibility = 'workspace'
    OR sqlc.arg(actor_role)::text IN ('owner', 'admin')
    OR project.owner_user_id = sqlc.arg(actor_user_id)
    OR EXISTS (
      SELECT 1 FROM projects.project_assignments assignment
      WHERE assignment.workspace_id = project.workspace_id
        AND assignment.project_id = project.id
        AND (
          assignment.user_id = sqlc.arg(actor_user_id)
          OR EXISTS (
            SELECT 1 FROM tenancy.team_memberships department_member
            WHERE department_member.workspace_id = project.workspace_id
              AND department_member.team_id = assignment.department_id
              AND department_member.membership_id = sqlc.arg(membership_id)
          )
        )
    )
  );

-- name: CreateProject :one
INSERT INTO projects.projects (
  workspace_id, id, name, description, status, visibility,
  planned_start_date, target_end_date, owner_user_id, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, name, description, status, visibility, planned_start_date,
          target_end_date, owner_user_id, version, created_at, updated_at;

-- name: UpdateProject :one
UPDATE projects.projects
SET name = $3, description = $4, status = $5, visibility = $6,
    planned_start_date = $7, target_end_date = $8, owner_user_id = $9,
    version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $10 AND deleted_at IS NULL
RETURNING id, name, description, status, visibility, planned_start_date,
          target_end_date, owner_user_id, version, created_at, updated_at;

-- name: SoftDeleteProject :execrows
UPDATE projects.projects
SET deleted_at = now(), version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NULL;

-- name: ListProjectAssignments :many
SELECT assignment.id, assignment.project_id, assignment.assignment_kind,
       assignment.user_id, assignment.department_id,
       user_account.display_name AS user_name,
       department.name AS department_name,
       assignment.created_at
FROM projects.project_assignments assignment
LEFT JOIN identity.users user_account ON user_account.id = assignment.user_id
LEFT JOIN tenancy.teams department
  ON department.workspace_id = assignment.workspace_id
 AND department.id = assignment.department_id
WHERE assignment.workspace_id = $1 AND assignment.project_id = $2
ORDER BY assignment.assignment_kind, COALESCE(user_account.display_name, department.name), assignment.id;

-- name: DeleteProjectAssignments :exec
DELETE FROM projects.project_assignments
WHERE workspace_id = $1 AND project_id = $2;

-- name: BumpProjectAssignmentsVersion :one
UPDATE projects.projects
SET version = version + 1, updated_at = now()
WHERE workspace_id = $1 AND id = $2 AND version = $3 AND deleted_at IS NULL
RETURNING version;

-- name: CreateProjectAssignment :one
INSERT INTO projects.project_assignments (
  workspace_id, id, project_id, assignment_kind, user_id, department_id, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, assignment_kind, user_id, department_id, created_at;

-- name: LockLeadStageForAccessReplace :one
SELECT id
FROM sales.lead_stages
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(stage_id) AND archived_at IS NULL
FOR UPDATE;

-- name: GetLeadStageForAccessConfig :one
SELECT id
FROM sales.lead_stages
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(stage_id) AND archived_at IS NULL;

-- name: ListLeadStageRoleAccess :many
SELECT access.role_id, role.role_key, role.name AS role_name, role.base_role,
       access.can_view, access.can_enter, access.can_leave,
       access.created_at, access.updated_at
FROM sales.lead_stage_role_access access
JOIN tenancy.workspace_roles role
  ON role.workspace_id = access.workspace_id AND role.id = access.role_id
WHERE access.workspace_id = sqlc.arg(workspace_id)
  AND access.stage_id = sqlc.arg(stage_id)
ORDER BY role.is_system DESC, role.name, role.id
LIMIT sqlc.arg(result_limit);

-- name: DeleteLeadStageRoleAccess :exec
DELETE FROM sales.lead_stage_role_access
WHERE workspace_id = sqlc.arg(workspace_id) AND stage_id = sqlc.arg(stage_id);

-- name: CreateLeadStageRoleAccess :exec
INSERT INTO sales.lead_stage_role_access (
  workspace_id, stage_id, role_id, can_view, can_enter, can_leave
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(stage_id), sqlc.arg(role_id),
  sqlc.arg(can_view), sqlc.arg(can_enter), sqlc.arg(can_leave)
);

-- name: LeadStageAccessAllowed :one
SELECT sales.lead_stage_access_allowed(
  sqlc.arg(workspace_id), sqlc.arg(stage_id), sqlc.arg(access_action)::text
) AS allowed;

-- name: LeadStageTransitionAllowed :one
SELECT (CASE
  WHEN sqlc.arg(from_stage_id)::uuid = sqlc.arg(to_stage_id)::uuid
    THEN sales.lead_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(from_stage_id), 'view'
    )
  ELSE sales.lead_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(from_stage_id), 'leave'
    ) AND sales.lead_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(to_stage_id), 'enter'
    )
END)::boolean AS allowed;

-- name: LockPipelineStageForAccessReplace :one
SELECT id
FROM sales.pipeline_stages
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(stage_id)
FOR UPDATE;

-- name: GetPipelineStageForAccessConfig :one
SELECT id
FROM sales.pipeline_stages
WHERE workspace_id = sqlc.arg(workspace_id) AND id = sqlc.arg(stage_id);

-- name: ListPipelineStageRoleAccess :many
SELECT access.role_id, role.role_key, role.name AS role_name, role.base_role,
       access.can_view, access.can_enter, access.can_leave,
       access.created_at, access.updated_at
FROM sales.pipeline_stage_role_access access
JOIN tenancy.workspace_roles role
  ON role.workspace_id = access.workspace_id AND role.id = access.role_id
WHERE access.workspace_id = sqlc.arg(workspace_id)
  AND access.stage_id = sqlc.arg(stage_id)
ORDER BY role.is_system DESC, role.name, role.id
LIMIT sqlc.arg(result_limit);

-- name: DeletePipelineStageRoleAccess :exec
DELETE FROM sales.pipeline_stage_role_access
WHERE workspace_id = sqlc.arg(workspace_id) AND stage_id = sqlc.arg(stage_id);

-- name: CreatePipelineStageRoleAccess :exec
INSERT INTO sales.pipeline_stage_role_access (
  workspace_id, stage_id, role_id, can_view, can_enter, can_leave
) VALUES (
  sqlc.arg(workspace_id), sqlc.arg(stage_id), sqlc.arg(role_id),
  sqlc.arg(can_view), sqlc.arg(can_enter), sqlc.arg(can_leave)
);

-- name: PipelineStageAccessAllowed :one
SELECT sales.pipeline_stage_access_allowed(
  sqlc.arg(workspace_id), sqlc.arg(stage_id), sqlc.arg(access_action)::text
) AS allowed;

-- name: PipelineStageTransitionAllowed :one
SELECT (CASE
  WHEN sqlc.arg(from_stage_id)::uuid = sqlc.arg(to_stage_id)::uuid
    THEN sales.pipeline_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(from_stage_id), 'view'
    )
  ELSE sales.pipeline_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(from_stage_id), 'leave'
    ) AND sales.pipeline_stage_access_allowed(
      sqlc.arg(workspace_id), sqlc.arg(to_stage_id), 'enter'
    )
END)::boolean AS allowed;

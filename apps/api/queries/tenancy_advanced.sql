-- name: LockWorkspaceMembershipMutation :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(workspace_id)::text, 0));

-- name: ListWorkspaceMembers :many
SELECT m.workspace_id, m.id, m.user_id, m.role, m.role_id, role.name AS role_name,
       m.status, m.locale_override,
       m.timezone_override, m.created_at, m.updated_at,
       u.email, u.display_name, u.preferred_locale
FROM tenancy.memberships m
JOIN identity.users u ON u.id = m.user_id
JOIN tenancy.workspace_roles role ON role.workspace_id = m.workspace_id AND role.id = m.role_id
WHERE m.workspace_id = $1
ORDER BY u.display_name, u.id;

-- name: GetMembershipByIDForUpdate :one
SELECT workspace_id, id, user_id, role, status, locale_override,
       timezone_override, created_at, updated_at, role_id
FROM tenancy.memberships
WHERE workspace_id = $1 AND id = $2
FOR UPDATE;

-- name: GetMembershipByUserID :one
SELECT workspace_id, id, user_id, role, status, locale_override,
       timezone_override, created_at, updated_at, role_id
FROM tenancy.memberships
WHERE workspace_id = $1 AND user_id = $2;

-- name: UpdateMembershipRole :one
UPDATE tenancy.memberships membership
SET role = role.base_role, role_id = role.id, updated_at = now()
FROM tenancy.workspace_roles role
WHERE membership.workspace_id = $1 AND membership.id = $2
  AND role.workspace_id = membership.workspace_id AND role.role_key = $3 AND role.is_system
RETURNING membership.workspace_id, membership.id, membership.user_id, membership.role,
          membership.status, membership.locale_override, membership.timezone_override,
          membership.created_at, membership.updated_at, membership.role_id;

-- name: UpdateMembershipStatus :one
UPDATE tenancy.memberships
SET status = $3, updated_at = now()
WHERE workspace_id = $1 AND id = $2
RETURNING workspace_id, id, user_id, role, status, locale_override,
          timezone_override, created_at, updated_at, role_id;

-- name: UpdateMembershipLocaleOverride :one
UPDATE tenancy.memberships
SET locale_override = $3, updated_at = now()
WHERE workspace_id = $1 AND user_id = $2
RETURNING workspace_id, id, user_id, role, status, locale_override,
          timezone_override, created_at, updated_at, role_id;

-- name: UpdateWorkspaceDefaults :one
UPDATE tenancy.workspaces
SET default_locale = sqlc.arg(default_locale),
    timezone = sqlc.arg(timezone),
    default_currency = sqlc.arg(default_currency),
    version = version + 1,
    updated_at = now()
WHERE id = sqlc.arg(workspace_id) AND version = sqlc.arg(expected_version)
RETURNING id, name, slug, default_locale, timezone, default_currency,
          supported_locales, version, created_at, updated_at;

-- name: CreateInvitation :one
INSERT INTO tenancy.invitations (
  workspace_id, id, email_normalized, role, token_hash, invited_by, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING workspace_id, id, email_normalized, role, token_hash, invited_by,
          expires_at, accepted_at, accepted_by_user_id, created_at;

-- name: LockInvitationByHash :one
SELECT workspace_id, id, email_normalized, role, token_hash, invited_by,
       expires_at, accepted_at, accepted_by_user_id, created_at
FROM tenancy.invitations
WHERE workspace_id = $1 AND token_hash = $2
  AND accepted_at IS NULL AND expires_at > now()
FOR UPDATE;

-- name: AcceptInvitation :execrows
UPDATE tenancy.invitations
SET accepted_at = now(), accepted_by_user_id = $3
WHERE workspace_id = $1 AND id = $2 AND accepted_at IS NULL;

-- name: CreateTeam :one
INSERT INTO tenancy.teams (workspace_id, id, name)
VALUES ($1, $2, $3)
RETURNING workspace_id, id, name, version, created_at, updated_at;

-- name: ListTeams :many
SELECT workspace_id, id, name, version, created_at, updated_at
FROM tenancy.teams
WHERE workspace_id = $1
ORDER BY name, id;

-- name: GetTeam :one
SELECT workspace_id, id, name, version, created_at, updated_at
FROM tenancy.teams
WHERE workspace_id = $1 AND id = $2;

-- name: AddTeamMembership :exec
INSERT INTO tenancy.team_memberships (workspace_id, team_id, membership_id)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveTeamMembership :execrows
DELETE FROM tenancy.team_memberships
WHERE workspace_id = $1 AND team_id = $2 AND membership_id = $3;

-- name: ListTeamMemberships :many
SELECT tm.workspace_id, tm.team_id, tm.membership_id, tm.created_at,
       u.id AS user_id, u.email, u.display_name
FROM tenancy.team_memberships tm
JOIN tenancy.memberships m
  ON m.workspace_id = tm.workspace_id AND m.id = tm.membership_id
JOIN identity.users u ON u.id = m.user_id
WHERE tm.workspace_id = $1 AND tm.team_id = $2
ORDER BY u.display_name, u.id;

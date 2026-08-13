-- name: GetUserByNormalizedEmail :one
SELECT id, email, email_normalized, display_name, password_hash, preferred_locale,
       status, session_version, failed_login_count, locked_until, password_changed_at,
       created_at, updated_at
FROM identity.users
WHERE email_normalized = $1;

-- name: GetUserByID :one
SELECT id, email, email_normalized, display_name, password_hash, preferred_locale,
       status, session_version, failed_login_count, locked_until, password_changed_at,
       created_at, updated_at
FROM identity.users
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO identity.users (
  id, email, email_normalized, display_name, password_hash, preferred_locale
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, email_normalized, display_name, password_hash, preferred_locale,
          status, session_version, failed_login_count, locked_until, password_changed_at,
          created_at, updated_at;

-- name: RecordLoginFailure :exec
UPDATE identity.users
SET failed_login_count = failed_login_count + 1,
    locked_until = CASE
      WHEN failed_login_count + 1 >= 10 THEN now() + interval '15 minutes'
      WHEN failed_login_count + 1 >= 5 THEN now() + interval '1 minute'
      ELSE locked_until
    END,
    updated_at = now()
WHERE id = $1;

-- name: ClearLoginFailures :exec
UPDATE identity.users
SET failed_login_count = 0, locked_until = NULL, updated_at = now()
WHERE id = $1 AND (failed_login_count <> 0 OR locked_until IS NOT NULL);

-- name: CreateSession :exec
INSERT INTO identity.sessions (
  id, user_id, token_hash, csrf_hash, session_version, user_agent, ip_address, expires_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetSessionPrincipal :one
SELECT s.id AS session_id, s.user_id, s.csrf_hash, s.expires_at, s.last_seen_at,
       u.email, u.display_name, u.preferred_locale, u.status, u.session_version
FROM identity.sessions s
JOIN identity.users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND s.session_version = u.session_version;

-- name: TouchSession :exec
UPDATE identity.sessions
SET last_seen_at = now()
WHERE id = $1 AND last_seen_at < now() - interval '5 minutes';

-- name: RevokeSessionByToken :exec
UPDATE identity.sessions SET revoked_at = COALESCE(revoked_at, now())
WHERE token_hash = $1;

-- name: RevokeAllUserSessions :exec
WITH bumped AS (
  UPDATE identity.users
  SET session_version = session_version + 1, updated_at = now()
  WHERE id = $1
  RETURNING id
)
UPDATE identity.sessions SET revoked_at = COALESCE(revoked_at, now())
WHERE user_id = $1 AND revoked_at IS NULL AND EXISTS (SELECT 1 FROM bumped);

-- name: UpdateUserLocale :one
UPDATE identity.users
SET preferred_locale = $2, updated_at = now()
WHERE id = $1
RETURNING id, email, display_name, preferred_locale;


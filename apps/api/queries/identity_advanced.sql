-- name: GetUserByIDForPasswordChange :one
SELECT id, password_hash, status, session_version
FROM identity.users
WHERE id = $1
FOR UPDATE;

-- name: ReplacePasswordAndRevokeSessions :one
WITH updated AS (
  UPDATE identity.users
  SET password_hash = sqlc.arg(password_hash),
      password_changed_at = now(),
      session_version = session_version + 1,
      failed_login_count = 0,
      locked_until = NULL,
      updated_at = now()
  WHERE identity.users.id = sqlc.arg(target_user_id)
  RETURNING session_version
), revoked AS (
  UPDATE identity.sessions
  SET revoked_at = COALESCE(revoked_at, now())
  WHERE identity.sessions.user_id = sqlc.arg(target_user_id) AND revoked_at IS NULL
  RETURNING id
)
SELECT session_version FROM updated;

-- name: DeleteActivePasswordResetTokens :exec
DELETE FROM identity.password_reset_tokens
WHERE user_id = $1 AND used_at IS NULL;

-- name: CreatePasswordResetToken :exec
INSERT INTO identity.password_reset_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- name: DeletePasswordResetToken :exec
DELETE FROM identity.password_reset_tokens WHERE id = $1;

-- name: LockPasswordResetToken :one
SELECT id, user_id, expires_at
FROM identity.password_reset_tokens
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
FOR UPDATE;

-- name: ConsumePasswordResetToken :execrows
UPDATE identity.password_reset_tokens
SET used_at = now()
WHERE id = $1 AND used_at IS NULL;

-- name: IsMFAEnabled :one
SELECT EXISTS (
  SELECT 1 FROM identity.mfa_configurations
  WHERE user_id = $1 AND enabled_at IS NOT NULL
);

-- name: UpsertPendingMFAConfiguration :exec
INSERT INTO identity.mfa_configurations (
  user_id, secret_ciphertext, secret_nonce, key_id, enabled_at, last_accepted_step,
  pending_secret_ciphertext, pending_secret_nonce, pending_key_id, pending_created_at
) VALUES ($1, $2, $3, $4, NULL, NULL, $2, $3, $4, now())
ON CONFLICT (user_id) DO UPDATE
SET pending_secret_ciphertext = EXCLUDED.pending_secret_ciphertext,
    pending_secret_nonce = EXCLUDED.pending_secret_nonce,
    pending_key_id = EXCLUDED.pending_key_id,
    pending_created_at = now();

-- name: LockMFAConfiguration :one
SELECT user_id, secret_ciphertext, secret_nonce, key_id, enabled_at,
       last_accepted_step, created_at, pending_secret_ciphertext,
       pending_secret_nonce, pending_key_id, pending_created_at
FROM identity.mfa_configurations
WHERE user_id = $1
FOR UPDATE;

-- name: EnableMFAConfiguration :execrows
UPDATE identity.mfa_configurations
SET secret_ciphertext = pending_secret_ciphertext,
    secret_nonce = pending_secret_nonce,
    key_id = pending_key_id,
    pending_secret_ciphertext = NULL,
    pending_secret_nonce = NULL,
    pending_key_id = NULL,
    pending_created_at = NULL,
    enabled_at = now(),
    last_accepted_step = $2
WHERE user_id = $1 AND pending_secret_ciphertext IS NOT NULL;

-- name: AdvanceMFATimeStep :execrows
UPDATE identity.mfa_configurations
SET last_accepted_step = $2
WHERE user_id = $1
  AND enabled_at IS NOT NULL
  AND (last_accepted_step IS NULL OR last_accepted_step < $2);

-- name: DisableMFAConfiguration :exec
DELETE FROM identity.mfa_configurations WHERE user_id = $1;

-- name: DeleteRecoveryCodes :exec
DELETE FROM identity.recovery_codes WHERE user_id = $1;

-- name: CreateRecoveryCode :exec
INSERT INTO identity.recovery_codes (id, user_id, code_hash)
VALUES ($1, $2, $3);

-- name: ConsumeRecoveryCode :execrows
UPDATE identity.recovery_codes
SET used_at = now()
WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL;

-- name: CreateMFAChallenge :exec
INSERT INTO identity.mfa_login_challenges (
  id, user_id, token_hash, user_agent, ip_address, expires_at
) VALUES ($1, $2, $3, $4, $5, $6);

-- name: LockMFAChallenge :one
SELECT id, user_id, user_agent, ip_address, expires_at, attempts
FROM identity.mfa_login_challenges
WHERE token_hash = $1
  AND consumed_at IS NULL
  AND expires_at > now()
FOR UPDATE;

-- name: RecordMFAChallengeFailure :exec
UPDATE identity.mfa_login_challenges
SET attempts = attempts + 1,
    consumed_at = CASE WHEN attempts + 1 >= 8 THEN now() ELSE consumed_at END
WHERE id = $1 AND consumed_at IS NULL;

-- name: ConsumeMFAChallenge :execrows
UPDATE identity.mfa_login_challenges
SET consumed_at = now()
WHERE id = $1 AND consumed_at IS NULL;

-- name: DeleteExpiredMFAChallenges :exec
DELETE FROM identity.mfa_login_challenges
WHERE expires_at < now() - interval '1 day' OR consumed_at < now() - interval '1 day';

-- name: DeleteExpiredPasswordResetTokens :exec
DELETE FROM identity.password_reset_tokens
WHERE expires_at < now() - interval '1 day' OR used_at < now() - interval '1 day';

-- name: DeleteExpiredSessions :exec
DELETE FROM identity.sessions
WHERE expires_at < now() - interval '1 day' OR revoked_at < now() - interval '30 days';

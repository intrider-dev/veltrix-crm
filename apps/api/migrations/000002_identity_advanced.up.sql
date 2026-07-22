SET ROLE veltrix_owner;

CREATE TABLE identity.mfa_login_challenges (
  id uuid PRIMARY KEY,
  user_id uuid NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
  user_agent text NOT NULL DEFAULT '' CHECK (char_length(user_agent) <= 512),
  ip_address inet,
  expires_at timestamptz NOT NULL,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 8),
  consumed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX mfa_login_challenges_user_active_idx
  ON identity.mfa_login_challenges (user_id, expires_at DESC)
  WHERE consumed_at IS NULL;

CREATE INDEX password_reset_tokens_user_active_idx
  ON identity.password_reset_tokens (user_id, expires_at DESC)
  WHERE used_at IS NULL;

ALTER TABLE identity.mfa_configurations
  ADD COLUMN pending_secret_ciphertext bytea,
  ADD COLUMN pending_secret_nonce bytea,
  ADD COLUMN pending_key_id text,
  ADD COLUMN pending_created_at timestamptz,
  ADD CONSTRAINT mfa_pending_secret_complete CHECK (
    (pending_secret_ciphertext IS NULL AND pending_secret_nonce IS NULL
      AND pending_key_id IS NULL AND pending_created_at IS NULL)
    OR
    (pending_secret_ciphertext IS NOT NULL AND pending_secret_nonce IS NOT NULL
      AND pending_key_id IS NOT NULL AND pending_created_at IS NOT NULL)
  );

ALTER TABLE tenancy.invitations
  ADD COLUMN accepted_by_user_id uuid REFERENCES identity.users(id);

CREATE INDEX invitations_pending_email_idx
  ON tenancy.invitations (workspace_id, email_normalized, expires_at DESC)
  WHERE accepted_at IS NULL;

DROP POLICY workspace_insert ON tenancy.workspaces;
CREATE POLICY workspace_insert ON tenancy.workspaces
  FOR INSERT TO veltrix_app
  WITH CHECK (
    security.current_actor_id() IS NOT NULL
    AND EXISTS (
      SELECT 1 FROM identity.users
      WHERE id = security.current_actor_id() AND status = 'active'
    )
  );

RESET ROLE;

GRANT SELECT, INSERT, UPDATE, DELETE ON identity.mfa_login_challenges TO veltrix_app;

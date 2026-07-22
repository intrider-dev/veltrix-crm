SET ROLE veltrix_owner;

DROP POLICY IF EXISTS workspace_insert ON tenancy.workspaces;
CREATE POLICY workspace_insert ON tenancy.workspaces
  FOR INSERT TO veltrix_app WITH CHECK (true);
DROP INDEX IF EXISTS tenancy.invitations_pending_email_idx;
ALTER TABLE tenancy.invitations DROP COLUMN IF EXISTS accepted_by_user_id;
DROP INDEX IF EXISTS identity.password_reset_tokens_user_active_idx;
DROP TABLE IF EXISTS identity.mfa_login_challenges;
ALTER TABLE identity.mfa_configurations
  DROP CONSTRAINT IF EXISTS mfa_pending_secret_complete,
  DROP COLUMN IF EXISTS pending_created_at,
  DROP COLUMN IF EXISTS pending_key_id,
  DROP COLUMN IF EXISTS pending_secret_nonce,
  DROP COLUMN IF EXISTS pending_secret_ciphertext;

RESET ROLE;

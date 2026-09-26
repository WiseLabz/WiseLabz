DROP INDEX IF EXISTS idx_user_mfa_factors_credential_id;
ALTER TABLE user_mfa_factors DROP COLUMN credential;
ALTER TABLE user_mfa_factors DROP COLUMN credential_id;

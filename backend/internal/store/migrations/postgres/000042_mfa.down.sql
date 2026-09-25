ALTER TABLE auth_config DROP COLUMN require_2fa;
DROP TABLE IF EXISTS user_recovery_codes;
DROP TABLE IF EXISTS user_mfa_factors;

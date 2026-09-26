-- WebAuthn as a second factor (#279 part 2). user_mfa_factors already allows
-- type='webauthn' (000042_mfa); this adds the columns a webauthn factor needs.
-- credential holds the JSON-marshaled webauthn.Credential (public key,
-- sign count, transports, aaguid, backup state, ...); credential_id is
-- pulled out into its own indexed column since lookups on assertion/finish
-- need it directly. Both are nullable because a totp factor leaves them unset.
ALTER TABLE user_mfa_factors ADD COLUMN credential_id TEXT;
ALTER TABLE user_mfa_factors ADD COLUMN credential TEXT;

CREATE UNIQUE INDEX idx_user_mfa_factors_credential_id ON user_mfa_factors(credential_id) WHERE credential_id IS NOT NULL;

CREATE TABLE user_mfa_factors (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL CHECK(type IN ('totp','webauthn')),
    name            TEXT NOT NULL DEFAULT '',
    secret          TEXT NOT NULL DEFAULT '',
    confirmed_at    TEXT,
    last_used_step  INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_user_mfa_factors_user_id ON user_mfa_factors(user_id);

CREATE TABLE user_recovery_codes (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash   TEXT NOT NULL,
    used_at     TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_user_recovery_codes_user_id ON user_recovery_codes(user_id);

ALTER TABLE auth_config ADD COLUMN require_2fa TEXT NOT NULL DEFAULT 'none' CHECK(require_2fa IN ('none','admins','all'));

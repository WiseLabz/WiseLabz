-- 000002_audit_log.up.sql — audit trail for operator actions (issue #24)

-- No FK to users(id): the audit trail must survive the actor being deleted
-- later (accountability outlives the account), so actor_user_id is a plain
-- denormalized TEXT rather than a REFERENCES column.
CREATE TABLE audit_log (
    id              TEXT PRIMARY KEY,
    actor_user_id   TEXT NOT NULL,
    actor_role      TEXT NOT NULL,
    action          TEXT NOT NULL,
    target_type     TEXT NOT NULL DEFAULT '',
    target_id       TEXT NOT NULL DEFAULT '',
    detail          TEXT NOT NULL DEFAULT '{}',
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_audit_log_created ON audit_log(created_at DESC);
CREATE INDEX idx_audit_log_action ON audit_log(action);

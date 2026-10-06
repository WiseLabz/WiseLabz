ALTER TABLE runbook_steps ALTER COLUMN connector_id DROP NOT NULL;
ALTER TABLE runbook_steps ALTER COLUMN verb DROP NOT NULL;
ALTER TABLE runbook_steps ADD COLUMN kind TEXT NOT NULL DEFAULT 'lifecycle'
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual'));
ALTER TABLE runbook_steps ADD COLUMN timeout_seconds INTEGER NOT NULL DEFAULT 300;

CREATE TABLE runbook_runs (
    id            TEXT PRIMARY KEY,
    runbook_id    TEXT REFERENCES runbooks(id) ON DELETE SET NULL,
    runbook_title TEXT NOT NULL,
    state         TEXT NOT NULL CHECK(state IN ('running','waiting_manual','failed','succeeded','cancelled','expired')),
    reason        TEXT NOT NULL DEFAULT '',
    started_by    TEXT NOT NULL,
    resumed_by    TEXT,
    cancelled_by  TEXT,
    started_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    finished_at   TEXT
);
CREATE INDEX idx_runbook_runs_history ON runbook_runs(runbook_id, started_at DESC, id DESC);
CREATE UNIQUE INDEX idx_runbook_runs_one_active ON runbook_runs(runbook_id)
    WHERE state IN ('running','waiting_manual','failed');

CREATE TABLE runbook_run_steps (
    id              TEXT PRIMARY KEY,
    run_id          TEXT NOT NULL REFERENCES runbook_runs(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    kind            TEXT NOT NULL CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual')),
    title           TEXT NOT NULL,
    connector_id    TEXT,
    verb            TEXT,
    entity_ref      TEXT NOT NULL DEFAULT '',
    timeout_seconds INTEGER NOT NULL DEFAULT 0,
    state           TEXT NOT NULL CHECK(state IN ('pending','running','waiting','succeeded','failed','skipped','unknown')),
    started_at      TEXT,
    finished_at     TEXT,
    error           TEXT NOT NULL DEFAULT '',
    confirmed_by    TEXT,
    UNIQUE(run_id, position)
);

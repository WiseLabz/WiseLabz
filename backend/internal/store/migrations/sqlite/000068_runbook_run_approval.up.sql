ALTER TABLE runbooks ADD COLUMN requires_approval INTEGER NOT NULL DEFAULT 0 CHECK(requires_approval IN (0,1));
ALTER TABLE retention_settings ADD COLUMN runbook_approval_hours INTEGER NOT NULL DEFAULT 24;

CREATE TABLE runbook_runs_new (
    id                TEXT PRIMARY KEY,
    runbook_id        TEXT REFERENCES runbooks(id) ON DELETE SET NULL,
    runbook_title     TEXT NOT NULL,
    state             TEXT NOT NULL CHECK(state IN ('running','waiting_manual','failed','succeeded','cancelled','expired','awaiting_approval','rejected')),
    reason            TEXT NOT NULL DEFAULT '',
    started_by        TEXT NOT NULL,
    resumed_by        TEXT,
    cancelled_by      TEXT,
    requires_approval INTEGER NOT NULL DEFAULT 0 CHECK(requires_approval IN (0,1)),
    approved_by       TEXT,
    approved_at       TEXT,
    rejected_by       TEXT,
    started_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    finished_at       TEXT
);
INSERT INTO runbook_runs_new (id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by,
    started_at, updated_at, finished_at)
SELECT id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by,
    started_at, updated_at, finished_at
FROM runbook_runs;
DROP TABLE runbook_runs;
ALTER TABLE runbook_runs_new RENAME TO runbook_runs;

CREATE INDEX idx_runbook_runs_history ON runbook_runs(runbook_id, started_at DESC, id DESC);
CREATE UNIQUE INDEX idx_runbook_runs_one_active ON runbook_runs(runbook_id)
    WHERE state IN ('running','waiting_manual','failed','awaiting_approval');

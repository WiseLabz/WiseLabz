-- An open approval request never started: skip its pending steps and stamp the finish time as a cancel would.
UPDATE runbook_run_steps
SET state = 'skipped',
    finished_at = (SELECT updated_at FROM runbook_runs WHERE runbook_runs.id = runbook_run_steps.run_id)
WHERE state = 'pending'
  AND run_id IN (SELECT id FROM runbook_runs WHERE state = 'awaiting_approval');
UPDATE runbook_runs SET state = 'cancelled', finished_at = COALESCE(finished_at, updated_at)
WHERE state IN ('awaiting_approval','rejected');
DROP INDEX idx_runbook_runs_one_active;

CREATE TABLE runbook_runs_new (
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
INSERT INTO runbook_runs_new (id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by,
    started_at, updated_at, finished_at)
SELECT id, runbook_id, runbook_title, state, reason, started_by, resumed_by, cancelled_by,
    started_at, updated_at, finished_at
FROM runbook_runs;
DROP TABLE runbook_runs;
ALTER TABLE runbook_runs_new RENAME TO runbook_runs;

CREATE INDEX idx_runbook_runs_history ON runbook_runs(runbook_id, started_at DESC, id DESC);
CREATE UNIQUE INDEX idx_runbook_runs_one_active ON runbook_runs(runbook_id)
    WHERE state IN ('running','waiting_manual','failed');
ALTER TABLE retention_settings DROP COLUMN runbook_approval_hours;
ALTER TABLE runbooks DROP COLUMN requires_approval;

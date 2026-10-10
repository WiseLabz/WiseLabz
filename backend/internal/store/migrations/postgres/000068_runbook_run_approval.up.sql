ALTER TABLE runbooks ADD COLUMN requires_approval INTEGER NOT NULL DEFAULT 0 CHECK(requires_approval IN (0,1));
ALTER TABLE retention_settings ADD COLUMN runbook_approval_hours INTEGER NOT NULL DEFAULT 24;

ALTER TABLE runbook_runs DROP CONSTRAINT runbook_runs_state_check;
ALTER TABLE runbook_runs ADD COLUMN requires_approval INTEGER NOT NULL DEFAULT 0 CHECK(requires_approval IN (0,1));
ALTER TABLE runbook_runs ADD COLUMN approved_by TEXT;
ALTER TABLE runbook_runs ADD COLUMN approved_at TEXT;
ALTER TABLE runbook_runs ADD COLUMN rejected_by TEXT;
ALTER TABLE runbook_runs ADD CONSTRAINT runbook_runs_state_check
    CHECK(state IN ('running','waiting_manual','failed','succeeded','cancelled','expired','awaiting_approval','rejected'));

DROP INDEX idx_runbook_runs_one_active;
CREATE UNIQUE INDEX idx_runbook_runs_one_active ON runbook_runs(runbook_id)
    WHERE state IN ('running','waiting_manual','failed','awaiting_approval');

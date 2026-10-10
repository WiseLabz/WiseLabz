UPDATE runbook_runs SET state = 'cancelled' WHERE state IN ('awaiting_approval','rejected');
DROP INDEX idx_runbook_runs_one_active;
ALTER TABLE runbook_runs DROP CONSTRAINT runbook_runs_state_check;
ALTER TABLE runbook_runs DROP COLUMN requires_approval;
ALTER TABLE runbook_runs DROP COLUMN approved_by;
ALTER TABLE runbook_runs DROP COLUMN approved_at;
ALTER TABLE runbook_runs DROP COLUMN rejected_by;
ALTER TABLE runbook_runs ADD CONSTRAINT runbook_runs_state_check
    CHECK(state IN ('running','waiting_manual','failed','succeeded','cancelled','expired'));

CREATE UNIQUE INDEX idx_runbook_runs_one_active ON runbook_runs(runbook_id)
    WHERE state IN ('running','waiting_manual','failed');
ALTER TABLE retention_settings DROP COLUMN runbook_approval_hours;
ALTER TABLE runbooks DROP COLUMN requires_approval;

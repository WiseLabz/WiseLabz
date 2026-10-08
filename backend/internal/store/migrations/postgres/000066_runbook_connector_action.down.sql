CREATE TEMP TABLE runbook_runs_with_connector_action ON COMMIT DROP AS
SELECT DISTINCT run_id
FROM runbook_run_steps
WHERE kind = 'connector_action';

DELETE FROM runbook_run_steps
WHERE run_id IN (SELECT run_id FROM runbook_runs_with_connector_action);
DELETE FROM runbook_runs
WHERE id IN (SELECT run_id FROM runbook_runs_with_connector_action);
DROP TABLE runbook_runs_with_connector_action;

DELETE FROM runbook_steps
WHERE kind = 'connector_action';

ALTER TABLE runbook_run_steps DROP COLUMN action_fingerprint;
ALTER TABLE runbook_run_steps DROP COLUMN action;
ALTER TABLE runbook_run_steps DROP CONSTRAINT runbook_run_steps_kind_check;
ALTER TABLE runbook_run_steps ADD CONSTRAINT runbook_run_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity'));

ALTER TABLE runbook_steps DROP COLUMN action;
ALTER TABLE runbook_steps DROP CONSTRAINT runbook_steps_kind_check;
ALTER TABLE runbook_steps ADD CONSTRAINT runbook_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity'));

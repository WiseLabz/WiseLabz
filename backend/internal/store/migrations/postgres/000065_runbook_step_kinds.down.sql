CREATE TEMP TABLE runbook_runs_with_new_steps ON COMMIT DROP AS
SELECT DISTINCT run_id
FROM runbook_run_steps
WHERE kind IN ('config_push','wait_for_entity');

DELETE FROM runbook_run_steps
WHERE run_id IN (SELECT run_id FROM runbook_runs_with_new_steps);
DELETE FROM runbook_runs
WHERE id IN (SELECT run_id FROM runbook_runs_with_new_steps);
DROP TABLE runbook_runs_with_new_steps;

DELETE FROM runbook_steps
WHERE kind IN ('config_push','wait_for_entity');

ALTER TABLE runbook_steps DROP COLUMN field_key;
ALTER TABLE runbook_steps DROP COLUMN target_value;
ALTER TABLE runbook_steps DROP COLUMN attribute;
ALTER TABLE runbook_steps DROP COLUMN operator;
ALTER TABLE runbook_steps DROP COLUMN expected_value;
ALTER TABLE runbook_steps DROP CONSTRAINT runbook_steps_kind_check;
ALTER TABLE runbook_steps ADD CONSTRAINT runbook_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual'));

ALTER TABLE runbook_run_steps DROP COLUMN field_key;
ALTER TABLE runbook_run_steps DROP COLUMN target_value;
ALTER TABLE runbook_run_steps DROP COLUMN attribute;
ALTER TABLE runbook_run_steps DROP COLUMN operator;
ALTER TABLE runbook_run_steps DROP COLUMN expected_value;
ALTER TABLE runbook_run_steps DROP CONSTRAINT runbook_run_steps_kind_check;
ALTER TABLE runbook_run_steps ADD CONSTRAINT runbook_run_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual'));

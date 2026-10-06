-- Data loss: rollback removes run history and deletes authored non-lifecycle steps or steps without a connector and verb.
DROP TABLE runbook_run_steps;
DROP TABLE runbook_runs;

DELETE FROM runbook_steps
WHERE kind <> 'lifecycle' OR connector_id IS NULL OR verb IS NULL;

ALTER TABLE runbook_steps DROP COLUMN timeout_seconds;
ALTER TABLE runbook_steps DROP COLUMN kind;
ALTER TABLE runbook_steps ALTER COLUMN connector_id SET NOT NULL;
ALTER TABLE runbook_steps ALTER COLUMN verb SET NOT NULL;

ALTER TABLE runbook_steps DROP CONSTRAINT runbook_steps_kind_check;
ALTER TABLE runbook_steps ADD CONSTRAINT runbook_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity','connector_action'));
ALTER TABLE runbook_steps ADD COLUMN action TEXT NOT NULL DEFAULT '';

ALTER TABLE runbook_run_steps DROP CONSTRAINT runbook_run_steps_kind_check;
ALTER TABLE runbook_run_steps ADD CONSTRAINT runbook_run_steps_kind_check
    CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity','connector_action'));
ALTER TABLE runbook_run_steps ADD COLUMN action TEXT NOT NULL DEFAULT '';
ALTER TABLE runbook_run_steps ADD COLUMN action_fingerprint TEXT NOT NULL DEFAULT '';

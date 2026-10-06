-- Data loss: rollback removes run history and deletes authored non-lifecycle steps or steps without a connector and verb.
DROP TABLE runbook_run_steps;
DROP TABLE runbook_runs;

DELETE FROM runbook_steps
WHERE kind <> 'lifecycle' OR connector_id IS NULL OR verb IS NULL;

DROP INDEX idx_runbook_steps_runbook;
DROP INDEX idx_runbook_steps_connector;
ALTER TABLE runbook_steps RENAME TO runbook_steps_new;
CREATE TABLE runbook_steps (
    id           TEXT PRIMARY KEY,
    runbook_id   TEXT NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    position     INTEGER NOT NULL,
    title        TEXT NOT NULL,
    connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    verb         TEXT NOT NULL CHECK(verb IN ('restart','start','stop')),
    entity_ref   TEXT NOT NULL DEFAULT '',
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
INSERT INTO runbook_steps (id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at)
SELECT id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at
FROM runbook_steps_new;
DROP TABLE runbook_steps_new;
CREATE INDEX idx_runbook_steps_runbook ON runbook_steps(runbook_id, position);
CREATE INDEX idx_runbook_steps_connector ON runbook_steps(connector_id);

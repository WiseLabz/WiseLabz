DROP INDEX idx_runbook_steps_runbook;
DROP INDEX idx_runbook_steps_connector;
ALTER TABLE runbook_steps RENAME TO runbook_steps_old;

CREATE TABLE runbook_steps (
    id              TEXT PRIMARY KEY,
    runbook_id      TEXT NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    position        INTEGER NOT NULL,
    title           TEXT NOT NULL,
    connector_id    TEXT REFERENCES connectors(id) ON DELETE CASCADE,
    verb            TEXT CHECK(verb IN ('restart','start','stop')),
    entity_ref      TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    kind            TEXT NOT NULL DEFAULT 'lifecycle' CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity','connector_action')),
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    field_key       TEXT NOT NULL DEFAULT '',
    target_value    TEXT NOT NULL DEFAULT '',
    attribute       TEXT NOT NULL DEFAULT '',
    operator        TEXT NOT NULL DEFAULT '',
    expected_value  TEXT NOT NULL DEFAULT '',
    action          TEXT NOT NULL DEFAULT ''
);
INSERT INTO runbook_steps (id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at,
    kind, timeout_seconds, field_key, target_value, attribute, operator, expected_value)
SELECT id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at,
    kind, timeout_seconds, field_key, target_value, attribute, operator, expected_value
FROM runbook_steps_old;
DROP TABLE runbook_steps_old;
CREATE INDEX idx_runbook_steps_runbook ON runbook_steps(runbook_id, position);
CREATE INDEX idx_runbook_steps_connector ON runbook_steps(connector_id);

ALTER TABLE runbook_run_steps RENAME TO runbook_run_steps_old;
CREATE TABLE runbook_run_steps (
    id                TEXT PRIMARY KEY,
    run_id            TEXT NOT NULL REFERENCES runbook_runs(id) ON DELETE CASCADE,
    position          INTEGER NOT NULL,
    kind              TEXT NOT NULL CHECK(kind IN ('lifecycle','sync_and_wait','wait_until_healthy','manual','config_push','wait_for_entity','connector_action')),
    title             TEXT NOT NULL,
    connector_id      TEXT,
    verb              TEXT,
    entity_ref        TEXT NOT NULL DEFAULT '',
    timeout_seconds   INTEGER NOT NULL DEFAULT 0,
    state             TEXT NOT NULL CHECK(state IN ('pending','running','waiting','succeeded','failed','skipped','unknown')),
    started_at        TEXT,
    finished_at       TEXT,
    error             TEXT NOT NULL DEFAULT '',
    confirmed_by      TEXT,
    field_key         TEXT NOT NULL DEFAULT '',
    target_value      TEXT NOT NULL DEFAULT '',
    attribute         TEXT NOT NULL DEFAULT '',
    operator          TEXT NOT NULL DEFAULT '',
    expected_value    TEXT NOT NULL DEFAULT '',
    action            TEXT NOT NULL DEFAULT '',
    action_fingerprint TEXT NOT NULL DEFAULT '',
    UNIQUE(run_id, position)
);
INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref,
    timeout_seconds, state, started_at, finished_at, error, confirmed_by,
    field_key, target_value, attribute, operator, expected_value)
SELECT id, run_id, position, kind, title, connector_id, verb, entity_ref,
    timeout_seconds, state, started_at, finished_at, error, confirmed_by,
    field_key, target_value, attribute, operator, expected_value
FROM runbook_run_steps_old;
DROP TABLE runbook_run_steps_old;

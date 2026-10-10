-- Connector scope is a write-time snapshot and survives target deletion.
CREATE TABLE audit_log_connectors (
    audit_id     TEXT NOT NULL REFERENCES audit_log(id) ON DELETE CASCADE,
    connector_id TEXT NOT NULL,
    PRIMARY KEY (audit_id, connector_id)
);
CREATE INDEX idx_audit_log_connectors_connector ON audit_log_connectors(connector_id);

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT id, target_id FROM audit_log
WHERE target_type = 'connector' AND target_id <> '';

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT a.id, c.service_id FROM audit_log a JOIN changes c ON c.id = a.target_id
WHERE a.target_type = 'change' AND c.service_id <> '';

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT a.id, r.service_id FROM audit_log a JOIN alerts r ON r.id = a.target_id
WHERE a.target_type = 'alert' AND r.service_id <> '';

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT a.id, d.service_id FROM audit_log a JOIN docs d ON d.id = a.target_id
WHERE a.target_type = 'doc' AND d.service_id IS NOT NULL AND d.service_id <> '';

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT a.id, s.connector_id FROM audit_log a JOIN runbook_run_steps s ON s.run_id = a.target_id
WHERE a.target_type = 'runbook_run' AND s.connector_id IS NOT NULL AND s.connector_id <> '';

INSERT INTO audit_log_connectors (audit_id, connector_id)
SELECT DISTINCT a.id, json_extract(step.value, '$.connectorId')
FROM audit_log a, json_each(CASE WHEN json_valid(a.detail) THEN
    CASE WHEN json_type(a.detail, '$.steps') = 'array' THEN json_extract(a.detail, '$.steps') ELSE '[]' END
    ELSE '[]' END) step
WHERE a.target_type = 'runbook' AND step.type = 'object'
    AND json_type(step.value, '$.connectorId') = 'text'
    AND json_extract(step.value, '$.connectorId') <> '';

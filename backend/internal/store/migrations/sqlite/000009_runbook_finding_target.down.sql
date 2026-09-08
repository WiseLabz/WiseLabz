-- Revert to supporting only change_type and alert_severity.
-- Note: rows with target_type = 'finding_check_type' will violate the new constraint.
-- This is expected for a down migration.

DROP INDEX IF EXISTS idx_runbooks_target;

CREATE TABLE runbooks_new (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    body         TEXT NOT NULL DEFAULT '',
    target_type  TEXT NOT NULL CHECK(target_type IN ('change_type','alert_severity')),
    target_value TEXT NOT NULL,
    snapshot_id  TEXT REFERENCES service_snapshots(id) ON DELETE SET NULL,
    doc_id       TEXT REFERENCES docs(id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_runbooks_target ON runbooks_new(target_type, target_value);

INSERT INTO runbooks_new SELECT * FROM runbooks WHERE target_type IN ('change_type', 'alert_severity');
DROP TABLE runbooks;
ALTER TABLE runbooks_new RENAME TO runbooks;

-- Add finding_check_type as a valid runbook target type.
-- SQLite doesn't support ALTER TABLE ... ALTER CONSTRAINT, so we rebuild the table.

DROP INDEX IF EXISTS idx_runbooks_target;

CREATE TABLE runbooks_new (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    body         TEXT NOT NULL DEFAULT '',
    target_type  TEXT NOT NULL CHECK(target_type IN ('change_type','alert_severity','finding_check_type')),
    target_value TEXT NOT NULL,
    snapshot_id  TEXT REFERENCES service_snapshots(id) ON DELETE SET NULL,
    doc_id       TEXT REFERENCES docs(id) ON DELETE SET NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);
CREATE UNIQUE INDEX idx_runbooks_target ON runbooks_new(target_type, target_value);

INSERT INTO runbooks_new SELECT * FROM runbooks;
DROP TABLE runbooks;
ALTER TABLE runbooks_new RENAME TO runbooks;

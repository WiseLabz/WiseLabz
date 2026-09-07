CREATE TABLE runbooks (
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
CREATE UNIQUE INDEX idx_runbooks_target ON runbooks(target_type, target_value);

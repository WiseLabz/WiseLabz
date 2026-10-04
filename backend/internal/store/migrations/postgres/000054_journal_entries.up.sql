CREATE TABLE journal_entries (
    id TEXT PRIMARY KEY,
    body TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    created_by TEXT NOT NULL,
    connector_id TEXT REFERENCES connectors(id) ON DELETE SET NULL,
    doc_id TEXT REFERENCES docs(id) ON DELETE SET NULL,
    entity_kind TEXT NOT NULL DEFAULT '',
    entity_name TEXT NOT NULL DEFAULT '',
    entity_ref TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_journal_occurred ON journal_entries(occurred_at DESC, id DESC);
CREATE INDEX idx_journal_connector_occurred ON journal_entries(connector_id, occurred_at DESC, id DESC);

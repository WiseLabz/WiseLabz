CREATE TABLE entity_identity_overrides (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL CHECK (action IN ('merge', 'detach')),
    connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    ref TEXT NOT NULL,
    other_connector_id TEXT REFERENCES connectors(id) ON DELETE CASCADE,
    other_kind TEXT,
    other_ref TEXT,
    note TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL,
    CHECK (
        (action = 'detach' AND other_connector_id IS NULL AND other_kind IS NULL AND other_ref IS NULL) OR
        (action = 'merge' AND other_connector_id IS NOT NULL AND other_kind IS NOT NULL AND other_ref IS NOT NULL)
    )
);
CREATE UNIQUE INDEX idx_entity_identity_overrides_tuple ON entity_identity_overrides(
    action, connector_id, kind, ref,
    COALESCE(other_connector_id, ''), COALESCE(other_kind, ''), COALESCE(other_ref, '')
);
CREATE INDEX idx_entity_identity_overrides_member ON entity_identity_overrides(connector_id, kind, ref);
CREATE INDEX idx_entity_identity_overrides_other ON entity_identity_overrides(other_connector_id, other_kind, other_ref);

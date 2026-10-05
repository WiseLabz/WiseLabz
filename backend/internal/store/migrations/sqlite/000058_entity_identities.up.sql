CREATE TABLE entities (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    gone_at TEXT,
    merged_into TEXT REFERENCES entities(id) ON DELETE SET NULL
);
CREATE INDEX idx_entities_gone_at ON entities(gone_at);
CREATE INDEX idx_entities_merged_into ON entities(merged_into);

CREATE TABLE entity_members (
    entity_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    ref TEXT NOT NULL,
    name TEXT NOT NULL,
    PRIMARY KEY (connector_id, kind, ref)
);
CREATE INDEX idx_entity_members_entity ON entity_members(entity_id);

ALTER TABLE quality_findings ADD COLUMN entity_kind TEXT;
ALTER TABLE quality_findings ADD COLUMN entity_ref TEXT;
DROP INDEX idx_quality_findings_open;
CREATE UNIQUE INDEX idx_quality_findings_open ON quality_findings(
    connector_id, check_type, COALESCE(rule_id, ''), COALESCE(entity_kind, ''), COALESCE(entity_ref, '')
) WHERE status = 'open';

CREATE TABLE entity_index (
 connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
 kind TEXT NOT NULL,
 name TEXT NOT NULL,
 external_id TEXT NOT NULL DEFAULT '',
 ip TEXT NOT NULL DEFAULT '',
 hostname TEXT NOT NULL DEFAULT '',
 mac TEXT NOT NULL DEFAULT '',
 aliases TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX idx_entity_index_connector ON entity_index(connector_id);

CREATE TABLE entity_index (
 connector_id TEXT NOT NULL REFERENCES connectors(id) ON DELETE CASCADE,
 kind TEXT NOT NULL,
 name TEXT NOT NULL,
 external_id TEXT NOT NULL DEFAULT '',
 ip TEXT NOT NULL DEFAULT '',
 hostname TEXT NOT NULL DEFAULT '',
 mac TEXT NOT NULL DEFAULT '',
 aliases TEXT NOT NULL DEFAULT '[]',
 kind_folded TEXT NOT NULL DEFAULT '',
 name_folded TEXT NOT NULL DEFAULT '',
 external_id_folded TEXT NOT NULL DEFAULT '',
 ip_folded TEXT NOT NULL DEFAULT '',
 hostname_folded TEXT NOT NULL DEFAULT '',
 mac_folded TEXT NOT NULL DEFAULT '',
 aliases_folded TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX idx_entity_index_connector ON entity_index(connector_id);

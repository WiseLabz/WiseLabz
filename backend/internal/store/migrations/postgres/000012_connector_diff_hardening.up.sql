-- 000012_connector_diff_hardening.up.sql — connector credential expiry +
-- dependency-aware / repeat-drift diff support (issues #102, #105)

ALTER TABLE connectors ADD COLUMN credential_expires_at TEXT;

ALTER TABLE changes ADD COLUMN related_service_ids TEXT NOT NULL DEFAULT '[]';
ALTER TABLE changes ADD COLUMN pattern_id TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_changes_pattern ON changes(service_id, pattern_id, detected_at);

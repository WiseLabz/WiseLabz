-- Entity-bound findings cannot remain open under the older connector/rule-only
-- uniqueness key, so preserve their history as resolved during downgrade.
UPDATE quality_findings SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP, notified_severity = NULL
WHERE status = 'open' AND id NOT IN (
    SELECT MIN(id) FROM quality_findings WHERE status = 'open'
    GROUP BY connector_id, check_type, COALESCE(rule_id, '')
);
DROP INDEX IF EXISTS idx_quality_findings_open;
CREATE UNIQUE INDEX idx_quality_findings_open ON quality_findings(connector_id, check_type, COALESCE(rule_id, '')) WHERE status = 'open';
ALTER TABLE quality_findings DROP COLUMN entity_ref;
ALTER TABLE quality_findings DROP COLUMN entity_kind;
DROP TABLE entity_members;
DROP TABLE entities;

-- 000011_flexible_doc_trigger.up.sql — Allow flexible trigger values for doc versions
-- Remove the CHECK constraint that limited trigger to ('ai','template','manual')
-- to allow formats like 'ai:gpt-4o:model', 'ai:claude-3-opus', etc.

-- PostgreSQL: ALTER TABLE to drop constraint
ALTER TABLE doc_versions DROP CONSTRAINT doc_versions_trigger_check;

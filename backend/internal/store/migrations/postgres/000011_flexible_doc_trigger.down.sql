-- 000011_flexible_doc_trigger.down.sql — Restore the original CHECK constraint

ALTER TABLE doc_versions
ADD CONSTRAINT doc_versions_trigger_check CHECK (trigger IN ('ai','template','manual'));

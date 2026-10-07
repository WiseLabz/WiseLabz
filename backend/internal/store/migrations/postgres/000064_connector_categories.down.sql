-- 000064_connector_categories.down.sql
-- Restricts categories back to virtualization, containers_paas, networking, dns.
-- Rollback note: when any row uses storage, monitoring, media or other, the
-- down migration fails on the CHECK constraint, the transaction rolls back and
-- no row is changed or deleted. As with any failed golang-migrate step the
-- version is left marked dirty (63, while the schema is still the 000064 one).
-- To recover, move or delete those connectors and set the migration version
-- back (UPDATE schema_migrations SET version = 64, dirty = false), then retry.

ALTER TABLE connectors DROP CONSTRAINT connectors_category_check;
ALTER TABLE connectors ADD CONSTRAINT connectors_category_check
    CHECK (category IN ('virtualization','containers_paas','networking','dns'));

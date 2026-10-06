-- 000064_connector_categories.down.sql
-- Restricts categories back to virtualization, containers_paas, networking, dns.
-- Fails safely (constraint violation) if rows use storage, monitoring, media, or other.

ALTER TABLE connectors DROP CONSTRAINT connectors_category_check;
ALTER TABLE connectors ADD CONSTRAINT connectors_category_check
    CHECK (category IN ('virtualization','containers_paas','networking','dns'));

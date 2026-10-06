-- 000064_connector_categories.up.sql — widen connector categories (#513)
-- Allows the four new connector categories: storage, monitoring, media, other.

ALTER TABLE connectors DROP CONSTRAINT connectors_category_check;
ALTER TABLE connectors ADD CONSTRAINT connectors_category_check
    CHECK (category IN ('virtualization','containers_paas','networking','dns','storage','monitoring','media','other'));

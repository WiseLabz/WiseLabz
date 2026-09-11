-- Mirrors the SQLite down-migration: drop rows that would violate the narrower constraint
-- before re-adding it, so this rollback can't fail partway on existing discord/slack deliveries.
DELETE FROM notification_deliveries WHERE channel NOT IN ('in_app','smtp','webhook');
ALTER TABLE notification_deliveries DROP CONSTRAINT notification_deliveries_channel_check;
ALTER TABLE notification_deliveries ADD CONSTRAINT notification_deliveries_channel_check CHECK (channel IN ('in_app','smtp','webhook'));

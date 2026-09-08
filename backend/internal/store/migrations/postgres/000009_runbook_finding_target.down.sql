-- Revert to supporting only change_type and alert_severity.
ALTER TABLE runbooks DROP CONSTRAINT runbooks_target_type_check;
ALTER TABLE runbooks ADD CONSTRAINT runbooks_target_type_check CHECK (target_type IN ('change_type','alert_severity'));

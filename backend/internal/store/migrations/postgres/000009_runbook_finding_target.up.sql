-- Add finding_check_type as a valid runbook target type.
ALTER TABLE runbooks DROP CONSTRAINT runbooks_target_type_check;
ALTER TABLE runbooks ADD CONSTRAINT runbooks_target_type_check CHECK (target_type IN ('change_type','alert_severity','finding_check_type'));

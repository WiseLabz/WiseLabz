-- UTC normalization is intentionally irreversible: the original offset is not
-- retained, and normalized RFC3339 timestamps remain valid for older clients.
SELECT 1;

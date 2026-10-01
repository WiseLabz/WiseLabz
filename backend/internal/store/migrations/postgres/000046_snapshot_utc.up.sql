-- Convert whole seconds to UTC while preserving nanoseconds as text (Postgres
-- timestamps themselves only retain microseconds).
UPDATE service_snapshots SET fetched_at =
    to_char(regexp_replace(fetched_at, '\.[0-9]+', '')::timestamptz AT TIME ZONE 'UTC',
        'YYYY-MM-DD"T"HH24:MI:SS') || '.' ||
    rpad(COALESCE(substring(fetched_at FROM '\.([0-9]+)'), ''), 9, '0') || 'Z'
WHERE fetched_at ~ '^\d{4}-\d{2}-\d{2}T';

-- Strip the fractional part before timezone conversion to avoid SQLite rounding
-- it to milliseconds. Preserve and pad the original nanoseconds separately.
UPDATE service_snapshots SET fetched_at =
    strftime('%Y-%m-%dT%H:%M:%S', substr(fetched_at, 1, 19) ||
        CASE WHEN substr(fetched_at, -1) = 'Z' THEN 'Z' ELSE substr(fetched_at, -6) END)
    || '.' ||
    CASE WHEN substr(fetched_at, 20, 1) = '.' THEN
        substr(substr(fetched_at, 21, length(fetched_at) - 20 -
            CASE WHEN substr(fetched_at, -1) = 'Z' THEN 1 ELSE 6 END) || '000000000', 1, 9)
    ELSE '000000000' END || 'Z'
WHERE length(fetched_at) >= 20 AND substr(fetched_at, 11, 1) = 'T';

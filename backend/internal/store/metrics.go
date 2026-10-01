package store

import (
	"context"
	"fmt"
)

// SyncRunStat aggregates the retained sync_runs rows of one status: how many there are and their
// summed duration (only runs with a recorded duration contribute).
type SyncRunStat struct {
	Status     string
	Count      int
	DurationMs int64
}

// SyncRunStats groups retained sync runs by status. It backs the Prometheus endpoint, which
// instance-wide counts are exposed for, so it is deliberately not connector-scoped.
func (s *Store) SyncRunStats(ctx context.Context) ([]SyncRunStat, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT status, COUNT(*), COALESCE(SUM(duration_ms), 0) FROM sync_runs GROUP BY status ORDER BY status`)
	if err != nil {
		return nil, fmt.Errorf("sync run stats: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var out []SyncRunStat
	for rows.Next() {
		var st SyncRunStat
		if err := rows.Scan(&st.Status, &st.Count, &st.DurationMs); err != nil {
			return nil, fmt.Errorf("scan sync run stat: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync run stats: %w", err)
	}
	return out, nil
}

// CountAllConnectorsByStatus counts every connector grouped by status (unscoped, unlike
// CountConnectorsByStatus).
func (s *Store) CountAllConnectorsByStatus(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{"online": 0, "degraded": 0, "offline": 0, "unknown": 0}
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM connectors GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("count connectors by status: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("scan connector status count: %w", err)
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// CountAllAlertsPending counts pending alerts across every connector (unscoped, unlike
// CountAlertsPending).
func (s *Store) CountAllAlertsPending(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM alerts WHERE status = 'pending'`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pending alerts: %w", err)
	}
	return n, nil
}

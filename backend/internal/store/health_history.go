package store

import (
	"context"
	"fmt"
	"time"
)

// ListHealthCheckConnectors returns the enabled connectors that should be
// health-checked now: those with no active maintenance window at now
// (RFC3339). Same maintenance predicate as ListDueConnectors.
func (s *Store) ListHealthCheckConnectors(ctx context.Context, now string) ([]ConnectorRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+connectorColumns+`
		FROM connectors
		WHERE enabled = 1
		AND NOT EXISTS (SELECT 1 FROM maintenance_windows mw WHERE mw.connector_id = connectors.id AND mw.starts_at <= ? AND mw.ends_at > ?)
		ORDER BY created_at ASC
	`, now, now)
	if err != nil {
		return nil, fmt.Errorf("list health check connectors: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	return scanConnectorRows(rows)
}

// HealthBucket is one downsampled slice of a connector's health history.
type HealthBucket struct {
	Start        string `json:"start"`
	Status       string `json:"status"` // worst status in the bucket
	AvgLatencyMs *int64 `json:"avgLatencyMs,omitempty"`
	CheckCount   int    `json:"checkCount"`
}

// statusSeverity orders statuses for "worst in bucket" selection.
func statusSeverity(status string) int {
	switch status {
	case "offline":
		return 2
	case "degraded":
		return 1
	}
	return 0
}

// GetHealthHistory returns the connector's checks in [since, until) grouped
// into `buckets` equal-width buckets (oldest first), each reporting its worst
// status and mean latency. Empty buckets are omitted. Bucketing is done in Go
// (rather than SQL) because checked_at is stored as an RFC3339 string and the
// sqlite/postgres date-math dialects differ.
func (s *Store) GetHealthHistory(ctx context.Context, connectorID string, since, until time.Time, buckets int) ([]HealthBucket, error) {
	if buckets <= 0 {
		buckets = 1
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, latency_ms, checked_at FROM health_checks
		WHERE connector_id = ? AND checked_at >= ? AND checked_at < ?
		ORDER BY checked_at ASC
	`, connectorID, since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get health history: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	width := until.Sub(since) / time.Duration(buckets)
	type acc struct {
		sev, count, latN int
		latSum           int64
	}
	accs := make([]acc, buckets)
	for rows.Next() {
		var status, checkedAt string
		var latency *int64
		if err := rows.Scan(&status, &latency, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan health history: %w", err)
		}
		ts, err := time.Parse(time.RFC3339, checkedAt)
		if err != nil {
			return nil, fmt.Errorf("parse checked_at %q: %w", checkedAt, err)
		}
		idx := int(ts.Sub(since) / width)
		idx = min(max(idx, 0), buckets-1)
		a := &accs[idx]
		a.count++
		a.sev = max(a.sev, statusSeverity(status))
		if latency != nil {
			a.latSum += *latency
			a.latN++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health history: %w", err)
	}

	statuses := [...]string{"online", "degraded", "offline"}
	out := []HealthBucket{}
	for i, a := range accs {
		if a.count == 0 {
			continue
		}
		b := HealthBucket{
			Start:      since.Add(time.Duration(i) * width).UTC().Format(time.RFC3339),
			Status:     statuses[a.sev],
			CheckCount: a.count,
		}
		if a.latN > 0 {
			avg := a.latSum / int64(a.latN)
			b.AvgLatencyMs = &avg
		}
		out = append(out, b)
	}
	return out, nil
}

// GetFleetUptime computes uptime stats over [since, until) for each of
// connectorIDs using one health_checks query and one maintenance query.
// Connectors without checks are returned with CheckCount 0. Results follow
// the order of connectorIDs.
func (s *Store) GetFleetUptime(ctx context.Context, connectorIDs []string, since, until time.Time) ([]UptimeStats, error) {
	if len(connectorIDs) == 0 {
		return []UptimeStats{}, nil
	}
	args := []any{since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339)}
	for _, id := range connectorIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, status, checked_at FROM health_checks
		WHERE checked_at >= ? AND checked_at < ? AND connector_id IN (`+placeholders(len(connectorIDs))+`)
		ORDER BY checked_at ASC
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("get fleet uptime: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	byConn := map[string][]healthPoint{}
	for rows.Next() {
		var id, status, checkedAt string
		if err := rows.Scan(&id, &status, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan fleet health check: %w", err)
		}
		ts, err := time.Parse(time.RFC3339, checkedAt)
		if err != nil {
			return nil, fmt.Errorf("parse checked_at %q: %w", checkedAt, err)
		}
		byConn[id] = append(byConn[id], healthPoint{status: status, checkedAt: ts})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fleet health checks: %w", err)
	}

	maint, err := s.loadMaintenanceSpans(ctx, connectorIDs, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]UptimeStats, 0, len(connectorIDs))
	for _, id := range connectorIDs {
		out = append(out, computeUptime(id, byConn[id], maint[id], since, until))
	}
	return out, nil
}

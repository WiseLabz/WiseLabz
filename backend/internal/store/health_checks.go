package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// HealthCheckRecord represents a row in the health_checks table: one
// connector/health.go check result, persisted as a time-series point rather
// than overwriting the connector's latest status.
type HealthCheckRecord struct {
	ID          string `json:"id"`
	ConnectorID string `json:"connectorId"`
	Status      string `json:"status"` // "online", "degraded", or "offline"
	Message     string `json:"message"`
	LatencyMs   *int64 `json:"latencyMs,omitempty"`
	CheckedAt   string `json:"checkedAt"`
}

// RecordHealthCheck inserts a health_checks row. ID and CheckedAt default to
// a fresh UUID and now (UTC) respectively when unset.
func (s *Store) RecordHealthCheck(ctx context.Context, hc *HealthCheckRecord) error {
	if hc.ID == "" {
		hc.ID = uuid.New().String()
	}
	if hc.CheckedAt == "" {
		hc.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO health_checks (id, connector_id, status, message, latency_ms, checked_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, hc.ID, hc.ConnectorID, hc.Status, hc.Message, hc.LatencyMs, hc.CheckedAt)
	if err != nil {
		return fmt.Errorf("record health check: %w", err)
	}
	return nil
}

// UptimeStats summarizes health_checks for one connector over a window,
// time-weighted between consecutive checks (see GetConnectorUptime).
type UptimeStats struct {
	ConnectorID     string  `json:"connectorId"`
	WindowStart     string  `json:"windowStart"`
	WindowEnd       string  `json:"windowEnd"`
	CheckCount      int     `json:"checkCount"`      // checks that contributed measured time; 0 means no data
	AvailabilityPct float64 `json:"availabilityPct"` // meaningless (0) when CheckCount == 0
	MTTRSeconds     float64 `json:"mttrSeconds"`     // mean time to recovery; 0 when no resolved outage
	OutageCount     int     `json:"outageCount"`     // resolved outages the MTTR mean is over
}

// GetConnectorUptime computes availability % and MTTR for connectorID over
// [since, until) from the health_checks time series.
//
// Availability is time-weighted, not a simple pass/fail ratio over samples:
// each check's status is assumed to hold from its checked_at until the next
// check (or until `until`, for the last one), and the fraction of that
// window spent in a non-"offline" status is the availability. This matches
// how checks are actually taken — on an interval, not continuously — without
// letting an uneven sampling rate skew the result.
//
// MTTR (mean time to recovery) is the mean, over every outage that both
// started and recovered inside the window, of the time from when a check
// first reported "offline" to the next check that reported something else.
// An outage still ongoing at `until` has no recovery time yet and is
// excluded from the mean, consistent with the usual MTTR definition (mean
// time to recover from *resolved* incidents).
//
// Gaps are not extrapolated: a status is only assumed to hold for a bounded
// time after its check (see maxExtension), so a disabled connector or a
// stopped server does not count as up or down for the whole gap. The
// unmeasured remainder is excluded from availability and from outages.
//
// Maintenance windows are excluded: time inside one counts toward neither
// availability nor downtime, and an offline stretch lying entirely inside one
// is not an outage (see computeUptime).
func (s *Store) GetConnectorUptime(ctx context.Context, connectorID string, since, until time.Time) (UptimeStats, error) {
	points, err := s.loadHealthPoints(ctx, connectorID, since, until)
	if err != nil {
		return UptimeStats{ConnectorID: connectorID}, err
	}
	maint, err := s.loadMaintenanceSpans(ctx, []string{connectorID}, since, until)
	if err != nil {
		return UptimeStats{ConnectorID: connectorID}, err
	}
	return computeUptime(connectorID, points, maint[connectorID], since, until), nil
}

// GetConnectorUptimeWindows computes GetConnectorUptime for several lookback
// windows ending at until with a single scan of health_checks (the longest
// window's rows are loaded once and narrowed per window). Results are in the
// order of lookbacks.
func (s *Store) GetConnectorUptimeWindows(ctx context.Context, connectorID string, until time.Time, lookbacks []time.Duration) ([]UptimeStats, error) {
	var longest time.Duration
	for _, d := range lookbacks {
		longest = max(longest, d)
	}
	points, err := s.loadHealthPoints(ctx, connectorID, until.Add(-longest), until)
	if err != nil {
		return nil, err
	}
	maint, err := s.loadMaintenanceSpans(ctx, []string{connectorID}, until.Add(-longest), until)
	if err != nil {
		return nil, err
	}
	out := make([]UptimeStats, 0, len(lookbacks))
	for _, d := range lookbacks {
		since := until.Add(-d)
		// points are ascending, so the window is a suffix.
		i := sort.Search(len(points), func(i int) bool { return !points[i].checkedAt.Before(since) })
		out = append(out, computeUptime(connectorID, points[i:], maint[connectorID], since, until))
	}
	return out, nil
}

type healthPoint struct {
	status    string
	checkedAt time.Time
}

// loadHealthPoints returns the connector's checks in [since, until), oldest first.
func (s *Store) loadHealthPoints(ctx context.Context, connectorID string, since, until time.Time) ([]healthPoint, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status, checked_at FROM health_checks
		WHERE connector_id = ? AND checked_at >= ? AND checked_at < ?
		ORDER BY checked_at ASC
	`, connectorID, since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get connector uptime: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var points []healthPoint
	for rows.Next() {
		var status, checkedAt string
		if err := rows.Scan(&status, &checkedAt); err != nil {
			return nil, fmt.Errorf("scan health check: %w", err)
		}
		ts, err := time.Parse(time.RFC3339, checkedAt)
		if err != nil {
			return nil, fmt.Errorf("parse checked_at %q: %w", checkedAt, err)
		}
		points = append(points, healthPoint{status: status, checkedAt: ts})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health checks: %w", err)
	}
	return points, nil
}

// span is a half-open time interval.
type span struct{ start, end time.Time }

// loadMaintenanceSpans returns, per connector, the maintenance windows that
// overlap [since, until).
func (s *Store) loadMaintenanceSpans(ctx context.Context, connectorIDs []string, since, until time.Time) (map[string][]span, error) {
	out := map[string][]span{}
	if len(connectorIDs) == 0 {
		return out, nil
	}
	args := []any{since.UTC().Format(time.RFC3339), until.UTC().Format(time.RFC3339)}
	for _, id := range connectorIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT connector_id, starts_at, ends_at FROM maintenance_windows
		WHERE ends_at > ? AND starts_at < ? AND connector_id IN (`+placeholders(len(connectorIDs))+`)
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("load maintenance spans: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var id, startsAt, endsAt string
		if err := rows.Scan(&id, &startsAt, &endsAt); err != nil {
			return nil, fmt.Errorf("scan maintenance span: %w", err)
		}
		st, err1 := time.Parse(time.RFC3339, startsAt)
		en, err2 := time.Parse(time.RFC3339, endsAt)
		if err1 != nil || err2 != nil {
			continue // unparseable window: ignore rather than fail uptime reads
		}
		out[id] = append(out[id], span{st, en})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate maintenance spans: %w", err)
	}
	return out, nil
}

// outsideMaintenance returns how much of [a, b) is not covered by any
// maintenance span (spans may overlap).
func outsideMaintenance(a, b time.Time, maint []span) time.Duration {
	if !b.After(a) {
		return 0
	}
	covered := []span{}
	for _, m := range maint {
		st, en := m.start, m.end
		if st.Before(a) {
			st = a
		}
		if en.After(b) {
			en = b
		}
		if en.After(st) {
			covered = append(covered, span{st, en})
		}
	}
	sort.Slice(covered, func(i, j int) bool { return covered[i].start.Before(covered[j].start) })
	total := b.Sub(a)
	var cur span
	for i, c := range covered {
		if i == 0 {
			cur = c
			continue
		}
		if c.start.After(cur.end) {
			total -= cur.end.Sub(cur.start)
			cur = c
			continue
		}
		if c.end.After(cur.end) {
			cur.end = c.end
		}
	}
	if len(covered) > 0 {
		total -= cur.end.Sub(cur.start)
	}
	return total
}

const minExtension = 5 * time.Minute

// maxExtension bounds how long a check's status is assumed to hold: 3x the
// median gap between the window's consecutive checks (the effective check
// interval), but at least minExtension. With fewer than two checks there is no
// interval to infer, so minExtension applies.
func maxExtension(points []healthPoint) time.Duration {
	if len(points) < 2 {
		return minExtension
	}
	gaps := make([]time.Duration, 0, len(points)-1)
	for i := 1; i < len(points); i++ {
		gaps = append(gaps, points[i].checkedAt.Sub(points[i-1].checkedAt))
	}
	sort.Slice(gaps, func(i, j int) bool { return gaps[i] < gaps[j] })
	return max(3*gaps[len(gaps)/2], minExtension)
}

// computeUptime derives UptimeStats from the window's ascending points,
// excluding the maintenance spans from availability and outages.
func computeUptime(connectorID string, points []healthPoint, maint []span, since, until time.Time) UptimeStats {
	stats := UptimeStats{
		ConnectorID: connectorID,
		WindowStart: since.UTC().Format(time.RFC3339),
		WindowEnd:   until.UTC().Format(time.RFC3339),
	}
	if len(points) == 0 {
		return stats
	}
	limit := maxExtension(points)

	var upDuration, totalDuration time.Duration
	var outageDur time.Duration
	inOutage := false
	var recoveries []time.Duration

	for i, p := range points {
		windowEnd := until
		if i+1 < len(points) {
			windowEnd = points[i+1].checkedAt
		}
		if limitAt := p.checkedAt.Add(limit); windowEnd.After(limitAt) {
			windowEnd = limitAt
		}
		// Only the part of the segment outside maintenance counts.
		segment := outsideMaintenance(p.checkedAt, windowEnd, maint)
		totalDuration += segment
		if p.status != "offline" {
			upDuration += segment
		}

		switch {
		case p.status == "offline":
			// An offline stretch fully inside maintenance is not an outage.
			if segment > 0 {
				inOutage = true
			}
			if inOutage {
				outageDur += segment
			}
		case inOutage:
			recoveries = append(recoveries, outageDur)
			inOutage = false
			outageDur = 0
		}
	}

	if totalDuration <= 0 {
		// Every check fell in maintenance or at the window edge: nothing was
		// measured, so report "no data" rather than 0% availability.
		return stats
	}
	stats.CheckCount = len(points)
	stats.AvailabilityPct = float64(upDuration) / float64(totalDuration) * 100
	stats.OutageCount = len(recoveries)
	if len(recoveries) > 0 {
		var sum time.Duration
		for _, d := range recoveries {
			sum += d
		}
		stats.MTTRSeconds = (sum / time.Duration(len(recoveries))).Seconds()
	}

	return stats
}

// DeleteOldHealthChecks removes health_checks rows checked before cutoff.
// There is no "keep latest" guard, unlike snapshots: a connector's current
// status lives on the connectors row itself, not derived from this table.
func (s *Store) DeleteOldHealthChecks(ctx context.Context, cutoff string) (int64, error) {
	n, err := s.batchDelete(ctx, "health_checks", `t.checked_at < ?`, cutoff)
	if err != nil {
		return n, fmt.Errorf("delete old health checks: %w", err)
	}
	return n, nil
}

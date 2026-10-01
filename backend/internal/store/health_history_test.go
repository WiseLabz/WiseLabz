package store

import (
	"context"
	"testing"
	"time"
)

func recordChecks(ctx context.Context, t *testing.T, s *Store, connectorID string, t0 time.Time, checks []struct {
	offset time.Duration
	status string
},
) {
	t.Helper()
	for _, c := range checks {
		hc := &HealthCheckRecord{ConnectorID: connectorID, Status: c.status, CheckedAt: t0.Add(c.offset).Format(time.RFC3339)}
		if err := s.RecordHealthCheck(ctx, hc); err != nil {
			t.Fatalf("RecordHealthCheck(%v) error: %v", c.offset, err)
		}
	}
}

func addMaintenance(ctx context.Context, t *testing.T, s *Store, connectorID string, start, end time.Time) {
	t.Helper()
	if err := s.CreateMaintenanceWindow(ctx, &MaintenanceWindowRecord{
		ConnectorID: connectorID, StartsAt: start.Format(time.RFC3339), EndsAt: end.Format(time.RFC3339), CreatedBy: "u",
	}); err != nil {
		t.Fatalf("CreateMaintenanceWindow() error: %v", err)
	}
}

// Outage entirely inside maintenance: availability unaffected, no outage.
func TestUptimeOutageInsideMaintenance(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// online 0-2h, offline 2-4h, online 4-6h; maintenance 2h-4h.
	recordChecks(ctx, t, s, id, t0, []struct {
		offset time.Duration
		status string
	}{{0, "online"}, {2 * time.Hour, "offline"}, {4 * time.Hour, "online"}})
	addMaintenance(ctx, t, s, id, t0.Add(2*time.Hour), t0.Add(4*time.Hour))

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.AvailabilityPct != 100 {
		t.Errorf("AvailabilityPct = %v, want 100", stats.AvailabilityPct)
	}
	if stats.OutageCount != 0 || stats.MTTRSeconds != 0 {
		t.Errorf("outages = %d mttr = %v, want none", stats.OutageCount, stats.MTTRSeconds)
	}
}

// Outage straddling the start of maintenance: only the part outside counts.
func TestUptimeOutageStraddlingMaintenance(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// online 0-1h, offline 1-5h, online 5-6h; maintenance 3h-5h.
	// Outside maintenance: 0-3h and 5-6h = 4h; down = 1h-3h = 2h => 50%.
	recordChecks(ctx, t, s, id, t0, []struct {
		offset time.Duration
		status string
	}{{0, "online"}, {1 * time.Hour, "offline"}, {5 * time.Hour, "online"}})
	addMaintenance(ctx, t, s, id, t0.Add(3*time.Hour), t0.Add(5*time.Hour))

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.AvailabilityPct != 50 {
		t.Errorf("AvailabilityPct = %v, want 50", stats.AvailabilityPct)
	}
	if stats.OutageCount != 1 || stats.MTTRSeconds != (2*time.Hour).Seconds() {
		t.Errorf("outages = %d mttr = %v, want 1 / 7200", stats.OutageCount, stats.MTTRSeconds)
	}
}

// Maintenance elsewhere in the window leaves an unrelated outage untouched.
func TestUptimeOutageOutsideMaintenance(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Same timeline as the deterministic test (66.67%), maintenance 0h-1h only.
	recordChecks(ctx, t, s, id, t0, []struct {
		offset time.Duration
		status string
	}{{0, "online"}, {1 * time.Hour, "online"}, {2 * time.Hour, "offline"}, {4 * time.Hour, "online"}})
	addMaintenance(ctx, t, s, id, t0, t0.Add(1*time.Hour))

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Outside maintenance: 5h, down 2h => 60%.
	if stats.AvailabilityPct != 60 {
		t.Errorf("AvailabilityPct = %v, want 60", stats.AvailabilityPct)
	}
	if stats.OutageCount != 1 || stats.MTTRSeconds != (2*time.Hour).Seconds() {
		t.Errorf("outages = %d mttr = %v, want 1 / 7200", stats.OutageCount, stats.MTTRSeconds)
	}
}

func TestListHealthCheckConnectors(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	now := time.Now().UTC()

	mk := func(name string, enabled bool) string {
		c := &ConnectorRecord{Name: name, Category: "virtualization", Type: "proxmox", URL: "https://example.com", Enabled: enabled}
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	enabledID := mk("enabled", true)
	mk("disabled", false)
	windowedID := mk("windowed", true)
	expiredID := mk("expired-window", true)
	addMaintenance(ctx, t, s, windowedID, now.Add(-time.Hour), now.Add(time.Hour))
	addMaintenance(ctx, t, s, expiredID, now.Add(-2*time.Hour), now.Add(-time.Hour))

	got, err := s.ListHealthCheckConnectors(ctx, now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if len(got) != 2 || !ids[enabledID] || !ids[expiredID] {
		t.Fatalf("got %v, want enabled + expired-window only", ids)
	}
}

func TestGetHealthHistoryBuckets(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	lat := func(v int64) *int64 { return &v }
	for _, c := range []struct {
		offset time.Duration
		status string
		lat    *int64
	}{
		{0, "online", lat(100)},
		{10 * time.Minute, "degraded", lat(300)},
		{50 * time.Minute, "offline", nil}, // bucket 1 (30m buckets over 2h)
	} {
		hc := &HealthCheckRecord{ConnectorID: id, Status: c.status, LatencyMs: c.lat, CheckedAt: t0.Add(c.offset).Format(time.RFC3339)}
		if err := s.RecordHealthCheck(ctx, hc); err != nil {
			t.Fatal(err)
		}
	}

	got, err := s.GetHealthHistory(ctx, id, t0, t0.Add(2*time.Hour), 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("buckets = %d, want 2 (empty omitted): %+v", len(got), got)
	}
	if got[0].Status != "degraded" || got[0].CheckCount != 2 || got[0].AvgLatencyMs == nil || *got[0].AvgLatencyMs != 200 {
		t.Errorf("bucket0 = %+v", got[0])
	}
	if got[1].Status != "offline" || got[1].AvgLatencyMs != nil {
		t.Errorf("bucket1 = %+v", got[1])
	}
}

func TestGetFleetUptime(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	a := createTestConnector(ctx, t, s)
	b := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordChecks(ctx, t, s, a, t0, []struct {
		offset time.Duration
		status string
	}{{0, "online"}, {1 * time.Hour, "offline"}})

	got, err := s.GetFleetUptime(ctx, []string{a, b}, t0, t0.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ConnectorID != a || got[1].ConnectorID != b {
		t.Fatalf("unexpected result order: %+v", got)
	}
	if got[0].AvailabilityPct != 50 || got[0].CheckCount != 2 {
		t.Errorf("a = %+v", got[0])
	}
	if got[1].CheckCount != 0 {
		t.Errorf("b = %+v, want no checks", got[1])
	}
}

type chk = struct {
	offset time.Duration
	status string
}

// Only checks inside maintenance: nothing measured, so "no data" (count 0),
// not 0% availability.
func TestUptimeAllChecksInMaintenanceIsNoData(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	recordChecks(ctx, t, s, id, t0, []chk{{time.Hour, "online"}})
	addMaintenance(ctx, t, s, id, t0, t0.Add(6*time.Hour))

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.CheckCount != 0 || stats.AvailabilityPct != 0 {
		t.Errorf("stats = %+v, want no data (CheckCount 0)", stats)
	}
}

// A check at exactly `until` is outside the window: no data.
func TestUptimeCheckAtWindowEndIsNoData(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until := t0.Add(time.Hour)

	recordChecks(ctx, t, s, id, until, []chk{{0, "offline"}})
	stats, err := s.GetConnectorUptime(ctx, id, t0, until)
	if err != nil {
		t.Fatal(err)
	}
	if stats.CheckCount != 0 {
		t.Errorf("CheckCount = %d, want 0", stats.CheckCount)
	}
}

// The last status is not extrapolated to the window end: after the extension
// cap the time is unknown and counts as neither up nor down.
func TestUptimeStaleLastStatusIsCapped(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// 1-minute checks for 10 minutes (online), then the last one is offline
	// and nothing is recorded for the next 20 hours.
	var cs []chk
	for i := range 10 {
		cs = append(cs, chk{time.Duration(i) * time.Minute, "online"})
	}
	cs = append(cs, chk{10 * time.Minute, "offline"})
	recordChecks(ctx, t, s, id, t0, cs)

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(20*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// 10 min online + 5 min (min cap) offline; the remaining ~20h is unknown.
	want := 10.0 / 15.0 * 100
	if d := stats.AvailabilityPct - want; d > 0.01 || d < -0.01 {
		t.Errorf("AvailabilityPct = %v, want ~%v", stats.AvailabilityPct, want)
	}
	if stats.OutageCount != 0 {
		t.Errorf("OutageCount = %d, want 0 (outage unresolved)", stats.OutageCount)
	}
}

// A long gap between checks (connector disabled, server down) is bounded too.
func TestUptimeGapBetweenChecksIsCapped(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	id := createTestConnector(ctx, t, s)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	var cs []chk
	for i := range 5 {
		cs = append(cs, chk{time.Duration(i) * time.Minute, "online"})
	}
	// 10h gap, then offline checks.
	cs = append(cs, chk{10 * time.Hour, "offline"}, chk{10*time.Hour + time.Minute, "offline"})
	recordChecks(ctx, t, s, id, t0, cs)

	stats, err := s.GetConnectorUptime(ctx, id, t0, t0.Add(11*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	// Up: 4 min + 5 min cap = 9 min. Down: 1 min + 5 min cap = 6 min.
	want := 9.0 / 15.0 * 100
	if d := stats.AvailabilityPct - want; d > 0.01 || d < -0.01 {
		t.Errorf("AvailabilityPct = %v, want ~%v", stats.AvailabilityPct, want)
	}
}

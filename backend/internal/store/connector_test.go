package store

import (
	"context"
	"testing"
	"time"
)

func TestConnectorOwnerRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com", Owner: "Platform"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	got, err := s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() error: %v", err)
	}
	if got.Owner != "Platform" {
		t.Fatalf("Owner = %q, want Platform", got.Owner)
	}
	if err := s.UpdateConnector(ctx, c.ID, map[string]any{"owner": "Operations"}); err != nil {
		t.Fatalf("UpdateConnector() error: %v", err)
	}
	got, err = s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() after update error: %v", err)
	}
	if got.Owner != "Operations" {
		t.Fatalf("Owner = %q, want Operations", got.Owner)
	}
}

func TestConnectorScheduleFieldsDefaultNull(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	c := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}

	got, err := s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() error: %v", err)
	}
	if got.ScheduleSeconds != nil {
		t.Fatalf("ScheduleSeconds = %v, want nil (manual only)", got.ScheduleSeconds)
	}
	if got.NextRunAt != "" {
		t.Fatalf("NextRunAt = %q, want empty", got.NextRunAt)
	}
	if got.LastSyncDurationMs != nil {
		t.Fatalf("LastSyncDurationMs = %v, want nil", got.LastSyncDurationMs)
	}
	if got.LastSyncError != "" {
		t.Fatalf("LastSyncError = %q, want empty", got.LastSyncError)
	}
	if got.RetryCount != 0 {
		t.Fatalf("RetryCount = %d, want 0", got.RetryCount)
	}
}

func TestConnectorScheduleFieldsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	c := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}

	schedule := 1800
	durationMs := 42
	next := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := s.UpdateConnector(ctx, c.ID, map[string]any{
		"schedule_seconds":      schedule,
		"next_run_at":           next,
		"last_sync_duration_ms": durationMs,
		"last_sync_error":       "boom",
		"retry_count":           2,
	}); err != nil {
		t.Fatalf("UpdateConnector() error: %v", err)
	}

	got, err := s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() error: %v", err)
	}
	if got.ScheduleSeconds == nil || *got.ScheduleSeconds != schedule {
		t.Fatalf("ScheduleSeconds = %v, want %d", got.ScheduleSeconds, schedule)
	}
	if got.NextRunAt != next {
		t.Fatalf("NextRunAt = %q, want %q", got.NextRunAt, next)
	}
	if got.LastSyncDurationMs == nil || *got.LastSyncDurationMs != durationMs {
		t.Fatalf("LastSyncDurationMs = %v, want %d", got.LastSyncDurationMs, durationMs)
	}
	if got.LastSyncError != "boom" {
		t.Fatalf("LastSyncError = %q, want boom", got.LastSyncError)
	}
	if got.RetryCount != 2 {
		t.Fatalf("RetryCount = %d, want 2", got.RetryCount)
	}

	// Clearing scheduleSeconds/next_run_at back to null (e.g. disabling the
	// schedule) must round-trip to nil, not a zero value.
	if err := s.UpdateConnector(ctx, c.ID, map[string]any{
		"schedule_seconds": nil,
		"next_run_at":      nil,
	}); err != nil {
		t.Fatalf("UpdateConnector(clear) error: %v", err)
	}
	got, err = s.GetConnector(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetConnector() error: %v", err)
	}
	if got.ScheduleSeconds != nil {
		t.Fatalf("ScheduleSeconds after clear = %v, want nil", got.ScheduleSeconds)
	}
	if got.NextRunAt != "" {
		t.Fatalf("NextRunAt after clear = %q, want empty", got.NextRunAt)
	}
}

func TestListDueConnectors(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	now := time.Now().UTC()
	past := now.Add(-time.Hour).Format(time.RFC3339)
	future := now.Add(time.Hour).Format(time.RFC3339)
	nowStr := now.Format(time.RFC3339)

	mustCreate := func(name string, enabled bool) string {
		c := &ConnectorRecord{Name: name, Category: "virtualization", Type: "proxmox", URL: "https://example.com", Enabled: enabled}
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatalf("CreateConnector(%s) error: %v", name, err)
		}
		return c.ID
	}
	schedule := 60

	dueID := mustCreate("due-past", true)
	if err := s.UpdateConnector(ctx, dueID, map[string]any{"schedule_seconds": schedule, "next_run_at": past}); err != nil {
		t.Fatalf("UpdateConnector(due) error: %v", err)
	}

	dueNullID := mustCreate("due-null-next-run", true)
	if err := s.UpdateConnector(ctx, dueNullID, map[string]any{"schedule_seconds": schedule}); err != nil {
		t.Fatalf("UpdateConnector(due-null) error: %v", err)
	}

	notDueFutureID := mustCreate("not-due-future", true)
	if err := s.UpdateConnector(ctx, notDueFutureID, map[string]any{"schedule_seconds": schedule, "next_run_at": future}); err != nil {
		t.Fatalf("UpdateConnector(not-due-future) error: %v", err)
	}

	_ = mustCreate("manual-only", true) // schedule_seconds left NULL

	disabledID := mustCreate("disabled", false)
	if err := s.UpdateConnector(ctx, disabledID, map[string]any{"schedule_seconds": schedule, "next_run_at": past}); err != nil {
		t.Fatalf("UpdateConnector(disabled) error: %v", err)
	}

	due, err := s.ListDueConnectors(ctx, nowStr, 20)
	if err != nil {
		t.Fatalf("ListDueConnectors() error: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, c := range due {
		gotIDs[c.ID] = true
	}
	if len(due) != 2 || !gotIDs[dueID] || !gotIDs[dueNullID] {
		t.Fatalf("ListDueConnectors() = %+v, want exactly [dueID, dueNullID]", due)
	}
}

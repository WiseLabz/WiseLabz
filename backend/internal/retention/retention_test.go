package retention

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := store.RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	return store.New(db, "sqlite")
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestRunCleanupSkipsDisabledCategories verifies a *Days value of 0 leaves
// that category untouched, while an enabled category is still cleaned up.
func TestRunCleanupSkipsDisabledCategories(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	c := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}

	old := time.Now().UTC().AddDate(0, 0, -365).Format(time.RFC3339)

	// sync_runs: retention enabled, should be purged.
	if err := s.CreateSyncRun(ctx, &store.SyncRunRecord{ConnectorID: c.ID, StartedAt: old, Status: store.SyncRunStatusSuccess}); err != nil {
		t.Fatalf("CreateSyncRun() error: %v", err)
	}
	// alerts: retention disabled (AlertDays: 0), should survive even though old + resolved.
	alert := &store.AlertRecord{ServiceID: c.ID, Severity: "info", Title: "t", Status: "resolved", CreatedAt: old}
	if err := s.CreateAlert(ctx, alert); err != nil {
		t.Fatalf("CreateAlert() error: %v", err)
	}

	cfg := config.RetentionSettings{
		SnapshotDays:   0,
		DocVersionDays: 0,
		AlertDays:      0, // disabled
		SyncRunDays:    30,
		IntervalHours:  24,
	}

	runCleanup(ctx, s, cfg, testLogger())

	runs, err := s.ListSyncRunsByConnector(ctx, c.ID, 20)
	if err != nil {
		t.Fatalf("ListSyncRunsByConnector() error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("sync runs after cleanup = %d, want 0 (SyncRunDays enabled)", len(runs))
	}

	if _, err := s.GetAlert(ctx, alert.ID); err != nil {
		t.Fatalf("alert should survive when AlertDays=0 (disabled), GetAlert() error: %v", err)
	}
}

// TestRunCleanupIdempotent verifies a second pass with the same config
// deletes nothing further.
func TestRunCleanupIdempotent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	c := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	old := time.Now().UTC().AddDate(0, 0, -365).Format(time.RFC3339)
	if err := s.CreateSyncRun(ctx, &store.SyncRunRecord{ConnectorID: c.ID, StartedAt: old, Status: store.SyncRunStatusSuccess}); err != nil {
		t.Fatalf("CreateSyncRun() error: %v", err)
	}

	cfg := config.RetentionSettings{SyncRunDays: 30, IntervalHours: 24}

	runCleanup(ctx, s, cfg, testLogger())
	runCleanup(ctx, s, cfg, testLogger()) // must not error or panic on an already-clean table

	runs, err := s.ListSyncRunsByConnector(ctx, c.ID, 20)
	if err != nil {
		t.Fatalf("ListSyncRunsByConnector() error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("sync runs after second cleanup = %d, want 0", len(runs))
	}
}

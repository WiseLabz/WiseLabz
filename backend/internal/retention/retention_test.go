package retention

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if pgDSN := os.Getenv("WISELABZ_TEST_POSTGRES_DSN"); pgDSN != "" {
		return newPostgresTestStore(t, pgDSN, logger)
	}
	dsn := "file:" + storetest.MigratedSQLite(t) + "?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store.New(db, "sqlite")
}

func newPostgresTestStore(t *testing.T, dsn string, logger *slog.Logger) *store.Store {
	t.Helper()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse postgres dsn: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open postgres schema db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})
	if err := store.RunMigrations(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}
	return store.New(db, "postgres")
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
	// audit_log: retention enabled, should be purged.
	if err := s.CreateAuditRecord(ctx, &store.AuditRecord{ActorUserID: "u1", ActorRole: "operator", Action: "test.action", CreatedAt: old}); err != nil {
		t.Fatalf("CreateAuditRecord() error: %v", err)
	}

	cfg := store.RetentionSettings{
		SnapshotDays:   0,
		DocVersionDays: 0,
		AlertDays:      0, // disabled
		SyncRunDays:    30,
		AuditDays:      30,
		CronExpr:       "0 0 * * *",
	}

	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce: %v", err)
	}

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

	_, auditTotal, err := s.ListAuditRecords(ctx, "", "", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if auditTotal != 0 {
		t.Fatalf("audit records after cleanup = %d, want 0 (AuditDays enabled)", auditTotal)
	}
}

func TestRunCleanupPurgesOnlyExpiredGoneAndMergedIdentities(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	c := &store.ConnectorRecord{Name: "identity-retention", Category: "virtualization", Type: "test", URL: "https://retention.test"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatal(err)
	}
	activeID, goneID, recentID, mergedID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	old := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339Nano)
	recent := time.Now().UTC().AddDate(0, 0, -2).Format(time.RFC3339Nano)
	for _, entity := range []struct{ id, gone, merged string }{
		{activeID, "", ""}, {goneID, old, ""}, {recentID, recent, ""}, {mergedID, "", activeID},
	} {
		var goneAt, mergedInto any
		if entity.gone != "" {
			goneAt = entity.gone
		}
		if entity.merged != "" {
			mergedInto = entity.merged
		}
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities(id,kind,display_name,first_seen_at,last_seen_at,gone_at,merged_into) VALUES(?,?,?,?,?,?,?)`, entity.id, "vm", entity.id, old, old, goneAt, mergedInto); err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range []struct{ id, ref, gone string }{{activeID, "active", ""}, {goneID, "gone", old}, {recentID, "recent", recent}, {mergedID, "merged", old}} {
		var goneAt any
		if member.gone != "" {
			goneAt = member.gone
		}
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO entity_members(entity_id,connector_id,kind,ref,name,gone_at) VALUES(?,?,?,?,?,?)`, member.id, c.ID, "vm", member.ref, member.ref, goneAt); err != nil {
			t.Fatal(err)
		}
	}
	cfg := store.RetentionSettings{SnapshotDays: 30, CronExpr: "0 0 * * *"}
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatal(err)
	}
	var entities, members int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&entities); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_members`).Scan(&members); err != nil {
		t.Fatal(err)
	}
	if entities != 2 || members != 2 {
		t.Fatalf("after purge entities=%d members=%d, want recent-gone plus active", entities, members)
	}
}

// TestRunCleanupPrunesOldHealthChecks verifies health_checks rows older than
// the configured window are purged while recent ones survive, and that
// HealthCheckDays=0 disables cleanup for the category like the other
// *Days settings.
func TestRunCleanupPrunesOldHealthChecks(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	c := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}

	old := time.Now().UTC().AddDate(0, 0, -365).Format(time.RFC3339)
	recent := time.Now().UTC().Format(time.RFC3339)

	oldCheck := &store.HealthCheckRecord{ConnectorID: c.ID, Status: "offline", CheckedAt: old}
	if err := s.RecordHealthCheck(ctx, oldCheck); err != nil {
		t.Fatalf("RecordHealthCheck() old error: %v", err)
	}
	recentCheck := &store.HealthCheckRecord{ConnectorID: c.ID, Status: "online", CheckedAt: recent}
	if err := s.RecordHealthCheck(ctx, recentCheck); err != nil {
		t.Fatalf("RecordHealthCheck() recent error: %v", err)
	}

	// HealthCheckDays=0 disables cleanup: nothing purged.
	if err := RunCleanupOnce(ctx, s, store.RetentionSettings{HealthCheckDays: 0}, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce: %v", err)
	}
	stats, err := s.GetConnectorUptime(ctx, c.ID, time.Now().UTC().AddDate(-1, 0, -1), time.Now().UTC().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetConnectorUptime() error: %v", err)
	}
	if stats.CheckCount != 2 {
		t.Fatalf("check count after disabled cleanup = %d, want 2", stats.CheckCount)
	}

	// HealthCheckDays=30 purges the year-old row, keeps the recent one.
	if err := RunCleanupOnce(ctx, s, store.RetentionSettings{HealthCheckDays: 30}, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce: %v", err)
	}
	stats, err = s.GetConnectorUptime(ctx, c.ID, time.Now().UTC().AddDate(-1, 0, -1), time.Now().UTC().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("GetConnectorUptime() error: %v", err)
	}
	if stats.CheckCount != 1 {
		t.Fatalf("check count after cleanup = %d, want 1", stats.CheckCount)
	}
}

// TestRunCleanupAllDBErrors verifies that when every delete call errors
// (closed DB), RunCleanupOnce logs and returns without panicking.
func TestRunCleanupAllDBErrors(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.DB().Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	cfg := store.RetentionSettings{
		SnapshotDays: 30, DocVersionDays: 30, AlertDays: 30, SyncRunDays: 30, AuditDays: 30,
	}

	// Must not panic; every category errors on the closed DB, so the joined
	// error (job health, #384) must be non-nil.
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err == nil {
		t.Fatal("RunCleanupOnce() with a closed DB: want an error, got nil")
	}
}

// TestRunCleanupPartialFailure verifies that an error in one category
// (sync_runs table dropped, simulating a DB error) does not stop the other
// enabled categories from running.
func TestRunCleanupPartialFailure(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	c := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := s.CreateConnector(ctx, c); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	old := time.Now().UTC().AddDate(0, 0, -365).Format(time.RFC3339)
	if err := s.CreateAuditRecord(ctx, &store.AuditRecord{ActorUserID: "u1", ActorRole: "operator", Action: "test.action", CreatedAt: old}); err != nil {
		t.Fatalf("CreateAuditRecord() error: %v", err)
	}

	if _, err := s.DB().ExecContext(ctx, `DROP TABLE sync_runs`); err != nil {
		t.Fatalf("drop sync_runs table: %v", err)
	}

	cfg := store.RetentionSettings{SyncRunDays: 30, AuditDays: 30}

	// sync_runs errors, audit_log must still run; the returned error must
	// still reflect the sync_runs failure (job health, #384).
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err == nil {
		t.Fatal("RunCleanupOnce() with sync_runs dropped: want an error, got nil")
	}

	_, auditTotal, err := s.ListAuditRecords(ctx, "", "", "", "", 0, 20)
	if err != nil {
		t.Fatalf("ListAuditRecords() error: %v", err)
	}
	if auditTotal != 0 {
		t.Fatalf("audit records after cleanup = %d, want 0 (audit cleanup must still run after sync_runs error)", auditTotal)
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

	cfg := store.RetentionSettings{SyncRunDays: 30, CronExpr: "@daily"}

	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce: %v", err)
	}
	// Must not error or panic on an already-clean table.
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce (second pass): %v", err)
	}

	runs, err := s.ListSyncRunsByConnector(ctx, c.ID, 20)
	if err != nil {
		t.Fatalf("ListSyncRunsByConnector() error: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("sync runs after second cleanup = %d, want 0", len(runs))
	}
}

func TestCleanupPurgesOldTrash(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	old := &store.DocRecord{Title: "Old", Origin: store.DocOriginHuman, DeletedAt: time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339)}
	recent := &store.DocRecord{Title: "Recent", Origin: store.DocOriginHuman, DeletedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, d := range []*store.DocRecord{old, recent} {
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunCleanupOnce(ctx, s, store.RetentionSettings{DeletedDocsDays: 30}, testLogger()); err != nil {
		t.Fatal(err)
	}
	trash, err := s.ListDeletedDocs(ctx)
	if err != nil || len(trash) != 1 || trash[0].ID != recent.ID {
		t.Fatalf("trash: %+v %v", trash, err)
	}
}

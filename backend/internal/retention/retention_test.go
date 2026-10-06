package retention

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	activeID, goneID, recentID, mergedID, mergedRecentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	old := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339Nano)
	recent := time.Now().UTC().AddDate(0, 0, -2).Format(time.RFC3339Nano)
	// mergedRecentID was last observed long ago but merged recently: its
	// redirect must survive. mergedID was merged longer ago than the cutoff.
	for _, entity := range []struct{ id, gone, merged, mergedAt string }{
		{activeID, "", "", ""}, {goneID, old, "", ""}, {recentID, recent, "", ""},
		{mergedID, "", activeID, old}, {mergedRecentID, "", activeID, recent},
	} {
		var goneAt, mergedInto, mergedAt any
		if entity.gone != "" {
			goneAt = entity.gone
		}
		if entity.merged != "" {
			mergedInto, mergedAt = entity.merged, entity.mergedAt
		}
		if _, err := s.DB().ExecContext(ctx, `INSERT INTO entities(id,kind,display_name,first_seen_at,last_seen_at,gone_at,merged_into,merged_at) VALUES(?,?,?,?,?,?,?,?)`, entity.id, "vm", entity.id, old, old, goneAt, mergedInto, mergedAt); err != nil {
			t.Fatal(err)
		}
	}
	for _, member := range []struct{ id, ref, gone string }{{activeID, "active", ""}, {goneID, "gone", old}, {recentID, "recent", recent}, {mergedID, "merged", old}, {mergedRecentID, "merged-recent", old}} {
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
	if entities != 3 || members != 3 {
		t.Fatalf("after purge entities=%d members=%d, want active, recent-gone and recently merged", entities, members)
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

func createRetentionRunbookFixture(t *testing.T, s *store.Store) (*store.RunbookRecord, string) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	connector := &store.ConnectorRecord{
		Name:     "runbook-conn-" + suffix,
		Category: "virtualization",
		Type:     "proxmox",
		URL:      "https://example.com",
	}
	if err := s.CreateConnector(ctx, connector); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	runbook, err := s.CreateRunbook(ctx, &store.RunbookRecord{
		Title:       "Runbook " + suffix,
		TargetType:  "change_type",
		TargetValue: suffix,
	})
	if err != nil {
		t.Fatalf("CreateRunbook() error: %v", err)
	}
	return runbook, connector.ID
}

func createManualWaitRun(t *testing.T, s *store.Store, runbookID string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord) {
	t.Helper()
	ctx := context.Background()
	step := &store.RunbookRunStepRecord{
		Kind:  "manual",
		Title: "Operator confirmation",
	}
	run, steps, err := s.CreateRunbookRun(ctx, runbookID, "operator", []*store.RunbookRunStepRecord{step})
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "waiting"}); err != nil {
		t.Fatalf("UpdateRunbookRunStep() error: %v", err)
	}
	updatedRun, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "waiting_manual"})
	if err != nil {
		t.Fatalf("UpdateRunbookRun() error: %v", err)
	}
	return updatedRun, steps[0]
}

func createFailedRun(t *testing.T, s *store.Store, runbookID string) *store.RunbookRunRecord {
	t.Helper()
	ctx := context.Background()
	step := &store.RunbookRunStepRecord{
		Kind:  "manual",
		Title: "Operator step",
	}
	run, steps, err := s.CreateRunbookRun(ctx, runbookID, "operator", []*store.RunbookRunStepRecord{step})
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "failed", "error": "step error"}); err != nil {
		t.Fatalf("UpdateRunbookRunStep() error: %v", err)
	}
	updatedRun, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "failed", "reason": "step failed"})
	if err != nil {
		t.Fatalf("UpdateRunbookRun() error: %v", err)
	}
	return updatedRun
}

func createSucceededRun(t *testing.T, s *store.Store, runbookID, connectorID string) *store.RunbookRunRecord {
	t.Helper()
	ctx := context.Background()
	step := &store.RunbookRunStepRecord{
		Kind:        "lifecycle",
		Title:       "Restart service",
		ConnectorID: connectorID,
		Verb:        "restart",
		EntityRef:   "100",
	}
	run, steps, err := s.CreateRunbookRun(ctx, runbookID, "operator", []*store.RunbookRunStepRecord{step})
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
		t.Fatalf("UpdateRunbookRunStep() running error: %v", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "running", map[string]any{"state": "succeeded"}); err != nil {
		t.Fatalf("UpdateRunbookRunStep() succeeded error: %v", err)
	}
	updated, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "succeeded"})
	if err != nil {
		t.Fatalf("UpdateRunbookRun() succeeded error: %v", err)
	}
	return updated
}

// TestRunCleanupExpiresForgottenManualRun verifies a forgotten manual run
// waiting for longer than RunbookOpenRunHours becomes expired and skips its
// unfinished steps, allowing a new run of that runbook to be started.
func TestRunCleanupExpiresForgottenManualRun(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	runbook, _ := createRetentionRunbookFixture(t, s)

	run, step := createManualWaitRun(t, s, runbook.ID)
	oldUpdatedAt := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339Nano)
	if _, err := s.DB().ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, oldUpdatedAt, run.ID); err != nil {
		t.Fatalf("set updated_at error: %v", err)
	}

	cfg := store.RetentionSettings{RunbookOpenRunHours: 24}
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce() error: %v", err)
	}

	gotRun, gotSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRunbookRun() error: %v", err)
	}
	if gotRun.State != "expired" {
		t.Fatalf("run state = %q, want %q", gotRun.State, "expired")
	}
	if len(gotSteps) != 1 || gotSteps[0].ID != step.ID || gotSteps[0].State != "skipped" {
		t.Fatalf("run steps = %+v, want step %s skipped", gotSteps, step.ID)
	}

	// An expired run frees the runbook: a new run can be started.
	newRun, _, err := s.CreateRunbookRun(ctx, runbook.ID, "operator", []*store.RunbookRunStepRecord{
		{Kind: "manual", Title: "Fresh run"},
	})
	if err != nil {
		t.Fatalf("CreateRunbookRun after expiry error: %v", err)
	}
	if newRun.State != "running" {
		t.Fatalf("new run state = %q, want %q", newRun.State, "running")
	}
}

// backdateRunbookRun sets a run's updated_at to age ago, simulating a run
// whose last activity was that long in the past.
func backdateRunbookRun(t *testing.T, s *store.Store, runID string, age time.Duration) {
	t.Helper()
	updatedAt := time.Now().UTC().Add(-age).Format(time.RFC3339Nano)
	if _, err := s.DB().ExecContext(context.Background(), `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, updatedAt, runID); err != nil {
		t.Fatalf("set updated_at error: %v", err)
	}
}

// TestRunCleanupRecentActivityNotExpired verifies that in one pass runs idle
// for longer than RunbookOpenRunHours expire (skipping unfinished steps) while
// runs with recent activity keep their state, including a stale run that saw
// real activity before cleanup.
func TestRunCleanupRecentActivityNotExpired(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	staleWaitingRunbook, _ := createRetentionRunbookFixture(t, s)
	staleFailedRunbook, _ := createRetentionRunbookFixture(t, s)
	recentWaitingRunbook, _ := createRetentionRunbookFixture(t, s)
	recentFailedRunbook, _ := createRetentionRunbookFixture(t, s)
	activeRunbook, _ := createRetentionRunbookFixture(t, s)

	staleWaiting, staleWaitingStep := createManualWaitRun(t, s, staleWaitingRunbook.ID)
	staleFailed := createFailedRun(t, s, staleFailedRunbook.ID)
	recentWaiting, _ := createManualWaitRun(t, s, recentWaitingRunbook.ID)
	recentFailed := createFailedRun(t, s, recentFailedRunbook.ID)
	activeFailed := createFailedRun(t, s, activeRunbook.ID)

	backdateRunbookRun(t, s, staleWaiting.ID, 25*time.Hour)
	backdateRunbookRun(t, s, staleFailed.ID, 25*time.Hour)
	backdateRunbookRun(t, s, recentWaiting.ID, time.Hour)
	backdateRunbookRun(t, s, recentFailed.ID, time.Hour)

	// A stale failed run that is resumed and fails again has had real activity
	// (UpdateRunbookRun touches updated_at), so it must not expire.
	backdateRunbookRun(t, s, activeFailed.ID, 25*time.Hour)
	if _, err := s.UpdateRunbookRun(ctx, activeFailed.ID, "failed", map[string]any{"state": "running", "resumed_by": "operator"}); err != nil {
		t.Fatalf("resume stale run error: %v", err)
	}
	if _, err := s.UpdateRunbookRun(ctx, activeFailed.ID, "running", map[string]any{"state": "failed", "reason": "step failed again"}); err != nil {
		t.Fatalf("fail resumed run error: %v", err)
	}

	cfg := store.RetentionSettings{RunbookOpenRunHours: 24}
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce() error: %v", err)
	}

	for name, id := range map[string]string{"stale waiting": staleWaiting.ID, "stale failed": staleFailed.ID} {
		got, steps, err := s.GetRunbookRun(ctx, id)
		if err != nil {
			t.Fatalf("GetRunbookRun(%s) error: %v", name, err)
		}
		if got.State != "expired" {
			t.Fatalf("%s run state = %q, want %q", name, got.State, "expired")
		}
		if len(steps) != 1 {
			t.Fatalf("%s run steps = %+v, want 1 step", name, steps)
		}
		// The stale failed run's only step already failed; only unfinished steps are skipped.
		if id == staleWaiting.ID && (steps[0].ID != staleWaitingStep.ID || steps[0].State != "skipped") {
			t.Fatalf("%s run steps = %+v, want step %s skipped", name, steps, staleWaitingStep.ID)
		}
	}

	survivors := []struct {
		name      string
		id        string
		wantState string
	}{
		{"recent waiting", recentWaiting.ID, "waiting_manual"},
		{"recent failed", recentFailed.ID, "failed"},
		{"stale then active", activeFailed.ID, "failed"},
	}
	for _, tc := range survivors {
		got, steps, err := s.GetRunbookRun(ctx, tc.id)
		if err != nil {
			t.Fatalf("GetRunbookRun(%s) error: %v", tc.name, err)
		}
		if got.State != tc.wantState {
			t.Fatalf("%s run state = %q, want %q", tc.name, got.State, tc.wantState)
		}
		if tc.id == recentWaiting.ID && (len(steps) != 1 || steps[0].State != "waiting") {
			t.Fatalf("%s run steps = %+v, want step waiting", tc.name, steps)
		}
	}
}

// TestRunCleanupOpenRunHoursNonPositiveFallsBackToDefault verifies a
// non-positive RunbookOpenRunHours uses the default window instead of expiring
// everything or disabling expiry.
func TestRunCleanupOpenRunHoursNonPositiveFallsBackToDefault(t *testing.T) {
	for _, hours := range []int{0, -5} {
		t.Run(fmt.Sprintf("hours=%d", hours), func(t *testing.T) {
			ctx := context.Background()
			s := newTestStore(t)
			recentRunbook, _ := createRetentionRunbookFixture(t, s)
			staleRunbook, _ := createRetentionRunbookFixture(t, s)

			recent, _ := createManualWaitRun(t, s, recentRunbook.ID)
			stale, _ := createManualWaitRun(t, s, staleRunbook.ID)
			backdateRunbookRun(t, s, recent.ID, time.Hour)
			backdateRunbookRun(t, s, stale.ID, time.Duration(store.DefaultRunbookOpenRunHours+1)*time.Hour)

			cfg := store.RetentionSettings{RunbookOpenRunHours: hours}
			if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
				t.Fatalf("RunCleanupOnce() error: %v", err)
			}

			gotRecent, _, err := s.GetRunbookRun(ctx, recent.ID)
			if err != nil {
				t.Fatalf("GetRunbookRun(recent) error: %v", err)
			}
			if gotRecent.State != "waiting_manual" {
				t.Fatalf("recent run state = %q, want %q", gotRecent.State, "waiting_manual")
			}
			gotStale, _, err := s.GetRunbookRun(ctx, stale.ID)
			if err != nil {
				t.Fatalf("GetRunbookRun(stale) error: %v", err)
			}
			if gotStale.State != "expired" {
				t.Fatalf("stale run state = %q, want %q", gotStale.State, "expired")
			}
		})
	}
}

// TestRunCleanupPrunesFinishedRunbookRuns verifies terminal runs older than
// RunbookRunDays are pruned along with their steps, while recent terminal runs
// survive.
func TestRunCleanupPrunesFinishedRunbookRuns(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	runbook, connectorID := createRetentionRunbookFixture(t, s)

	oldRun := createSucceededRun(t, s, runbook.ID, connectorID)
	recentRun := createSucceededRun(t, s, runbook.ID, connectorID)

	oldFinishedAt := time.Now().UTC().AddDate(0, 0, -95).Format(time.RFC3339Nano)
	recentFinishedAt := time.Now().UTC().AddDate(0, 0, -5).Format(time.RFC3339Nano)
	if _, err := s.DB().ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, oldFinishedAt, oldRun.ID); err != nil {
		t.Fatalf("set finished_at error: %v", err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, recentFinishedAt, recentRun.ID); err != nil {
		t.Fatalf("set finished_at error: %v", err)
	}

	cfg := store.RetentionSettings{RunbookRunDays: 90}
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce() error: %v", err)
	}

	_, _, err := s.GetRunbookRun(ctx, oldRun.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old run lookup error = %v, want ErrNotFound", err)
	}
	var stepCount int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id = ?`, oldRun.ID).Scan(&stepCount); err != nil {
		t.Fatalf("count old run steps error: %v", err)
	}
	if stepCount != 0 {
		t.Fatalf("pruned run step count = %d, want 0", stepCount)
	}

	gotRecent, _, err := s.GetRunbookRun(ctx, recentRun.ID)
	if err != nil {
		t.Fatalf("recent run should survive, error: %v", err)
	}
	if gotRecent.ID != recentRun.ID {
		t.Fatalf("recent run ID = %q, want %q", gotRecent.ID, recentRun.ID)
	}
}

// TestRunCleanupRunbookHistoryPeriodZeroKeepsEverything verifies that a
// history period of 0 days (RunbookRunDays: 0) keeps all finished runs indefinitely.
func TestRunCleanupRunbookHistoryPeriodZeroKeepsEverything(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	runbook, connectorID := createRetentionRunbookFixture(t, s)

	oldRun := createSucceededRun(t, s, runbook.ID, connectorID)
	veryOldFinishedAt := time.Now().UTC().AddDate(0, 0, -365).Format(time.RFC3339Nano)
	if _, err := s.DB().ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, veryOldFinishedAt, oldRun.ID); err != nil {
		t.Fatalf("set finished_at error: %v", err)
	}

	cfg := store.RetentionSettings{RunbookRunDays: 0}
	if err := RunCleanupOnce(ctx, s, cfg, testLogger()); err != nil {
		t.Fatalf("RunCleanupOnce() error: %v", err)
	}

	gotOld, _, err := s.GetRunbookRun(ctx, oldRun.ID)
	if err != nil {
		t.Fatalf("run with RunbookRunDays=0 should survive, got error: %v", err)
	}
	if gotOld.ID != oldRun.ID {
		t.Fatalf("surviving run ID = %q, want %q", gotOld.ID, oldRun.ID)
	}
}

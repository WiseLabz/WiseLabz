package store

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
)

func rollbackRunbookApproval(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !hasColumn(t, db, driver, "runbook_runs", "requires_approval") {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback 000068: %v", err)
	}
	if hasColumn(t, db, driver, "runbook_runs", "requires_approval") {
		t.Fatal("000068 down left runbook approval columns")
	}
}

func TestRunbookRunApprovalMigrationUpDownPreservesFrozenSteps(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	run := func(t *testing.T, s *Store) {
		db := s.rawDB
		if s.driver == "sqlite" {
			if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
				t.Fatal(err)
			}
			requireSQLiteForeignKeysOn(t, db, s.driver, "before approval migration")
		}
		rollbackRunbookApproval(t, db, s.driver, logger)

		if err := migrationExec(t, db, s.driver, `
			INSERT INTO runbooks (id, title, target_type, target_value, created_at, updated_at)
			VALUES ('approval-preserved-book', 'Preserved', 'change_type', 'approval-preserved', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`); err != nil {
			t.Fatalf("insert runbook before 000068: %v", err)
		}
		if err := migrationExec(t, db, s.driver, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('approval-preserved-run', 'approval-preserved-book', 'Preserved', 'running', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`); err != nil {
			t.Fatalf("insert run before 000068: %v", err)
		}
		if err := migrationExec(t, db, s.driver, `
			INSERT INTO retention_settings (id, snapshot_days, doc_version_days, alert_days, sync_run_days,
				audit_days, health_check_days, report_days, deleted_docs_days, runbook_open_run_hours, runbook_run_days, cron_expr, updated_at)
			VALUES ('default', 90, 30, 90, 30, 180, 30, 30, 30, 24, 90, '0 3 * * *', '2026-01-01T00:00:00Z')
		`); err != nil {
			t.Fatalf("insert retention settings before 000068: %v", err)
		}
		if err := migrationExec(t, db, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state, action, action_fingerprint)
			VALUES ('approval-preserved-step', 'approval-preserved-run', 0, 'connector_action', 'Frozen action', 'pending', 'rescan', 'fingerprint')
		`); err != nil {
			t.Fatalf("insert connector action frozen step before 000068: %v", err)
		}
		if err := RunMigrations(db, s.driver, logger); err != nil {
			t.Fatalf("apply 000068: %v", err)
		}
		requireForeignKeysClean(t, db, s.driver, "after 000068 up")
		var runbookApproval, runApproval int
		if err := migrationQueryRow(t, db, s.driver, `SELECT requires_approval FROM runbooks WHERE id = 'approval-preserved-book'`).Scan(&runbookApproval); err != nil || runbookApproval != 0 {
			t.Fatalf("existing runbook requires_approval = %d, %v; want disabled", runbookApproval, err)
		}
		if err := migrationQueryRow(t, db, s.driver, `SELECT requires_approval FROM runbook_runs WHERE id = 'approval-preserved-run'`).Scan(&runApproval); err != nil || runApproval != 0 {
			t.Fatalf("existing run requires_approval = %d, %v; want disabled", runApproval, err)
		}
		var action, fingerprint string
		if err := migrationQueryRow(t, db, s.driver, `SELECT action, action_fingerprint FROM runbook_run_steps WHERE id = 'approval-preserved-step'`).Scan(&action, &fingerprint); err != nil || action != "rescan" || fingerprint != "fingerprint" {
			t.Fatalf("frozen connector action = %q/%q, %v; want intact", action, fingerprint, err)
		}
		var approvalHours int
		if err := migrationQueryRow(t, db, s.driver, `SELECT runbook_approval_hours FROM retention_settings WHERE id = 'default'`).Scan(&approvalHours); err != nil || approvalHours != DefaultRunbookApprovalHours {
			t.Fatalf("default runbook approval hours = %d, %v; want %d", approvalHours, err, DefaultRunbookApprovalHours)
		}

		// A real rejected run is finished and its pending steps were skipped by
		// the reject path; an open request has neither. updated_at differs from
		// the other timestamps so the down's COALESCE is observable.
		const (
			awaitingUpdated = "2026-01-01T06:00:00Z"
			rejectedUpdated = "2026-01-02T00:00:00Z"
			rejectedDone    = "2026-01-02T01:00:00Z"
			skippedDone     = "2026-01-02T00:30:00Z"
		)
		for _, row := range []struct {
			bookID, runID, state, updatedAt string
			finishedAt, stepState, stepDone any
		}{
			{"approval-awaiting-book", "approval-awaiting-run", "awaiting_approval", awaitingUpdated, nil, "pending", nil},
			{"approval-rejected-book", "approval-rejected-run", "rejected", rejectedUpdated, rejectedDone, "skipped", skippedDone},
		} {
			if err := migrationExec(t, db, s.driver, `
				INSERT INTO runbooks (id, title, target_type, target_value, requires_approval, created_at, updated_at)
				VALUES (?, 'Approval', 'change_type', ?, 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, row.bookID, row.bookID); err != nil {
				t.Fatalf("insert approval runbook: %v", err)
			}
			if err := migrationExec(t, db, s.driver, `
				INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, requires_approval, started_at, updated_at, finished_at)
				VALUES (?, ?, 'Approval', ?, 'actor', 1, '2026-01-01T00:00:00Z', ?, ?)
			`, row.runID, row.bookID, row.state, row.updatedAt, row.finishedAt); err != nil {
				t.Fatalf("insert %s run: %v", row.state, err)
			}
			if err := migrationExec(t, db, s.driver, `
				INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state, finished_at)
				VALUES (?, ?, 0, 'manual', 'Frozen', ?, ?)
			`, row.runID+"-step", row.runID, row.stepState, row.stepDone); err != nil {
				t.Fatalf("insert %s frozen step: %v", row.state, err)
			}
		}
		if err := RunMigrationsDown(db, s.driver, logger); err != nil {
			t.Fatalf("rollback 000068 with approval states: %v", err)
		}
		if hasColumn(t, db, s.driver, "runbooks", "requires_approval") ||
			hasColumn(t, db, s.driver, "runbook_runs", "requires_approval") ||
			hasColumn(t, db, s.driver, "retention_settings", "runbook_approval_hours") {
			t.Fatal("000068 down left an approval column")
		}
		for _, id := range []string{"approval-awaiting-run", "approval-rejected-run"} {
			var state string
			if err := migrationQueryRow(t, db, s.driver, `SELECT state FROM runbook_runs WHERE id = ?`, id).Scan(&state); err != nil || state != "cancelled" {
				t.Fatalf("state for %s after down = %q, %v; want cancelled", id, state, err)
			}
			var count int
			if err := migrationQueryRow(t, db, s.driver, `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id = ?`, id).Scan(&count); err != nil || count != 1 {
				t.Fatalf("frozen steps for %s after down = %d, %v; want 1", id, count, err)
			}
		}
		for id, want := range map[string]string{
			"approval-awaiting-run": awaitingUpdated, // open request: finished_at falls back to updated_at
			"approval-rejected-run": rejectedDone,    // finished run keeps its own finished_at
		} {
			var finished sql.NullString
			if err := migrationQueryRow(t, db, s.driver, `SELECT finished_at FROM runbook_runs WHERE id = ?`, id).Scan(&finished); err != nil || !finished.Valid || finished.String != want {
				t.Fatalf("finished_at for %s after down = %v, %v; want %s", id, finished, err, want)
			}
		}
		var stepState string
		var stepFinished sql.NullString
		if err := migrationQueryRow(t, db, s.driver, `SELECT state, finished_at FROM runbook_run_steps WHERE run_id = 'approval-awaiting-run'`).Scan(&stepState, &stepFinished); err != nil || stepState != "skipped" || !stepFinished.Valid || stepFinished.String != awaitingUpdated {
			t.Fatalf("awaiting run step after down = %q/%v, %v; want skipped at %s", stepState, stepFinished, err, awaitingUpdated)
		}
		if err := migrationQueryRow(t, db, s.driver, `SELECT state, finished_at FROM runbook_run_steps WHERE run_id = 'approval-rejected-run'`).Scan(&stepState, &stepFinished); err != nil || stepState != "skipped" || !stepFinished.Valid || stepFinished.String != skippedDone {
			t.Fatalf("rejected run step after down = %q/%v, %v; want untouched skipped at %s", stepState, stepFinished, err, skippedDone)
		}
		if err := migrationExec(t, db, s.driver, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('approval-state-check', 'approval-rejected-book', 'Approval', 'awaiting_approval', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`); err == nil {
			t.Fatal("000068 down still accepts awaiting_approval state")
		}
		requireForeignKeysClean(t, db, s.driver, "after 000068 down")
		if err := RunMigrations(db, s.driver, logger); err != nil {
			t.Fatalf("reapply 000068: %v", err)
		}
	}

	t.Run("sqlite", func(t *testing.T) { run(t, newCascadeTestStore(t)) })
	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("WISELABZ_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("WISELABZ_TEST_POSTGRES_DSN not set")
		}
		run(t, newPostgresTestStore(t, dsn, logger))
	})
}

func migrationQueryRow(t *testing.T, db *sql.DB, driver, query string, args ...any) *sql.Row {
	t.Helper()
	return db.QueryRowContext(context.Background(), bind(driver, query), args...)
}

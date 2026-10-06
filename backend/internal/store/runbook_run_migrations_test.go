package store

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
)

func rollbackRunbookRuns(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !hasColumn(t, db, driver, "retention_settings", "runbook_run_days") {
		t.Fatal("runbook retention columns missing before 000063 rollback")
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback runbook retention: %v", err)
	}
	if hasColumn(t, db, driver, "retention_settings", "runbook_open_run_hours") ||
		hasColumn(t, db, driver, "retention_settings", "runbook_run_days") ||
		!attachmentTableExists(t, db, driver, "runbook_runs") {
		t.Fatal("000063 rollback must remove only the new retention columns")
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback runbook runs: %v", err)
	}
	if attachmentTableExists(t, db, driver, "runbook_runs") ||
		attachmentTableExists(t, db, driver, "runbook_run_steps") ||
		hasColumn(t, db, driver, "runbook_steps", "kind") ||
		hasColumn(t, db, driver, "runbook_steps", "timeout_seconds") {
		t.Fatal("000062 rollback must remove run history and new step columns")
	}
}

func TestRunbookRunsMigrationsUpDownUp(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	run := func(t *testing.T, s *Store) {
		ctx := context.Background()
		rollbackRunbookRuns(t, s.rawDB, s.driver, logger)
		conn := &ConnectorRecord{Name: "migration connector", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
		if err := s.CreateConnector(ctx, conn); err != nil {
			t.Fatal(err)
		}
		r, err := s.CreateRunbook(ctx, &RunbookRecord{Title: "Original title", TargetType: "change_type", TargetValue: "migrations"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at)
			VALUES ('original-step', ?, 0, 'Original step', ?, 'restart', '100', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, r.ID, conn.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO retention_settings (id, snapshot_days, doc_version_days, alert_days, sync_run_days, audit_days, cron_expr, updated_at)
			VALUES ('default', 31, 32, 33, 34, 35, '0 3 * * *', '2026-01-01T00:00:00Z')
		`); err != nil {
			t.Fatal(err)
		}
		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatal(err)
		}
		step, err := s.GetRunbookStep(ctx, r.ID, "original-step")
		if err != nil || step.Kind != "lifecycle" || step.ConnectorID != conn.ID || step.Verb != "restart" || step.EntityRef != "100" {
			t.Fatalf("legacy step after upgrade = %+v, %v", step, err)
		}
		rs, err := s.GetRetentionSettings(ctx)
		if err != nil || rs.RunbookOpenRunHours != 24 || rs.RunbookRunDays != 90 || rs.SnapshotDays != 31 {
			t.Fatalf("retention after upgrade = %+v, %v", rs, err)
		}
		for i, kind := range []string{"manual", "sync_and_wait", "wait_until_healthy"} {
			connectorID := nilToStr(conn.ID)
			if kind == "manual" {
				connectorID = nil
			}
			if _, err := s.db.ExecContext(ctx, `
				INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, kind, r.ID, i+1, kind, kind, connectorID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('history-run', ?, 'Original title', 'running', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, r.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state)
			VALUES ('history-step', 'history-run', 0, 'manual', 'Frozen', 'pending')
		`); err != nil {
			t.Fatal(err)
		}
		for _, state := range []string{"running", "waiting_manual", "failed"} {
			if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET state = ? WHERE id = 'history-run'`, state); err != nil {
				t.Fatal(err)
			}
			_, err := s.db.ExecContext(ctx, `
				INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
				VALUES (?, ?, 'Second', 'running', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, "second-"+state, r.ID)
			if !isUniqueViolation(err) {
				t.Fatalf("second run with existing state %s: %v; want unique violation", state, err)
			}
		}
		rollbackRunbookRuns(t, s.rawDB, s.driver, logger)
		var title, verb, entityRef string
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM runbook_steps WHERE runbook_id = ?`, r.ID).Scan(&count); err != nil || count != 1 {
			t.Fatalf("steps after down = %d, %v; want original lifecycle only", count, err)
		}
		if err := s.db.QueryRowContext(ctx, `SELECT title, verb, entity_ref FROM runbook_steps WHERE id = 'original-step'`).Scan(&title, &verb, &entityRef); err != nil ||
			title != "Original step" || verb != "restart" || entityRef != "100" {
			t.Fatalf("original step after down = %q, %q, %q, %v", title, verb, entityRef, err)
		}
		if _, err := s.GetRunbook(ctx, r.ID); err != nil {
			t.Fatal("rollback removed original runbook:", err)
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, created_at, updated_at)
			VALUES ('invalid-manual', ?, 1, 'Manual', 'now', 'now')
		`, r.ID)
		if err == nil {
			t.Fatal("down did not restore non-null lifecycle connector and verb")
		}
		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatal(err)
		}
		step, err = s.GetRunbookStep(ctx, r.ID, "original-step")
		if err != nil || step.Kind != "lifecycle" || step.Title != "Original step" {
			t.Fatalf("legacy step after reapply = %+v, %v", step, err)
		}
		rs, err = s.GetRetentionSettings(ctx)
		if err != nil || rs.RunbookOpenRunHours != 24 || rs.RunbookRunDays != 90 || rs.SnapshotDays != 31 {
			t.Fatalf("retention after reapply = %+v, %v", rs, err)
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

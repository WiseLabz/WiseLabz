package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
)

var runbookStepKindFields = []string{"field_key", "target_value", "attribute", "operator", "expected_value"}

func runbookStepKindsWider(t *testing.T, db *sql.DB, driver string) bool {
	t.Helper()
	for _, table := range []string{"runbook_steps", "runbook_run_steps"} {
		var definition string
		if driver == "postgres" {
			err := db.QueryRow(`
				SELECT pg_get_constraintdef(c.oid)
				FROM pg_constraint c
				JOIN pg_class cl ON cl.oid = c.conrelid
				JOIN pg_namespace n ON n.oid = cl.relnamespace
				WHERE n.nspname = current_schema()
				  AND cl.relname = $1
				  AND c.conname = $2
			`, table, table+"_kind_check").Scan(&definition)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return false
				}
				t.Fatalf("query postgres %s kind constraint: %v", table, err)
			}
		} else {
			err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&definition)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return false
				}
				t.Fatalf("query sqlite_master for %s: %v", table, err)
			}
		}
		if !strings.Contains(definition, "config_push") || !strings.Contains(definition, "wait_for_entity") {
			return false
		}
	}
	return true
}

func rollbackRunbookStepKinds(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !runbookStepKindsWider(t, db, driver) {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback runbook step kinds migration: %v", err)
	}
	if runbookStepKindsWider(t, db, driver) {
		t.Fatal("000065 rollback must restore the original runbook step kind checks")
	}
}

func runbookStepKindsHaveFields(t *testing.T, db *sql.DB, driver string, want bool) {
	t.Helper()
	for _, table := range []string{"runbook_steps", "runbook_run_steps"} {
		for _, field := range runbookStepKindFields {
			if got := hasColumn(t, db, driver, table, field); got != want {
				t.Fatalf("%s.%s exists = %v, want %v", table, field, got, want)
			}
		}
	}
}

func requireRunbookStepIndexes(t *testing.T, db *sql.DB, driver string) {
	t.Helper()
	query := `SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='runbook_steps'`
	if driver == "postgres" {
		query = `SELECT indexname FROM pg_indexes WHERE schemaname=current_schema() AND tablename='runbook_steps'`
	}
	indexes := make(map[string]bool)
	for _, name := range queryStrings(t, db, query, 1) {
		indexes[name] = true
	}
	for _, name := range []string{"idx_runbook_steps_runbook", "idx_runbook_steps_connector"} {
		if !indexes[name] {
			t.Fatalf("runbook_steps index %s missing", name)
		}
	}
}

func migrationExec(t *testing.T, db *sql.DB, driver, query string, args ...any) error {
	t.Helper()
	_, err := db.Exec(bind(driver, query), args...)
	return err
}

func requireEmptyStepFields(t *testing.T, db *sql.DB, driver, table, id string) {
	t.Helper()
	query := "SELECT field_key, target_value, attribute, operator, expected_value FROM " + table + " WHERE id = ?"
	var values [5]string
	if err := db.QueryRow(bind(driver, query), id).Scan(&values[0], &values[1], &values[2], &values[3], &values[4]); err != nil {
		t.Fatalf("read %s fields for %s: %v", table, id, err)
	}
	for i, value := range values {
		if value != "" {
			t.Fatalf("%s.%s for %s = %q, want empty default", table, runbookStepKindFields[i], id, value)
		}
	}
}

func requireCount(t *testing.T, db *sql.DB, driver, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(bind(driver, query)).Scan(&got); err != nil {
		t.Fatalf("count query %q: %v", query, err)
	}
	if got != want {
		t.Fatalf("count for %q = %d, want %d", query, got, want)
	}
}

func TestRunbookStepKindsMigrationUpDown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	run := func(t *testing.T, s *Store) {
		ctx := context.Background()
		rollbackRunbookConnectorAction(t, s.rawDB, s.driver, logger)
		rollbackRunbookStepKinds(t, s.rawDB, s.driver, logger)
		runbookStepKindsHaveFields(t, s.rawDB, s.driver, false)

		connector := &ConnectorRecord{Name: "migration connector", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
		if err := s.CreateConnector(ctx, connector); err != nil {
			t.Fatal(err)
		}
		runbook := &RunbookRecord{ID: "step-kinds-runbook", Title: "Migration runbook", TargetType: "change_type", TargetValue: "migration"}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbooks (id, title, body, target_type, target_value, created_at, updated_at)
			VALUES (?, ?, '', ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, runbook.Title, runbook.TargetType, runbook.TargetValue); err != nil {
			t.Fatalf("insert runbook before 000065: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at)
			VALUES (?, ?, 0, 'Legacy lifecycle', ?, 'restart', 'vm-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, "legacy-step", runbook.ID, connector.ID); err != nil {
			t.Fatalf("insert legacy step before 000065: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('keep-run', ?, 'Migration runbook', 'succeeded', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID); err != nil {
			t.Fatalf("insert legacy run before 000065: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref, state)
			VALUES ('keep-step', 'keep-run', 0, 'lifecycle', 'Legacy lifecycle', ?, 'restart', 'vm-1', 'succeeded')
		`, connector.ID); err != nil {
			t.Fatalf("insert legacy frozen step before 000065: %v", err)
		}

		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatalf("apply 000065: %v", err)
		}
		runbookStepKindsHaveFields(t, s.rawDB, s.driver, true)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		requireEmptyStepFields(t, s.rawDB, s.driver, "runbook_steps", "legacy-step")
		requireEmptyStepFields(t, s.rawDB, s.driver, "runbook_run_steps", "keep-step")

		for i, kind := range []string{"config_push", "wait_for_entity"} {
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, verb, entity_ref,
				    created_at, updated_at, field_key, target_value, attribute, operator, expected_value)
				VALUES (?, ?, ?, ?, ?, ?, 'restart', 'vm-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z',
				    'cpu', '4', 'status', 'eq', 'running')
			`, "authored-"+kind, runbook.ID, i+1, kind, kind, connector.ID); err != nil {
				t.Fatalf("insert authored %s after 000065: %v", kind, err)
			}
		}
		for _, runID := range []string{"push-run", "wait-run"} {
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
				VALUES (?, ?, 'Migration runbook', 'succeeded', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, runID, runbook.ID); err != nil {
				t.Fatalf("insert %s: %v", runID, err)
			}
		}
		for i, kind := range []string{"config_push", "wait_for_entity"} {
			runID := []string{"push-run", "wait-run"}[i]
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref,
				    state, field_key, target_value, attribute, operator, expected_value)
				VALUES (?, ?, 0, ?, ?, ?, 'restart', 'vm-1', 'succeeded', 'cpu', '4', 'status', 'eq', 'running')
			`, "frozen-"+kind, runID, kind, kind, connector.ID); err != nil {
				t.Fatalf("insert frozen %s after 000065: %v", kind, err)
			}
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state)
			VALUES ('push-run-old-step', 'push-run', 1, 'lifecycle', 'Also frozen', 'succeeded')
		`); err != nil {
			t.Fatalf("insert additional push-run frozen step: %v", err)
		}

		// The apply above also brought up 000066, so roll that back first.
		rollbackRunbookConnectorAction(t, s.rawDB, s.driver, logger)
		if err := RunMigrationsDown(s.rawDB, s.driver, logger); err != nil {
			t.Fatalf("rollback 000065: %v", err)
		}
		runbookStepKindsHaveFields(t, s.rawDB, s.driver, false)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_steps WHERE id IN ('legacy-step')`, 1)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_steps WHERE kind IN ('config_push','wait_for_entity')`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_runs WHERE id IN ('push-run','wait-run')`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id IN ('push-run','wait-run')`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_runs WHERE id = 'keep-run'`, 1)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_run_steps WHERE id = 'keep-step'`, 1)

		for i, kind := range []string{"config_push", "wait_for_entity"} {
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, verb, created_at, updated_at)
				VALUES (?, ?, ?, 'Rejected', ?, ?, 'restart', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, "rejected-authored-"+kind, runbook.ID, i+10, kind, connector.ID); err == nil {
				t.Fatalf("runbook_steps accepted %s after 000065 down", kind)
			}
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state)
				VALUES (?, 'keep-run', ?, ?, 'Rejected', 'pending')
			`, "rejected-frozen-"+kind, i+10, kind); err == nil {
				t.Fatalf("runbook_run_steps accepted %s after 000065 down", kind)
			}
		}

		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatalf("reapply 000065: %v", err)
		}
		runbookStepKindsHaveFields(t, s.rawDB, s.driver, true)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		for i, kind := range []string{"config_push", "wait_for_entity"} {
			if err := migrationExec(t, s.rawDB, s.driver, `
				INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, verb, created_at, updated_at)
				VALUES (?, ?, ?, 'Reapplied', ?, ?, 'restart', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
			`, "reapplied-"+kind, runbook.ID, i+10, kind, connector.ID); err != nil {
				t.Fatalf("runbook_steps rejected %s after 000065 reapply: %v", kind, err)
			}
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

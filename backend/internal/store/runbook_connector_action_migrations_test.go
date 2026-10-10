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

// runbookConnectorActionTables are the two kind-checked tables that 000066 widens.
var runbookConnectorActionTables = []string{"runbook_steps", "runbook_run_steps"}

// runbookKindCheckDefinition returns the stored definition of table's kind
// check (postgres constraint or sqlite table SQL), and false when absent.
func runbookKindCheckDefinition(t *testing.T, db *sql.DB, driver, table string) (string, bool) {
	t.Helper()
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
				return "", false
			}
			t.Fatalf("query postgres %s kind constraint: %v", table, err)
		}
		return definition, true
	}
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&definition)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		t.Fatalf("query sqlite_master for %s: %v", table, err)
	}
	return definition, true
}

// runbookConnectorActionWider reports whether both kind checks accept connector_action.
func runbookConnectorActionWider(t *testing.T, db *sql.DB, driver string) bool {
	t.Helper()
	for _, table := range runbookConnectorActionTables {
		definition, ok := runbookKindCheckDefinition(t, db, driver, table)
		if !ok || !strings.Contains(definition, "connector_action") {
			return false
		}
	}
	return true
}

// rollbackRunbookConnectorAction rolls back 000066 when it is applied, so
// chained migration tests can start from the 000065 schema.
func rollbackRunbookConnectorAction(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	// Newer migration tests begin at the current schema: return to 000066's
	// predecessors newest first, 000068 then 000067.
	rollbackRunbookApproval(t, db, driver, logger)
	rollbackAuditLogConnectors(t, db, driver, logger)
	if !runbookConnectorActionWider(t, db, driver) {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback runbook connector action migration: %v", err)
	}
	if runbookConnectorActionWider(t, db, driver) {
		t.Fatal("000066 rollback must restore the 000065 runbook step kind checks")
	}
}

func runbookConnectorActionHaveColumns(t *testing.T, db *sql.DB, driver string, want bool) {
	t.Helper()
	for _, table := range runbookConnectorActionTables {
		if got := hasColumn(t, db, driver, table, "action"); got != want {
			t.Fatalf("%s.action exists = %v, want %v", table, got, want)
		}
	}
	if got := hasColumn(t, db, driver, "runbook_run_steps", "action_fingerprint"); got != want {
		t.Fatalf("runbook_run_steps.action_fingerprint exists = %v, want %v", got, want)
	}
}

// requireActionDefaults checks that a row's action and fingerprint hold their empty defaults.
func requireActionDefaults(t *testing.T, db *sql.DB, driver, table, id string) {
	t.Helper()
	query := "SELECT action FROM " + table + " WHERE id = ?"
	var action string
	if err := db.QueryRow(bind(driver, query), id).Scan(&action); err != nil {
		t.Fatalf("read action for %s.%s: %v", table, id, err)
	}
	if action != "" {
		t.Fatalf("%s.action for %s = %q, want empty default", table, id, action)
	}
}

// requireStepFieldValues checks the 000065 field columns of a row kept through 000066 up and down.
func requireStepFieldValues(t *testing.T, db *sql.DB, driver, table, id string) {
	t.Helper()
	query := "SELECT field_key, target_value, attribute, operator, expected_value FROM " + table + " WHERE id = ?"
	var values [5]string
	if err := db.QueryRow(bind(driver, query), id).Scan(&values[0], &values[1], &values[2], &values[3], &values[4]); err != nil {
		t.Fatalf("read %s fields for %s: %v", table, id, err)
	}
	want := [5]string{"cpu", "4", "status", "eq", "running"}
	if values != want {
		t.Fatalf("%s fields for %s = %v, want %v", table, id, values, want)
	}
}

func TestRunbookConnectorActionMigrationUpDown(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	run := func(t *testing.T, s *Store) {
		ctx := context.Background()
		rollbackRunbookConnectorAction(t, s.rawDB, s.driver, logger)
		runbookConnectorActionHaveColumns(t, s.rawDB, s.driver, false)
		if runbookConnectorActionWider(t, s.rawDB, s.driver) {
			t.Fatal("connector_action still accepted after 000066 rollback")
		}

		connector := &ConnectorRecord{Name: "connector action migration", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
		if err := s.CreateConnector(ctx, connector); err != nil {
			t.Fatal(err)
		}
		runbook := &RunbookRecord{ID: "connector-action-runbook", Title: "Connector action runbook", TargetType: "change_type", TargetValue: "connector-action"}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbooks (id, title, body, target_type, target_value, created_at, updated_at)
			VALUES (?, ?, '', ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, runbook.Title, runbook.TargetType, runbook.TargetValue); err != nil {
			t.Fatalf("insert runbook before 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, connector_id, verb, entity_ref, created_at, updated_at)
			VALUES ('legacy-step', ?, 0, 'Legacy lifecycle', ?, 'restart', 'vm-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, connector.ID); err != nil {
			t.Fatalf("insert legacy step before 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, entity_ref, created_at, updated_at,
			    field_key, target_value, attribute, operator, expected_value)
			VALUES ('legacy-config', ?, 1, 'Legacy config', 'config_push', ?, 'vm-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z',
			    'cpu', '4', 'status', 'eq', 'running')
		`, runbook.ID, connector.ID); err != nil {
			t.Fatalf("insert legacy config step before 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('keep-run', ?, 'Connector action runbook', 'succeeded', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID); err != nil {
			t.Fatalf("insert legacy run before 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref, state)
			VALUES ('keep-step', 'keep-run', 0, 'lifecycle', 'Legacy lifecycle', ?, 'restart', 'vm-1', 'succeeded')
		`, connector.ID); err != nil {
			t.Fatalf("insert legacy frozen step before 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, verb, entity_ref,
			    state, field_key, target_value, attribute, operator, expected_value)
			VALUES ('keep-config', 'keep-run', 1, 'config_push', 'Legacy config', ?, NULL, 'vm-1', 'succeeded',
			    'cpu', '4', 'status', 'eq', 'running')
		`, connector.ID); err != nil {
			t.Fatalf("insert legacy frozen config step before 000066: %v", err)
		}

		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatalf("apply 000066: %v", err)
		}
		runbookConnectorActionHaveColumns(t, s.rawDB, s.driver, true)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		requireActionDefaults(t, s.rawDB, s.driver, "runbook_steps", "legacy-step")
		requireActionDefaults(t, s.rawDB, s.driver, "runbook_steps", "legacy-config")
		requireActionDefaults(t, s.rawDB, s.driver, "runbook_run_steps", "keep-step")
		requireActionDefaults(t, s.rawDB, s.driver, "runbook_run_steps", "keep-config")
		requireStepFieldValues(t, s.rawDB, s.driver, "runbook_steps", "legacy-config")
		requireStepFieldValues(t, s.rawDB, s.driver, "runbook_run_steps", "keep-config")

		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, entity_ref, action,
			    created_at, updated_at)
			VALUES ('authored-connector-action', ?, 2, 'Rescan', 'connector_action', ?, 'vm-1', 'rescan',
			    '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, connector.ID); err != nil {
			t.Fatalf("insert authored connector_action after 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_runs (id, runbook_id, runbook_title, state, started_by, started_at, updated_at)
			VALUES ('action-run', ?, 'Connector action runbook', 'succeeded', 'actor', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID); err != nil {
			t.Fatalf("insert action-run: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, connector_id, entity_ref,
			    state, action, action_fingerprint)
			VALUES ('frozen-connector-action', 'action-run', 0, 'connector_action', 'Rescan', ?, 'vm-1',
			    'succeeded', 'rescan', 'sha256:fingerprint')
		`, connector.ID); err != nil {
			t.Fatalf("insert frozen connector_action after 000066: %v", err)
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state)
			VALUES ('action-run-old-step', 'action-run', 1, 'lifecycle', 'Also frozen', 'succeeded')
		`); err != nil {
			t.Fatalf("insert additional action-run frozen step: %v", err)
		}

		rollbackRunbookConnectorAction(t, s.rawDB, s.driver, logger)
		runbookConnectorActionHaveColumns(t, s.rawDB, s.driver, false)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_steps WHERE kind = 'connector_action'`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_steps WHERE id IN ('legacy-step','legacy-config')`, 2)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_runs WHERE id = 'action-run'`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id = 'action-run'`, 0)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_runs WHERE id = 'keep-run'`, 1)
		requireCount(t, s.rawDB, s.driver, `SELECT COUNT(*) FROM runbook_run_steps WHERE id IN ('keep-step','keep-config')`, 2)
		requireStepFieldValues(t, s.rawDB, s.driver, "runbook_steps", "legacy-config")
		requireStepFieldValues(t, s.rawDB, s.driver, "runbook_run_steps", "keep-config")

		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, created_at, updated_at)
			VALUES ('rejected-authored', ?, 10, 'Rejected', 'connector_action', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, connector.ID); err == nil {
			t.Fatal("runbook_steps accepted connector_action after 000066 down")
		}
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_run_steps (id, run_id, position, kind, title, state)
			VALUES ('rejected-frozen', 'keep-run', 10, 'connector_action', 'Rejected', 'pending')
		`); err == nil {
			t.Fatal("runbook_run_steps accepted connector_action after 000066 down")
		}

		if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
			t.Fatalf("reapply 000066: %v", err)
		}
		runbookConnectorActionHaveColumns(t, s.rawDB, s.driver, true)
		requireRunbookStepIndexes(t, s.rawDB, s.driver)
		if err := migrationExec(t, s.rawDB, s.driver, `
			INSERT INTO runbook_steps (id, runbook_id, position, title, kind, connector_id, created_at, updated_at)
			VALUES ('reapplied-connector-action', ?, 11, 'Reapplied', 'connector_action', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
		`, runbook.ID, connector.ID); err != nil {
			t.Fatalf("runbook_steps rejected connector_action after 000066 reapply: %v", err)
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

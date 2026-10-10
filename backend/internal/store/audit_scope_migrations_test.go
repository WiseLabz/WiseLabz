package store

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"slices"
	"testing"
)

func rollbackAuditLogConnectors(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !attachmentTableExists(t, db, driver, "audit_log_connectors") {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback audit connector scope: %v", err)
	}
	if attachmentTableExists(t, db, driver, "audit_log_connectors") {
		t.Fatal("000067 rollback must remove audit_log_connectors")
	}
}

func TestAuditScopeMigrationsBackfillUpDownUp(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	run := func(t *testing.T, s *Store) {
		ctx := context.Background()
		rollbackAuditLogConnectors(t, s.rawDB, s.driver, logger)
		a := createTestConnector(ctx, t, s)
		b := createTestConnector(ctx, t, s)
		change := ChangeRecord{ID: "change", ServiceID: a, ChangeType: "updated", Severity: "info"}
		if err := s.CreateChange(ctx, &change); err != nil {
			t.Fatal(err)
		}
		alert := AlertRecord{ID: "alert", ServiceID: b, Title: "alert", Severity: "info"}
		if err := s.CreateAlert(ctx, &alert); err != nil {
			t.Fatal(err)
		}
		for _, doc := range []DocRecord{
			{ID: "doc", Title: "Scoped", Kind: "service", ServiceID: a, Origin: DocOriginHuman},
			{ID: "lab-doc", Title: "Lab", Kind: "service", Origin: DocOriginHuman},
		} {
			if err := s.CreateDoc(ctx, &doc); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.db.ExecContext(ctx, `INSERT INTO runbook_runs
 (id, runbook_title, state, started_by, started_at, updated_at)
 VALUES ('run', 'Frozen', 'succeeded', 'actor', 'now', 'now')`); err != nil {
			t.Fatal(err)
		}
		for i, cid := range []string{a, a, b, ""} {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO runbook_run_steps
 (id, run_id, position, kind, title, connector_id, state) VALUES (?, 'run', ?, 'manual', 'step', ?, 'succeeded')`,
				[]string{"step-a", "step-a2", "step-b", "step-empty"}[i], i, cid); err != nil {
				t.Fatal(err)
			}
		}
		records := []AuditRecord{
			{ID: "connector", TargetType: "connector", TargetID: a},
			{ID: "deleted-connector", TargetType: "connector", TargetID: "gone"},
			{ID: "empty-connector", TargetType: "connector"},
			{ID: "change", TargetType: "change", TargetID: "change"},
			{ID: "pruned-change", TargetType: "change", TargetID: "pruned"},
			{ID: "alert", TargetType: "alert", TargetID: "alert"},
			{ID: "pruned-alert", TargetType: "alert", TargetID: "pruned"},
			{ID: "doc", TargetType: "doc", TargetID: "doc"},
			{ID: "lab-doc", TargetType: "doc", TargetID: "lab-doc"},
			{ID: "pruned-doc", TargetType: "doc", TargetID: "pruned"},
			{ID: "run", TargetType: "runbook_run", TargetID: "run"},
			{ID: "pruned-run", TargetType: "runbook_run", TargetID: "pruned"},
			{ID: "runbook", TargetType: "runbook", TargetID: "deleted-runbook", Detail: `{"steps":[{"connectorId":"` + a + `"},{"connectorId":"` + b + `"},{"connectorId":"` + a + `"},{"connectorId":""},{},{"connectorId":12},null,42,"text"]}`},
			{ID: "manual-runbook", TargetType: "runbook", Detail: `{"steps":[{"connectorId":""}]}`},
			{ID: "no-steps", TargetType: "runbook", Detail: `{"title":"old update"}`},
			{ID: "wrong-steps", TargetType: "runbook", Detail: `{"steps":{}}`},
			{ID: "other", TargetType: "user", TargetID: a, Detail: "invalid JSON outside runbook filter"},
		}
		if s.driver == "sqlite" {
			records = append(records, AuditRecord{ID: "invalid-runbook", TargetType: "runbook", Detail: "invalid JSON"})
		}
		for _, record := range records {
			if record.Detail == "" {
				record.Detail = "{}"
			}
			// Seed the old schema; the current writer requires the new scope table.
			if _, err := s.db.ExecContext(ctx, `INSERT INTO audit_log
 (id, actor_user_id, actor_role, action, target_type, target_id, detail, created_at)
 VALUES (?, '', '', 'legacy', ?, ?, ?, 'now')`, record.ID, record.TargetType, record.TargetID, record.Detail); err != nil {
				t.Fatal(err)
			}
		}
		want := []string{"alert:" + b, "change:" + a, "connector:" + a, "deleted-connector:gone", "doc:" + a,
			"run:" + a, "run:" + b, "runbook:" + a, "runbook:" + b}
		slices.Sort(want)
		for upgrade := 0; upgrade < 2; upgrade++ {
			if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
				t.Fatalf("apply 000067: %v", err)
			}
			got, err := scanAll(ctx, s.db, "audit scope", `SELECT audit_id || ':' || connector_id
 FROM audit_log_connectors ORDER BY audit_id, connector_id`, nil, func(row rowScanner) (string, error) {
				var pair string
				err := row.Scan(&pair)
				return pair, err
			})
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("backfilled scopes = %v, want %v: %v", got, want, err)
			}
			rollbackAuditLogConnectors(t, s.rawDB, s.driver, logger)
			var count int
			if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log").Scan(&count); err != nil || count != len(records) {
				t.Fatalf("down must preserve audit rows: %d, %v", count, err)
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

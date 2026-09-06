package store

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

// tablesCreatedByMigrations lists every table the initial migration creates,
// shared by the SQLite and PostgreSQL migration tests.
var tablesCreatedByMigrations = []string{
	"users", "sessions", "oidc_provider_flags", "connectors",
	"service_snapshots", "docs", "doc_versions", "templates",
	"template_sections", "changes", "alerts", "dashboard_layouts",
	"auth_config", "ai_config", "notification_config", "in_app_notifications",
	"quality_findings",
}

func TestRunMigrations(t *testing.T) {
	// Use file-based SQLite so golang-migrate can track schema version.
	// :memory: won't work because golang-migrate uses a separate connection
	// for the schema_migrations table.
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	// Verify tables exist by querying each one
	for _, table := range tablesCreatedByMigrations {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist or is not queryable: %v", table, err)
		}
	}

	// Verify idempotent — running again should be no-op
	if err := RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrations() second run error: %v", err)
	}
}

// TestRunMigrationsPostgres runs the postgres migration path against a real
// PostgreSQL instance. Opt-in: set WISELABZ_TEST_POSTGRES_DSN (e.g.
// "postgres://wiselabz:wiselabz@localhost:5432/wiselabz?sslmode=disable")
// to a database that RunMigrations is allowed to create tables in. Skipped
// otherwise, so `go test ./...` needs no Postgres instance by default.
func TestRunMigrationsPostgres(t *testing.T) {
	dsn := os.Getenv("WISELABZ_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WISELABZ_TEST_POSTGRES_DSN not set; skipping postgres migration test")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := RunMigrations(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	for _, table := range tablesCreatedByMigrations {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist or is not queryable: %v", table, err)
		}
	}

	// Verify idempotent — running again should be no-op
	if err := RunMigrations(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrations() second run error: %v", err)
	}
}

func TestRunMigrationsDown(t *testing.T) {
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	if err := RunMigrationsDown(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrationsDown() error: %v", err)
	}

	// RunMigrationsDown rolls back only 000005_quality_findings.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM quality_findings").Scan(&count); err == nil {
		t.Error("table quality_findings still exists after RunMigrationsDown()")
	}
	if rows, err := db.Query("SELECT owner FROM connectors"); err == nil {
		_ = rows.Close()
		t.Error("connectors.owner still exists after RunMigrationsDown()")
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count); err != nil {
		t.Errorf("table audit_log should still exist after rolling back only the last migration: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM saved_views").Scan(&count); err != nil {
		t.Errorf("table saved_views should still exist after rolling back only the last migration: %v", err)
	}
	for _, table := range tablesCreatedByMigrations {
		if table == "quality_findings" {
			continue
		}
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Errorf("table %s should still exist after rolling back only the last migration: %v", table, err)
		}
	}
}

// TestRunMigrationsDownPostgres mirrors TestRunMigrationsPostgres but for the
// down path. Opt-in via WISELABZ_TEST_POSTGRES_DSN; skipped otherwise.
func TestRunMigrationsDownPostgres(t *testing.T) {
	dsn := os.Getenv("WISELABZ_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("WISELABZ_TEST_POSTGRES_DSN not set; skipping postgres migration test")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	if err := db.Ping(); err != nil {
		t.Fatalf("ping db: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := RunMigrations(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	if err := RunMigrationsDown(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrationsDown() error: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM quality_findings").Scan(&count); err == nil {
		t.Error("table quality_findings still exists after RunMigrationsDown()")
	}
	if rows, err := db.Query("SELECT owner FROM connectors"); err == nil {
		_ = rows.Close()
		t.Error("connectors.owner still exists after RunMigrationsDown()")
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&count); err != nil {
		t.Errorf("table audit_log should still exist after rolling back only the last migration: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM saved_views").Scan(&count); err != nil {
		t.Errorf("table saved_views should still exist after rolling back only the last migration: %v", err)
	}
	for _, table := range tablesCreatedByMigrations {
		if table == "quality_findings" {
			continue
		}
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Errorf("table %s should still exist after rolling back only the last migration: %v", table, err)
		}
	}
}

func TestRunMigrationsDownUnsupportedDriver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	err = RunMigrationsDown(db, "mysql", logger)
	if err == nil {
		t.Error("expected error for unsupported driver, got nil")
	}
}

func TestRunMigrationsUnsupportedDriver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	err = RunMigrations(db, "mysql", logger)
	if err == nil {
		t.Error("expected error for unsupported driver, got nil")
	}
}

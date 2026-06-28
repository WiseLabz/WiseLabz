package store

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRunMigrations(t *testing.T) {
	// Use file-based SQLite so golang-migrate can track schema version.
	// :memory: won't work because golang-migrate uses a separate connection
	// for the schema_migrations table.
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	if err := RunMigrations(db, "sqlite3", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	// Verify tables exist by querying each one
	tables := []string{
		"users", "sessions", "oidc_provider_flags", "connectors",
		"service_snapshots", "docs", "doc_versions", "templates",
		"template_sections", "changes", "alerts", "dashboard_layouts",
		"auth_config", "ai_config", "notification_config", "in_app_notifications",
	}

	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s does not exist or is not queryable: %v", table, err)
		}
	}

	// Verify idempotent — running again should be no-op
	if err := RunMigrations(db, "sqlite3", logger); err != nil {
		t.Fatalf("RunMigrations() second run error: %v", err)
	}
}

func TestRunMigrationsUnsupportedDriver(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close() //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	err = RunMigrations(db, "postgres", logger)
	if err == nil {
		t.Error("expected error for unsupported driver, got nil")
	}
}

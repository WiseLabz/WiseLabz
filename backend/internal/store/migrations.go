package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/source/file" // required for file:// migration sources
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

// RunMigrations runs all pending database migrations for the given driver.
// Currently supports "sqlite3". PostgreSQL support will be added via build tags.
func RunMigrations(db *sql.DB, driver string, logger *slog.Logger) error {
	logger.Info("running database migrations")

	switch driver {
	case "sqlite3":
		return runSQLiteMigrations(db)
	case "postgres":
		return fmt.Errorf("postgres migrations not yet implemented (use -tags postgres)")
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func runSQLiteMigrations(db *sql.DB) error {
	sub, err := fs.Sub(sqliteMigrations, "migrations/sqlite")
	if err != nil {
		return fmt.Errorf("read sqlite migrations: %w", err)
	}

	src, err := iofs.New(sub, ".")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}

	driver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
	if err != nil {
		return fmt.Errorf("create migration driver: %w", err)
	}

	// golang-migrate's Close() calls database.Close() on the SQLite3 driver instance,
	// which is owned by the caller. Calling Close here breaks idempotent migration re-runs.
	//noinspection ALL
	m, err := migrate.NewWithInstance("iofs", src, "sqlite3", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	_ "github.com/golang-migrate/migrate/v4/source/file" // required for file:// migration sources
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

// RunMigrations runs all pending database migrations for the given driver.
// Supports "sqlite" and "postgres".
func RunMigrations(db *sql.DB, driver string, logger *slog.Logger) error {
	logger.Info("running database migrations")

	m, err := newMigrator(db, driver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// RunMigrationsDown rolls back the most recently applied migration for the
// given driver. Supports "sqlite" and "postgres".
func RunMigrationsDown(db *sql.DB, driver string, logger *slog.Logger) error {
	logger.Info("rolling back database migration")

	m, err := newMigrator(db, driver)
	if err != nil {
		return err
	}

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run down migration: %w", err)
	}

	return nil
}

// newMigrator builds a golang-migrate instance for the given driver, reusing
// the caller-owned *sql.DB connection.
//
// golang-migrate's Close() calls database.Close() on the underlying driver
// instance, which is owned by the caller. Never call Close() on the returned
// migrator — doing so breaks idempotent migration re-runs.
func newMigrator(db *sql.DB, driver string) (*migrate.Migrate, error) {
	switch driver {
	case "sqlite":
		sub, err := fs.Sub(sqliteMigrations, "migrations/sqlite")
		if err != nil {
			return nil, fmt.Errorf("read sqlite migrations: %w", err)
		}

		src, err := iofs.New(sub, ".")
		if err != nil {
			return nil, fmt.Errorf("create migration source: %w", err)
		}

		dbDriver, err := sqlite3.WithInstance(db, &sqlite3.Config{})
		if err != nil {
			return nil, fmt.Errorf("create migration driver: %w", err)
		}

		m, err := migrate.NewWithInstance("iofs", src, "sqlite3", dbDriver)
		if err != nil {
			return nil, fmt.Errorf("create migrator: %w", err)
		}
		return m, nil
	case "postgres":
		sub, err := fs.Sub(postgresMigrations, "migrations/postgres")
		if err != nil {
			return nil, fmt.Errorf("read postgres migrations: %w", err)
		}

		src, err := iofs.New(sub, ".")
		if err != nil {
			return nil, fmt.Errorf("create migration source: %w", err)
		}

		dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return nil, fmt.Errorf("create migration driver: %w", err)
		}

		m, err := migrate.NewWithInstance("iofs", src, "postgres", dbDriver)
		if err != nil {
			return nil, fmt.Errorf("create migrator: %w", err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

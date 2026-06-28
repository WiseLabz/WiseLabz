package store

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// OpenDB opens a database connection based on the driver and DSN.
// Currently supports "sqlite3". PostgreSQL support will be added via build tags.
func OpenDB(driver, dsn string) (*sql.DB, error) {
	if driver == "" {
		driver = "sqlite3"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Sensible defaults for connection pool
	db.SetMaxOpenConns(1) // SQLite single-writer. PostgreSQL can override later.
	db.SetMaxIdleConns(1)

	return db, nil
}

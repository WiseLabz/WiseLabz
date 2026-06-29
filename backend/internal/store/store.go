// Package store provides the central data store for WiseLabz.
package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Store is the central data access layer, holding the database connection
// and all repository implementations.
type Store struct {
	db *sql.DB
}

// New creates a new Store with the given database connection.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB returns the underlying database connection for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks the database connection health.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// initSingletons ensures singleton config rows exist (auth_config, ai_config, notification_config).
// Called after migrations run.
func (s *Store) initSingletons(ctx context.Context) error {
	rows := []string{
		`INSERT OR IGNORE INTO auth_config (id, local_enabled, access_token_ttl, refresh_token_ttl, step_up_for_destructive)
		 VALUES (1, 1, 900, 604800, 1)`,
		`INSERT OR IGNORE INTO ai_config (id, enabled, provider, model, api_key_encrypted, base_url, mode)
		 VALUES (1, 0, NULL, NULL, '', NULL, 'suggest_only')`,
		`INSERT OR IGNORE INTO notification_config (id, config_json)
		 VALUES (1, '{}')`,
	}

	for _, q := range rows {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("init singleton: %w", err)
		}
	}
	return nil
}

// Init bootstraps the store after migrations: seeds singletons and creates the admin user if needed.
func (s *Store) Init(ctx context.Context, adminPassword string) error {
	if err := s.initSingletons(ctx); err != nil {
		return err
	}

	// Check if any users exist
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if count == 0 && adminPassword != "" {
		// Seed admin user will be handled in Phase 2 when we have bcrypt
		// For now, just note that seeding is needed
		return nil
	}

	return nil
}

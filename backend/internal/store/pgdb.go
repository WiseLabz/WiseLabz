package store

import (
	"context"
	"database/sql"
)

// pgPlaceholderDB wraps a *sql.DB connected to PostgreSQL and rewrites
// SQLite-style `?` placeholders to PostgreSQL-style `$1, $2, ...` on every
// query, since the entire store package (and a few API handlers that issue
// raw SQL via Store.DB()) is written using `?` placeholders.
type pgPlaceholderDB struct {
	*sql.DB
}

func (p pgPlaceholderDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.DB.ExecContext(ctx, rewritePlaceholders(query), args...)
}

func (p pgPlaceholderDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.DB.QueryContext(ctx, rewritePlaceholders(query), args...)
}

func (p pgPlaceholderDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.DB.QueryRowContext(ctx, rewritePlaceholders(query), args...)
}

func (p pgPlaceholderDB) QueryRow(query string, args ...any) *sql.Row {
	return p.DB.QueryRow(rewritePlaceholders(query), args...)
}

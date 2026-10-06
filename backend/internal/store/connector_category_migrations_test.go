package store

import (
	"database/sql"
	"log/slog"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type columnMeta struct {
	Name         string
	Type         string
	NotNull      bool
	DefaultValue any
	PK           bool
}

func getTableColumns(t *testing.T, db *sql.DB, driver, table string) []columnMeta {
	t.Helper()
	var cols []columnMeta
	if driver == "postgres" {
		rows, err := db.Query(`
			SELECT column_name, data_type, is_nullable = 'NO', column_default
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = $1
			ORDER BY ordinal_position
		`, table)
		if err != nil {
			t.Fatalf("query postgres columns for %s: %v", table, err)
		}
		defer rows.Close() //nolint:errcheck
		for rows.Next() {
			var c columnMeta
			var dflt sql.NullString
			if err := rows.Scan(&c.Name, &c.Type, &c.NotNull, &dflt); err != nil {
				t.Fatalf("scan postgres column: %v", err)
			}
			if dflt.Valid {
				c.DefaultValue = dflt.String
			}
			cols = append(cols, c)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("postgres columns rows: %v", err)
		}
		return cols
	}

	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("query sqlite table_info for %s: %v", table, err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var cid int
		var c columnMeta
		var notNull int
		var pk int
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &c.DefaultValue, &pk); err != nil {
			t.Fatalf("scan sqlite table info: %v", err)
		}
		c.NotNull = notNull != 0
		c.PK = pk != 0
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("sqlite table info rows: %v", err)
	}
	return cols
}

func connectorCategoriesWider(t *testing.T, db *sql.DB, driver string) bool {
	t.Helper()
	if driver == "postgres" {
		var def string
		err := db.QueryRow(`
			SELECT pg_get_constraintdef(c.oid)
			FROM pg_constraint c
			JOIN pg_class cl ON cl.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = cl.relnamespace
			WHERE n.nspname = current_schema()
			  AND cl.relname = 'connectors'
			  AND c.conname = 'connectors_category_check'
		`).Scan(&def)
		if err != nil {
			if err == sql.ErrNoRows {
				return false
			}
			t.Fatalf("query postgres constraint connectors_category_check: %v", err)
		}
		return strings.Contains(def, "storage")
	}
	var sqlDef string
	err := db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='connectors'").Scan(&sqlDef)
	if err != nil {
		if err == sql.ErrNoRows {
			return false
		}
		t.Fatalf("query sqlite_master for connectors: %v", err)
	}
	return strings.Contains(sqlDef, "storage")
}

func rollbackConnectorCategories(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !connectorCategoriesWider(t, db, driver) {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback connector categories migration: %v", err)
	}
	if connectorCategoriesWider(t, db, driver) {
		t.Fatal("000064 rollback must restore original connector categories check constraint")
	}
}

func TestConnectorCategoriesMigrationUpDownUp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	run := func(t *testing.T, db *sql.DB, driver string) {
		// Ensure all migrations up to 000064 are applied initially.
		if err := RunMigrations(db, driver, logger); err != nil {
			t.Fatalf("initial RunMigrations: %v", err)
		}

		// Roll back 000064 so we are at schema 000063.
		rollbackConnectorCategories(t, db, driver, logger)

		// 1. Capture connectors table schema at version 000063.
		colsBefore := getTableColumns(t, db, driver, "connectors")
		if len(colsBefore) == 0 {
			t.Fatal("connectors columns missing before 000064 migration")
		}

		// 2. Seed rows with each of the 4 original categories.
		existingCategories := []string{"virtualization", "containers_paas", "networking", "dns"}
		for _, cat := range existingCategories {
			id := "conn-" + cat
			_, err := db.Exec(
				"INSERT INTO connectors (id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message, "+
					"created_at, updated_at, secret_rotated_at, user_expires_at, rotation_max_age_days, managed_by, config_hash) "+
					"VALUES (?, ?, ?, 'custom', 'https://example.com', 'admin', 1, '{}', 1, 'online', 'ok', "+
					"'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-12-31T00:00:00Z', 90, 'ui', 'hash1')",
				id, "Name "+cat, cat,
			)
			if err != nil {
				// PostgreSQL uses $1, $2, etc., but here we can handle driver placeholder differences if needed
				if driver == "postgres" {
					_, err = db.Exec(
						"INSERT INTO connectors (id, name, category, type, url, owner, verify_tls, config_data, enabled, status, status_message, "+
							"created_at, updated_at, secret_rotated_at, user_expires_at, rotation_max_age_days, managed_by, config_hash) "+
							"VALUES ($1, $2, $3, 'custom', 'https://example.com', 'admin', 1, '{}', 1, 'online', 'ok', "+
							"'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', '2026-12-31T00:00:00Z', 90, 'ui', 'hash1')",
						id, "Name "+cat, cat,
					)
				}
				if err != nil {
					t.Fatalf("seed existing connector %s: %v", cat, err)
				}
			}
		}

		// Seed a child row referencing conn-virtualization.
		var childErr error
		if driver == "postgres" {
			_, childErr = db.Exec("INSERT INTO service_snapshots (id, connector_id, data, fetched_at) VALUES ('snap-1', 'conn-virtualization', '{}', '2026-01-01T00:00:00Z')")
		} else {
			_, childErr = db.Exec("INSERT INTO service_snapshots (id, connector_id, data, fetched_at) VALUES ('snap-1', 'conn-virtualization', '{}', '2026-01-01T00:00:00Z')")
		}
		if childErr != nil {
			t.Fatalf("seed child snapshot: %v", childErr)
		}

		// Verify that at version 000063, new categories are rejected.
		insertStmt := func(id, cat string) error {
			if driver == "postgres" {
				_, err := db.Exec("INSERT INTO connectors (id, name, category, type, url, created_at, updated_at) VALUES ($1, 'Test', $2, 'custom', 'https://ex.com', '2026-01-01', '2026-01-01')", id, cat)
				return err
			}
			_, err := db.Exec("INSERT INTO connectors (id, name, category, type, url, created_at, updated_at) VALUES (?, 'Test', ?, 'custom', 'https://ex.com', '2026-01-01', '2026-01-01')", id, cat)
			return err
		}
		if err := insertStmt("conn-storage-early", "storage"); err == nil {
			t.Fatal("000063 accepted category 'storage'; want check constraint error")
		}

		// 3. Migrate UP to 000064.
		if err := RunMigrations(db, driver, logger); err != nil {
			t.Fatalf("RunMigrations() up to 000064 error: %v", err)
		}
		if !connectorCategoriesWider(t, db, driver) {
			t.Fatal("connectors_category_check was not widened after 000064 up")
		}

		// 4. Verify table schema is unchanged (columns, types, defaults, nullability).
		colsAfterUp := getTableColumns(t, db, driver, "connectors")
		if !reflect.DeepEqual(colsBefore, colsAfterUp) {
			t.Fatalf("connectors schema columns changed after 000064 up:\nbefore: %+v\nafter:  %+v", colsBefore, colsAfterUp)
		}

		// 5. Verify existing rows and child rows are intact.
		for _, cat := range existingCategories {
			var gotCat, gotOwner string
			var gotRotDays sql.NullInt64
			query := "SELECT category, owner, rotation_max_age_days FROM connectors WHERE id = "
			if driver == "postgres" {
				query += "$1"
			} else {
				query += "?"
			}
			if err := db.QueryRow(query, "conn-"+cat).Scan(&gotCat, &gotOwner, &gotRotDays); err != nil {
				t.Fatalf("query existing connector %s after up: %v", cat, err)
			}
			if gotCat != cat || gotOwner != "admin" || !gotRotDays.Valid || gotRotDays.Int64 != 90 {
				t.Fatalf("existing connector %s corrupted after up: cat=%s, owner=%s, rotDays=%v", cat, gotCat, gotOwner, gotRotDays)
			}
		}
		var childCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM service_snapshots WHERE id = 'snap-1'").Scan(&childCount); err != nil || childCount != 1 {
			t.Fatalf("child row count = %d, err = %v; want 1 preserved", childCount, err)
		}

		// 6. Verify all 4 new categories are accepted.
		newCategories := []string{"storage", "monitoring", "media", "other"}
		for _, cat := range newCategories {
			if err := insertStmt("conn-"+cat, cat); err != nil {
				t.Fatalf("failed to insert new category %s after 000064 up: %v", cat, err)
			}
			var readCat string
			q := "SELECT category FROM connectors WHERE id = "
			if driver == "postgres" {
				q += "$1"
			} else {
				q += "?"
			}
			if err := db.QueryRow(q, "conn-"+cat).Scan(&readCat); err != nil || readCat != cat {
				t.Fatalf("read new connector %s: got %s, err %v", cat, readCat, err)
			}
		}

		// 7. Verify an invalid category (e.g. gaming) is still rejected.
		if err := insertStmt("conn-gaming", "gaming"); err == nil {
			t.Fatal("category 'gaming' accepted; want check constraint error")
		}

		// 8. Down migration safety: when rows use new categories, down migration MUST fail.
		if err := RunMigrationsDown(db, driver, logger); err == nil {
			t.Fatal("RunMigrationsDown succeeded while rows had new categories; want failure")
		}

		// Verify rows were not deleted by the failed down migration.
		for _, cat := range newCategories {
			var count int
			q := "SELECT COUNT(*) FROM connectors WHERE id = "
			if driver == "postgres" {
				q += "$1"
			} else {
				q += "?"
			}
			if err := db.QueryRow(q, "conn-"+cat).Scan(&count); err != nil || count != 1 {
				t.Fatalf("row conn-%s missing after failed down migration (count=%d, err=%v)", cat, count, err)
			}
		}

		// Clear dirty state left by the intentional down migration failure so subsequent migration calls can proceed.
		m, cleanup, err := newMigrator(db, driver)
		if err != nil {
			t.Fatalf("newMigrator to clear dirty: %v", err)
		}
		if err := m.Force(64); err != nil {
			cleanup()
			t.Fatalf("m.Force(64): %v", err)
		}
		cleanup()

		// 9. Remove rows with new categories, then roll down.
		for _, cat := range newCategories {
			delQ := "DELETE FROM connectors WHERE id = "
			if driver == "postgres" {
				delQ += "$1"
			} else {
				delQ += "?"
			}
			if _, err := db.Exec(delQ, "conn-"+cat); err != nil {
				t.Fatalf("delete conn-%s: %v", cat, err)
			}
		}

		if err := RunMigrationsDown(db, driver, logger); err != nil {
			t.Fatalf("RunMigrationsDown after removing new category rows failed: %v", err)
		}
		if connectorCategoriesWider(t, db, driver) {
			t.Fatal("connectors categories check still wider after down migration")
		}

		// 10. Verify table schema after up+down matches the original pre-up schema.
		colsAfterDown := getTableColumns(t, db, driver, "connectors")
		if !reflect.DeepEqual(colsBefore, colsAfterDown) {
			t.Fatalf("connectors schema columns changed after up+down:\nbefore: %+v\nafter:  %+v", colsBefore, colsAfterDown)
		}

		// Existing rows still preserved after down.
		for _, cat := range existingCategories {
			var count int
			q := "SELECT COUNT(*) FROM connectors WHERE id = "
			if driver == "postgres" {
				q += "$1"
			} else {
				q += "?"
			}
			if err := db.QueryRow(q, "conn-"+cat).Scan(&count); err != nil || count != 1 {
				t.Fatalf("existing row conn-%s missing after down (count=%d, err=%v)", cat, count, err)
			}
		}

		// Inserting new category now fails again.
		if err := insertStmt("conn-storage-after-down", "storage"); err == nil {
			t.Fatal("category 'storage' accepted after down migration; want error")
		}

		// 11. Re-apply up to 000064.
		if err := RunMigrations(db, driver, logger); err != nil {
			t.Fatalf("re-applying 000064 up failed: %v", err)
		}
		if !connectorCategoriesWider(t, db, driver) {
			t.Fatal("connectors categories check not wider after re-apply")
		}
	}

	t.Run("sqlite", func(t *testing.T) {
		db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/categories.db?cache=shared")
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close() //nolint:errcheck
		run(t, db, "sqlite")
	})

	t.Run("postgres", func(t *testing.T) {
		dsn := os.Getenv("WISELABZ_TEST_POSTGRES_DSN")
		if dsn == "" {
			t.Skip("WISELABZ_TEST_POSTGRES_DSN not set; skipping postgres migration test")
		}
		admin, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer admin.Close() //nolint:errcheck
		schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE") }()
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		db, err := sql.Open("pgx", u.String())
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close() //nolint:errcheck
		run(t, db, "postgres")
	})
}

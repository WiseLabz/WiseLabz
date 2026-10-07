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

// bind rewrites ? placeholders to $1..$n for postgres, reusing the store's own
// rewriter.
func bind(driver, query string) string {
	if driver == "postgres" {
		return rewritePlaceholders(query)
	}
	return query
}

// connectorSchema is everything 000064 must leave unchanged besides the
// category CHECK: columns, indexes and the foreign keys pointing at connectors.
type connectorSchema struct {
	Columns     []columnMeta
	Indexes     []string
	ForeignKeys []string
}

func queryStrings(t *testing.T, db *sql.DB, query string, n int) []string {
	t.Helper()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close() //nolint:errcheck
	var out []string
	for rows.Next() {
		vals := make([]string, n)
		ptrs := make([]any, n)
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("scan %q: %v", query, err)
		}
		out = append(out, strings.Join(vals, " | "))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows %q: %v", query, err)
	}
	return out
}

func captureConnectorSchema(t *testing.T, db *sql.DB, driver string) connectorSchema {
	t.Helper()
	sc := connectorSchema{Columns: getTableColumns(t, db, driver, "connectors")}
	if driver == "postgres" {
		sc.Indexes = queryStrings(t, db, `SELECT indexname, indexdef FROM pg_indexes
			WHERE schemaname = current_schema() AND tablename = 'connectors' ORDER BY indexname`, 2)
		sc.ForeignKeys = queryStrings(t, db, `SELECT conrelid::regclass::text, pg_get_constraintdef(oid) FROM pg_constraint
			WHERE contype = 'f' AND confrelid = 'connectors'::regclass
			  AND connamespace = current_schema()::regnamespace ORDER BY 1, 2`, 2)
	} else {
		sc.Indexes = queryStrings(t, db, `SELECT name, COALESCE(sql, '') FROM sqlite_master
			WHERE type = 'index' AND tbl_name = 'connectors' ORDER BY name`, 2)
		// LIKE 'connectors%' also catches a stray reference to connectors_new.
		sc.ForeignKeys = queryStrings(t, db, `SELECT m.name, f."from", f."table", COALESCE(f."to", ''), f.on_delete
			FROM sqlite_master m, pragma_foreign_key_list(m.name) f
			WHERE m.type = 'table' AND f."table" LIKE 'connectors%' ORDER BY 1, 2`, 5)
		for _, fk := range sc.ForeignKeys {
			if parts := strings.Split(fk, " | "); parts[2] != "connectors" {
				t.Fatalf("foreign key %q does not reference connectors", fk)
			}
		}
	}
	if len(sc.ForeignKeys) == 0 {
		t.Fatal("no foreign keys reference connectors")
	}
	if !strings.Contains(strings.Join(sc.Indexes, "\n"), "idx_connectors_category") {
		t.Fatalf("idx_connectors_category missing: %v", sc.Indexes)
	}
	return sc
}

func requireSchemaUnchanged(t *testing.T, db *sql.DB, driver string, want connectorSchema, stage string) {
	t.Helper()
	if got := captureConnectorSchema(t, db, driver); !reflect.DeepEqual(want, got) {
		t.Fatalf("connectors schema changed %s:\nbefore: %+v\nafter:  %+v", stage, want, got)
	}
}

// snapshotConnectors reads every connector row with every column, keyed by id
// then column name, so a column dropped or swapped by a rebuild shows up.
func snapshotConnectors(t *testing.T, db *sql.DB) map[string]map[string]sql.NullString {
	t.Helper()
	rows, err := db.Query("SELECT * FROM connectors ORDER BY id")
	if err != nil {
		t.Fatalf("snapshot connectors: %v", err)
	}
	defer rows.Close() //nolint:errcheck
	cols, err := rows.Columns()
	if err != nil {
		t.Fatalf("snapshot columns: %v", err)
	}
	out := map[string]map[string]sql.NullString{}
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatalf("snapshot scan: %v", err)
		}
		row := map[string]sql.NullString{}
		for i, c := range cols {
			row[c] = vals[i]
		}
		out[row["id"].String] = row
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("snapshot rows: %v", err)
	}
	return out
}

// requireRowsPreserved checks every row captured in want is still present with
// identical values in every column.
func requireRowsPreserved(t *testing.T, db *sql.DB, want map[string]map[string]sql.NullString, stage string) {
	t.Helper()
	got := snapshotConnectors(t, db)
	for id, wantRow := range want {
		if !reflect.DeepEqual(wantRow, got[id]) {
			t.Fatalf("connector %s changed %s:\nbefore: %v\nafter:  %v", id, stage, wantRow, got[id])
		}
	}
}

// requireChecksEnforced proves the other connectors CHECK constraints survived
// the rebuild. verify_tls and enabled are INTEGER with CHECK(IN (0,1)) on both
// dialects, so every probe applies to both.
func requireChecksEnforced(t *testing.T, db *sql.DB, driver, stage string) {
	t.Helper()
	probes := []struct {
		column string
		value  any
	}{
		{"verify_tls", 2},
		{"enabled", 2},
		{"status", "bogus"},
		{"managed_by", "bogus"},
	}
	for _, p := range probes {
		_, err := db.Exec(bind(driver, "INSERT INTO connectors (id, name, category, type, url, created_at, updated_at, "+p.column+") "+
			"VALUES ('conn-probe', 'Probe', 'virtualization', 'custom', 'https://ex.com', '2026-01-01', '2026-01-01', ?)"), p.value)
		if err == nil {
			t.Fatalf("connectors accepted %s=%v %s; want check constraint error", p.column, p.value, stage)
		}
	}
}

func requireForeignKeysClean(t *testing.T, db *sql.DB, driver, stage string) {
	t.Helper()
	if driver != "sqlite" {
		return
	}
	if bad := queryStrings(t, db, "PRAGMA foreign_key_check", 4); len(bad) != 0 {
		t.Fatalf("foreign_key_check %s: %v", stage, bad)
	}
}

func requireSQLiteForeignKeysOn(t *testing.T, db *sql.DB, driver, stage string) {
	t.Helper()
	if driver != "sqlite" {
		return
	}
	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("PRAGMA foreign_keys %s = %d, %v; want 1", stage, fk, err)
	}
}

func TestConnectorCategoriesMigrationUpDownUp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	run := func(t *testing.T, db *sql.DB, driver string) {
		requireSQLiteForeignKeysOn(t, db, driver, "before the first migration")

		// Ensure all migrations up to 000064 are applied initially.
		if err := RunMigrations(db, driver, logger); err != nil {
			t.Fatalf("initial RunMigrations: %v", err)
		}

		// Roll back 000064 so we are at schema 000063.
		rollbackConnectorCategories(t, db, driver, logger)

		insertStmt := func(id, cat string) error {
			_, err := db.Exec(bind(driver, "INSERT INTO connectors (id, name, category, type, url, created_at, updated_at) "+
				"VALUES (?, 'Test', ?, 'custom', 'https://ex.com', '2026-01-01', '2026-01-01')"), id, cat)
			return err
		}

		// 1. Capture the 000063 schema.
		schemaBefore := captureConnectorSchema(t, db, driver)

		// 2. Seed one row per original category with a non-default, distinct
		// value in every column, so a column missing from the rebuild's
		// INSERT ... SELECT cannot go unnoticed.
		existingCategories := []string{"virtualization", "containers_paas", "networking", "dns"}
		for _, cat := range existingCategories {
			_, err := db.Exec(bind(driver, "INSERT INTO connectors (id, name, category, type, url, owner, verify_tls, config_data, enabled, "+
				"status, status_message, last_sync_at, schedule_seconds, next_run_at, last_sync_duration_ms, last_sync_error, retry_count, "+
				"credential_expires_at, created_at, updated_at, secret_rotated_at, user_expires_at, rotation_max_age_days, managed_by, config_hash) "+
				"VALUES (?, ?, ?, 'custom', 'https://example.com', 'admin', 0, '{\"k\":\"v\"}', 0, "+
				"'degraded', 'msg', '2026-02-01T00:00:00Z', 300, '2026-02-02T00:00:00Z', 1234, 'sync failed', 3, "+
				"'2026-02-03T00:00:00Z', '2026-02-04T00:00:00Z', '2026-02-05T00:00:00Z', '2026-02-06T00:00:00Z', '2026-12-31T00:00:00Z', 90, 'config', 'hash1')"),
				"conn-"+cat, "Name "+cat, cat)
			if err != nil {
				t.Fatalf("seed existing connector %s: %v", cat, err)
			}
		}
		rowsBefore := snapshotConnectors(t, db)
		if len(rowsBefore) != len(existingCategories) {
			t.Fatalf("seeded %d connectors, want %d", len(rowsBefore), len(existingCategories))
		}

		// Seed a child row referencing conn-virtualization.
		if _, err := db.Exec("INSERT INTO service_snapshots (id, connector_id, data, fetched_at) VALUES ('snap-1', 'conn-virtualization', '{}', '2026-01-01T00:00:00Z')"); err != nil {
			t.Fatalf("seed child snapshot: %v", err)
		}

		// Verify that at version 000063, new categories are rejected.
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

		// 4. Verify the schema is unchanged (columns, indexes, inbound foreign keys, other CHECKs).
		requireSchemaUnchanged(t, db, driver, schemaBefore, "after 000064 up")
		requireChecksEnforced(t, db, driver, "after 000064 up")
		requireForeignKeysClean(t, db, driver, "after 000064 up")

		// 5. Verify existing rows (every column) and child rows are intact.
		requireRowsPreserved(t, db, rowsBefore, "after 000064 up")
		var childCount int
		if err := db.QueryRow("SELECT COUNT(*) FROM service_snapshots WHERE id = 'snap-1'").Scan(&childCount); err != nil || childCount != 1 {
			t.Fatalf("child row count = %d, err = %v; want 1 preserved", childCount, err)
		}

		// The foreign key must still resolve to the rebuilt table and cascade.
		if err := insertStmt("conn-throwaway", "dns"); err != nil {
			t.Fatalf("insert throwaway connector: %v", err)
		}
		if _, err := db.Exec("INSERT INTO service_snapshots (id, connector_id, data, fetched_at) VALUES ('snap-throwaway', 'conn-throwaway', '{}', '2026-01-01T00:00:00Z')"); err != nil {
			t.Fatalf("insert throwaway snapshot: %v", err)
		}
		if _, err := db.Exec("DELETE FROM connectors WHERE id = 'conn-throwaway'"); err != nil {
			t.Fatalf("delete throwaway connector: %v", err)
		}
		if err := db.QueryRow("SELECT COUNT(*) FROM service_snapshots WHERE id = 'snap-throwaway'").Scan(&childCount); err != nil || childCount != 0 {
			t.Fatalf("throwaway snapshot count = %d, err = %v; want 0 after cascade", childCount, err)
		}

		// 6. Verify all 4 new categories are accepted.
		newCategories := []string{"storage", "monitoring", "media", "other"}
		for _, cat := range newCategories {
			if err := insertStmt("conn-"+cat, cat); err != nil {
				t.Fatalf("failed to insert new category %s after 000064 up: %v", cat, err)
			}
			var readCat string
			if err := db.QueryRow(bind(driver, "SELECT category FROM connectors WHERE id = ?"), "conn-"+cat).Scan(&readCat); err != nil || readCat != cat {
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

		// The failed down must change nothing: still the wide CHECK, same
		// schema, no row changed or deleted.
		if !connectorCategoriesWider(t, db, driver) {
			t.Fatal("connectors_category_check narrowed by a failed down migration")
		}
		requireSchemaUnchanged(t, db, driver, schemaBefore, "after failed 000064 down")
		requireRowsPreserved(t, db, rowsBefore, "after failed 000064 down")
		for _, cat := range newCategories {
			var count int
			if err := db.QueryRow(bind(driver, "SELECT COUNT(*) FROM connectors WHERE id = ?"), "conn-"+cat).Scan(&count); err != nil || count != 1 {
				t.Fatalf("row conn-%s missing after failed down migration (count=%d, err=%v)", cat, count, err)
			}
		}

		// golang-migrate leaves the version marked dirty after a failed step.
		st, err := GetMigrationStatus(db, driver)
		if err != nil {
			t.Fatalf("GetMigrationStatus after failed down: %v", err)
		}
		if !st.Dirty {
			t.Fatalf("migration status after failed down = %+v; want Dirty", st)
		}

		// Clear the dirty state: this documents the recovery for the dirty
		// version a failed golang-migrate step leaves behind.
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
			if _, err := db.Exec(bind(driver, "DELETE FROM connectors WHERE id = ?"), "conn-"+cat); err != nil {
				t.Fatalf("delete conn-%s: %v", cat, err)
			}
		}

		if err := RunMigrationsDown(db, driver, logger); err != nil {
			t.Fatalf("RunMigrationsDown after removing new category rows failed: %v", err)
		}
		if connectorCategoriesWider(t, db, driver) {
			t.Fatal("connectors categories check still wider after down migration")
		}

		// 10. Verify the schema after up+down matches the original pre-up schema.
		requireSchemaUnchanged(t, db, driver, schemaBefore, "after up+down")
		requireChecksEnforced(t, db, driver, "after 000064 down")
		requireForeignKeysClean(t, db, driver, "after 000064 down")
		requireRowsPreserved(t, db, rowsBefore, "after 000064 down")
		if err := db.QueryRow("SELECT COUNT(*) FROM service_snapshots WHERE id = 'snap-1'").Scan(&childCount); err != nil || childCount != 1 {
			t.Fatalf("child row count after down = %d, err = %v; want 1 preserved", childCount, err)
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
		requireSQLiteForeignKeysOn(t, db, driver, "at the end")
	}

	t.Run("sqlite", func(t *testing.T) {
		db, err := OpenDB("sqlite", "file:"+t.TempDir()+"/categories.db")
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

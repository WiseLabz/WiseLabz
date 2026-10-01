package main

import (
	"database/sql"
	"io"
	"log/slog"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:" + storetest.MigratedSQLite(t) + "?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrateStatusCommand(t *testing.T) {
	db := newTestDB(t)
	st, err := store.GetMigrationStatus(db, "sqlite")
	if err != nil {
		t.Fatalf("GetMigrationStatus: %v", err)
	}

	if st.Current == 0 {
		t.Errorf("migration current version should not be 0 after running migrations")
	}

	if st.Latest == 0 {
		t.Errorf("migration latest version should not be 0")
	}

	if st.Current != st.Latest {
		t.Errorf("after migrations complete, current (%d) should equal latest (%d)", st.Current, st.Latest)
	}

	if st.Dirty {
		t.Errorf("after clean migrations, Dirty should be false")
	}

	if st.Pending() {
		t.Errorf("after clean migrations, Pending() should be false")
	}
}

func TestMigrateUpIdempotent(t *testing.T) {
	db := newTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	st1, err := store.GetMigrationStatus(db, "sqlite")
	if err != nil {
		t.Fatalf("first status check: %v", err)
	}

	// Running migrations again should be idempotent
	if err := store.RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	st2, err := store.GetMigrationStatus(db, "sqlite")
	if err != nil {
		t.Fatalf("second status check: %v", err)
	}

	if st1.Current != st2.Current {
		t.Errorf("running migrations twice should not change version: %d != %d", st1.Current, st2.Current)
	}
}

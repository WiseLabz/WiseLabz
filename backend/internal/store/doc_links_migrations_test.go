package store

import (
	"database/sql"
	"log/slog"
	"testing"
)

func rollbackDocLinks(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	rollbackNotificationFiles(t, db, driver, logger)
	if !hasDocLinksTable(t, db, driver) {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback doc links: %v", err)
	}
	if hasDocLinksTable(t, db, driver) {
		t.Fatal("doc_links still exists after down")
	}
}

// rollbackNotificationFiles removes migration 000070 before helpers that
// intentionally exercise the previous latest migration, 000069_doc_links.
func rollbackNotificationFiles(t *testing.T, db *sql.DB, driver string, logger *slog.Logger) {
	t.Helper()
	if !attachmentTableExists(t, db, driver, "notification_files") {
		return
	}
	if err := RunMigrationsDown(db, driver, logger); err != nil {
		t.Fatalf("rollback notification files: %v", err)
	}
	if attachmentTableExists(t, db, driver, "notification_files") {
		t.Fatal("notification_files still exists after down")
	}
}

func TestDocLinksMigrationUpDownUp(t *testing.T) {
	t.Parallel()
	s := newDocTestStore(t)
	logger := slog.New(slog.DiscardHandler)
	if !hasDocLinksTable(t, s.rawDB, s.driver) {
		t.Fatal("doc_links missing after up")
	}
	rollbackDocLinks(t, s.rawDB, s.driver, logger)
	if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
		t.Fatal(err)
	}
	if !hasDocLinksTable(t, s.rawDB, s.driver) {
		t.Fatal("doc_links missing after re-up")
	}
}

func hasDocLinksTable(t *testing.T, db *sql.DB, driver string) bool {
	t.Helper()
	var count int
	query := `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'doc_links'`
	if driver == "postgres" {
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = 'doc_links'`
	}
	if err := db.QueryRow(query).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count > 0
}

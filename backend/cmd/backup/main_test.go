package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"
)

func newSeededStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := "file:" + storetest.MigratedSQLite(t) + "?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	s := store.New(db, "sqlite")
	if err := s.Init(context.Background(), "admin-seed-pw-1234"); err != nil {
		t.Fatalf("store init: %v", err)
	}
	if err := s.CreateDoc(context.Background(), &store.DocRecord{ID: "doc-1", Title: "Runbook", Content: "steps"}); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	return s
}

func TestRunVerifyPassAndFail(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()
	s := newSeededStore(t)

	run, err := backup.ExportToFile(ctx, s, srcDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	if code := runVerify([]string{"-file", run.FilePath}); code != 0 {
		t.Errorf("runVerify(intact bundle) = %d, want 0", code)
	}

	// Corrupt the bundle in place and confirm verify now fails.
	data, err := os.ReadFile(run.FilePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	data[len(data)/2] ^= 0xFF
	if err := os.WriteFile(run.FilePath, data, 0o600); err != nil {
		t.Fatalf("corrupt bundle: %v", err)
	}
	if code := runVerify([]string{"-file", run.FilePath}); code != 1 {
		t.Errorf("runVerify(corrupted bundle) = %d, want 1", code)
	}
}

func TestRunVerifyDefaultsToLatestInDir(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()
	s := newSeededStore(t)

	if _, err := backup.ExportToFile(ctx, s, srcDir); err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	if code := runVerify([]string{"-dir", srcDir}); code != 0 {
		t.Errorf("runVerify(-dir) = %d, want 0", code)
	}
}

func TestRunVerifyRequiresABundle(t *testing.T) {
	if code := runVerify([]string{"-dir", t.TempDir()}); code != 1 {
		t.Errorf("runVerify(empty dir) = %d, want 1", code)
	}
}

func TestRunRestoreImportsIntoConfiguredDatabase(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()
	s := newSeededStore(t)

	run, err := backup.ExportToFile(ctx, s, srcDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	targetDir := t.TempDir()
	dsn := fmt.Sprintf("file:%s/target.db?cache=shared", targetDir)
	t.Setenv("WISELABZ_DB_DRIVER", "sqlite")
	t.Setenv("WISELABZ_DB_DSN", dsn)

	if code := runRestore([]string{"-file", run.FilePath, "-yes"}); code != 0 {
		t.Fatalf("runRestore = %d, want 0", code)
	}

	// Confirm the doc actually landed in the target database.
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer func() { _ = db.Close() }()

	target := store.New(db, "sqlite")
	doc, err := target.GetDoc(ctx, "doc-1")
	if err != nil {
		t.Fatalf("GetDoc after restore: %v", err)
	}
	if doc.Title != "Runbook" {
		t.Errorf("restored doc title = %q, want Runbook", doc.Title)
	}
}

func TestRunRestoreRejectsCorruptedBundle(t *testing.T) {
	ctx := context.Background()
	srcDir := t.TempDir()
	s := newSeededStore(t)

	run, err := backup.ExportToFile(ctx, s, srcDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}
	data, err := os.ReadFile(run.FilePath)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	data[len(data)/2] ^= 0xFF
	if err := os.WriteFile(run.FilePath, data, 0o600); err != nil {
		t.Fatalf("corrupt bundle: %v", err)
	}

	targetDir := t.TempDir()
	t.Setenv("WISELABZ_DB_DRIVER", "sqlite")
	t.Setenv("WISELABZ_DB_DSN", fmt.Sprintf("file:%s/target.db?cache=shared", targetDir))

	if code := runRestore([]string{"-file", run.FilePath, "-yes"}); code != 1 {
		t.Errorf("runRestore(corrupted bundle) = %d, want 1", code)
	}
}

func TestRunRestoreRequiresFileFlag(t *testing.T) {
	if code := runRestore(nil); code != 1 {
		t.Errorf("runRestore(no -file) = %d, want 1", code)
	}
}

func TestConfirmYes(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_, _ = w.WriteString("y\n")
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if !confirm("Continue?") {
		t.Errorf("confirm with 'y' input should return true")
	}
}

func TestConfirmYesLong(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_, _ = w.WriteString("yes\n")
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if !confirm("Continue?") {
		t.Errorf("confirm with 'yes' input should return true")
	}
}

func TestConfirmNo(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_, _ = w.WriteString("n\n")
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if confirm("Continue?") {
		t.Errorf("confirm with 'n' input should return false")
	}
}

func TestConfirmEOF(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if confirm("Continue?") {
		t.Errorf("confirm with EOF should return false")
	}
}

func TestConfirmEmpty(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_, _ = w.WriteString("\n")
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if confirm("Continue?") {
		t.Errorf("confirm with empty input should return false")
	}
}

func TestConfirmCaseInsensitive(t *testing.T) {
	r, w, _ := os.Pipe()
	defer func() { _ = r.Close() }()

	go func() {
		_, _ = w.WriteString("Y\n")
		_ = w.Close()
	}()

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r

	if !confirm("Continue?") {
		t.Errorf("confirm with 'Y' input should return true (case insensitive)")
	}
}

func TestRestoreValidatesCustomConnectorRecipes(t *testing.T) {
	ctx := context.Background()

	// The backup binary must register connector types itself: without them the
	// import skips recipe validation.
	schema, err := connector.GetTypeSchema("custom")
	if err != nil {
		t.Fatalf("GetTypeSchema(custom) = %v; cmd/backup must import connector/all", err)
	}
	if schema.ImportConfigCheck == nil {
		t.Fatal("custom ImportConfigCheck = nil, want recipe validation on import")
	}

	srcDir := t.TempDir()
	s := newSeededStore(t)
	recipe := `version: 1
category: monitoring
auth: {mode: none}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
actions:
  purge:
    method: GET
    path: /purge
`
	configData, err := json.Marshal(map[string]any{"recipe": recipe})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateConnector(ctx, &store.ConnectorRecord{
		ID: "bad-custom", Name: "Invalid Recipe", Category: "monitoring", Type: "custom",
		URL: "https://api.example.com", ConfigData: string(configData),
	}); err != nil {
		t.Fatalf("seed connector: %v", err)
	}
	run, err := backup.ExportToFile(ctx, s, srcDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	dsn := "file:" + storetest.MigratedSQLite(t) + "?cache=shared"
	t.Setenv("WISELABZ_DB_DRIVER", "sqlite")
	t.Setenv("WISELABZ_DB_DSN", dsn)
	if code := runRestore([]string{"-file", run.FilePath, "-yes"}); code != 1 {
		t.Fatalf("runRestore(invalid custom recipe) = %d, want 1", code)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open target db: %v", err)
	}
	defer func() { _ = db.Close() }()
	connectors, err := store.New(db, "sqlite").ListAllConnectors(ctx)
	if err != nil {
		t.Fatalf("ListAllConnectors: %v", err)
	}
	if len(connectors) != 0 {
		t.Fatalf("connectors after rejected restore = %d, want 0", len(connectors))
	}
}

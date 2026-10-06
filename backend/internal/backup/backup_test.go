package backup_test

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"

	// Register connector implementations (proxmox, opnsense, ...) so
	// GetTypeSchema resolves their secret fields the same way it does in
	// production, wired via internal/api/router.go's blank import.
	_ "github.com/WiseLabz/wiselabz/internal/connector/all"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if pgDSN := os.Getenv("WISELABZ_TEST_POSTGRES_DSN"); pgDSN != "" {
		return newPostgresTestStore(t, pgDSN, logger)
	}
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
	return s
}

func newPostgresTestStore(t *testing.T, dsn string, logger *slog.Logger) *store.Store {
	t.Helper()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse postgres dsn: %v", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open postgres schema db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = admin.Close()
	})
	if err := store.RunMigrations(db, "postgres", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}
	s := store.New(db, "postgres")
	if err := s.Init(context.Background(), "admin-seed-pw-1234"); err != nil {
		t.Fatalf("store init: %v", err)
	}
	return s
}

// testEncKey is a fixed valid base64-encoded 32-byte AES-256 key for tests
// that need to round-trip connector config through MarshalConnectorConfig
// (mirrors backend/internal/api/testapp_test.go's test key).
const testEncKey = "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="

func TestExportRedactsConnectorSecrets(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	proxmoxCfg, err := store.MarshalConnectorConfig("proxmox", map[string]any{
		"url":          "https://pve.example.com:8006",
		"token_id":     "root@pam!monitoring",
		"token_secret": "super-secret-token",
	}, testEncKey)
	if err != nil {
		t.Fatalf("marshal proxmox config: %v", err)
	}
	if err := s.CreateConnector(ctx, &store.ConnectorRecord{
		Name: "pve1", Category: "virtualization", Type: "proxmox", URL: "https://pve.example.com:8006",
		ConfigData: proxmoxCfg,
	}); err != nil {
		t.Fatalf("create proxmox connector: %v", err)
	}

	opnsenseCfg, err := store.MarshalConnectorConfig("opnsense", map[string]any{
		"url":        "https://fw.example.com",
		"api_key":    "some-key",
		"api_secret": "super-secret-api-secret",
	}, testEncKey)
	if err != nil {
		t.Fatalf("marshal opnsense config: %v", err)
	}
	if err := s.CreateConnector(ctx, &store.ConnectorRecord{
		Name: "fw1", Category: "networking", Type: "opnsense", URL: "https://fw.example.com",
		ConfigData: opnsenseCfg,
	}); err != nil {
		t.Fatalf("create opnsense connector: %v", err)
	}

	b, err := backup.Export(ctx, s)
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	rawStr := string(raw)

	if strings.Contains(rawStr, "super-secret-token") {
		t.Error("exported bundle contains the proxmox token_secret value")
	}
	if strings.Contains(rawStr, "super-secret-api-secret") {
		t.Error("exported bundle contains the opnsense api_secret value")
	}
	if strings.Contains(rawStr, "apiKey") || strings.Contains(rawStr, "api_key_encrypted") {
		t.Error("exported bundle contains an AI API key field")
	}

	for _, c := range b.Connectors {
		cfg, err := store.ParseConnectorConfig(c.Type, c.ConfigData, testEncKey)
		if err != nil {
			t.Fatalf("parse connector config: %v", err)
		}
		if c.Type == "proxmox" {
			if _, ok := cfg["token_secret"]; ok {
				t.Error("proxmox connector still has token_secret in exported config")
			}
			if _, ok := cfg["token_id"]; !ok {
				t.Error("proxmox connector lost non-secret field token_id")
			}
		}
		if c.Type == "opnsense" {
			if _, ok := cfg["api_secret"]; ok {
				t.Error("opnsense connector still has api_secret in exported config")
			}
			if _, ok := cfg["api_key"]; ok {
				t.Error("opnsense connector still has api_key in exported config")
			}
			if _, ok := cfg["url"]; !ok {
				t.Error("opnsense connector lost non-secret field url")
			}
		}
	}
}

func TestValidateBundleRejectsWrongVersion(t *testing.T) {
	b := &backup.Bundle{Version: backup.BundleVersion + 1}
	err := backup.ValidateBundle(b)
	if err == nil {
		t.Fatal("expected error for mismatched version, got nil")
	}
}

func TestValidateBundleRejectsOrphanDocVersion(t *testing.T) {
	b := &backup.Bundle{
		Version: backup.BundleVersion,
		Docs:    []store.DocRecord{{ID: "doc-1"}},
		DocVersions: []store.DocVersionRecord{
			{ID: "ver-1", DocID: "doc-does-not-exist", Rev: 1},
		},
	}
	err := backup.ValidateBundle(b)
	if err == nil {
		t.Fatal("expected error for doc version referencing unknown doc, got nil")
	}
}

func TestImportRejectsInvalidBundleBeforeWriting(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	b := &backup.Bundle{
		Version: backup.BundleVersion + 1,
		Connectors: []store.ConnectorRecord{{
			Name: "must-not-import", Category: "virtualization", Type: "proxmox", URL: "https://pve.example.com",
		}},
	}

	if _, err := backup.Import(ctx, s, b); err == nil {
		t.Fatal("Import() error = nil, want invalid bundle error")
	}
	connectors, err := s.ListAllConnectors(ctx)
	if err != nil {
		t.Fatalf("ListAllConnectors() error = %v", err)
	}
	if len(connectors) != 0 {
		t.Fatalf("connectors after rejected import = %d, want 0", len(connectors))
	}
}

func TestImportIsIdempotent(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)

	if err := src.CreateConnector(ctx, &store.ConnectorRecord{
		Name: "pve1", Category: "virtualization", Type: "proxmox", URL: "https://pve.example.com",
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	doc := &store.DocRecord{Title: "Doc 1", Kind: "lab", Content: "hello"}
	if err := src.CreateDoc(ctx, doc); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	if err := src.CreateDocVersion(ctx, &store.DocVersionRecord{
		DocID: doc.ID, Rev: 1, Content: "hello", Trigger: "manual",
	}); err != nil {
		t.Fatalf("create doc version: %v", err)
	}
	tmpl := &store.TemplateRecord{Name: "Template 1"}
	if err := src.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := src.CreateTemplateSection(ctx, &store.TemplateSectionRecord{
		TemplateID: tmpl.ID, Title: "Section 1", Ord: 0, Body: "body",
	}); err != nil {
		t.Fatalf("create template section: %v", err)
	}

	b, err := backup.Export(ctx, src)
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newTestStore(t)

	first, err := backup.Import(ctx, dst, b)
	if err != nil {
		t.Fatalf("first Import() error: %v", err)
	}
	if first.Connectors.Imported != 1 || first.Docs.Imported != 1 ||
		first.DocVersions.Imported != 1 || first.Templates.Imported != 1 ||
		first.TemplateSections.Imported != 1 {
		t.Fatalf("expected everything imported on first run, got %+v", first)
	}

	second, err := backup.Import(ctx, dst, b)
	if err != nil {
		t.Fatalf("second Import() error: %v", err)
	}
	if second.Connectors.Imported != 0 || second.Docs.Imported != 0 ||
		second.DocVersions.Imported != 0 || second.Templates.Imported != 0 ||
		second.TemplateSections.Imported != 0 {
		t.Fatalf("expected nothing newly imported on second run, got %+v", second)
	}
	if second.Connectors.Skipped != 1 || second.Docs.Skipped != 1 ||
		second.DocVersions.Skipped != 1 || second.Templates.Skipped != 1 ||
		second.TemplateSections.Skipped != 1 {
		t.Fatalf("expected everything skipped on second run, got %+v", second)
	}
}

func TestImportRollsBackOnLateFailure(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	b := &backup.Bundle{
		Version: backup.BundleVersion,
		Docs:    []store.DocRecord{{ID: "doc-transaction", Title: "Transaction test", Kind: "lab"}},
		DocVersions: []store.DocVersionRecord{
			{ID: "version-one", DocID: "doc-transaction", Rev: 1, Content: "first", Trigger: "manual"},
			{ID: "version-two", DocID: "doc-transaction", Rev: 1, Content: "duplicate", Trigger: "manual"},
		},
	}

	if _, err := backup.Import(ctx, s, b); err == nil {
		t.Fatal("Import() error = nil, want late duplicate version failure")
	}
	if _, err := s.GetDoc(ctx, "doc-transaction"); err == nil {
		t.Fatal("imported doc remained after failed transaction")
	}
}

func TestExportIncludesRecordsBeyondAPage(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for i := range 1001 {
		if err := s.CreateDoc(ctx, &store.DocRecord{
			ID:      fmt.Sprintf("doc-%04d", i),
			Title:   fmt.Sprintf("Document %04d", i),
			Kind:    "lab",
			Content: "content",
		}); err != nil {
			t.Fatalf("create doc %d: %v", i, err)
		}
	}

	b, err := backup.Export(ctx, s)
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}
	if got := len(b.Docs); got != 1001 {
		t.Fatalf("exported docs = %d, want 1001", got)
	}
}

func TestExportToFile(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Create a test backup
	tmpDir := t.TempDir()
	run, err := backup.ExportToFile(ctx, s, tmpDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	// Verify returned metadata
	if run.ID == "" {
		t.Error("Run.ID is empty")
	}
	if run.FilePath == "" {
		t.Error("Run.FilePath is empty")
	}
	if run.SizeBytes <= 0 {
		t.Error("Run.SizeBytes should be positive")
	}
	if run.CreatedAt == "" {
		t.Error("Run.CreatedAt is empty")
	}

	// Verify the v2 ZIP contains a valid bundle.
	if _, err := os.Stat(run.FilePath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	archive, err := zip.OpenReader(run.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = archive.Close() }()
	var data []byte
	for _, entry := range archive.File {
		if entry.Name == "bundle.json" {
			r, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err = io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	var b backup.Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatalf("unmarshal backup JSON: %v", err)
	}

	if b.Version != backup.BundleVersion {
		t.Errorf("Bundle version mismatch: got %d, want %d", b.Version, backup.BundleVersion)
	}
}

// TestExportToFileDirNotWritable verifies ExportToFile returns an error
// (rather than silently succeeding or panicking) when the backup directory
// exists but the process can't write to it. Skipped when running as root,
// since root ignores directory write permissions.
func TestExportToFileDirNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits don't block writes")
	}

	ctx := context.Background()
	s := newTestStore(t)

	tmpDir := t.TempDir()
	if err := os.Chmod(tmpDir, 0o500); err != nil {
		t.Fatalf("chmod backup dir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(tmpDir, 0o700) }) // let t.TempDir() clean up

	if _, err := backup.ExportToFile(ctx, s, tmpDir); err == nil {
		t.Fatal("ExportToFile into a read-only directory: got nil error, want an error")
	}
}

func TestExportToFileCreatesDirectory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Use a path that doesn't exist yet
	tmpBase := t.TempDir()
	tmpDir := fmt.Sprintf("%s/subdir/backups", tmpBase)

	run, err := backup.ExportToFile(ctx, s, tmpDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(tmpDir); err != nil {
		t.Fatalf("backup directory not created: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(run.FilePath); err != nil {
		t.Fatalf("backup file not found: %v", err)
	}
}

// TestExportToFilePermissions is a regression test for GHSA-c753: the backup
// bundle is a full infrastructure inventory (secrets redacted, but still
// sensitive) and must not be readable by other local users.
func TestExportToFilePermissions(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits aren't enforced")
	}

	ctx := context.Background()
	s := newTestStore(t)

	tmpBase := t.TempDir()
	tmpDir := fmt.Sprintf("%s/backups", tmpBase)

	run, err := backup.ExportToFile(ctx, s, tmpDir)
	if err != nil {
		t.Fatalf("ExportToFile: %v", err)
	}

	dirInfo, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("stat backup dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("backup directory permissions = %o, want 0700", perm)
	}

	fileInfo, err := os.Stat(run.FilePath)
	if err != nil {
		t.Fatalf("stat backup file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("backup file permissions = %o, want 0600", perm)
	}
}

// Round-trips the doc provenance columns. Templates are imported before docs
// so docs.template_id resolves; SQLite tests don't enforce that FK, Postgres does.
func TestImportRestoresDocTemplateAndOrigin(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	tmpl := &store.TemplateRecord{Name: "Template 1"}
	if err := src.CreateTemplate(ctx, tmpl); err != nil {
		t.Fatalf("create template: %v", err)
	}
	doc := &store.DocRecord{Title: "Doc 1", Kind: "lab", Content: "x", TemplateID: tmpl.ID, Origin: store.DocOriginHuman}
	if err := src.CreateDoc(ctx, doc); err != nil {
		t.Fatalf("create doc: %v", err)
	}
	b, err := backup.Export(ctx, src)
	if err != nil {
		t.Fatalf("Export() error: %v", err)
	}

	dst := newTestStore(t)
	if _, err := backup.Import(ctx, dst, b); err != nil {
		t.Fatalf("Import() error: %v", err)
	}
	got, err := dst.GetDoc(ctx, doc.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.TemplateID != tmpl.ID || got.Origin != store.DocOriginHuman {
		t.Fatalf("restored doc = %+v, want template %s and human origin", got, tmpl.ID)
	}
}

func TestHumanDocsBackupHierarchyAndTrash(t *testing.T) {
	ctx := context.Background()
	source := newTestStore(t)
	parent := &store.DocRecord{ID: "z-parent", Title: "Parent", Origin: store.DocOriginHuman, CreatedBy: "writer"}
	if err := source.CreateHumanDoc(ctx, parent); err != nil {
		t.Fatal(err)
	}
	child := &store.DocRecord{ID: "a-child", Title: "Child", ParentID: parent.ID, CreatedBy: "writer"}
	if err := source.CreateHumanDoc(ctx, child); err != nil {
		t.Fatal(err)
	}
	if err := source.SoftDeleteDoc(ctx, parent.ID); err != nil {
		t.Fatal(err)
	}
	bundle, err := backup.Export(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Docs) != 2 || bundle.Docs[0].ID != child.ID {
		t.Fatalf("backup: %+v", bundle.Docs)
	}
	target := newTestStore(t)
	if _, err := backup.Import(ctx, target, bundle); err != nil {
		t.Fatal(err)
	}
	restored, err := target.GetDeletedDoc(ctx, child.ID)
	if err != nil || restored.ParentID != parent.ID || restored.CreatedBy != "writer" || restored.Origin != store.DocOriginHuman {
		t.Fatalf("metadata: %+v %v", restored, err)
	}
	if err := target.RestoreDeletedDoc(ctx, parent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := target.GetDoc(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
}

func TestJournalBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	dst := newTestStore(t)
	c := store.ConnectorRecord{Name: "Notes", Category: "virtualization", Type: "proxmox", URL: "https://example.com"}
	if err := src.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	d := store.DocRecord{Title: "Context", Kind: "service", ServiceID: c.ID, Origin: store.DocOriginHuman}
	if err := src.CreateDoc(ctx, &d); err != nil {
		t.Fatal(err)
	}
	e := store.JournalEntry{Body: "Backdated **note**", OccurredAt: "2020-01-01T00:00:00Z", CreatedBy: "former-user", ConnectorID: c.ID, DocID: d.ID, EntityKind: "vm", EntityName: "router", EntityRef: "vm/100"}
	if err := src.CreateJournalEntry(ctx, &e); err != nil {
		t.Fatal(err)
	}
	b, err := backup.Export(ctx, src)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var restored backup.Bundle
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	res, err := backup.Import(ctx, dst, &restored)
	if err != nil {
		t.Fatal(err)
	}
	if res.JournalEntries.Imported != 1 {
		t.Fatalf("counts %+v", res.JournalEntries)
	}
	got, err := dst.GetJournalEntry(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	run, err := backup.ExportToFile(ctx, src, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	verified := backup.VerifyBundleFile(ctx, run.FilePath)
	if verified.Status != "pass" || verified.ActualCounts["journalEntries"] != 1 || verified.ExpectedCounts["journalEntries"] != 1 {
		t.Fatalf("journal archive verification: %+v", verified)
	}
	if got != e {
		t.Fatalf("got %+v want %+v", got, e)
	}
	res, err = backup.Import(ctx, dst, &restored)
	if err != nil || res.JournalEntries.Skipped != 1 {
		t.Fatalf("idempotence %+v: %v", res, err)
	}
}

func TestEntityIdentityOverridesBackupRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newTestStore(t)
	dst := newTestStore(t)
	var members []store.EntityMemberRecord
	for _, name := range []string{"one", "two"} {
		c := store.ConnectorRecord{Name: name, Category: "virtualization", Type: "proxmox", URL: "https://" + name + ".example.com"}
		if err := src.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		members = append(members, store.EntityMemberRecord{ConnectorID: c.ID, Kind: "vm", Ref: name, Name: name})
	}
	clusters := [][]store.EntityMemberRecord{{members[0]}, {members[1]}}
	if err := src.ReconcileEntityIdentities(ctx, clusters); err != nil {
		t.Fatal(err)
	}
	merge := store.EntityIdentityOverride{Action: store.EntityOverrideMerge, ConnectorID: members[1].ConnectorID, Kind: "vm", Ref: "two", OtherConnectorID: members[0].ConnectorID, OtherKind: "vm", OtherRef: "one", Note: "same box", CreatedBy: "admin"}
	detach := store.EntityIdentityOverride{Action: store.EntityOverrideDetach, ConnectorID: members[0].ConnectorID, Kind: "vm", Ref: "one", CreatedBy: "admin"}
	for _, o := range []*store.EntityIdentityOverride{&merge, &detach} {
		if err := src.CreateEntityIdentityOverride(ctx, o); err != nil {
			t.Fatal(err)
		}
	}
	want, err := src.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(want) != 2 {
		t.Fatalf("source overrides = %v, %v", want, err)
	}

	run, err := backup.ExportToFile(ctx, src, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if verified := backup.VerifyBundleFile(ctx, run.FilePath); verified.Status != "pass" || verified.ActualCounts["entityIdentityOverrides"] != 2 || verified.ExpectedCounts["entityIdentityOverrides"] != 2 {
		t.Fatalf("override archive verification: %+v", verified)
	}
	res, err := backup.ImportFromFile(ctx, dst, run.FilePath, backup.ArchiveOptions{BlobDir: t.TempDir()})
	if err != nil || res.EntityIdentityOverrides.Imported != 2 {
		t.Fatalf("restore result %+v: %v", res, err)
	}
	got, err := dst.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("restored overrides = %v, %v", got, err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("restored override %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	// Members are rebuilt from snapshots, so restored overrides start dormant.
	views, err := dst.ListEntityIdentityOverrides(ctx)
	if err != nil || len(views) != 2 || views[0].State != store.EntityOverrideDormant || views[1].State != store.EntityOverrideDormant {
		t.Fatalf("restored override states = %+v, %v; want dormant", views, err)
	}
	res, err = backup.ImportFromFile(ctx, dst, run.FilePath, backup.ArchiveOptions{BlobDir: t.TempDir()})
	if err != nil || res.EntityIdentityOverrides.Skipped != 2 || res.EntityIdentityOverrides.Imported != 0 {
		t.Fatalf("idempotent restore %+v: %v", res, err)
	}
}

func TestValidateBundleRejectsOverrideOfUnknownConnector(t *testing.T) {
	b := &backup.Bundle{Version: backup.BundleVersion, EntityIdentityOverrides: []store.EntityIdentityOverride{
		{ID: "o1", Action: store.EntityOverrideDetach, ConnectorID: "missing", Kind: "vm", Ref: "x", CreatedBy: "admin", CreatedAt: "2026-01-01T00:00:00Z"},
	}}
	if err := backup.ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an override of a connector missing from the bundle")
	}
}

// overrideSource returns a store holding two connectors and one merge override
// between members that were never recorded (as after a restore).
func overrideBundle(t *testing.T) *backup.Bundle {
	t.Helper()
	ctx := context.Background()
	src := newTestStore(t)
	var members []store.EntityMemberRecord
	for _, name := range []string{"one", "two"} {
		c := store.ConnectorRecord{Name: name, Category: "virtualization", Type: "proxmox", URL: "https://" + name + ".example.com"}
		if err := src.CreateConnector(ctx, &c); err != nil {
			t.Fatal(err)
		}
		members = append(members, store.EntityMemberRecord{ConnectorID: c.ID, Kind: "vm", Ref: name, Name: name})
	}
	if err := src.ReconcileEntityIdentities(ctx, [][]store.EntityMemberRecord{{members[0]}, {members[1]}}); err != nil {
		t.Fatal(err)
	}
	merge := store.EntityIdentityOverride{Action: store.EntityOverrideMerge, ConnectorID: members[0].ConnectorID, Kind: "vm", Ref: "one",
		OtherConnectorID: members[1].ConnectorID, OtherKind: "vm", OtherRef: "two", CreatedBy: "admin"}
	if err := src.CreateEntityIdentityOverride(ctx, &merge); err != nil {
		t.Fatal(err)
	}
	b, err := backup.Export(ctx, src)
	if err != nil || len(b.EntityIdentityOverrides) != 1 {
		t.Fatalf("export = %v, %v", b, err)
	}
	return b
}

func TestRestoredEntityIdentityOverridesSurviveRetention(t *testing.T) {
	ctx := context.Background()
	b := overrideBundle(t)
	dst := newTestStore(t)
	if res, err := backup.Import(ctx, dst, b); err != nil || res.EntityIdentityOverrides.Imported != 1 {
		t.Fatalf("import = %+v, %v", res, err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)
	if _, err := dst.DeleteExpiredEntityIdentities(ctx, cutoff); err != nil {
		t.Fatal(err)
	}
	left, err := dst.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(left) != 1 {
		t.Fatalf("restored overrides after one retention pass = %d, %v; want 1", len(left), err)
	}
}

func TestImportEntityIdentityOverrideEquivalentTuples(t *testing.T) {
	ctx := context.Background()
	b := overrideBundle(t)
	dst := newTestStore(t)
	if _, err := backup.Import(ctx, dst, b); err != nil {
		t.Fatal(err)
	}
	o := b.EntityIdentityOverrides[0]

	same := o
	same.ID = "different-id"
	reversed := same
	reversed.ID = "reversed-id"
	reversed.ConnectorID, reversed.OtherConnectorID = o.OtherConnectorID, o.ConnectorID
	reversed.Ref, reversed.OtherRef = o.OtherRef, o.Ref
	// A bundle carrying the stored pair under new IDs, in either order, adds nothing.
	b.EntityIdentityOverrides = []store.EntityIdentityOverride{same, reversed}
	res, err := backup.Import(ctx, dst, b)
	if err != nil || res.EntityIdentityOverrides.Imported != 0 || res.EntityIdentityOverrides.Skipped != 2 {
		t.Fatalf("equivalent tuples: %+v, %v; want both skipped", res, err)
	}

	// A fresh target given the pair in both orders stores it once.
	fresh := newTestStore(t)
	b.EntityIdentityOverrides = []store.EntityIdentityOverride{reversed, same}
	conns, err := dst.ListAllConnectors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b.Connectors = conns
	res, err = backup.Import(ctx, fresh, b)
	if err != nil || res.EntityIdentityOverrides.Imported != 1 || res.EntityIdentityOverrides.Skipped != 1 {
		t.Fatalf("reversed pair import: %+v, %v; want 1 imported, 1 skipped", res, err)
	}
	stored, err := fresh.LoadEntityIdentityOverrides(ctx)
	if err != nil || len(stored) != 1 || stored[0].Ref != o.Ref {
		t.Fatalf("stored overrides = %+v, %v; want one row in sorted order", stored, err)
	}
}

func TestValidateBundleRejectsMalformedOverrides(t *testing.T) {
	good := store.EntityIdentityOverride{ID: "o1", Action: store.EntityOverrideMerge, ConnectorID: "c1", Kind: "vm", Ref: "a",
		OtherConnectorID: "c2", OtherKind: "vm", OtherRef: "b", CreatedBy: "admin", CreatedAt: "2026-01-01T00:00:00Z"}
	bundle := func(o ...store.EntityIdentityOverride) *backup.Bundle {
		return &backup.Bundle{Version: backup.BundleVersion, EntityIdentityOverrides: o,
			Connectors: []store.ConnectorRecord{{ID: "c1", Category: "virtualization"}, {ID: "c2", Category: "virtualization"}}}
	}
	if err := backup.ValidateBundle(bundle(good)); err != nil {
		t.Fatalf("valid override rejected: %v", err)
	}
	mutate := func(f func(*store.EntityIdentityOverride)) store.EntityIdentityOverride { o := good; f(&o); return o }
	cases := map[string]*backup.Bundle{
		"duplicate id":       bundle(good, mutate(func(o *store.EntityIdentityOverride) { o.Ref = "c" })),
		"cross-kind merge":   bundle(mutate(func(o *store.EntityIdentityOverride) { o.OtherKind = "host" })),
		"same member twice":  bundle(mutate(func(o *store.EntityIdentityOverride) { o.OtherConnectorID, o.OtherRef = "c1", "a" })),
		"detach with second": bundle(mutate(func(o *store.EntityIdentityOverride) { o.Action = store.EntityOverrideDetach })),
		"unknown action":     bundle(mutate(func(o *store.EntityIdentityOverride) { o.Action = "split" })),
		"unknown other":      bundle(mutate(func(o *store.EntityIdentityOverride) { o.OtherConnectorID = "missing" })),
		"missing creator":    bundle(mutate(func(o *store.EntityIdentityOverride) { o.CreatedBy = "" })),
	}
	for name, b := range cases {
		if err := backup.ValidateBundle(b); err == nil {
			t.Errorf("%s: ValidateBundle accepted it", name)
		}
	}
}

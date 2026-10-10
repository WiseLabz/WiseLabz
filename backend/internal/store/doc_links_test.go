package store

import (
	"context"
	"reflect"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/doclink"
)

func linkRows(t *testing.T, s *Store, source string) [][2]string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `SELECT target_type, target_id FROM doc_links WHERE source_doc_id = ? ORDER BY target_type, target_id`, source)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck
	var got [][2]string
	for rows.Next() {
		var row [2]string
		if err := rows.Scan(&row[0], &row[1]); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestDocLinkIndexTracksEveryDocWriter(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	content1 := "[doc](/docs/10000000-0000-4000-8000-000000000001) [entity](/entities/20000000-0000-4000-8000-000000000001)"
	d := &DocRecord{ID: "source", Title: "Source", Content: content1}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	want1 := [][2]string{{"doc", "10000000-0000-4000-8000-000000000001"}, {"entity", "20000000-0000-4000-8000-000000000001"}}
	if got := linkRows(t, s, d.ID); !reflect.DeepEqual(got, want1) {
		t.Fatalf("CreateDoc index = %v, want %v", got, want1)
	}
	if err := s.UpdateDoc(ctx, d.ID, "[doc](/docs/10000000-0000-4000-8000-000000000002)", nil); err != nil {
		t.Fatal(err)
	}
	want2 := [][2]string{{"doc", "10000000-0000-4000-8000-000000000002"}}
	if got := linkRows(t, s, d.ID); !reflect.DeepEqual(got, want2) {
		t.Fatalf("UpdateDoc index = %v, want %v", got, want2)
	}
	if _, err := s.UpdateDocWithVersion(ctx, d.ID, "[entity](/entities/20000000-0000-4000-8000-000000000002)", nil, "writer", "manual"); err != nil {
		t.Fatal(err)
	}
	want3 := [][2]string{{"entity", "20000000-0000-4000-8000-000000000002"}}
	if got := linkRows(t, s, d.ID); !reflect.DeepEqual(got, want3) {
		t.Fatalf("UpdateDocWithVersion index = %v, want %v", got, want3)
	}
	topology := "[topology](/docs/10000000-0000-4000-8000-000000000011)"
	if err := s.SetDocTopologyFingerprint(ctx, d.ID, "fingerprint", &topology, d.CurrentVersion+2); err != nil {
		t.Fatal(err)
	}
	if got, want := linkRows(t, s, d.ID), [][2]string{{"doc", "10000000-0000-4000-8000-000000000011"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("topology update index = %v, want %v", got, want)
	}
}

func TestDocLinkIndexTracksImportAndVersionRestore(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	imported := DocRecord{ID: "imported", Title: "Imported", Content: "[target](/docs/10000000-0000-4000-8000-000000000003)"}
	if err := s.ImportDocs(ctx, []DocRecord{imported}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := linkRows(t, s, imported.ID), [][2]string{{"doc", "10000000-0000-4000-8000-000000000003"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportDocs index = %v, want %v", got, want)
	}

	d := &DocRecord{ID: "restore", Title: "Restore", Content: "[current](/docs/10000000-0000-4000-8000-000000000004)"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	old := "[restored](/entities/20000000-0000-4000-8000-000000000003)"
	if err := s.CreateDocVersion(ctx, &DocVersionRecord{DocID: d.ID, Rev: 1, Content: old, Trigger: "manual"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateDocWithVersion(ctx, d.ID, "[new](/docs/10000000-0000-4000-8000-000000000005)", nil, "writer", "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateDocWithVersion(ctx, d.ID, old, nil, "writer", "restore"); err != nil {
		t.Fatal(err)
	}
	if got, want := linkRows(t, s, d.ID), [][2]string{{"entity", "20000000-0000-4000-8000-000000000003"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("restored version index = %v, want %v", got, want)
	}
}

func TestDocLinkIndexSyncFailureRollsBackContent(t *testing.T) {
	skipOnPostgres(t, "SQLite trigger syntax is used to deterministically reject a doc link insert")
	ctx := context.Background()
	s := newDocTestStore(t)
	d := &DocRecord{ID: "source", Title: "Source", Content: "[old](/docs/10000000-0000-4000-8000-000000000006)"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER reject_doc_links BEFORE INSERT ON doc_links BEGIN SELECT RAISE(ABORT, 'index unavailable'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateDoc(ctx, d.ID, "[new](/docs/10000000-0000-4000-8000-000000000007)", nil); err == nil {
		t.Fatal("UpdateDoc() succeeded despite failed link-index write")
	}
	got, err := s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "[old](/docs/10000000-0000-4000-8000-000000000006)" {
		t.Fatalf("content after failed index write = %q, want old body", got.Content)
	}
	if want := [][2]string{{"doc", "10000000-0000-4000-8000-000000000006"}}; !reflect.DeepEqual(linkRows(t, s, d.ID), want) {
		t.Fatalf("index after failed update = %v, want %v", linkRows(t, s, d.ID), want)
	}
}

func TestDocLinkBackfillIsIdempotentAndDoesNotRewriteBodies(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	d := &DocRecord{ID: "source", Title: "Source", Content: "original"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	body := "legacy [doc](/docs/10000000-0000-4000-8000-000000000008) and [entity](/entities/20000000-0000-4000-8000-000000000004)"
	if _, err := s.db.ExecContext(ctx, `UPDATE docs SET content = ? WHERE id = ?`, body, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM doc_links WHERE source_doc_id = ?`, d.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.BackfillDocLinks(ctx); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != body {
		t.Fatalf("backfill rewrote body: got %q, want %q", got.Content, body)
	}
	want := [][2]string{{"doc", "10000000-0000-4000-8000-000000000008"}, {"entity", "20000000-0000-4000-8000-000000000004"}}
	if got := linkRows(t, s, d.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("backfill index = %v, want %v", got, want)
	}
}

func TestDeleteDocCascadesDocLinks(t *testing.T) {
	s := newDocTestStore(t)
	if s.driver == "sqlite" {
		if _, err := s.db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`); err != nil {
			t.Fatalf("enable SQLite foreign keys: %v", err)
		}
	}
	d := &DocRecord{ID: "source", Title: "Source", Content: "[target](/docs/10000000-0000-4000-8000-000000000009)"}
	if err := s.CreateDoc(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteDoc(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	if got := linkRows(t, s, d.ID); len(got) != 0 {
		t.Fatalf("links remain after deleting source doc: %v", got)
	}
}

func TestLookupDocLinkVisibilityAndEntityIdentity(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	visible := &ConnectorRecord{Name: "visible", Category: "virtualization", Type: "test", URL: "https://visible.test"}
	hidden := &ConnectorRecord{Name: "hidden", Category: "virtualization", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*ConnectorRecord{visible, hidden} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	userID := "link-reader"
	mustCreateUser(t, s, userID)
	if _, err := s.UpsertConnectorGrant(ctx, userID, visible.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDoc(ctx, &DocRecord{ID: "visible-doc", Title: "Shared title", ServiceID: visible.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDoc(ctx, &DocRecord{ID: "hidden-doc", Title: "Secret title", ServiceID: hidden.ID}); err != nil {
		t.Fatal(err)
	}
	targets, err := s.LookupDocLink(ctx, userID, "SHARED TITLE")
	if err != nil || !reflect.DeepEqual(targets, []doclink.Target{{Type: "doc", ID: "visible-doc", Label: "Shared title"}}) {
		t.Fatalf("visible title lookup = %+v, %v", targets, err)
	}
	if hiddenTargets, err := s.LookupDocLink(ctx, userID, "Secret title"); err != nil || len(hiddenTargets) != 0 {
		t.Fatalf("hidden title lookup = %+v, %v; must not reveal hidden docs", hiddenTargets, err)
	}

	for _, entity := range []struct{ id, name, merged string }{
		{"30000000-0000-4000-8000-000000000001", "Canonical", ""},
		{"30000000-0000-4000-8000-000000000002", "Old name", "30000000-0000-4000-8000-000000000001"},
	} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO entities (id, kind, display_name, first_seen_at, last_seen_at, merged_into) VALUES (?, 'vm', ?, '2026-01-01', '2026-01-01', ?)`, entity.id, entity.name, nullable(entity.merged)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name, gone_at) VALUES (?, ?, 'vm', '103', 'Old name', NULL)`, "30000000-0000-4000-8000-000000000002", visible.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO entity_members (entity_id, connector_id, kind, ref, name, gone_at) VALUES (?, ?, 'vm', '104', 'Inactive', '2026-01-02')`, "30000000-0000-4000-8000-000000000001", visible.ID); err != nil {
		t.Fatal(err)
	}
	got, err := s.LookupDocLink(ctx, userID, "vm:103")
	if err != nil || !reflect.DeepEqual(got, []doclink.Target{{Type: "entity", ID: "30000000-0000-4000-8000-000000000001", Label: "Canonical"}}) {
		t.Fatalf("kind/ref lookup through merged identity = %+v, %v", got, err)
	}
	got, err = s.LookupDocLink(ctx, userID, "vm:104")
	if err != nil || len(got) != 0 {
		t.Fatalf("inactive member lookup = %+v, %v; want no match", got, err)
	}
	if err := s.CreateDoc(ctx, &DocRecord{ID: "source", Title: "Source", Content: "[old](/entities/30000000-0000-4000-8000-000000000002)", Origin: DocOriginHuman}); err != nil {
		t.Fatal(err)
	}
	backlinks, err := s.ListDocBacklinks(ctx, userID, "entity", "30000000-0000-4000-8000-000000000001")
	if err != nil || !reflect.DeepEqual(backlinks, []DocBacklink{{ID: "source", Title: "Source"}}) {
		t.Fatalf("merged target backlinks = %+v, %v", backlinks, err)
	}
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func TestListDocBacklinksFiltersHiddenAndSoftDeletedSources(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	if s.driver == "sqlite" {
		if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			t.Fatal(err)
		}
	}
	visible := &ConnectorRecord{Name: "visible", Category: "virtualization", Type: "test", URL: "https://visible.test"}
	hidden := &ConnectorRecord{Name: "hidden", Category: "virtualization", Type: "test", URL: "https://hidden.test"}
	for _, c := range []*ConnectorRecord{visible, hidden} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	userID := "link-reader"
	mustCreateUser(t, s, userID)
	if _, err := s.UpsertConnectorGrant(ctx, userID, visible.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*DocRecord{
		{ID: "visible-source", Title: "Visible", ServiceID: visible.ID, Content: "[target](/docs/10000000-0000-4000-8000-000000000010)"},
		{ID: "hidden-source", Title: "Hidden", ServiceID: hidden.ID, Content: "[target](/docs/10000000-0000-4000-8000-000000000010)"},
		{ID: "deleted-source", Title: "Deleted", ServiceID: visible.ID, Content: "[target](/docs/10000000-0000-4000-8000-000000000010)"},
	} {
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SoftDeleteDoc(ctx, "deleted-source"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListDocBacklinks(ctx, userID, "doc", "10000000-0000-4000-8000-000000000010")
	if err != nil || !reflect.DeepEqual(got, []DocBacklink{{ID: "visible-source", Title: "Visible"}}) {
		t.Fatalf("visible backlinks = %+v, %v", got, err)
	}
	mustCreateUser(t, s, "admin")
	adminCtx := auth.ContextWithUser(ctx, "admin", true)
	for _, connectorID := range []string{visible.ID, hidden.ID} {
		if _, err := s.UpsertConnectorGrant(ctx, "admin", connectorID, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	got, err = s.ListDocBacklinks(adminCtx, "admin", "doc", "10000000-0000-4000-8000-000000000010")
	if err != nil || len(got) != 2 {
		t.Fatalf("admin backlinks = %+v, %v; want visible and hidden but no soft-deleted source", got, err)
	}
}

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

func humanNote(t *testing.T, s *Store, parent string) *DocRecord {
	t.Helper()
	d := &DocRecord{Title: "nebula note", Content: "nebula operational manual", ParentID: parent, CreatedBy: "writer"}
	if err := s.CreateHumanDoc(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestHumanDocHierarchy(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	root := humanNote(t, s, "")
	child := humanNote(t, s, root.ID)
	if err := s.UpdateDocMetadata(ctx, root.ID, nil, &child.ID); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("cycle: %v", err)
	}
	last := child
	for range 3 {
		last = humanNote(t, s, last.ID)
	}
	tooDeep := &DocRecord{Title: "six", ParentID: last.ID}
	if err := s.CreateHumanDoc(ctx, tooDeep); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("depth: %v", err)
	}
	if err := s.UpdateDocMetadata(ctx, root.ID, nil, &last.ID); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("cycle: %v", err)
	}
	other := humanNote(t, s, "")
	if err := s.UpdateDocMetadata(ctx, root.ID, nil, &other.ID); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("subtree height: %v", err)
	}
	conn := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "http://example.com"}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatal(err)
	}
	scoped := &DocRecord{Title: "service", ServiceID: conn.ID, ParentID: other.ID}
	if err := s.CreateHumanDoc(ctx, scoped); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("scope: %v", err)
	}
	versions, err := s.GetDocVersions(ctx, root.ID)
	if err != nil || len(versions) != 1 || versions[0].Trigger != "create" || versions[0].Author != "writer" {
		t.Fatalf("versions: %+v %v", versions, err)
	}
}

func TestSoftDeleteVisibilityAndBatchRestore(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", false)
	root := humanNote(t, s, "")
	child := humanNote(t, s, root.ID)
	older := humanNote(t, s, child.ID)
	if err := s.SoftDeleteDoc(ctx, older.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertDocSectionEmbedding(ctx, &DocSectionEmbeddingRecord{DocID: root.ID, SectionKey: "body", Content: "nebula", Vector: []byte{1}, Model: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SoftDeleteDoc(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetDoc(ctx, root.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get: %v", err)
	}
	if _, err := s.UpdateDocWithVersion(ctx, root.ID, "resurrection", nil, "writer", "manual"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("save: %v", err)
	}
	for _, list := range []func(context.Context, string, int, int) ([]DocRecord, int, error){s.ListAllDocs, s.ListAllDocsWithContent} {
		ds, n, err := list(ctx, "", 0, 100)
		if err != nil || n != 0 || len(ds) != 0 {
			t.Fatalf("list: %+v %d %v", ds, n, err)
		}
	}
	ds, n, err := s.ListViewableDocs(ctx, "reader", "", 0, 100)
	if err != nil || n != 0 || len(ds) != 0 {
		t.Fatalf("viewable: %+v %d %v", ds, n, err)
	}
	grouped, err := s.ListDocsGroupedByService(ctx)
	if err != nil || len(grouped) != 0 {
		t.Fatalf("grouped: %+v %v", grouped, err)
	}
	hits, err := s.SearchContent(ctx, "reader", "nebula", 20)
	if err != nil || len(hits) != 0 {
		t.Fatalf("search: %+v %v", hits, err)
	}
	embeddings, err := s.ListDocSectionEmbeddings(ctx, "reader", "")
	if err != nil || len(embeddings) != 0 {
		t.Fatalf("embeddings: %+v %v", embeddings, err)
	}
	versions, err := s.GetDocVersions(ctx, root.ID)
	if err != nil || len(versions) != 0 {
		t.Fatalf("versions: %+v %v", versions, err)
	}
	exportVersions, err := s.ListDocVersionsAfter(ctx, map[string]int{}, 100)
	if err != nil || len(exportVersions) != 0 {
		t.Fatalf("export versions: %+v %v", exportVersions, err)
	}
	trash, err := s.ListDeletedDocs(ctx)
	if err != nil || len(trash) != 3 {
		t.Fatalf("trash: %+v %v", trash, err)
	}
	dr, err := s.GetDeletedDoc(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	dc, err := s.GetDeletedDoc(ctx, child.ID)
	if err != nil || dr.DeletedAt != dc.DeletedAt {
		t.Fatalf("batch: %+v %v", dc, err)
	}
	if err := s.RestoreDeletedDoc(ctx, root.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetDoc(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetDoc(ctx, older.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("older deletion restored: %v", err)
	}
	embeddings, err = s.ListDocSectionEmbeddings(ctx, "reader", "")
	if err != nil || len(embeddings) != 1 {
		t.Fatalf("restored embeddings: %+v %v", embeddings, err)
	}
}

func TestPurgeDeletedDocsCascades(t *testing.T) {
	s := newDocTestStore(t)
	if s.driver != "postgres" {
		if _, err := s.DB().ExecContext(context.Background(), `PRAGMA foreign_keys = ON`); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	old := humanNote(t, s, "")
	u := &User{Username: "purge-user"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcquireDocLock(ctx, old.ID, u.ID); err != nil {
		t.Fatal(err)
	}
	recent := humanNote(t, s, old.ID)
	live := humanNote(t, s, "")
	if err := s.UpsertDocSectionEmbedding(ctx, &DocSectionEmbeddingRecord{DocID: old.ID, SectionKey: "body", Content: "nebula", Vector: []byte{1}, Model: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SoftDeleteDoc(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE docs SET deleted_at = ? WHERE id = ?`, time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339), old.ID); err != nil {
		t.Fatal(err)
	}
	count, err := s.PurgeDeletedDocs(ctx, time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339))
	if err != nil || count != 1 {
		t.Fatalf("purge: %d %v", count, err)
	}
	if _, err := s.GetDeletedDoc(ctx, old.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old remains: %v", err)
	}
	if _, err := s.GetDeletedDoc(ctx, recent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetDoc(ctx, live.ID); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"doc_versions", "doc_locks", "doc_section_embeddings"} {
		var n int
		if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE doc_id = ?`, old.ID).Scan(&n); err != nil || n != 0 {
			t.Fatalf("cascade %s: %d %v", table, n, err)
		}
	}
}

func TestDeletedServiceDocLists(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	conn := &ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "http://example.com"}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatal(err)
	}
	d := &DocRecord{Title: "service note", ServiceID: conn.ID}
	if err := s.CreateHumanDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := s.SoftDeleteDoc(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	for _, list := range []func(context.Context, string) ([]DocRecord, error){s.ListDocsByService, s.ListDocPreviewsByService} {
		docs, err := list(ctx, conn.ID)
		if err != nil || len(docs) != 0 {
			t.Fatalf("service list: %+v %v", docs, err)
		}
	}
	count, err := s.CountDocs(ctx)
	if err != nil || count != 0 {
		t.Fatalf("count: %d %v", count, err)
	}
}

func TestHumanLabDiscoveryACL(t *testing.T) {
	s := newDocTestStore(t)
	ctx := auth.ContextWithUser(context.Background(), "reader", false)
	human := humanNote(t, s, "")
	generated := &DocRecord{Title: "nebula topology", Content: "nebula secret inventory"}
	if err := s.CreateDoc(ctx, generated); err != nil {
		t.Fatal(err)
	}
	docs, n, err := s.ListViewableDocs(ctx, "reader", "", 0, 100)
	if err != nil || n != 1 || len(docs) != 1 || docs[0].ID != human.ID {
		t.Fatalf("docs: %+v %d %v", docs, n, err)
	}
	hits, err := s.SearchContent(ctx, "reader", "nebula", 20)
	if err != nil || len(hits) != 1 || hits[0].ID != human.ID {
		t.Fatalf("search: %+v %v", hits, err)
	}
	for _, d := range []*DocRecord{human, generated} {
		if err := s.UpsertDocSectionEmbedding(ctx, &DocSectionEmbeddingRecord{DocID: d.ID, SectionKey: "body", Content: d.Content, Vector: []byte{1}, Model: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	embeddings, err := s.ListDocSectionEmbeddings(ctx, "reader", "")
	if err != nil || len(embeddings) != 1 || embeddings[0].DocID != human.ID {
		t.Fatalf("embeddings: %+v %v", embeddings, err)
	}
}

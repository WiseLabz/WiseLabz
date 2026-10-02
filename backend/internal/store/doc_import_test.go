package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestImportDocsSuffixesCollisionsAtomically(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	existing := &DocRecord{Title: "Network", CreatedBy: "writer"}
	if err := s.CreateHumanDoc(ctx, existing); err != nil {
		t.Fatal(err)
	}
	docs := []DocRecord{
		{ID: "11111111-1111-4111-8111-111111111111", Title: "Network", Content: "imported", CreatedBy: "admin"},
		{ID: "22222222-2222-4222-8222-222222222222", Title: "Network", CreatedBy: "admin"},
		{ID: "33333333-3333-4333-8333-333333333333", Title: "Network", ParentID: "11111111-1111-4111-8111-111111111111", CreatedBy: "admin"},
	}
	attachments := []DocAttachment{{DocID: docs[0].ID, SHA256: strings.Repeat("a", 64), Filename: "a.png", ContentType: "image/png", Size: 1, CreatedBy: "admin"}}
	if err := s.ImportDocs(ctx, docs, attachments); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"Network (imported)", "Network (imported 2)", "Network"} {
		got, err := s.GetDoc(ctx, docs[i].ID)
		if err != nil || got.Title != want || got.Origin != DocOriginHuman || got.Kind != "lab" {
			t.Fatalf("doc %d: %+v %v", i, got, err)
		}
	}
	versions, err := s.GetDocVersions(ctx, docs[0].ID)
	if err != nil || len(versions) != 1 || versions[0].Trigger != "import" || versions[0].Author != "admin" || versions[0].Content != "imported" {
		t.Fatalf("versions %+v %v", versions, err)
	}
	if got, err := s.ListDocAttachments(ctx, docs[0].ID); err != nil || len(got) != 1 {
		t.Fatalf("attachments %+v %v", got, err)
	}
	// A bad parent rolls back the whole import.
	bad := []DocRecord{
		{ID: "44444444-4444-4444-8444-444444444444", Title: "ok", CreatedBy: "admin"},
		{ID: "55555555-5555-4555-8555-555555555555", Title: "orphan", ParentID: "missing", CreatedBy: "admin"},
	}
	if err := s.ImportDocs(ctx, bad, nil); !errors.Is(err, ErrDocHierarchy) {
		t.Fatalf("bad parent: %v", err)
	}
	if _, err := s.GetDoc(ctx, bad[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("partial import committed: %v", err)
	}
}

package store

import (
	"context"
	"errors"
	"testing"
)

func TestDocGenerationDefaultsAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	d := &DocRecord{Title: "Legacy", Content: "x"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	got, err := s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.Origin != DocOriginGenerated || got.TemplateID != "" || got.LastSyncedAt != "" || got.GenKeys != nil {
		t.Fatalf("defaults = %+v, want generated origin and empty provenance", got)
	}

	keys := `["head"]`
	if err := s.SetDocGeneration(ctx, d.ID, DocGeneration{Origin: DocOriginGenerated, GenKeys: &keys, Synced: true}); err != nil {
		t.Fatalf("SetDocGeneration() error: %v", err)
	}
	if err := s.SetDocOrigin(ctx, d.ID, DocOriginHuman); err != nil {
		t.Fatalf("SetDocOrigin() error: %v", err)
	}
	got, _ = s.GetDoc(ctx, d.ID)
	if got.Origin != DocOriginHuman || got.GenKeys == nil || *got.GenKeys != keys || got.LastSyncedAt == "" {
		t.Fatalf("after set = %+v", got)
	}
	if err := s.SetDocOrigin(ctx, "missing", DocOriginHuman); !errors.Is(err, ErrNotFound) {
		t.Fatalf("SetDocOrigin(missing) = %v, want ErrNotFound", err)
	}
}

func TestApplyGeneratedRender(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	d := &DocRecord{Title: "Doc", Content: "v1"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	v := 1
	rev, err := s.ApplyGeneratedRender(ctx, d.ID, "v2", `["a"]`, &v, "", "sync")
	if err != nil || rev != 2 {
		t.Fatalf("ApplyGeneratedRender() = %d, %v; want 2, nil", rev, err)
	}
	got, _ := s.GetDoc(ctx, d.ID)
	if got.Content != "v2" || got.GenKeys == nil || *got.GenKeys != `["a"]` || got.LastSyncedAt == "" {
		t.Fatalf("after apply = %+v", got)
	}
	versions, _ := s.GetDocVersions(ctx, d.ID)
	if len(versions) != 1 || versions[0].Trigger != "sync" || versions[0].Rev != 2 {
		t.Fatalf("versions = %+v, want one sync version at rev 2", versions)
	}

	// Stale version: nothing lands, gen_keys unchanged.
	if _, err := s.ApplyGeneratedRender(ctx, d.ID, "v3", `["b"]`, &v, "", "sync"); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale ApplyGeneratedRender() = %v, want ErrVersionConflict", err)
	}
	got, _ = s.GetDoc(ctx, d.ID)
	if got.Content != "v2" || *got.GenKeys != `["a"]` {
		t.Fatalf("after rejected apply = %+v", got)
	}
}

func TestTouchDocSyncedKeepsUpdatedAt(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	d := &DocRecord{Title: "Doc", Content: "v1", UpdatedAt: "2020-01-01T00:00:00Z"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	if err := s.TouchDocSynced(ctx, d.ID); err != nil {
		t.Fatalf("TouchDocSynced() error: %v", err)
	}
	got, _ := s.GetDoc(ctx, d.ID)
	if got.UpdatedAt != "2020-01-01T00:00:00Z" || got.LastSyncedAt == "" || got.CurrentVersion != 1 {
		t.Fatalf("after touch = %+v", got)
	}
}

func TestGetOpenChangeByPattern(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	if _, err := s.GetOpenChangeByPattern(ctx, "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty GetOpenChangeByPattern() = %v, want ErrNotFound", err)
	}
	c := &ChangeRecord{ChangeType: "doc_conflict", Severity: "info", Summary: "s", PatternID: "p1"}
	if err := s.CreateChange(ctx, c); err != nil {
		t.Fatalf("CreateChange() error: %v", err)
	}
	got, err := s.GetOpenChangeByPattern(ctx, "p1")
	if err != nil || got.ID != c.ID {
		t.Fatalf("GetOpenChangeByPattern() = %+v, %v", got, err)
	}
	if err := s.UpdateChangeStatus(ctx, c.ID, "dismissed"); err != nil {
		t.Fatalf("UpdateChangeStatus() error: %v", err)
	}
	if _, err := s.GetOpenChangeByPattern(ctx, "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after dismiss = %v, want ErrNotFound", err)
	}
}

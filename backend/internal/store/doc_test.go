package store

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func newDocTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}

	return New(db, "sqlite")
}

func TestUpdateDocOptimisticConcurrency(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	d := &DocRecord{Title: "Test Doc", Content: "v1"}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	if d.CurrentVersion != 1 {
		t.Fatalf("CreateDoc() CurrentVersion = %d, want 1", d.CurrentVersion)
	}

	// Matching expectedVersion succeeds and bumps the version.
	match := d.CurrentVersion
	if err := s.UpdateDoc(ctx, d.ID, "v2", &match); err != nil {
		t.Fatalf("UpdateDoc() with matching version error: %v", err)
	}
	got, err := s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.Content != "v2" || got.CurrentVersion != 2 {
		t.Fatalf("after matching update: content=%q version=%d, want content=v2 version=2", got.Content, got.CurrentVersion)
	}

	// Stale expectedVersion is rejected and leaves the doc untouched.
	stale := 1
	err = s.UpdateDoc(ctx, d.ID, "v3-should-not-land", &stale)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("UpdateDoc() with stale version error = %v, want ErrVersionConflict", err)
	}
	got, err = s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.Content != "v2" || got.CurrentVersion != 2 {
		t.Fatalf("after rejected update: content=%q version=%d, want unchanged content=v2 version=2", got.Content, got.CurrentVersion)
	}

	// nil expectedVersion keeps today's unconditional last-write-wins behavior.
	if err := s.UpdateDoc(ctx, d.ID, "v4", nil); err != nil {
		t.Fatalf("UpdateDoc() with nil version error: %v", err)
	}
	got, err = s.GetDoc(ctx, d.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.Content != "v4" || got.CurrentVersion != 3 {
		t.Fatalf("after unconditional update: content=%q version=%d, want content=v4 version=3", got.Content, got.CurrentVersion)
	}

	// Non-existent doc: ErrNotFound with or without an expected version.
	if err := s.UpdateDoc(ctx, "missing-id", "x", nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateDoc() on missing doc (nil version) error = %v, want ErrNotFound", err)
	}
	someVersion := 1
	if err := s.UpdateDoc(ctx, "missing-id", "x", &someVersion); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateDoc() on missing doc (with version) error = %v, want ErrNotFound", err)
	}
}

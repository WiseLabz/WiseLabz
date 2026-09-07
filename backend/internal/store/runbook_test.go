package store

import (
	"context"
	"errors"
	"testing"
)

func TestRunbookRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	r := &RunbookRecord{
		Title:       "Restart the service",
		Body:        "Step 1: check logs.",
		TargetType:  "change_type",
		TargetValue: "config_update",
	}
	created, err := s.CreateRunbook(ctx, r)
	if err != nil {
		t.Fatalf("CreateRunbook() error: %v", err)
	}

	got, err := s.GetRunbook(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRunbook() error: %v", err)
	}
	if got.Body != "Step 1: check logs." {
		t.Fatalf("Body = %q, want %q", got.Body, "Step 1: check logs.")
	}

	updated, err := s.UpdateRunbook(ctx, created.ID, map[string]any{"body": "Step 1: check logs. Step 2: restart."})
	if err != nil {
		t.Fatalf("UpdateRunbook() error: %v", err)
	}
	if updated.Body != "Step 1: check logs. Step 2: restart." {
		t.Fatalf("updated.Body = %q, want %q", updated.Body, "Step 1: check logs. Step 2: restart.")
	}

	got, err = s.GetRunbook(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRunbook() after update error: %v", err)
	}
	if got.Body != "Step 1: check logs. Step 2: restart." {
		t.Fatalf("Body after update = %q, want %q", got.Body, "Step 1: check logs. Step 2: restart.")
	}

	if err := s.DeleteRunbook(ctx, created.ID); err != nil {
		t.Fatalf("DeleteRunbook() error: %v", err)
	}
	if _, err := s.GetRunbook(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRunbook() after delete error = %v, want ErrNotFound", err)
	}
}

func TestGetRunbookByTarget(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	r := &RunbookRecord{
		Title:       "High severity triage",
		Body:        "Page the on-call engineer.",
		TargetType:  "alert_severity",
		TargetValue: "critical",
	}
	if _, err := s.CreateRunbook(ctx, r); err != nil {
		t.Fatalf("CreateRunbook() error: %v", err)
	}

	got, err := s.GetRunbookByTarget(ctx, "alert_severity", "critical")
	if err != nil {
		t.Fatalf("GetRunbookByTarget() error: %v", err)
	}
	if got.Title != "High severity triage" {
		t.Fatalf("Title = %q, want %q", got.Title, "High severity triage")
	}

	if _, err := s.GetRunbookByTarget(ctx, "alert_severity", "low"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRunbookByTarget(not found) error = %v, want ErrNotFound", err)
	}
}

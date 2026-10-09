package store

import (
	"context"
	"errors"
	"testing"
)

// failedRunWithStepState returns a failed run whose first step is in stepState
// (unknown or failed) and whose second step is pending.
func failedRunWithStepState(t *testing.T, s *Store, stepState string) (*RunbookRunRecord, []*RunbookRunStepRecord) {
	t.Helper()
	ctx := context.Background()
	run, steps := createOpenManualRunbookRun(t, s, "first", "second")
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
		t.Fatalf("start first step: %v", err)
	}
	if _, _, err := s.FailRunbookRunStep(ctx, run.ID, steps[0].ID, stepState, "lost", "step_failed"); err != nil {
		t.Fatalf("fail first step: %v", err)
	}
	failed, all, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRunbookRun() error: %v", err)
	}
	return failed, all
}

func TestResumeRunbookRunMarkingStepDone(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	t.Run("marks the first unknown step succeeded and resumes the run as the user", func(t *testing.T) {
		failed, steps := failedRunWithStepState(t, s, "unknown")
		resumed, err := s.ResumeRunbookRunMarkingStepDone(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, "operator")
		if err != nil {
			t.Fatalf("ResumeRunbookRunMarkingStepDone() error: %v", err)
		}
		if resumed.State != "running" || resumed.Reason != "" || resumed.ResumedBy == nil || *resumed.ResumedBy != "operator" {
			t.Fatalf("resumed run = %+v, want running, no reason, resumed by operator", resumed)
		}
		if resumed.UpdatedAt <= failed.UpdatedAt {
			t.Fatalf("resumed updated_at = %q, want after %q", resumed.UpdatedAt, failed.UpdatedAt)
		}
		run, after, err := s.GetRunbookRun(ctx, failed.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run.State != "running" {
			t.Fatalf("stored run state = %s, want running", run.State)
		}
		if after[0].State != "succeeded" || after[0].Error != "" || after[0].FinishedAt == "" {
			t.Fatalf("marked step = %+v, want succeeded with no error and a finish time", after[0])
		}
		if after[1].State != "pending" {
			t.Fatalf("next step = %s, want pending", after[1].State)
		}
	})

	t.Run("a run that is not failed is a conflict and nothing changes", func(t *testing.T) {
		run, steps := createOpenManualRunbookRun(t, s, "only")
		if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, run.ID, run.UpdatedAt, steps[0].ID, "operator"); !errors.Is(err, ErrConflict) {
			t.Fatalf("ResumeRunbookRunMarkingStepDone() on a running run = %v, want ErrConflict", err)
		}
		got, after, err := s.GetRunbookRun(ctx, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "running" || after[0].State != "pending" {
			t.Fatalf("run = %+v, step = %+v; want both unchanged", got, after[0])
		}
	})

	t.Run("a step that is not unknown is a conflict and nothing changes", func(t *testing.T) {
		failed, steps := failedRunWithStepState(t, s, "failed")
		if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, "operator"); !errors.Is(err, ErrConflict) {
			t.Fatalf("ResumeRunbookRunMarkingStepDone() on a failed step = %v, want ErrConflict", err)
		}
		got, after, err := s.GetRunbookRun(ctx, failed.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "failed" || after[0].State != "failed" || after[0].Error != "lost" {
			t.Fatalf("run = %+v, steps = %+v; want both unchanged", got, after)
		}
	})

	t.Run("a step that is not the first unfinished one is a conflict and nothing changes", func(t *testing.T) {
		run, steps := createOpenManualRunbookRun(t, s, "first", "second")
		// The second step is unknown while the first has not succeeded.
		if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[1].ID, "pending", map[string]any{"state": "running"}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := s.FailRunbookRunStep(ctx, run.ID, steps[1].ID, "unknown", "lost", "internal_error"); err != nil {
			t.Fatal(err)
		}
		failed, _, err := s.GetRunbookRun(ctx, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, stepID := range []string{steps[1].ID, "missing-step"} {
			if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, run.ID, failed.UpdatedAt, stepID, "operator"); !errors.Is(err, ErrConflict) {
				t.Fatalf("ResumeRunbookRunMarkingStepDone(%s) = %v, want ErrConflict", stepID, err)
			}
		}
		got, after, err := s.GetRunbookRun(ctx, run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.State != "failed" || after[0].State != "pending" || after[1].State != "unknown" {
			t.Fatalf("run = %+v, steps = %+v; want the run failed and both steps unchanged", got, after)
		}
	})
}

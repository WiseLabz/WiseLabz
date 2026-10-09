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
		resumed, err := s.ResumeRunbookRunMarkingStepDone(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, "operator", nil)
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
		if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, run.ID, run.UpdatedAt, steps[0].ID, "operator", nil); !errors.Is(err, ErrConflict) {
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
		if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, "operator", nil); !errors.Is(err, ErrConflict) {
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
			if _, err := s.ResumeRunbookRunMarkingStepDone(ctx, run.ID, failed.UpdatedAt, stepID, "operator", nil); !errors.Is(err, ErrConflict) {
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

func TestResumeRunbookRunExpectations(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	audit := func() *AuditRecord {
		return &AuditRecord{ActorUserID: "operator", ActorRole: "user", Action: "runbook.run.step_resent", TargetType: "runbook_run", TargetID: "r"}
	}
	for _, markDone := range []bool{false, true} {
		name := "plain"
		if markDone {
			name = "mark done"
		}
		resume := func(runID, updatedAt, stepID string, rec *AuditRecord) error {
			var err error
			if markDone {
				_, err = s.ResumeRunbookRunMarkingStepDone(ctx, runID, updatedAt, stepID, "operator", rec)
			} else {
				_, err = s.ResumeRunbookRun(ctx, runID, updatedAt, stepID, "operator", rec)
			}
			return err
		}
		assertUnchanged := func(t *testing.T, runID string, steps []*RunbookRunStepRecord, updatedAt string) {
			t.Helper()
			got, after, err := s.GetRunbookRun(ctx, runID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != "failed" || got.UpdatedAt != updatedAt || after[0].State != steps[0].State || after[1].State != "pending" {
				t.Fatalf("run = %+v, steps = %+v; want everything unchanged", got, after)
			}
			records, _, err := s.ListAuditRecords(ctx, "runbook.run.step_resent", "", "", "", 0, 10)
			if err != nil || len(records) != 0 {
				t.Fatalf("audit rows = %v (err %v), want none", records, err)
			}
		}

		t.Run(name+": a stale updatedAt is a conflict and changes nothing", func(t *testing.T) {
			failed, steps := failedRunWithStepState(t, s, "unknown")
			if err := resume(failed.ID, failed.UpdatedAt+"x", steps[0].ID, audit()); !errors.Is(err, ErrConflict) {
				t.Fatalf("resume with a stale updatedAt = %v, want ErrConflict", err)
			}
			assertUnchanged(t, failed.ID, steps, failed.UpdatedAt)
		})

		t.Run(name+": a step that is not the first unfinished one is a conflict and changes nothing", func(t *testing.T) {
			failed, steps := failedRunWithStepState(t, s, "unknown")
			if err := resume(failed.ID, failed.UpdatedAt, steps[1].ID, audit()); !errors.Is(err, ErrConflict) {
				t.Fatalf("resume with the wrong step = %v, want ErrConflict", err)
			}
			assertUnchanged(t, failed.ID, steps, failed.UpdatedAt)
		})
	}

	t.Run("the audit row is written with the resume and only then", func(t *testing.T) {
		failed, steps := failedRunWithStepState(t, s, "unknown")
		if _, err := s.ResumeRunbookRun(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, "operator", &AuditRecord{
			ActorUserID: "operator", ActorRole: "user", Action: "runbook.run.step_resent", TargetType: "runbook_run", TargetID: failed.ID,
		}); err != nil {
			t.Fatalf("ResumeRunbookRun() error: %v", err)
		}
		records, _, err := s.ListAuditRecords(ctx, "runbook.run.step_resent", "runbook_run", "", "", 0, 10)
		if err != nil || len(records) != 1 || records[0].TargetID != failed.ID {
			t.Fatalf("audit rows = %v (err %v), want one for the run", records, err)
		}
	})

	t.Run("a plain resume without an expected step still resumes", func(t *testing.T) {
		failed, _ := failedRunWithStepState(t, s, "failed")
		if _, err := s.ResumeRunbookRun(ctx, failed.ID, failed.UpdatedAt, "", "operator", nil); err != nil {
			t.Fatalf("ResumeRunbookRun() error: %v", err)
		}
	})
}

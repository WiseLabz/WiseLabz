package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func createAwaitingApprovalRun(t *testing.T, s *Store, titles ...string) (*RunbookRunRecord, []*RunbookRunStepRecord) {
	t.Helper()
	book, _ := createRunbookRunFixture(t, s)
	steps := make([]*RunbookRunStepRecord, 0, len(titles))
	for _, title := range titles {
		steps = append(steps, runbookRunStep("manual", title, "", ""))
	}
	run, frozen, err := s.CreateRunbookRunWithState(context.Background(), book.ID, "requester", steps, "awaiting_approval", true)
	if err != nil {
		t.Fatalf("CreateRunbookRunWithState(awaiting_approval): %v", err)
	}
	return run, frozen
}

func TestRunbookApprovalRequestCreationAndConditionalTransitions(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	book, _ := createRunbookRunFixture(t, s)
	updatedBook, err := s.UpdateRunbook(ctx, book.ID, map[string]any{"requires_approval": true})
	if err != nil || !updatedBook.RequiresApproval {
		t.Fatalf("UpdateRunbook(requires_approval): record %+v, err %v", updatedBook, err)
	}
	if _, err := s.UpdateRunbook(ctx, book.ID, map[string]any{"requires_approval": false}); err != nil {
		t.Fatalf("UpdateRunbook(disable approval): %v", err)
	}
	createdWithApproval, err := s.CreateRunbook(ctx, &RunbookRecord{Title: "Opt-in", TargetType: "change_type", TargetValue: "created-opt-in", RequiresApproval: true})
	if err != nil || !createdWithApproval.RequiresApproval {
		t.Fatalf("CreateRunbook(requires_approval): record %+v, err %v", createdWithApproval, err)
	}

	run, steps := createAwaitingApprovalRun(t, s, "Check service", "Confirm recovery")
	if run.State != "awaiting_approval" || !run.RequiresApproval {
		t.Fatalf("approval run = %+v; want awaiting_approval with approval flag", run)
	}
	if len(steps) != 2 || steps[0].State != "pending" || steps[1].State != "pending" {
		t.Fatalf("approval steps = %+v; want frozen pending steps", steps)
	}
	if _, err := s.UpdateRunbookRun(ctx, run.ID, "awaiting_approval", map[string]any{"state": "running"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("generic awaiting approval transition = %v; want ErrConflict", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("step update while awaiting approval = %v; want ErrConflict", err)
	}
	if _, _, err := s.CreateRunbookRun(ctx, *run.RunbookID, "requester", []*RunbookRunStepRecord{runbookRunStep("manual", "another", "", "")}); !errors.Is(err, ErrConflict) {
		t.Fatalf("second run while approval is pending = %v; want ErrConflict", err)
	}

	approved, approvedSteps, err := s.ApproveRunbookRun(ctx, run.ID, "approver")
	if err != nil {
		t.Fatalf("ApproveRunbookRun(): %v", err)
	}
	if approved.State != "running" || !approved.RequiresApproval || approved.ApprovedBy == nil || *approved.ApprovedBy != "approver" || approved.ApprovedAt == "" || approved.FinishedAt != "" {
		t.Fatalf("approved run = %+v; want running with approver and timestamp", approved)
	}
	if len(approvedSteps) != 2 || approvedSteps[0].State != "pending" || approvedSteps[1].State != "pending" {
		t.Fatalf("approved frozen steps = %+v; want pending until executor starts", approvedSteps)
	}
	if _, _, err := s.ApproveRunbookRun(ctx, run.ID, "another-approver"); !errors.Is(err, ErrConflict) {
		t.Fatalf("second approval = %v; want ErrConflict", err)
	}
	if _, _, err := s.RejectRunbookRun(ctx, run.ID, "another-approver"); !errors.Is(err, ErrConflict) {
		t.Fatalf("reject approved run = %v; want ErrConflict", err)
	}
	if got, _, err := s.GetRunbookRun(ctx, run.ID); err != nil || got.State != "running" || got.ApprovedBy == nil || *got.ApprovedBy != "approver" {
		t.Fatalf("stored approved run = %+v, %v", got, err)
	}
}

func TestRunbookApprovalRejectCancelExpireAndPrune(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	rejected, rejectedSteps := createAwaitingApprovalRun(t, s, "Rejected check")
	gotRejected, gotRejectedSteps, err := s.RejectRunbookRun(ctx, rejected.ID, "reviewer")
	if err != nil || gotRejected.State != "rejected" || gotRejected.RejectedBy == nil || *gotRejected.RejectedBy != "reviewer" || gotRejected.FinishedAt == "" {
		t.Fatalf("rejected run = %+v, %v; want terminal run and reviewer", gotRejected, err)
	}
	if len(gotRejectedSteps) != 1 || gotRejectedSteps[0].State != "skipped" || gotRejectedSteps[0].FinishedAt == "" || rejectedSteps[0].State != "pending" {
		t.Fatalf("rejected steps = %+v; want persisted pending step skipped on transition", gotRejectedSteps)
	}
	if _, _, err := s.RejectRunbookRun(ctx, rejected.ID, "reviewer"); !errors.Is(err, ErrConflict) {
		t.Fatalf("second rejection = %v; want ErrConflict", err)
	}
	if _, _, err := s.ApproveRunbookRun(ctx, rejected.ID, "reviewer"); !errors.Is(err, ErrConflict) {
		t.Fatalf("approve rejected run = %v; want ErrConflict", err)
	}

	cancelled, cancelledSteps := createAwaitingApprovalRun(t, s, "Cancelled check")
	if err := s.CancelRunbookRun(ctx, cancelled.ID, "requester"); err != nil {
		t.Fatalf("CancelRunbookRun(awaiting_approval): %v", err)
	}
	gotCancelled, gotCancelledSteps, err := s.GetRunbookRun(ctx, cancelled.ID)
	if err != nil || gotCancelled.State != "cancelled" || gotCancelled.CancelledBy == nil || *gotCancelled.CancelledBy != "requester" ||
		len(gotCancelledSteps) != 1 || gotCancelledSteps[0].State != "skipped" || gotCancelledSteps[0].FinishedAt == "" || cancelledSteps[0].State != "pending" {
		t.Fatalf("cancelled approval run = %+v, steps %+v, err %v; want skipped pending work", gotCancelled, gotCancelledSteps, err)
	}

	stale, _ := createAwaitingApprovalRun(t, s, "Expired check")
	fresh, _ := createAwaitingApprovalRun(t, s, "Fresh check")
	old := runbookRunTimestamp(time.Now().Add(-48 * time.Hour))
	if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, old, stale.ID); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	ids, err := s.ExpireAwaitingApprovalRunbookRuns(ctx, cutoff)
	if err != nil || len(ids) != 1 || ids[0] != stale.ID {
		t.Fatalf("ExpireAwaitingApprovalRunbookRuns() = %v, %v; want stale run %s only", ids, err, stale.ID)
	}
	expired, expiredSteps, err := s.GetRunbookRun(ctx, stale.ID)
	if err != nil || expired.State != "expired" || expired.Reason != "approval_expired" || expired.FinishedAt == "" || len(expiredSteps) != 1 || expiredSteps[0].State != "skipped" {
		t.Fatalf("expired approval run = %+v, steps %+v, err %v; want skipped pending work", expired, expiredSteps, err)
	}
	if got, _, err := s.GetRunbookRun(ctx, fresh.ID); err != nil || got.State != "awaiting_approval" {
		t.Fatalf("fresh approval run = %+v, %v; want request left open", got, err)
	}
	if ids, err := s.ExpireAwaitingApprovalRunbookRuns(ctx, cutoff); err != nil || len(ids) != 0 {
		t.Fatalf("second expiry pass = %v, %v; want no transitions", ids, err)
	}

	oldFinished := runbookRunTimestamp(time.Now().Add(-48 * time.Hour))
	for _, id := range []string{gotRejected.ID, gotCancelled.ID, expired.ID} {
		if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, oldFinished, id); err != nil {
			t.Fatal(err)
		}
	}
	pruned, err := s.PruneRunbookRuns(ctx, cutoff)
	if err != nil || pruned != 3 {
		t.Fatalf("PruneRunbookRuns() = %d, %v; want rejected, cancelled and expired rows", pruned, err)
	}
	for _, id := range []string{rejected.ID, cancelled.ID, stale.ID} {
		if _, _, err := s.GetRunbookRun(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("pruned approval run %s lookup = %v; want ErrNotFound", id, err)
		}
	}
}

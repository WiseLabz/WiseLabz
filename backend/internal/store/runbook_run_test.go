package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createRunbookRunFixture(t *testing.T, s *Store) (*RunbookRecord, string) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	connector := &ConnectorRecord{
		Name:     "runbook-run-" + suffix,
		Category: "virtualization",
		Type:     "proxmox",
		URL:      "https://example.com",
	}
	if err := s.CreateConnector(ctx, connector); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	runbook, err := s.CreateRunbook(ctx, &RunbookRecord{
		Title:       "Runbook " + suffix,
		TargetType:  "change_type",
		TargetValue: suffix,
	})
	if err != nil {
		t.Fatalf("CreateRunbook() error: %v", err)
	}
	return runbook, connector.ID
}

func runbookRunStep(kind, title, connectorID, verb string) *RunbookRunStepRecord {
	step := &RunbookRunStepRecord{
		Kind:        kind,
		Title:       title,
		ConnectorID: connectorID,
		Verb:        verb,
	}
	if kind == "lifecycle" {
		step.EntityRef = "100"
	}
	return step
}

func enableRunbookRunForeignKeys(t *testing.T, s *Store) {
	t.Helper()
	if s.driver != "sqlite" {
		return
	}
	if _, err := s.db.ExecContext(context.Background(), `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable SQLite foreign keys: %v", err)
	}
}

func TestRunbookRunCreateGetFreezeAndDeleteRunbook(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	enableRunbookRunForeignKeys(t, s)
	runbook, connectorID := createRunbookRunFixture(t, s)
	authoredSteps, err := s.ReplaceRunbookSteps(ctx, runbook.ID, []*RunbookStepRecord{
		{Title: "Restart original", Kind: "lifecycle", ConnectorID: connectorID, Verb: "restart", EntityRef: "100"},
		{Title: "Operator check", Kind: "manual"},
	})
	if err != nil {
		t.Fatalf("ReplaceRunbookSteps() error: %v", err)
	}

	run, savedSteps, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", []*RunbookRunStepRecord{
		runbookRunStep("lifecycle", authoredSteps[0].Title, authoredSteps[0].ConnectorID, authoredSteps[0].Verb),
		runbookRunStep("manual", authoredSteps[1].Title, "", ""),
	})
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	if run.State != "running" || run.RunbookID == nil || *run.RunbookID != runbook.ID || run.RunbookTitle != runbook.Title {
		t.Fatalf("created run = %+v, want active run with frozen runbook identity", run)
	}
	if len(savedSteps) != 2 || savedSteps[0].Position != 0 || savedSteps[1].Position != 1 || savedSteps[0].ID == authoredSteps[0].ID {
		t.Fatalf("saved frozen steps = %+v, want two independent rows in order", savedSteps)
	}
	if savedSteps[0].State != "pending" || savedSteps[1].State != "pending" || savedSteps[1].ConnectorID != "" {
		t.Fatalf("initial frozen steps = %+v, want pending lifecycle and connector-free manual step", savedSteps)
	}

	if _, _, err := s.UpdateRunbookWithSteps(ctx, runbook.ID, map[string]any{"title": "Edited title"}, []*RunbookStepRecord{
		{Title: "Replacement", Kind: "lifecycle", ConnectorID: connectorID, Verb: "stop"},
	}, true); err != nil {
		t.Fatalf("UpdateRunbookWithSteps() error: %v", err)
	}
	if err := s.DeleteRunbook(ctx, runbook.ID); err != nil {
		t.Fatalf("DeleteRunbook() error: %v", err)
	}

	gotRun, gotSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRunbookRun() after authored runbook changes: %v", err)
	}
	if gotRun.RunbookID != nil || gotRun.RunbookTitle != runbook.Title {
		t.Fatalf("run after runbook deletion = %+v, want a null runbook ID and original title", gotRun)
	}
	if len(gotSteps) != 2 || gotSteps[0].Title != "Restart original" || gotSteps[0].Verb != "restart" || gotSteps[0].ConnectorID != connectorID || gotSteps[1].Title != "Operator check" {
		t.Fatalf("frozen steps after authored runbook changes = %+v", gotSteps)
	}
}

func TestRunbookRunActiveConflictAndConcurrentCreate(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	runbook, connectorID := createRunbookRunFixture(t, s)
	if _, _, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", nil); err == nil {
		t.Fatal("CreateRunbookRun() accepted a run without steps")
	}
	tooManySteps := make([]*RunbookRunStepRecord, 21)
	for i := range tooManySteps {
		tooManySteps[i] = runbookRunStep("manual", fmt.Sprintf("Step %d", i), "", "")
	}
	if _, _, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", tooManySteps); err == nil {
		t.Fatal("CreateRunbookRun() accepted more than 20 frozen steps")
	}
	const callers = 8
	start := make(chan struct{})
	results := make(chan struct {
		run *RunbookRunRecord
		err error
	}, callers)
	var wg sync.WaitGroup
	for i := range callers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			run, _, err := s.CreateRunbookRun(ctx, runbook.ID, fmt.Sprintf("starter-%d", i), []*RunbookRunStepRecord{
				runbookRunStep("lifecycle", "Restart", connectorID, "restart"),
			})
			results <- struct {
				run *RunbookRunRecord
				err error
			}{run: run, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	var winnerID string
	conflicts := 0
	conflictRunIDs := make([]string, 0, callers-1)
	for result := range results {
		if result.err == nil {
			if winnerID != "" {
				t.Fatalf("more than one active run created: %s and %s", winnerID, result.run.ID)
			}
			winnerID = result.run.ID
			continue
		}
		if !errors.Is(result.err, ErrConflict) {
			t.Fatalf("concurrent CreateRunbookRun() error = %v, want ErrConflict", result.err)
		}
		var conflict *RunbookRunConflictError
		if !errors.As(result.err, &conflict) || conflict.RunID == "" {
			t.Fatalf("conflict error = %T %v, want existing run ID", result.err, result.err)
		}
		conflictRunIDs = append(conflictRunIDs, conflict.RunID)
		conflicts++
	}
	if winnerID == "" || conflicts != callers-1 {
		t.Fatalf("winner=%q, conflicts=%d, want one winner and %d conflicts", winnerID, conflicts, callers-1)
	}
	for _, conflictID := range conflictRunIDs {
		if conflictID != winnerID {
			t.Fatalf("conflict RunID = %q, want winner %q", conflictID, winnerID)
		}
	}
	got, _, err := s.GetRunbookRun(ctx, winnerID)
	if err != nil || got.ID != winnerID {
		t.Fatalf("GetRunbookRun(winner) = %+v, %v", got, err)
	}

	// A schema failure after the run header insert must roll the whole create
	// back and must not be misreported as a duplicate active run.
	invalidRunbook, _ := createRunbookRunFixture(t, s)
	_, _, err = s.CreateRunbookRun(ctx, invalidRunbook.ID, "starter", []*RunbookRunStepRecord{
		runbookRunStep("not-a-kind", "Invalid", "", ""),
	})
	if err == nil || errors.Is(err, ErrConflict) {
		t.Fatalf("invalid frozen step error = %v, want a non-conflict database error", err)
	}
	items, total, err := s.ListRunbookRuns(ctx, invalidRunbook.ID, 20, 0)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("runs after failed atomic create = %d rows, total %d, err %v", len(items), total, err)
	}
}

func TestRunbookRunTransitionsConfirmCancelAndTerminalGuard(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	runbook, connectorID := createRunbookRunFixture(t, s)
	run, steps, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", []*RunbookRunStepRecord{
		runbookRunStep("lifecycle", "Restart", connectorID, "restart"),
		runbookRunStep("manual", "Verify", "", ""),
		runbookRunStep("lifecycle", "Stop", connectorID, "stop"),
	})
	if err != nil {
		t.Fatal(err)
	}

	started, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"})
	if err != nil {
		t.Fatalf("start first step: %v", err)
	}
	parentAfterStart, _, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNineFractionalDigits(started.StartedAt) || parentAfterStart.UpdatedAt == run.UpdatedAt || !hasNineFractionalDigits(parentAfterStart.UpdatedAt) {
		t.Fatalf("step/run timestamps after start = %q / %q; want fixed precision and parent activity", started.StartedAt, parentAfterStart.UpdatedAt)
	}

	completed, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "running", map[string]any{"state": "succeeded"})
	if err != nil {
		t.Fatalf("complete first step: %v", err)
	}
	if completed.StartedAt != started.StartedAt || completed.FinishedAt == "" || completed.FinishedAt < completed.StartedAt {
		t.Fatalf("completed step times = started %q, finished %q", completed.StartedAt, completed.FinishedAt)
	}

	waiting, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[1].ID, "pending", map[string]any{"state": "waiting"})
	if err != nil {
		t.Fatalf("wait on manual step: %v", err)
	}
	if waiting.StartedAt == "" {
		t.Fatal("manual waiting transition did not set started_at")
	}
	waitingRun, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "waiting_manual"})
	if err != nil {
		t.Fatalf("pause on manual step: %v", err)
	}
	if waitingRun.UpdatedAt <= parentAfterStart.UpdatedAt {
		t.Fatalf("run updated_at after manual pause = %q, want later than %q", waitingRun.UpdatedAt, parentAfterStart.UpdatedAt)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[1].ID, "waiting", map[string]any{"state": "succeeded"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("generic step mutation while waiting_manual = %v, want ErrConflict", err)
	}
	if err := s.ConfirmRunbookRunStep(ctx, run.ID, steps[0].ID, "operator"); !errors.Is(err, ErrConflict) {
		t.Fatalf("confirm lifecycle step = %v, want ErrConflict", err)
	}
	if err := s.ConfirmRunbookRunStep(ctx, run.ID, steps[1].ID, "operator"); err != nil {
		t.Fatalf("ConfirmRunbookRunStep() error: %v", err)
	}
	confirmedRun, confirmedSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmedRun.State != "running" || confirmedRun.UpdatedAt <= waitingRun.UpdatedAt || confirmedSteps[1].State != "succeeded" || confirmedSteps[1].ConfirmedBy != "operator" {
		t.Fatalf("confirmed run/step = %+v / %+v", confirmedRun, confirmedSteps[1])
	}
	if err := s.ConfirmRunbookRunStep(ctx, run.ID, steps[1].ID, "operator"); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate manual confirmation = %v, want ErrConflict", err)
	}

	if err := s.CancelRunbookRun(ctx, run.ID, "canceller"); err != nil {
		t.Fatalf("CancelRunbookRun() error: %v", err)
	}
	cancelledRun, cancelledSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelledRun.State != "cancelled" || cancelledRun.CancelledBy == nil || *cancelledRun.CancelledBy != "canceller" || cancelledRun.FinishedAt == "" {
		t.Fatalf("cancelled run = %+v", cancelledRun)
	}
	if cancelledSteps[0].State != "succeeded" || cancelledSteps[1].State != "succeeded" || cancelledSteps[2].State != "skipped" || cancelledSteps[2].FinishedAt == "" {
		t.Fatalf("steps after cancel = %+v", cancelledSteps)
	}
	if _, err := s.UpdateRunbookRun(ctx, run.ID, "cancelled", map[string]any{"state": "running"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("resume terminal run = %v, want ErrConflict", err)
	}
}

func TestRunbookRunListPaginationAndFrozenTimeouts(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	runbook, connectorID := createRunbookRunFixture(t, s)
	steps := []*RunbookRunStepRecord{
		runbookRunStep("sync_and_wait", "Sync", connectorID, ""),
		runbookRunStep("wait_until_healthy", "Wait healthy", connectorID, ""),
	}
	steps[0].TimeoutSeconds = 90
	steps[1].TimeoutSeconds = 1500
	first, _, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", steps)
	if err != nil {
		t.Fatalf("CreateRunbookRun(first) error: %v", err)
	}
	if _, err := s.UpdateRunbookRun(ctx, first.ID, "running", map[string]any{"state": "succeeded"}); err != nil {
		t.Fatal(err)
	}
	second, _, err := s.CreateRunbookRun(ctx, runbook.ID, "starter", steps)
	if err != nil {
		t.Fatalf("CreateRunbookRun(second) error: %v", err)
	}
	if _, err := s.UpdateRunbookRun(ctx, second.ID, "running", map[string]any{"state": "succeeded"}); err != nil {
		t.Fatal(err)
	}

	newest, total, err := s.ListRunbookRuns(ctx, runbook.ID, 1, 0)
	if err != nil || total != 2 || len(newest) != 1 || newest[0].ID != second.ID {
		t.Fatalf("newest run page = %+v, total %d, err %v; want second run and total 2", newest, total, err)
	}
	older, total, err := s.ListRunbookRuns(ctx, runbook.ID, 1, 1)
	if err != nil || total != 2 || len(older) != 1 || older[0].ID != first.ID {
		t.Fatalf("older run page = %+v, total %d, err %v; want first run and total 2", older, total, err)
	}
	_, frozen, err := s.GetRunbookRun(ctx, first.ID)
	if err != nil || len(frozen) != 2 || frozen[0].TimeoutSeconds != 90 || frozen[1].TimeoutSeconds != 1500 {
		t.Fatalf("frozen timeout steps = %+v, err %v", frozen, err)
	}
}

func TestRunbookRunInterruptExpireAndPrune(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	activeBook, connectorID := createRunbookRunFixture(t, s)
	activeRun, activeSteps, err := s.CreateRunbookRun(ctx, activeBook.ID, "starter", []*RunbookRunStepRecord{runbookRunStep("lifecycle", "Restart", connectorID, "restart")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, activeRun.ID, activeSteps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
		t.Fatal(err)
	}
	waitingBook, _ := createRunbookRunFixture(t, s)
	waitingRun, waitingSteps, err := s.CreateRunbookRun(ctx, waitingBook.ID, "starter", []*RunbookRunStepRecord{runbookRunStep("manual", "Check", "", "")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, waitingRun.ID, waitingSteps[0].ID, "pending", map[string]any{"state": "waiting"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRunbookRun(ctx, waitingRun.ID, "running", map[string]any{"state": "waiting_manual"}); err != nil {
		t.Fatal(err)
	}
	if ids, err := s.InterruptRunningRunbookRuns(ctx); err != nil || len(ids) != 1 || ids[0] != activeRun.ID {
		t.Fatalf("InterruptRunningRunbookRuns() = %v, %v; want only the running run %s", ids, err, activeRun.ID)
	}
	if ids, err := s.InterruptRunningRunbookRuns(ctx); err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("second InterruptRunningRunbookRuns() = %v, %v; want a non-nil empty slice", ids, err)
	}
	interrupted, interruptedSteps, err := s.GetRunbookRun(ctx, activeRun.ID)
	if err != nil || interrupted.State != "failed" || interrupted.Reason != "interrupted" || interruptedSteps[0].State != "unknown" || interruptedSteps[0].FinishedAt == "" {
		t.Fatalf("interrupted run = %+v, steps %+v, err %v", interrupted, interruptedSteps, err)
	}
	resumed, err := s.UpdateRunbookRun(ctx, activeRun.ID, "failed", map[string]any{"state": "running", "resumed_by": "resumer"})
	if err != nil || resumed.State != "running" || resumed.Reason != "" || resumed.FinishedAt != "" || resumed.ResumedBy == nil || *resumed.ResumedBy != "resumer" || resumed.UpdatedAt <= interrupted.UpdatedAt {
		t.Fatalf("resumed interrupted run = %+v, err %v", resumed, err)
	}
	stillWaiting, stillWaitingSteps, err := s.GetRunbookRun(ctx, waitingRun.ID)
	if err != nil || stillWaiting.State != "waiting_manual" || stillWaitingSteps[0].State != "waiting" {
		t.Fatalf("manual wait changed during recovery: run %+v, steps %+v, err %v", stillWaiting, stillWaitingSteps, err)
	}

	cutoffTime := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	cutoff := cutoffTime.Format(time.RFC3339)
	cutoffFixed := runbookRunTimestamp(cutoffTime)
	old := runbookRunTimestamp(cutoffTime.Add(-time.Second))
	after := runbookRunTimestamp(cutoffTime.Add(time.Second))
	setOpenRun := func(state, updatedAt string) (*RunbookRecord, *RunbookRunRecord, *RunbookRunStepRecord) {
		t.Helper()
		book, _ := createRunbookRunFixture(t, s)
		run, steps, err := s.CreateRunbookRun(ctx, book.ID, "starter", []*RunbookRunStepRecord{runbookRunStep("manual", "Check", "", "")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "waiting"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "waiting_manual"}); err != nil {
			t.Fatal(err)
		}
		if state == "failed" {
			if _, err := s.UpdateRunbookRun(ctx, run.ID, "waiting_manual", map[string]any{"state": "failed", "reason": "retry needed"}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, updatedAt, run.ID); err != nil {
			t.Fatal(err)
		}
		return book, run, steps[0]
	}
	oldFailedBook, oldFailed, oldFailedStep := setOpenRun("failed", old)
	_, oldWaiting, _ := setOpenRun("waiting_manual", old)
	_, boundaryFailed, _ := setOpenRun("failed", cutoffFixed)
	_, recentFailed, _ := setOpenRun("failed", after)
	if ids, err := s.ExpireOpenRunbookRuns(ctx, ""); err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("ExpireOpenRunbookRuns(empty) = %v, %v; want a non-nil empty slice", ids, err)
	}
	expiredIDs, err := s.ExpireOpenRunbookRuns(ctx, cutoff)
	if err != nil || len(expiredIDs) != 2 || !slices.Contains(expiredIDs, oldFailed.ID) || !slices.Contains(expiredIDs, oldWaiting.ID) {
		t.Fatalf("ExpireOpenRunbookRuns() = %v, %v; want stale failed run %s and manual run %s", expiredIDs, err, oldFailed.ID, oldWaiting.ID)
	}
	gotOld, gotOldSteps, err := s.GetRunbookRun(ctx, oldFailed.ID)
	if err != nil || gotOld.State != "expired" || gotOldSteps[0].State != "skipped" || gotOldSteps[0].FinishedAt == "" {
		t.Fatalf("expired run = %+v, steps %+v, err %v", gotOld, gotOldSteps, err)
	}
	if gotOldSteps[0].ID != oldFailedStep.ID {
		t.Fatalf("expired step ID = %q, want %q", gotOldSteps[0].ID, oldFailedStep.ID)
	}
	gotWaiting, gotWaitingSteps, err := s.GetRunbookRun(ctx, oldWaiting.ID)
	if err != nil || gotWaiting.State != "expired" || gotWaitingSteps[0].State != "skipped" {
		t.Fatalf("expired manual wait = %+v, steps %+v, err %v", gotWaiting, gotWaitingSteps, err)
	}
	for _, id := range []string{boundaryFailed.ID, recentFailed.ID} {
		got, _, err := s.GetRunbookRun(ctx, id)
		if err != nil || got.State != "failed" {
			t.Fatalf("run at or after cutoff %q = %+v, %v; want failed", id, got, err)
		}
	}
	if _, _, err := s.CreateRunbookRun(ctx, oldFailedBook.ID, "replacement", []*RunbookRunStepRecord{runbookRunStep("manual", "New", "", "")}); err != nil {
		t.Fatalf("start after expiration: %v", err)
	}

	pruneBook, _ := createRunbookRunFixture(t, s)
	finish := func() *RunbookRunRecord {
		t.Helper()
		run, steps, err := s.CreateRunbookRun(ctx, pruneBook.ID, "starter", []*RunbookRunStepRecord{runbookRunStep("lifecycle", "Restart", connectorID, "restart")})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "running", map[string]any{"state": "succeeded"}); err != nil {
			t.Fatal(err)
		}
		updated, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "succeeded"})
		if err != nil {
			t.Fatal(err)
		}
		return updated
	}
	oldSuccess := finish()
	boundarySuccess := finish()
	newerSuccess := finish()
	if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, old, oldSuccess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, cutoffFixed, boundarySuccess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, after, newerSuccess.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.PruneRunbookRuns(ctx, ""); err != nil || n != 0 {
		t.Fatalf("PruneRunbookRuns(empty) = %d, %v; want no-op", n, err)
	}
	if n, err := s.PruneRunbookRuns(ctx, cutoff); err != nil || n != 1 {
		t.Fatalf("PruneRunbookRuns(cutoff) = %d, %v; want one old terminal run", n, err)
	}
	if _, _, err := s.GetRunbookRun(ctx, oldSuccess.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old terminal run lookup error = %v, want ErrNotFound", err)
	}
	for _, id := range []string{boundarySuccess.ID, newerSuccess.ID} {
		if _, _, err := s.GetRunbookRun(ctx, id); err != nil {
			t.Fatalf("run %s at/after prune cutoff was removed: %v", id, err)
		}
	}
}

// createOpenManualRunbookRun starts a run on a fresh runbook with one pending
// manual step per title.
func createOpenManualRunbookRun(t *testing.T, s *Store, titles ...string) (*RunbookRunRecord, []*RunbookRunStepRecord) {
	t.Helper()
	book, _ := createRunbookRunFixture(t, s)
	frozen := make([]*RunbookRunStepRecord, 0, len(titles))
	for _, title := range titles {
		frozen = append(frozen, runbookRunStep("manual", title, "", ""))
	}
	run, steps, err := s.CreateRunbookRun(context.Background(), book.ID, "starter", frozen)
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	return run, steps
}

func runbookRunStepCount(t *testing.T, s *Store, runID string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM runbook_run_steps WHERE run_id = ?`, runID).Scan(&n); err != nil {
		t.Fatalf("count steps of run %s: %v", runID, err)
	}
	return n
}

func TestRunbookRunUpdateRejectsUnsupportedInput(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	run, steps := createOpenManualRunbookRun(t, s, "Check")

	if _, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "failed", "resumedBy": "typo"}); err == nil || !strings.Contains(err.Error(), "resumedBy") {
		t.Fatalf("UpdateRunbookRun(unknown key) error = %v, want one naming resumedBy", err)
	}
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "running", "startedAt": "typo"}); err == nil || !strings.Contains(err.Error(), "startedAt") {
		t.Fatalf("UpdateRunbookRunStep(unknown key) error = %v, want one naming startedAt", err)
	}
	got, gotSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil || got.State != "running" || got.UpdatedAt != run.UpdatedAt || gotSteps[0].State != "pending" || gotSteps[0].StartedAt != "" {
		t.Fatalf("run after rejected updates = %+v, steps %+v, err %v; want nothing changed", got, gotSteps, err)
	}

	for state, fn := range map[string]string{"cancelled": "CancelRunbookRun", "expired": "ExpireOpenRunbookRuns"} {
		if _, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": state}); err == nil || !strings.Contains(err.Error(), fn) {
			t.Fatalf("UpdateRunbookRun(state %s) error = %v, want one naming %s", state, err, fn)
		}
	}
	got, gotSteps, err = s.GetRunbookRun(ctx, run.ID)
	if err != nil || got.State != "running" || got.FinishedAt != "" || gotSteps[0].State != "pending" {
		t.Fatalf("run after rejected terminal states = %+v, steps %+v, err %v; want it still open", got, gotSteps, err)
	}

	done, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "succeeded", "finished_at": ""})
	if err != nil || done.State != "succeeded" || !hasNineFractionalDigits(done.FinishedAt) {
		t.Fatalf("succeeded run with empty finished_at = %+v, %v; want finished_at defaulted to now", done, err)
	}
}

func TestRunbookRunCancelWhileWaitingAndFailed(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	run, steps := createOpenManualRunbookRun(t, s, "Check", "Later")
	if _, err := s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, "pending", map[string]any{"state": "waiting"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRunbookRun(ctx, run.ID, "running", map[string]any{"state": "waiting_manual"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CancelRunbookRun(ctx, run.ID, "canceller"); err != nil {
		t.Fatalf("CancelRunbookRun(waiting_manual) error: %v", err)
	}
	got, gotSteps, err := s.GetRunbookRun(ctx, run.ID)
	if err != nil || got.State != "cancelled" || got.CancelledBy == nil || *got.CancelledBy != "canceller" || len(gotSteps) != 2 || gotSteps[0].State != "skipped" || gotSteps[1].State != "skipped" {
		t.Fatalf("cancelled waiting run = %+v, steps %+v, err %v; want both steps skipped", got, gotSteps, err)
	}
	if err := s.CancelRunbookRun(ctx, run.ID, "canceller"); !errors.Is(err, ErrConflict) {
		t.Fatalf("second CancelRunbookRun() = %v, want ErrConflict", err)
	}

	failed, _ := createOpenManualRunbookRun(t, s, "Check")
	if _, err := s.UpdateRunbookRun(ctx, failed.ID, "running", map[string]any{"state": "failed", "reason": "boom"}); err != nil {
		t.Fatal(err)
	}
	if err := s.CancelRunbookRun(ctx, failed.ID, "canceller"); err != nil {
		t.Fatalf("CancelRunbookRun(failed) error: %v", err)
	}
	if got, _, err := s.GetRunbookRun(ctx, failed.ID); err != nil || got.State != "cancelled" {
		t.Fatalf("cancelled failed run = %+v, %v", got, err)
	}
}

func TestRunbookRunResumeGuardAndExpireSkipsRunning(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	waiting, waitingSteps := createOpenManualRunbookRun(t, s, "Check")
	if _, err := s.UpdateRunbookRunStep(ctx, waiting.ID, waitingSteps[0].ID, "pending", map[string]any{"state": "waiting"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateRunbookRun(ctx, waiting.ID, "running", map[string]any{"state": "waiting_manual"}); err != nil {
		t.Fatal(err)
	}
	// Resume is only valid from failed; a manual wait must be confirmed instead.
	if _, err := s.UpdateRunbookRun(ctx, waiting.ID, "failed", map[string]any{"state": "running", "resumed_by": "resumer"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("resume a waiting_manual run = %v, want ErrConflict", err)
	}
	if got, _, err := s.GetRunbookRun(ctx, waiting.ID); err != nil || got.State != "waiting_manual" {
		t.Fatalf("run after rejected resume = %+v, %v; want waiting_manual", got, err)
	}

	running, _ := createOpenManualRunbookRun(t, s, "Check")
	old := runbookRunTimestamp(time.Now().Add(-48 * time.Hour))
	if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, old, running.ID); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	if ids, err := s.ExpireOpenRunbookRuns(ctx, cutoff); err != nil || len(ids) != 0 {
		t.Fatalf("ExpireOpenRunbookRuns() = %v, %v; want the stale running run left alone", ids, err)
	}
	if got, _, err := s.GetRunbookRun(ctx, running.ID); err != nil || got.State != "running" {
		t.Fatalf("stale running run = %+v, %v; want running", got, err)
	}
}

func TestRunbookRunPruneKeepsOpenAndDeletesTerminalHistory(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	old := runbookRunTimestamp(time.Now().Add(-48 * time.Hour))
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	failRun := func(id string) {
		t.Helper()
		if _, err := s.UpdateRunbookRun(ctx, id, "running", map[string]any{"state": "failed", "reason": "boom"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, old, id); err != nil {
			t.Fatal(err)
		}
	}

	cancelled, _ := createOpenManualRunbookRun(t, s, "Check")
	if err := s.CancelRunbookRun(ctx, cancelled.ID, "canceller"); err != nil {
		t.Fatal(err)
	}
	expired, _ := createOpenManualRunbookRun(t, s, "Check")
	failRun(expired.ID)
	if ids, err := s.ExpireOpenRunbookRuns(ctx, cutoff); err != nil || len(ids) != 1 || ids[0] != expired.ID {
		t.Fatalf("ExpireOpenRunbookRuns() = %v, %v; want the stale failed run %s", ids, err, expired.ID)
	}
	for _, id := range []string{cancelled.ID, expired.ID} {
		if _, err := s.db.ExecContext(ctx, `UPDATE runbook_runs SET finished_at = ? WHERE id = ?`, old, id); err != nil {
			t.Fatal(err)
		}
	}
	// An old failed run is still open (no finished_at), so pruning must keep it.
	openRun, _ := createOpenManualRunbookRun(t, s, "Check")
	failRun(openRun.ID)

	if n, err := s.PruneRunbookRuns(ctx, cutoff); err != nil || n != 2 {
		t.Fatalf("PruneRunbookRuns() = %d, %v; want the cancelled and expired runs", n, err)
	}
	if got, _, err := s.GetRunbookRun(ctx, openRun.ID); err != nil || got.State != "failed" || runbookRunStepCount(t, s, openRun.ID) != 1 {
		t.Fatalf("old open run = %+v, %v; want it kept with its steps", got, err)
	}
	for _, id := range []string{cancelled.ID, expired.ID} {
		if _, _, err := s.GetRunbookRun(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("pruned run %s lookup error = %v, want ErrNotFound", id, err)
		}
		if n := runbookRunStepCount(t, s, id); n != 0 {
			t.Fatalf("pruned run %s kept %d step rows, want 0", id, n)
		}
	}
}

func TestRunbookRunOrphanedActiveRunsCoexist(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	enableRunbookRunForeignKeys(t, s)
	first, _ := createOpenManualRunbookRun(t, s, "Check")
	second, _ := createOpenManualRunbookRun(t, s, "Check")
	for _, run := range []*RunbookRunRecord{first, second} {
		if err := s.DeleteRunbook(ctx, *run.RunbookID); err != nil {
			t.Fatalf("DeleteRunbook() error: %v", err)
		}
	}
	for _, id := range []string{first.ID, second.ID} {
		got, _, err := s.GetRunbookRun(ctx, id)
		if err != nil || got.RunbookID != nil || got.State != "running" {
			t.Fatalf("orphaned run %s = %+v, %v; want a running run with a nil runbook ID", id, got, err)
		}
	}
}

func hasNineFractionalDigits(value string) bool {
	if len(value) != len("2006-01-02T15:04:05.000000000Z") {
		return false
	}
	parsed, err := time.Parse(runbookRunTimestampLayout, value)
	return err == nil && parsed.UTC().Format(runbookRunTimestampLayout) == value
}

package runbookrun

import (
	"context"
	"errors"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// resumeInterleavingStore inserts a competing transition after Resume has read
// its decision target and before it tries to commit that decision.
type resumeInterleavingStore struct {
	*store.Store
	before func()
}

func (s *resumeInterleavingStore) ResumeRunbookRun(ctx context.Context, id, revision, actor string) (*store.RunbookRunRecord, error) {
	s.before()
	return s.Store.ResumeRunbookRun(ctx, id, revision, actor)
}

func (s *resumeInterleavingStore) ResumeRunbookRunMarkingStepDone(ctx context.Context, id, revision, stepID, actor string) (*store.RunbookRunRecord, error) {
	s.before()
	return s.Store.ResumeRunbookRunMarkingStepDone(ctx, id, revision, stepID, actor)
}

func TestConnectorActionResumeRejectsStaleDecision(t *testing.T) {
	for _, decision := range []ResumeDecision{ResumeResend, ResumeMarkDone} {
		t.Run(string(decision), func(t *testing.T) {
			ctx := context.Background()
			e, fa, failed, steps, _ := unknownActionRun(t, func(id string) []*store.RunbookStepRecord {
				return []*store.RunbookStepRecord{actionStep(id, "rescan", ""), actionStep(id, "refresh", "")}
			})
			e.exec.store = &resumeInterleavingStore{Store: e.s, before: func() {
				// The competing user marks the original action done. The next
				// action then loses its response and requires its own decision.
				if _, err := e.s.ResumeRunbookRunMarkingStepDone(ctx, failed.ID, failed.UpdatedAt, steps[0].ID, e.starter); err != nil {
					t.Fatal(err)
				}
				if _, err := e.s.UpdateRunbookRunStep(ctx, failed.ID, steps[1].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
					t.Fatal(err)
				}
				if _, _, err := e.s.FailRunbookRunStep(ctx, failed.ID, steps[1].ID, StepUnknown, "response lost", ReasonStepFailed); err != nil {
					t.Fatal(err)
				}
			}}
			if resumed, applied, err := e.exec.Resume(ctx, failed.ID, e.starter, decision); !errors.Is(err, store.ErrConflict) || resumed != nil || applied != nil {
				t.Fatalf("stale resume = %+v, %+v, %v; want conflict without applied decision", resumed, applied, err)
			}
			e.settle()
			got, after := e.get(failed.ID)
			if got.State != RunFailed || after[0].State != StepSucceeded || after[1].State != StepUnknown || len(fa.snapshot()) != 1 {
				t.Fatalf("stale decision changed run or sent an action: run=%+v steps=%v sends=%d", got, stepStates(after), len(fa.snapshot()))
			}
		})
	}
}

func TestConnectorActionResumeRejectsStaleDecisionAfterSameStepFailsAgain(t *testing.T) {
	ctx := context.Background()
	e, fa, failed, steps, _ := unknownActionRun(t, func(id string) []*store.RunbookStepRecord {
		return []*store.RunbookStepRecord{actionStep(id, "rescan", "")}
	})
	e.exec.store = &resumeInterleavingStore{Store: e.s, before: func() {
		if _, err := e.s.ResumeRunbookRun(ctx, failed.ID, failed.UpdatedAt, e.starter); err != nil {
			t.Fatal(err)
		}
		if _, err := e.s.UpdateRunbookRunStep(ctx, failed.ID, steps[0].ID, StepUnknown, map[string]any{"state": StepRunning}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := e.s.FailRunbookRunStep(ctx, failed.ID, steps[0].ID, StepUnknown, "lost again", ReasonStepFailed); err != nil {
			t.Fatal(err)
		}
	}}
	if _, _, err := e.exec.Resume(ctx, failed.ID, e.starter, ResumeResend); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("stale resend after same step failed again = %v, want conflict", err)
	}
	e.settle()
	if got, after := e.get(failed.ID); got.State != RunFailed || after[0].Error != "lost again" || len(fa.snapshot()) != 1 {
		t.Fatalf("stale resend changed run: %+v, %+v, sends=%d", got, after[0], len(fa.snapshot()))
	}
}

func TestConnectorActionFailedStepIgnoresResumeDecision(t *testing.T) {
	for _, decision := range []ResumeDecision{ResumeResend, ResumeMarkDone} {
		t.Run(string(decision), func(t *testing.T) {
			fa := newFakeActions("fingerprint")
			fa.fn = func(_ context.Context, n int, _ actionCall) (connector.ActionResult, error) {
				if n == 1 {
					return connector.ActionResult{Status: 500, Written: true}, errors.New("API returned 500")
				}
				return connector.ActionResult{Status: 204}, nil
			}
			e := newActionEnv(t, fa)
			run, _ := e.start(actionStep(e.connector(), "rescan", ""))
			e.settle()
			if _, applied, err := e.exec.Resume(context.Background(), run.ID, e.starter, decision); err != nil || applied != nil {
				t.Fatalf("resume failed action = %+v, %v; decision must be ignored", applied, err)
			}
			e.settle()
			if got, _ := e.get(run.ID); got.State != RunSucceeded || len(fa.snapshot()) != 2 {
				t.Fatalf("failed action was skipped: run=%+v sends=%d", got, len(fa.snapshot()))
			}
		})
	}
}

func TestConnectorActionRestartRecoveryNeedsDecisionAndNeverResends(t *testing.T) {
	ctx := context.Background()
	fa := newFakeActions("fingerprint")
	e := newActionEnv(t, fa)
	book, authored := e.runbook(actionStep(e.connector(), "rescan", ""))
	frozen := FreezeSteps(authored)
	frozen[0].ActionFingerprint = "fingerprint"
	run, steps, err := e.s.CreateRunbookRun(ctx, book.ID, e.starter, frozen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.UpdateRunbookRunStep(ctx, run.ID, steps[0].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
		t.Fatal(err)
	}
	if n, err := Recover(ctx, e.s, e.events, e.notes); err != nil || n != 1 {
		t.Fatalf("Recover = %d, %v", n, err)
	}
	if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); !errors.Is(err, ErrDecisionRequired) {
		t.Fatalf("recovered action resume = %v, want decision required", err)
	}
	e.settle()
	if got, after := e.get(run.ID); got.State != RunFailed || after[0].State != StepUnknown || len(fa.snapshot()) != 0 {
		t.Fatalf("recovery sent or altered action: run=%+v steps=%v sends=%d", got, stepStates(after), len(fa.snapshot()))
	}
}

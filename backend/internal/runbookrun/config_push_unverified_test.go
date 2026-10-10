package runbookrun

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func unknownConfigPushRun(t *testing.T) (*env, *store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	t.Helper()
	e := newEnv(t)
	a := e.connector()
	e.push.fn = func(_ context.Context, n int, _ configPushCall) error {
		if n == 1 {
			return &connectors.ConfigPushUnverifiedError{FetchErr: errors.New("secret=do-not-store")}
		}
		return nil
	}
	run, steps := e.start(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
	e.settle()
	run, _ = e.get(run.ID)
	return e, run, steps
}

func TestConfigPushUnverifiedStopsRunAndRequiresResumeDecision(t *testing.T) {
	e, run, _ := unknownConfigPushRun(t)
	got, after := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonStepFailed || !reflectStepStates(after, StepUnknown, StepPending) {
		t.Fatalf("run=%+v steps=%+v; want unknown push and stopped run", got, stepStates(after))
	}
	if strings.Contains(after[0].Error, "do-not-store") || !strings.Contains(after[0].Error, "resulting state is unknown") {
		t.Fatalf("unknown-step error=%q; want safe unknown-state message", after[0].Error)
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 0 {
		t.Fatalf("later lifecycle calls=%+v; want none", calls)
	}
	if _, decision, err := e.exec.Resume(context.Background(), run.ID, e.starter, ResumeNone); !errors.Is(err, ErrDecisionRequired) || decision != nil {
		t.Fatalf("Resume() decision=%+v err=%v; want decision required", decision, err)
	}
	e.settle()
	if got, after := e.get(run.ID); got.State != RunFailed || !reflectStepStates(after, StepUnknown, StepPending) {
		t.Fatalf("decision-free resume changed state: run=%+v steps=%v", got, stepStates(after))
	}
}

func TestConfigPushUnknownResumeRetriesFrozenStepOrMarksDone(t *testing.T) {
	t.Run("retry", func(t *testing.T) {
		e, run, steps := unknownConfigPushRun(t)
		resumed, applied, err := e.exec.Resume(context.Background(), run.ID, e.starter, ResumeResend)
		if err != nil || applied == nil || applied.StepID != steps[0].ID || applied.Decision != ResumeResend || resumed.State != RunRunning {
			t.Fatalf("Resume() = %+v, %+v, %v", resumed, applied, err)
		}
		e.settle()
		got, after := e.get(run.ID)
		calls := e.push.snapshot()
		if got.State != RunSucceeded || !reflectStepStates(after, StepSucceeded, StepSucceeded) || len(calls) != 2 {
			t.Fatalf("run=%+v steps=%v pushes=%+v; want one retry and continuation", got, stepStates(after), calls)
		}
		if calls[0].Value != calls[1].Value || calls[0].FieldKey != calls[1].FieldKey || calls[0].EntityRef != calls[1].EntityRef {
			t.Fatalf("retry changed frozen config-push input: first=%+v retry=%+v", calls[0], calls[1])
		}
	})

	t.Run("mark done", func(t *testing.T) {
		e, run, steps := unknownConfigPushRun(t)
		resumed, applied, err := e.exec.Resume(context.Background(), run.ID, e.starter, ResumeMarkDone)
		if err != nil || applied == nil || applied.StepID != steps[0].ID || applied.Decision != ResumeMarkDone || resumed.State != RunRunning {
			t.Fatalf("Resume() = %+v, %+v, %v", resumed, applied, err)
		}
		e.settle()
		got, after := e.get(run.ID)
		if got.State != RunSucceeded || !reflectStepStates(after, StepSucceeded, StepSucceeded) || len(e.push.snapshot()) != 1 {
			t.Fatalf("run=%+v steps=%v pushes=%d; want no additional push", got, stepStates(after), len(e.push.snapshot()))
		}
	})
}

func TestConfigPushUnknownResumeRejectsStaleRevision(t *testing.T) {
	e, run, steps := unknownConfigPushRun(t)
	wantRevision := run.UpdatedAt + "-stale"
	_, applied, err := e.exec.ResumeExpecting(context.Background(), run.ID, e.starter, ResumeResend,
		ResumeExpectation{StepID: steps[0].ID, UpdatedAt: wantRevision})
	if !errors.Is(err, ErrRunChanged) || applied != nil {
		t.Fatalf("stale resume applied=%+v err=%v; want ErrRunChanged without decision", applied, err)
	}
	e.settle()
	got, after := e.get(run.ID)
	if got.State != RunFailed || !reflectStepStates(after, StepUnknown, StepPending) || len(e.push.snapshot()) != 1 {
		t.Fatalf("stale decision changed run: run=%+v steps=%v pushes=%d", got, stepStates(after), len(e.push.snapshot()))
	}
}

type concurrentResumeStore struct {
	*store.Store
	arrived chan struct{}
	release chan struct{}
}

func (s *concurrentResumeStore) wait() {
	s.arrived <- struct{}{}
	<-s.release
}

func (s *concurrentResumeStore) ResumeRunbookRun(ctx context.Context, id, revision, stepID, actor string, audit *store.AuditRecord) (*store.RunbookRunRecord, error) {
	s.wait()
	return s.Store.ResumeRunbookRun(ctx, id, revision, stepID, actor, audit)
}

func (s *concurrentResumeStore) ResumeRunbookRunMarkingStepDone(ctx context.Context, id, revision, stepID, actor string, audit *store.AuditRecord) (*store.RunbookRunRecord, error) {
	s.wait()
	return s.Store.ResumeRunbookRunMarkingStepDone(ctx, id, revision, stepID, actor, audit)
}

func TestConcurrentConfigPushResumeDecisionsHaveOneWinner(t *testing.T) {
	e, run, steps := unknownConfigPushRun(t)
	barrier := &concurrentResumeStore{Store: e.s, arrived: make(chan struct{}, 2), release: make(chan struct{})}
	e.exec.store = barrier
	type result struct {
		decision ResumeDecision
		applied  *StepDecision
		err      error
	}
	results := make(chan result, 2)
	for _, decision := range []ResumeDecision{ResumeResend, ResumeMarkDone} {
		decision := decision
		go func() {
			action := "runbook.run.step_resent"
			if decision == ResumeMarkDone {
				action = "runbook.run.step_marked_done"
			}
			audit, err := store.NewAuditRecord(e.starter, false, action, "runbook_run", run.ID,
				map[string]any{"runId": run.ID, "stepId": steps[0].ID, "decision": string(decision)})
			if err != nil {
				results <- result{decision: decision, err: err}
				return
			}
			_, applied, err := e.exec.ResumeExpecting(context.Background(), run.ID, e.starter, decision,
				ResumeExpectation{StepID: steps[0].ID, UpdatedAt: run.UpdatedAt, Audit: audit})
			results <- result{decision: decision, applied: applied, err: err}
		}()
	}
	<-barrier.arrived
	<-barrier.arrived
	close(barrier.release)
	first, second := <-results, <-results
	e.settle()
	winnerCount := 0
	for _, got := range []result{first, second} {
		if got.err == nil {
			winnerCount++
			if got.applied == nil || got.applied.StepID != steps[0].ID || got.applied.Decision != got.decision {
				t.Fatalf("successful decision=%+v result=%+v", got.applied, got)
			}
		} else if !errors.Is(got.err, ErrRunChanged) {
			t.Fatalf("losing decision %s error=%v; want ErrRunChanged", got.decision, got.err)
		}
	}
	if winnerCount != 1 {
		t.Fatalf("successful decisions=%d; want one (%+v, %+v)", winnerCount, first, second)
	}
	got, after := e.get(run.ID)
	if got.State != RunSucceeded || !reflectStepStates(after, StepSucceeded, StepSucceeded) {
		t.Fatalf("run=%+v steps=%v; want one decision and continuation", got, stepStates(after))
	}
	if calls := e.push.snapshot(); len(calls) < 1 || len(calls) > 2 {
		t.Fatalf("config-push calls=%d; want initial write and at most one retry", len(calls))
	}
	resent, _, err := e.s.ListAuditRecords(context.Background(), "runbook.run.step_resent", "runbook_run", "", "", 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	markedDone, _, err := e.s.ListAuditRecords(context.Background(), "runbook.run.step_marked_done", "runbook_run", "", "", 0, 10)
	if err != nil || len(resent)+len(markedDone) != 1 {
		t.Fatalf("resume decision audits resent=%d mark_done=%d err=%v; want exactly one", len(resent), len(markedDone), err)
	}
}

func TestConfigPushUnknownResumeRequiresDecisionFields(t *testing.T) {
	e, run, steps := unknownConfigPushRun(t)
	for name, expect := range map[string]ResumeExpectation{
		"step id":  {UpdatedAt: run.UpdatedAt},
		"revision": {StepID: steps[0].ID},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := e.exec.ResumeExpecting(context.Background(), run.ID, e.starter, ResumeResend, expect); !errors.Is(err, ErrDecisionFieldsRequired) {
				t.Fatalf("ResumeExpecting() error=%v; want ErrDecisionFieldsRequired", err)
			}
		})
	}
	if got, after := e.get(run.ID); got.State != RunFailed || !reflectStepStates(after, StepUnknown, StepPending) || len(e.push.snapshot()) != 1 {
		t.Fatalf("field-free decisions changed run: %+v steps=%v pushes=%d", got, stepStates(after), len(e.push.snapshot()))
	}
}

func reflectStepStates(steps []*store.RunbookRunStepRecord, want ...string) bool {
	if len(steps) != len(want) {
		return false
	}
	for i := range steps {
		if steps[i].State != want[i] {
			return false
		}
	}
	return true
}

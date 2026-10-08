package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// actionCall is one request that reached the fake service.
type actionCall struct {
	ConnectorID         string
	Name                string
	EntityRef           string
	ExpectedFingerprint string
	Actor               connectors.LifecycleActor
	Audit               map[string]any
}

// fakeActions models the connectors handler: it reports one fingerprint for
// the action and, like the handler, refuses a request whose expected
// fingerprint differs without sending it.
type fakeActions struct {
	mu             sync.Mutex
	fingerprint    string
	fingerprintErr error
	sent           []actionCall
	refused        int
	// fn decides the outcome of the n-th request that reaches the service
	// (from 1); nil answers 204.
	fn func(ctx context.Context, n int, call actionCall) (connector.ActionResult, error)
}

func newFakeActions(fingerprint string) *fakeActions {
	return &fakeActions{fingerprint: fingerprint}
}

func (f *fakeActions) setFingerprint(fingerprint string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fingerprint = fingerprint
}

func (f *fakeActions) ActionFingerprint(context.Context, string, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fingerprintErr != nil {
		return "", f.fingerprintErr
	}
	return f.fingerprint, nil
}

func (f *fakeActions) MutateRunbookAction(ctx context.Context, connectorID, name, entityRef, expectedFingerprint string, actor connectors.LifecycleActor, extraAudit map[string]any) (connector.ActionResult, error) {
	call := actionCall{ConnectorID: connectorID, Name: name, EntityRef: entityRef, ExpectedFingerprint: expectedFingerprint, Actor: actor, Audit: extraAudit}
	f.mu.Lock()
	if expectedFingerprint != "" && expectedFingerprint != f.fingerprint {
		f.refused++
		f.mu.Unlock()
		return connector.ActionResult{}, fmt.Errorf("action %q: %w", name, connectors.ErrActionChanged)
	}
	f.sent = append(f.sent, call)
	n := len(f.sent)
	fn := f.fn
	f.mu.Unlock()
	if fn == nil {
		return connector.ActionResult{Status: http.StatusNoContent}, nil
	}
	return fn(ctx, n, call)
}

func (f *fakeActions) snapshot() []actionCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]actionCall(nil), f.sent...)
}

func (f *fakeActions) refusedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refused
}

// actionStep is an authored connector_action step.
func actionStep(connectorID, name, entityRef string) *store.RunbookStepRecord {
	return &store.RunbookStepRecord{Kind: KindConnectorAction, Title: "Run " + name, ConnectorID: connectorID, Action: name, EntityRef: entityRef}
}

// newActionEnv is an env whose executor performs connector actions through fa.
func newActionEnv(t *testing.T, fa *fakeActions) *env {
	t.Helper()
	e := newEnv(t)
	e.exec.actions = fa
	return e
}

// unknownActionRun starts a run whose first step loses its connection after
// sending: the step ends unknown and the run failed. Later requests answer 204.
func unknownActionRun(t *testing.T, build func(connectorID string) []*store.RunbookStepRecord) (*env, *fakeActions, *store.RunbookRunRecord, []*store.RunbookRunStepRecord, string) {
	t.Helper()
	fa := newFakeActions("fp-1")
	fa.fn = func(_ context.Context, n int, _ actionCall) (connector.ActionResult, error) {
		if n == 1 {
			return connector.ActionResult{Written: true}, errors.New("connection reset by peer")
		}
		return connector.ActionResult{Status: http.StatusNoContent}, nil
	}
	e := newActionEnv(t, fa)
	a := e.connector()
	run, _ := e.start(build(a)...)
	e.settle()
	failed, steps := e.get(run.ID)
	if failed.State != RunFailed || steps[0].State != StepUnknown {
		t.Fatalf("run = %+v, steps = %v; want failed with the action step unknown", failed, stepStates(steps))
	}
	return e, fa, failed, steps, a
}

// syncBuffer collects log output from the executor's goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// captureLogs sends the default logger to a buffer for the rest of the test.
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

func TestConnectorActionStepSucceeds(t *testing.T) {
	fa := newFakeActions("fp-1")
	e := newActionEnv(t, fa)
	a := e.connector()
	run, frozen := e.start(actionStep(a, "rescan", "100"), lifecycleStep(a, "start"))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) {
		t.Fatalf("run = %+v, steps = %v; want succeeded", got, stepStates(steps))
	}
	calls := fa.snapshot()
	if len(calls) != 1 {
		t.Fatalf("action requests = %+v, want exactly one", calls)
	}
	call := calls[0]
	if call.ConnectorID != a || call.Name != "rescan" || call.EntityRef != "100" || call.ExpectedFingerprint != "fp-1" {
		t.Fatalf("action request = %+v, want the frozen connector, action, entity and fingerprint", call)
	}
	if call.Actor.UserID != e.starter {
		t.Fatalf("actor = %q, want the starter %q", call.Actor.UserID, e.starter)
	}
	if call.Audit["runId"] != run.ID || call.Audit["stepId"] != frozen[0].ID || call.Audit["stepIndex"] != 0 {
		t.Fatalf("audit detail = %v, want the run, step and step index", call.Audit)
	}
	// The run continued: the next step executed.
	if lifecycle := e.lifecycle.snapshot(); len(lifecycle) != 1 {
		t.Fatalf("lifecycle calls = %+v, want the next step to run once", lifecycle)
	}
}

func TestConnectorActionStepRefusesChangedAction(t *testing.T) {
	fa := newFakeActions("fp-1")
	e := newActionEnv(t, fa)
	a := e.connector()
	run, frozen := e.start(manualStep("Check"), actionStep(a, "rescan", ""))
	e.settle()
	if got, _ := e.get(run.ID); got.State != RunWaitingManual {
		t.Fatalf("run = %+v, want it waiting on the manual step", got)
	}
	// The definition changes while the run waits on the manual step.
	fa.setFingerprint("fp-2")
	if err := e.exec.Confirm(context.Background(), run.ID, frozen[0].ID, e.starter); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonStepFailed {
		t.Fatalf("run = %+v, want failed for a step failure", got)
	}
	if steps[1].State != StepFailed || steps[1].Error != "The action changed since the run started; nothing was sent." {
		t.Fatalf("action step = %+v, want failed with the changed-action reason", steps[1])
	}
	if sent := fa.snapshot(); len(sent) != 0 || fa.refusedCount() != 1 {
		t.Fatalf("sent = %+v, refused = %d; want nothing sent and one refusal", sent, fa.refusedCount())
	}
}

func TestConnectorActionStepConnectionLostAfterSending(t *testing.T) {
	fa := newFakeActions("fp-1")
	fa.fn = func(context.Context, int, actionCall) (connector.ActionResult, error) {
		return connector.ActionResult{Written: true}, errors.New("connection reset by peer")
	}
	e := newActionEnv(t, fa)
	a := e.connector()
	run, _ := e.start(actionStep(a, "rescan", ""), lifecycleStep(a, "start"))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonStepFailed {
		t.Fatalf("run = %+v, want failed", got)
	}
	if steps[0].State != StepUnknown || !strings.Contains(steps[0].Error, "connection reset by peer") {
		t.Fatalf("action step = %+v, want unknown with the connection error", steps[0])
	}
	if steps[1].State != StepPending {
		t.Fatalf("next step = %s, want pending", steps[1].State)
	}
	if lifecycle := e.lifecycle.snapshot(); len(lifecycle) != 0 {
		t.Fatalf("lifecycle calls = %+v, want none after the run failed", lifecycle)
	}
}

func TestConnectorActionStepServiceRefuses(t *testing.T) {
	fa := newFakeActions("fp-1")
	fa.fn = func(context.Context, int, actionCall) (connector.ActionResult, error) {
		return connector.ActionResult{Status: http.StatusInternalServerError, Written: true}, errors.New("API returned 500: ")
	}
	e := newActionEnv(t, fa)
	a := e.connector()
	run, _ := e.start(actionStep(a, "rescan", ""), lifecycleStep(a, "start"))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonStepFailed {
		t.Fatalf("run = %+v, want failed", got)
	}
	if steps[0].State != StepFailed || !strings.Contains(steps[0].Error, "API returned 500") {
		t.Fatalf("action step = %+v, want failed with the status in its error", steps[0])
	}
}

func TestConnectorActionStepRefusedBeforeSending(t *testing.T) {
	fa := newFakeActions("fp-1")
	fa.fn = func(context.Context, int, actionCall) (connector.ActionResult, error) {
		return connector.ActionResult{}, errors.New("connector does not support action rescan")
	}
	e := newActionEnv(t, fa)
	a := e.connector()
	run, _ := e.start(actionStep(a, "rescan", ""))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || steps[0].State != StepFailed || steps[0].Error != "connector does not support action rescan" {
		t.Fatalf("run = %+v, steps = %+v; want the step failed, not unknown", got, steps)
	}
}

func TestConnectorActionExcerptIsNeverRecorded(t *testing.T) {
	logs := captureLogs(t)
	fa := newFakeActions("fp-1")
	fa.fn = func(context.Context, int, actionCall) (connector.ActionResult, error) {
		return connector.ActionResult{Status: http.StatusInternalServerError, Excerpt: "SECRET-BODY", Written: true}, errors.New("API returned 500: ")
	}
	e := newActionEnv(t, fa)
	a := e.connector()
	run, _ := e.start(actionStep(a, "rescan", ""))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed {
		t.Fatalf("run = %+v, want failed", got)
	}
	if strings.Contains(steps[0].Error, "SECRET-BODY") {
		t.Fatalf("step error = %q carries the response excerpt", steps[0].Error)
	}
	if out := fmt.Sprint(e.events.snapshot()); strings.Contains(out, "SECRET-BODY") {
		t.Fatalf("events carry the response excerpt: %s", out)
	}
	if out := fmt.Sprint(e.notes.snapshot()); strings.Contains(out, "SECRET-BODY") {
		t.Fatalf("notifications carry the response excerpt: %s", out)
	}
	if out := logs.String(); strings.Contains(out, "SECRET-BODY") {
		t.Fatalf("logs carry the response excerpt: %s", out)
	}
}

func TestConnectorActionStartFreezesAndRefusesWithoutFingerprint(t *testing.T) {
	ctx := context.Background()

	t.Run("freezes the action and its fingerprint", func(t *testing.T) {
		fa := newFakeActions("fp-1")
		e := newActionEnv(t, fa)
		a := e.connector()
		step := actionStep(a, "rescan", "100")
		step.TimeoutSeconds = 90
		run, frozen := e.start(step)
		e.settle()
		if frozen[0].Action != "rescan" || frozen[0].ActionFingerprint != "fp-1" || frozen[0].TimeoutSeconds != 0 {
			t.Fatalf("frozen step = %+v, want action, fingerprint and no timeout", frozen[0])
		}
		if _, steps := e.get(run.ID); steps[0].ActionFingerprint != "fp-1" || steps[0].Action != "rescan" {
			t.Fatalf("stored step = %+v, want the action and fingerprint persisted", steps[0])
		}
	})

	t.Run("creates no run when the fingerprint cannot be computed", func(t *testing.T) {
		fa := newFakeActions("fp-1")
		fa.fingerprintErr = errors.New("connector record has secret-token inside")
		e := newActionEnv(t, fa)
		a := e.connector()
		book, saved := e.runbook(actionStep(a, "rescan", ""))
		_, _, err := e.exec.Start(ctx, book.ID, e.starter, saved)
		if !errors.Is(err, ErrActionUnavailable) {
			t.Fatalf("Start() error = %v, want ErrActionUnavailable", err)
		}
		if strings.Contains(err.Error(), "secret-token") || !strings.Contains(err.Error(), "Run rescan") {
			t.Fatalf("Start() error = %q, want the step named and no cause text", err)
		}
		if _, total, err := e.s.ListRunbookRuns(ctx, book.ID, 10, 0); err != nil || total != 0 {
			t.Fatalf("runs = %d, %v; want none created", total, err)
		}
	})

	t.Run("creates no run without an action service", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		book, saved := e.runbook(actionStep(a, "rescan", ""))
		if _, _, err := e.exec.Start(ctx, book.ID, e.starter, saved); !errors.Is(err, ErrActionUnavailable) {
			t.Fatalf("Start() error = %v, want ErrActionUnavailable", err)
		}
		if _, total, err := e.s.ListRunbookRuns(ctx, book.ID, 10, 0); err != nil || total != 0 {
			t.Fatalf("runs = %d, %v; want none created", total, err)
		}
	})
}

func TestConnectorActionResumeWithoutDecisionIsRejected(t *testing.T) {
	ctx := context.Background()
	e, fa, failed, _, _ := unknownActionRun(t, func(a string) []*store.RunbookStepRecord {
		return []*store.RunbookStepRecord{actionStep(a, "rescan", ""), lifecycleStep(a, "start")}
	})

	if _, decision, err := e.exec.Resume(ctx, failed.ID, e.starter, ResumeNone); !errors.Is(err, ErrDecisionRequired) || decision != nil {
		t.Fatalf("Resume() = %v, %v; want ErrDecisionRequired and no decision", decision, err)
	}
	e.settle()
	got, steps := e.get(failed.ID)
	if got.State != RunFailed || steps[0].State != StepUnknown {
		t.Fatalf("run = %+v, steps = %v; want the run still failed with the step unknown", got, stepStates(steps))
	}
	if sent := fa.snapshot(); len(sent) != 1 {
		t.Fatalf("action requests = %d, want only the first attempt", len(sent))
	}
}

func TestConnectorActionResumeResendSendsOnce(t *testing.T) {
	ctx := context.Background()
	e, fa, failed, steps, _ := unknownActionRun(t, func(a string) []*store.RunbookStepRecord {
		return []*store.RunbookStepRecord{actionStep(a, "rescan", ""), lifecycleStep(a, "start")}
	})
	resumed, decision, err := e.exec.Resume(ctx, failed.ID, e.starter, ResumeResend)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if decision == nil || decision.StepID != steps[0].ID || decision.Decision != ResumeResend {
		t.Fatalf("decision = %+v, want resend for the action step", decision)
	}
	if resumed.State != RunRunning {
		t.Fatalf("resumed run = %+v, want running", resumed)
	}
	e.settle()

	got, after := e.get(failed.ID)
	if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(after), []string{StepSucceeded, StepSucceeded}) {
		t.Fatalf("run = %+v, steps = %v; want succeeded", got, stepStates(after))
	}
	if sent := fa.snapshot(); len(sent) != 2 || sent[1].ExpectedFingerprint != "fp-1" {
		t.Fatalf("action requests = %+v, want the resend once more with the frozen fingerprint", sent)
	}
}

func TestConnectorActionResumeMarkDoneSkipsTheRequest(t *testing.T) {
	ctx := context.Background()
	e, fa, failed, steps, a := unknownActionRun(t, func(a string) []*store.RunbookStepRecord {
		return []*store.RunbookStepRecord{actionStep(a, "rescan", ""), lifecycleStep(a, "start")}
	})
	resumer := e.operator(a)
	resumed, decision, err := e.exec.Resume(ctx, failed.ID, resumer, ResumeMarkDone)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if decision == nil || decision.StepID != steps[0].ID || decision.Decision != ResumeMarkDone {
		t.Fatalf("decision = %+v, want mark_done for the action step", decision)
	}
	if resumed.ResumedBy == nil || *resumed.ResumedBy != resumer {
		t.Fatalf("resumed run = %+v, want resumed by %s", resumed, resumer)
	}
	e.settle()

	got, after := e.get(failed.ID)
	if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(after), []string{StepSucceeded, StepSucceeded}) {
		t.Fatalf("run = %+v, steps = %v; want succeeded", got, stepStates(after))
	}
	if after[0].Error != "" || after[0].FinishedAt == "" {
		t.Fatalf("marked step = %+v, want no error and a finish time", after[0])
	}
	if sent := fa.snapshot(); len(sent) != 1 {
		t.Fatalf("action requests = %d, want none after the first attempt", len(sent))
	}
	if lifecycle := e.lifecycle.snapshot(); len(lifecycle) != 1 || lifecycle[0].Actor.UserID != resumer {
		t.Fatalf("lifecycle calls = %+v, want the next step to run once as the resuming user", lifecycle)
	}
}

func TestConnectorActionResumeMarkDoneOnLastStepFinishesRun(t *testing.T) {
	ctx := context.Background()
	e, fa, failed, _, _ := unknownActionRun(t, func(a string) []*store.RunbookStepRecord {
		return []*store.RunbookStepRecord{actionStep(a, "rescan", "")}
	})
	if _, decision, err := e.exec.Resume(ctx, failed.ID, e.starter, ResumeMarkDone); err != nil || decision == nil {
		t.Fatalf("Resume() = %v, %v; want the decision applied", decision, err)
	}
	e.settle()

	got, steps := e.get(failed.ID)
	if got.State != RunSucceeded || steps[0].State != StepSucceeded {
		t.Fatalf("run = %+v, steps = %v; want the run succeeded", got, stepStates(steps))
	}
	if sent := fa.snapshot(); len(sent) != 1 {
		t.Fatalf("action requests = %d, want none after the first attempt", len(sent))
	}
}

func TestConnectorActionResumeDecisionIgnoredForOtherSteps(t *testing.T) {
	ctx := context.Background()
	e := newActionEnv(t, newFakeActions("fp-1"))
	a := e.connector()
	book, saved := e.runbook(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
	// State left by a process that stopped during the first lifecycle step.
	run, frozen, err := e.s.CreateRunbookRun(ctx, book.ID, e.starter, FreezeSteps(saved))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.s.UpdateRunbookRunStep(ctx, run.ID, frozen[0].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
		t.Fatal(err)
	}
	if n, err := Recover(ctx, e.s, e.events, e.notes); err != nil || n != 1 {
		t.Fatalf("Recover() = %d, %v", n, err)
	}

	resumed, decision, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeMarkDone)
	if err != nil || decision != nil || resumed == nil {
		t.Fatalf("Resume() = %v, %v, %v; want a plain resume with no decision", resumed, decision, err)
	}
	e.settle()
	got, steps := e.get(run.ID)
	calls := e.lifecycle.snapshot()
	if got.State != RunSucceeded || len(calls) != 2 || calls[0].Audit["stepId"] != frozen[0].ID {
		t.Fatalf("run = %+v, steps = %v, calls = %+v; want the unknown lifecycle step to run again", got, stepStates(steps), calls)
	}
}

func TestResumeRejectsUnknownDecision(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.exec.Resume(context.Background(), "missing", e.starter, ResumeDecision("skip")); !errors.Is(err, ErrInvalidDecision) {
		t.Fatalf("Resume() error = %v, want ErrInvalidDecision", err)
	}
}

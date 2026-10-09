package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

// hookStore wraps the real store and lets a test intercept the writes that
// close a step or a run. A hook that returns an error replaces the call.
type hookStore struct {
	*store.Store
	pause    func(runID string) error
	finish   func(runID string) error
	failStep func(runID string) error
	// after is told when a resume (ResumeRunbookRun) or a confirm
	// (ConfirmRunbookRunStep) has returned from the store.
	after func(op string)
}

func (h *hookStore) ResumeRunbookRun(ctx context.Context, id, expectedUpdatedAt, expectedStepID, userID string, audit *store.AuditRecord) (*store.RunbookRunRecord, error) {
	run, err := h.Store.ResumeRunbookRun(ctx, id, expectedUpdatedAt, expectedStepID, userID, audit)
	if h.after != nil && err == nil {
		h.after("resume")
	}
	return run, err
}

func (h *hookStore) ConfirmRunbookRunStep(ctx context.Context, runID, stepID, confirmedBy string) error {
	err := h.Store.ConfirmRunbookRunStep(ctx, runID, stepID, confirmedBy)
	if h.after != nil {
		h.after("confirm")
	}
	return err
}

func (h *hookStore) PauseRunbookRunOnManualStep(ctx context.Context, runID, stepID string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error) {
	if h.pause != nil {
		if err := h.pause(runID); err != nil {
			return nil, nil, err
		}
	}
	return h.Store.PauseRunbookRunOnManualStep(ctx, runID, stepID)
}

func (h *hookStore) FinishRunbookRun(ctx context.Context, runID, stepID string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error) {
	if h.finish != nil {
		if err := h.finish(runID); err != nil {
			return nil, nil, err
		}
	}
	return h.Store.FinishRunbookRun(ctx, runID, stepID)
}

func (h *hookStore) FailRunbookRunStep(ctx context.Context, runID, stepID, stepState, stepError, reason string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error) {
	if h.failStep != nil {
		if err := h.failStep(runID); err != nil {
			return nil, nil, err
		}
	}
	return h.Store.FailRunbookRunStep(ctx, runID, stepID, stepState, stepError, reason)
}

// realConnector is a connector type whose Restart runs a test-supplied
// operation.
type realConnector struct {
	typ       string
	operation func(context.Context) error
}

func (c *realConnector) Name() string     { return "runbookrun-fixes" }
func (c *realConnector) Type() string     { return c.typ }
func (c *realConnector) Category() string { return "test" }
func (c *realConnector) Validate(context.Context, map[string]any) error {
	return nil
}
func (c *realConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{ServiceName: "runbookrun-fixes"}, nil
}
func (c *realConnector) Restart(ctx context.Context, _ map[string]any, _ string) error {
	return c.operation(ctx)
}

// waitSignal waits for ch to be closed or to receive, failing the test after
// testWait.
func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(testWait):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// useRealLifecycle points the env's executor at a real connectors.Handler over
// the env's store and returns the id of a connector of a freshly registered
// type, on which the starter is operator. restart is the connector's Restart.
func useRealLifecycle(t *testing.T, e *env, restart func(context.Context) error) string {
	t.Helper()
	typ := "runbookrun_fixes_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	connector.Register(
		connector.TypeSchema{Type: typ, Category: "networking", Name: "Runbookrun Fixes"},
		func(map[string]any) (connector.Connector, error) {
			return &realConnector{typ: typ, operation: restart}, nil
		},
	)
	rec := &store.ConnectorRecord{
		Name:       "Fixes " + uuid.NewString(),
		Category:   "networking",
		Type:       typ,
		URL:        "https://runbookrun-fixes-" + uuid.NewString() + ".example.com",
		ConfigData: "{}",
		Enabled:    true,
		VerifyTLS:  true,
	}
	if err := e.s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatalf("CreateConnector() error: %v", err)
	}
	apitest.GrantConnectorRole(t, e.s, e.starter, rec.ID, "operator")
	cfg := &config.Config{Encryption: config.EncryptionSettings{Key: "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="}}
	handler := connectors.NewHandler(e.s, nil, cfg, nil, nil)
	e.exec = e.newExecutor(e.s)
	e.exec.lifecycle = handler
	return rec.ID
}

func (e *env) alerts(connectorID string) []store.AlertRecord {
	e.t.Helper()
	alerts, _, err := e.s.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10)
	if err != nil {
		e.t.Fatalf("ListAlerts() error: %v", err)
	}
	return alerts
}

// TestRealLifecycleAlertsOnlyForConnectorFailures runs the executor over the
// real lifecycle core: a step abandoned because the run was cancelled or the
// server shut down raises no failure alert, a connector failure raises one.
func TestRealLifecycleAlertsOnlyForConnectorFailures(t *testing.T) {
	t.Run("user cancel mid step", func(t *testing.T) {
		e := newEnv(t)
		started := make(chan struct{})
		var once sync.Once
		a := useRealLifecycle(t, e, func(ctx context.Context) error {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return ctx.Err()
		})
		run, _ := e.start(lifecycleStep(a, "restart"))
		waitSignal(t, started, "the connector call to start")
		if err := e.exec.Cancel(context.Background(), run.ID, e.starter); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown}) {
			t.Fatalf("run = %+v, steps = %v; want cancelled with the step unknown", got, stepStates(steps))
		}
		if alerts := e.alerts(a); len(alerts) != 0 {
			t.Fatalf("alerts after a user cancel = %+v, want none", alerts)
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications after a user cancel = %+v, want none", notes)
		}
	})

	t.Run("shutdown mid step", func(t *testing.T) {
		e := newEnv(t)
		started := make(chan struct{})
		var once sync.Once
		a := useRealLifecycle(t, e, func(ctx context.Context) error {
			once.Do(func() { close(started) })
			<-ctx.Done()
			return ctx.Err()
		})
		run, _ := e.start(lifecycleStep(a, "restart"))
		waitSignal(t, started, "the connector call to start")
		e.spawner.cancel()
		e.settle()
		if got, steps := e.get(run.ID); got.State != RunRunning || !reflect.DeepEqual(stepStates(steps), []string{StepRunning}) {
			t.Fatalf("run = %+v, steps = %v; want it left running for recovery", got, stepStates(steps))
		}
		if alerts := e.alerts(a); len(alerts) != 0 {
			t.Fatalf("alerts after a shutdown = %+v, want none", alerts)
		}
	})

	t.Run("a connector failure alerts once", func(t *testing.T) {
		e := newEnv(t)
		a := useRealLifecycle(t, e, func(context.Context) error { return errors.New("upstream refused") })
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepFailed || !strings.Contains(steps[0].Error, "upstream refused") {
			t.Fatalf("run = %+v, step = %+v; want a failed step", got, steps[0])
		}
		alerts := e.alerts(a)
		if len(alerts) != 1 || alerts[0].Severity != "critical" {
			t.Fatalf("alerts = %+v, want exactly one critical alert", alerts)
		}
	})
}

// arrangeRun creates a run of the given steps without starting it.
func (e *env) arrangeRun(steps ...*store.RunbookStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	e.t.Helper()
	book, saved := e.runbook(steps...)
	run, frozen, err := e.s.CreateRunbookRun(context.Background(), book.ID, e.starter, FreezeSteps(saved))
	if err != nil {
		e.t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	return run, frozen
}

func TestNoPathLeavesARunRunningWithNothingDrivingIt(t *testing.T) {
	ctx := context.Background()

	t.Run("a step left running is failed unknown and resumable", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.arrangeRun(lifecycleStep(a, "restart"))
		if _, err := e.s.UpdateRunbookRunStep(ctx, run.ID, frozen[0].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.s.UpdateRunbookRun(ctx, run.ID, RunRunning, map[string]any{"state": RunFailed, "reason": ReasonInternalError}); err != nil {
			t.Fatal(err)
		}

		if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); err != nil {
			t.Fatalf("Resume() error: %v", err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown}) {
			t.Fatalf("run = %+v, steps = %v; want failed internal_error with the step unknown", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 0 {
			t.Fatalf("lifecycle calls = %+v, want none: the step must not run twice", calls)
		}
		notes := e.notes.snapshot()
		if len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunFailed || !strings.Contains(notes[0].Message, "outcome is unknown") {
			t.Fatalf("notifications = %+v, want one failure saying the outcome is unknown", notes)
		}

		if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); err != nil {
			t.Fatalf("second Resume() error: %v", err)
		}
		e.settle()
		got, steps = e.get(run.ID)
		if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded}) || len(e.lifecycle.snapshot()) != 1 {
			t.Fatalf("run = %+v, steps = %v, lifecycle calls = %d; want the step run once and the run succeeded", got, stepStates(steps), len(e.lifecycle.snapshot()))
		}
	})

	t.Run("a manual step that cannot pause fails a running run", func(t *testing.T) {
		e := newEnv(t)
		e.exec = e.newExecutor(&hookStore{Store: e.s, pause: func(string) error { return store.ErrConflict }})
		run, _ := e.start(manualStep("Check"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError || !reflect.DeepEqual(stepStates(steps), []string{StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want failed internal_error", got, stepStates(steps))
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunFailed {
			t.Fatalf("notifications = %+v, want one failure", notes)
		}
	})

	t.Run("a manual step that cannot pause leaves a cancelled run cancelled", func(t *testing.T) {
		e := newEnv(t)
		e.exec = e.newExecutor(&hookStore{Store: e.s, pause: func(runID string) error {
			// Cancelled in the store only, so the goroutine still runs.
			if err := e.s.CancelRunbookRun(ctx, runID, e.starter); err != nil {
				t.Errorf("CancelRunbookRun() error: %v", err)
			}
			return store.ErrConflict
		}})
		run, _ := e.start(manualStep("Check"))
		e.settle()
		if got, steps := e.get(run.ID); got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepSkipped}) {
			t.Fatalf("run = %+v, steps = %v; want it left cancelled", got, stepStates(steps))
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications = %+v, want none for a cancelled run", notes)
		}
	})

	t.Run("a conflict recording the last step fails a running run", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		var conflicts atomic.Int32
		e.exec = e.newExecutor(&hookStore{Store: e.s, finish: func(string) error {
			if conflicts.Add(1) == 1 {
				return store.ErrConflict
			}
			return nil
		}})
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown}) {
			t.Fatalf("run = %+v, steps = %v; want failed internal_error with the executed step unknown", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want 1", len(calls))
		}
		notes := e.notes.snapshot()
		if len(notes) != 1 || !strings.Contains(notes[0].Message, "outcome is unknown") {
			t.Fatalf("notifications = %+v, want one saying the outcome is unknown", notes)
		}
	})

	t.Run("a failed step write still fails the run", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec = e.newExecutor(&hookStore{Store: e.s, failStep: func(string) error { return errors.New("database is unavailable") }})
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error { return errors.New("boom") }
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		e.settle()
		got, _ := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError {
			t.Fatalf("run = %+v, want failed internal_error rather than running", got)
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want 1", len(calls))
		}
	})
}

// lastRunEvent returns the newest run-level event.
func lastRunEvent(t *testing.T, events *eventRecorder) Event {
	t.Helper()
	all := events.snapshot()
	for i := len(all) - 1; i >= 0; i-- {
		if all[i].Event.Step == nil {
			return all[i].Event
		}
	}
	t.Fatal("no run-level event was published")
	return Event{}
}

func TestShutdownRefusesStartResumeAndConfirm(t *testing.T) {
	ctx := context.Background()

	assertInterrupted := func(t *testing.T, e *env, runID string, notesBefore int) {
		t.Helper()
		got, _ := e.get(runID)
		if got.State != RunFailed || got.Reason != ReasonInterrupted {
			t.Fatalf("run = %+v, want failed with reason interrupted", got)
		}
		if ev := lastRunEvent(t, e.events); ev.RunID != runID || ev.State != RunFailed || ev.Reason != ReasonInterrupted {
			t.Fatalf("last run event = %+v, want failed/interrupted", ev)
		}
		if notes := e.notes.snapshot(); len(notes) != notesBefore {
			t.Fatalf("notifications = %+v, want %d: none for a refused request", notes, notesBefore)
		}
	}

	t.Run("start", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		book, saved := e.runbook(lifecycleStep(a, "restart"))
		e.spawner.refuse()
		run, steps, err := e.exec.Start(ctx, book.ID, e.starter, saved)
		if !errors.Is(err, ErrShuttingDown) || run != nil || steps != nil {
			t.Fatalf("Start() = %v, %v, %v; want ErrShuttingDown and no run", run, steps, err)
		}
		runID := e.events.snapshot()[0].Event.RunID
		assertInterrupted(t, e, runID, 0)
		if calls := e.lifecycle.snapshot(); len(calls) != 0 {
			t.Fatalf("lifecycle calls = %+v, want none", calls)
		}

		e.spawner.accept()
		if _, _, err := e.exec.Resume(ctx, runID, e.starter, ResumeNone); err != nil {
			t.Fatalf("Resume() error: %v", err)
		}
		e.settle()
		if got, _ := e.get(runID); got.State != RunSucceeded || len(e.lifecycle.snapshot()) != 1 {
			t.Fatalf("run = %+v, lifecycle calls = %d; want succeeded after the resume", got, len(e.lifecycle.snapshot()))
		}
	})

	t.Run("resume", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.lifecycle.fn = func(_ context.Context, n int, _ lifecycleCall) error {
			if n == 1 {
				return errors.New("boom")
			}
			return nil
		}
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		before := len(e.notes.snapshot())

		e.spawner.refuse()
		if resumed, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); !errors.Is(err, ErrShuttingDown) || resumed != nil {
			t.Fatalf("Resume() = %v, %v; want ErrShuttingDown and no run", resumed, err)
		}
		assertInterrupted(t, e, run.ID, before)
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want only the first attempt", len(calls))
		}

		e.spawner.accept()
		if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); err != nil {
			t.Fatalf("second Resume() error: %v", err)
		}
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded || len(e.lifecycle.snapshot()) != 2 {
			t.Fatalf("run = %+v, lifecycle calls = %d; want succeeded", got, len(e.lifecycle.snapshot()))
		}
	})

	t.Run("confirm", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.start(manualStep("Check"), lifecycleStep(a, "restart"))
		e.settle()
		before := len(e.notes.snapshot())

		e.spawner.refuse()
		if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, e.starter); !errors.Is(err, ErrShuttingDown) {
			t.Fatalf("Confirm() = %v, want ErrShuttingDown", err)
		}
		assertInterrupted(t, e, run.ID, before)
		_, steps := e.get(run.ID)
		if steps[0].State != StepSucceeded || steps[0].ConfirmedBy != e.starter || steps[1].State != StepPending || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("steps = %+v / %+v, lifecycle calls = %d; want the manual step confirmed and nothing after it run", steps[0], steps[1], len(e.lifecycle.snapshot()))
		}

		e.spawner.accept()
		if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); err != nil {
			t.Fatalf("Resume() error: %v", err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunSucceeded || steps[0].ConfirmedBy != e.starter || len(e.lifecycle.snapshot()) != 1 {
			t.Fatalf("run = %+v, manual step = %+v, lifecycle calls = %d; want the step after the confirmed one run", got, steps[0], len(e.lifecycle.snapshot()))
		}
	})
}

func TestStepTimeoutOnlyWhenTheStepDeadlineExpired(t *testing.T) {
	t.Run("a connector's own deadline error is a step failure", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error {
			return fmt.Errorf("proxmox: request: %w", context.DeadlineExceeded)
		}
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		want := "proxmox: request: context deadline exceeded"
		if got.State != RunFailed || got.Reason != ReasonStepFailed || steps[0].Error != want {
			t.Fatalf("run = %+v, step error = %q; want step_failed with %q", got, steps[0].Error, want)
		}
	})

	t.Run("a syncer deadline error before the step deadline is a step failure", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return time.Hour }
		e.sync.fn = func(context.Context, int) (*syncengine.RunResult, error) {
			return nil, fmt.Errorf("fetch: %w", context.DeadlineExceeded)
		}
		run, _ := e.start(syncStep(a))
		e.settle()
		got, steps := e.get(run.ID)
		want := "fetch: context deadline exceeded"
		if got.State != RunFailed || got.Reason != ReasonStepFailed || steps[0].Error != want {
			t.Fatalf("run = %+v, step error = %q; want step_failed with %q", got, steps[0].Error, want)
		}
	})
}

func TestStoreGrantsOperator(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()
	grants := StoreGrants{Store: e.s}

	actor, ok, err := grants.Operator(ctx, e.starter, a)
	if err != nil || !ok || actor.UserID != e.starter {
		t.Fatalf("Operator() = %+v, %v, %v; want the starter as operator", actor, ok, err)
	}
	if err := e.s.UpdateUser(ctx, e.starter, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	actor, ok, err = grants.Operator(ctx, e.starter, a)
	if err != nil || ok || actor != (connectors.LifecycleActor{}) {
		t.Fatalf("Operator() for a disabled user = %+v, %v, %v; want no actor, ok=false, no error", actor, ok, err)
	}
}

func TestPermissionDeniedNotifiesTheActingUser(t *testing.T) {
	ctx := context.Background()

	t.Run("the starter, then the resumer", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec.grants = fixedGrants{}
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		resumer := e.operator(a)
		if _, _, err := e.exec.Resume(ctx, run.ID, resumer, ResumeNone); err != nil {
			t.Fatal(err)
		}
		e.settle()
		notes := e.notes.snapshot()
		if len(notes) != 2 || notes[0].ActorID != e.starter || notes[1].ActorID != resumer {
			t.Fatalf("notifications = %+v, want actors %s then %s", notes, e.starter, resumer)
		}
		for _, n := range notes {
			if n.EventType != notifications.EventRunbookRunFailed || n.ConnectorID != a {
				t.Fatalf("notification = %+v, want a failure on connector %s", n, a)
			}
		}
	})

	t.Run("other outcomes carry no actor", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()

		// step_failed
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error { return errors.New("boom") }
		failed, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		if got, _ := e.get(failed.ID); got.Reason != ReasonStepFailed {
			t.Fatalf("run = %+v, want step_failed", got)
		}
		e.lifecycle.fn = nil

		// step_timeout
		e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return 20 * time.Millisecond }
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		timedOut, _ := e.start(syncStep(a))
		e.settle()
		if got, _ := e.get(timedOut.ID); got.Reason != ReasonStepTimeout {
			t.Fatalf("run = %+v, want step_timeout", got)
		}

		// internal_error
		e.exec = e.newExecutor(&faultyStore{Store: e.s, failStepState: StepSucceeded})
		internal, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		e.settle()
		if got, _ := e.get(internal.ID); got.Reason != ReasonInternalError {
			t.Fatalf("run = %+v, want internal_error", got)
		}

		// waiting
		e.exec = e.newExecutor(e.s)
		e.start(manualStep("Check"))
		e.settle()

		notes := e.notes.snapshot()
		if len(notes) != 4 {
			t.Fatalf("notifications = %+v, want 4", notes)
		}
		for _, n := range notes {
			if n.ActorID != "" {
				t.Fatalf("notification = %+v, want no actor", n)
			}
		}
	})

	t.Run("recovery carries no actor", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.arrangeRun(lifecycleStep(a, "restart"))
		if _, err := e.s.UpdateRunbookRunStep(ctx, run.ID, frozen[0].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
			t.Fatal(err)
		}
		if n, err := Recover(ctx, e.s, e.events, e.notes); err != nil || n != 1 {
			t.Fatalf("Recover() = %d, %v", n, err)
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].ActorID != "" {
			t.Fatalf("notifications = %+v, want one without an actor", notes)
		}
	})
}

// blockingNotifier holds the first notification until released.
type blockingNotifier struct {
	*noteRecorder
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	open    sync.Once
}

func newBlockingNotifier() *blockingNotifier {
	return &blockingNotifier{noteRecorder: &noteRecorder{}, entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *blockingNotifier) NotifyRunbookRun(ctx context.Context, eventType, severity, connectorID, actorID, title, message string) {
	b.once.Do(func() {
		close(b.entered)
		select {
		case <-b.release:
		case <-time.After(testWait):
		}
	})
	b.noteRecorder.NotifyRunbookRun(ctx, eventType, severity, connectorID, actorID, title, message)
}

func (b *blockingNotifier) unblock() { b.open.Do(func() { close(b.release) }) }

// ordering is one scenario of TestResumeAndConfirmPublishAfterThePreviousGoroutineStopped.
type ordering struct {
	// op is the store operation the follow-up signals: resume or confirm.
	op string
	// setup starts a run and returns it with its steps and the connector it
	// uses. The first notification must come from its goroutine.
	setup func(e *env) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, string)
	// follow is the resume or confirm.
	follow func(e *env, run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord) error
	// held is how many events the old goroutine has published when it is held.
	held int
	want []string
}

func TestResumeAndConfirmPublishAfterThePreviousGoroutineStopped(t *testing.T) {
	ctx := context.Background()

	// The old goroutine is held inside its notification, after it published its
	// events and stored its final state. The follow-up's own store write
	// signals when it has returned, and the test then checks that nothing was
	// published for the follow-up while the old goroutine is still alive. The
	// check can only fail when the follow-up publishes early; it cannot fail
	// spuriously, but it can pass when the follow-up is merely slow to publish,
	// so it is a regression detector rather than a proof. The final order
	// assertion is deterministic.
	check := func(t *testing.T, tc ordering) {
		e := newEnv(t)
		stored := make(chan struct{})
		var storedOnce sync.Once
		e.exec = e.newExecutor(&hookStore{Store: e.s, after: func(op string) {
			if op == tc.op {
				storedOnce.Do(func() { close(stored) })
			}
		}})
		blocker := newBlockingNotifier()
		t.Cleanup(blocker.unblock)
		e.exec.notifier = blocker
		started, steps, connectorID := tc.setup(e)
		select {
		case <-blocker.entered:
		case <-time.After(testWait):
			t.Fatal("the first notification was not sent")
		}
		if got := e.events.snapshot(); len(got) != tc.held {
			t.Fatalf("events before the follow-up = %d, want %d", len(got), tc.held)
		}

		errc := make(chan error, 1)
		go func() { errc <- tc.follow(e, started, steps) }()
		waitSignal(t, stored, "the follow-up's store write")
		if got := e.events.snapshot(); len(got) != tc.held {
			t.Fatalf("events while the previous goroutine is still stopping = %d, want %d: the follow-up published early", len(got), tc.held)
		}

		blocker.unblock()
		select {
		case err := <-errc:
			if err != nil {
				t.Fatalf("follow-up error: %v", err)
			}
		case <-time.After(testWait):
			t.Fatal("the follow-up did not return")
		}
		e.settle()
		got := describe(e.events.snapshot(), map[string]string{connectorID: "A"})
		if len(got) < len(tc.want) || !reflect.DeepEqual(got[:len(tc.want)], tc.want) {
			t.Fatalf("events = %q, want them to start with %q", got, tc.want)
		}
	}

	t.Run("resume", func(t *testing.T) {
		check(t, ordering{
			op: "resume",
			setup: func(e *env) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, string) {
				a := e.connector()
				e.lifecycle.fn = func(_ context.Context, n int, _ lifecycleCall) error {
					if n == 1 {
						return errors.New("boom")
					}
					return nil
				}
				run, steps := e.start(lifecycleStep(a, "restart"))
				return run, steps, a
			},
			follow: func(e *env, run *store.RunbookRunRecord, _ []*store.RunbookRunStepRecord) error {
				_, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone)
				return err
			},
			held: 4,
			want: []string{
				"all: run running", "A: step 0 running (run running)", "A: step 0 failed (run failed)", "all: run failed",
				"all: run running", "A: step 0 running (run running)", "A: step 0 succeeded (run succeeded)", "all: run succeeded",
			},
		})
	})

	t.Run("confirm", func(t *testing.T) {
		check(t, ordering{
			op: "confirm",
			setup: func(e *env) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, string) {
				a := e.connector()
				run, steps := e.start(manualStep("Check"), lifecycleStep(a, "restart"))
				return run, steps, a
			},
			follow: func(e *env, run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord) error {
				return e.exec.Confirm(ctx, run.ID, steps[0].ID, e.starter)
			},
			held: 3,
			want: []string{
				"all: run running", "all: step 0 waiting (run waiting_manual)", "all: run waiting_manual",
				"all: step 0 succeeded (run running)", "all: run running", "A: step 1 running (run running)",
				"A: step 1 succeeded (run succeeded)", "all: run succeeded",
			},
		})
	})
}

var _ Notifier = (*blockingNotifier)(nil)

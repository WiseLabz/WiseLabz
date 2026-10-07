package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// eventually polls cond until it holds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testWait)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// describe renders recorded events compactly: the audience ("all" or the
// connector's alias), then what changed.
func describe(events []published, aliases map[string]string) []string {
	out := make([]string, 0, len(events))
	for _, p := range events {
		audience := "all"
		if p.ConnectorID != "" {
			audience = aliases[p.ConnectorID]
		}
		if p.Event.Step == nil {
			out = append(out, fmt.Sprintf("%s: run %s", audience, p.Event.State))
			continue
		}
		out = append(out, fmt.Sprintf("%s: step %d %s (run %s)", audience, p.Event.Step.Position, p.Event.Step.State, p.Event.State))
	}
	return out
}

func TestRunAllStepsSucceed(t *testing.T) {
	e := newEnv(t)
	a, b := e.connector(), e.connector()
	book, saved := e.runbook(lifecycleStep(a, "stop"), syncStep(a), healthStep(b), lifecycleStep(b, "start"))
	run, frozen, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if run.State != RunRunning || run.StartedBy != e.starter || len(frozen) != 4 {
		t.Fatalf("started run = %+v with %d steps", run, len(frozen))
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunSucceeded || got.Reason != "" || got.FinishedAt == "" {
		t.Fatalf("run = %+v, want succeeded with an end time", got)
	}
	// Every transition is stamped, strictly after the one before it.
	previous := got.StartedAt
	for i, step := range steps {
		if step.State != StepSucceeded {
			t.Fatalf("step %d state = %q, want succeeded", i, step.State)
		}
		if step.StartedAt <= previous || step.FinishedAt <= step.StartedAt {
			t.Fatalf("step %d times = started %q, finished %q after %q; want strictly increasing", i, step.StartedAt, step.FinishedAt, previous)
		}
		previous = step.FinishedAt
	}
	if got.FinishedAt != previous || got.UpdatedAt != previous {
		t.Fatalf("run finished %q / updated %q, want the last step's end %q", got.FinishedAt, got.UpdatedAt, previous)
	}

	// Lifecycle steps act as the starter and carry the run and step in the
	// audit detail.
	wantCalls := []lifecycleCall{
		{ConnectorID: a, Verb: "stop", EntityRef: "100", Actor: connectors.LifecycleActor{UserID: e.starter},
			Audit: map[string]any{"runId": run.ID, "stepId": frozen[0].ID, "runbookId": book.ID}},
		{ConnectorID: b, Verb: "start", EntityRef: "100", Actor: connectors.LifecycleActor{UserID: e.starter},
			Audit: map[string]any{"runId": run.ID, "stepId": frozen[3].ID, "runbookId": book.ID}},
	}
	if calls := e.lifecycle.snapshot(); !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("lifecycle calls = %+v, want %+v", calls, wantCalls)
	}
	if e.sync.count() != 1 || e.health.count() != 1 {
		t.Fatalf("sync calls = %d, health calls = %d, want 1 each", e.sync.count(), e.health.count())
	}
	if notes := e.notes.snapshot(); len(notes) != 0 {
		t.Fatalf("notifications on success = %+v, want none", notes)
	}

	wantEvents := []string{
		"all: run running",
		"A: step 0 running (run running)", "A: step 0 succeeded (run running)",
		"A: step 1 running (run running)", "A: step 1 succeeded (run running)",
		"B: step 2 running (run running)", "B: step 2 succeeded (run running)",
		"B: step 3 running (run running)", "B: step 3 succeeded (run succeeded)",
		"all: run succeeded",
	}
	events := e.events.snapshot()
	if got := describe(events, map[string]string{a: "A", b: "B"}); !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("events = %q, want %q", got, wantEvents)
	}
	for _, p := range events {
		if p.Type != ws.EventRunbookRunUpdated || p.Event.RunID != run.ID || p.Event.RunbookID != book.ID {
			t.Fatalf("event = %+v, want %s for run %s", p, ws.EventRunbookRunUpdated, run.ID)
		}
	}
}

func TestFailureHaltsRunAndLeavesLaterStepsPending(t *testing.T) {
	e := newEnv(t)
	a := e.connector()
	e.lifecycle.fn = func(_ context.Context, n int, _ lifecycleCall) error {
		if n == 2 {
			return errors.New("connection refused")
		}
		return nil
	}
	book, saved := e.runbook(lifecycleStep(a, "stop"), lifecycleStep(a, "restart"), lifecycleStep(a, "start"), lifecycleStep(a, "stop"))
	run, _, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		t.Fatal(err)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunFailed || got.Reason != ReasonStepFailed || got.FinishedAt != "" {
		t.Fatalf("run = %+v, want failed with reason %s and no end time", got, ReasonStepFailed)
	}
	if states := stepStates(steps); !reflect.DeepEqual(states, []string{StepSucceeded, StepFailed, StepPending, StepPending}) {
		t.Fatalf("step states = %v", states)
	}
	failed := steps[1]
	if failed.Error != "connection refused" || failed.StartedAt == "" || failed.FinishedAt <= failed.StartedAt || failed.FinishedAt != got.UpdatedAt {
		t.Fatalf("failed step = %+v, run updated %q; want the error and the run failing in the same instant", failed, got.UpdatedAt)
	}
	for _, later := range steps[2:] {
		if later.StartedAt != "" || later.FinishedAt != "" {
			t.Fatalf("pending step has times: %+v", later)
		}
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 2 {
		t.Fatalf("lifecycle calls = %d, want 2: later steps must not execute", len(calls))
	}

	notes := e.notes.snapshot()
	if len(notes) != 1 {
		t.Fatalf("notifications = %+v, want one", notes)
	}
	n := notes[0]
	if n.EventType != notifications.EventRunbookRunFailed || n.ConnectorID != a ||
		!strings.Contains(n.Title, book.Title) || !strings.Contains(n.Message, `Step 2 "restart"`) || !strings.Contains(n.Message, "connection refused") {
		t.Fatalf("failure notification = %+v, want the runbook, the failed step and the reason", n)
	}
	events := describe(e.events.snapshot(), map[string]string{a: "A"})
	if tail := events[len(events)-2:]; !reflect.DeepEqual(tail, []string{"A: step 1 failed (run failed)", "all: run failed"}) {
		t.Fatalf("last events = %q", tail)
	}
}

func TestSyncAndWaitStep(t *testing.T) {
	t.Run("following step waits for the sync", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) {
			if err := g.block(ctx); err != nil {
				return nil, err
			}
			return &syncengine.RunResult{Status: "success"}, nil
		}
		run, _ := e.start(syncStep(a), lifecycleStep(a, "restart"))
		g.waitEntered(t)
		if _, steps := e.get(run.ID); !reflect.DeepEqual(stepStates(steps), []string{StepRunning, StepPending}) {
			t.Fatalf("states during sync = %v", stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 0 {
			t.Fatalf("lifecycle ran before the sync finished: %+v", calls)
		}
		close(g.release)
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded || len(e.lifecycle.snapshot()) != 1 {
			t.Fatalf("run = %+v, lifecycle calls = %d", got, len(e.lifecycle.snapshot()))
		}
	})

	t.Run("calls the syncer once under the step's deadline", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return time.Hour }
		var deadline time.Time
		var hasDeadline bool
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) {
			deadline, hasDeadline = ctx.Deadline()
			return &syncengine.RunResult{Status: "success"}, nil
		}
		started := time.Now()
		run, _ := e.start(syncStep(a))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded || e.sync.count() != 1 {
			t.Fatalf("run = %+v after %d sync calls, want succeeded after 1", got, e.sync.count())
		}
		if !hasDeadline || deadline.Before(started.Add(time.Hour)) || deadline.After(time.Now().Add(time.Hour)) {
			t.Fatalf("sync context deadline = %v (set %v), want the step's one hour timeout", deadline, hasDeadline)
		}
	})

	for name, tc := range map[string]struct {
		result *syncengine.RunResult
		err    error
		want   string
	}{
		"sync error":         {&syncengine.RunResult{Status: "error", Error: "fetch: boom"}, errors.New("fetch: boom"), "fetch: boom"},
		"disabled connector": {&syncengine.RunResult{Status: "skipped"}, nil, "connector is disabled"},
		"no result":          {nil, nil, "did not run"},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			a := e.connector()
			e.sync.fn = func(context.Context, int) (*syncengine.RunResult, error) { return tc.result, tc.err }
			run, _ := e.start(syncStep(a), lifecycleStep(a, "restart"))
			e.settle()
			got, steps := e.get(run.ID)
			if got.State != RunFailed || got.Reason != ReasonStepFailed || steps[0].State != StepFailed || !strings.Contains(steps[0].Error, tc.want) || steps[1].State != StepPending {
				t.Fatalf("run = %+v, steps = %+v / %+v; want the sync step failed with %q", got, steps[0], steps[1], tc.want)
			}
		})
	}

	t.Run("timeout", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return 20 * time.Millisecond }
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		run, _ := e.start(syncStep(a), lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepTimeout || steps[0].State != StepFailed || steps[1].State != StepPending {
			t.Fatalf("run = %+v, steps = %v; want a timed-out sync step", got, stepStates(steps))
		}
		if want := "Timed out after 20ms waiting for the sync to finish."; steps[0].Error != want {
			t.Fatalf("step error = %q, want %q", steps[0].Error, want)
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunFailed {
			t.Fatalf("notifications = %+v, want one failure", notes)
		}
	})

	t.Run("cancelled mid-wait", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) {
			return nil, g.block(ctx)
		}
		run, _ := e.start(syncStep(a), lifecycleStep(a, "restart"))
		g.waitEntered(t)
		canceller := e.operator(a)
		if err := e.exec.Cancel(context.Background(), run.ID, canceller); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		// The gate is never released: the sync returns only because its
		// context was cancelled.
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || got.CancelledBy == nil || *got.CancelledBy != canceller || got.FinishedAt == "" {
			t.Fatalf("run = %+v, want cancelled by %s", got, canceller)
		}
		if !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepSkipped}) {
			t.Fatalf("step states = %v, want the in-flight step unknown and the rest skipped", stepStates(steps))
		}
		if len(e.lifecycle.snapshot()) != 0 || len(e.notes.snapshot()) != 0 {
			t.Fatalf("after cancel: lifecycle calls %d, notifications %+v; want none", len(e.lifecycle.snapshot()), e.notes.snapshot())
		}
	})
}

func TestWaitUntilHealthyStep(t *testing.T) {
	t.Run("polls every 10 seconds by default", func(t *testing.T) {
		if HealthPollInterval != 10*time.Second || New(Deps{}).healthPollInterval != 10*time.Second {
			t.Fatalf("health poll interval = %s / %s, want 10s", HealthPollInterval, New(Deps{}).healthPollInterval)
		}
	})

	t.Run("succeeds once a check reports online", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.health.fn = func(_ context.Context, n int) (string, error) {
			return []string{"offline", "degraded", "online"}[n-1], nil
		}
		run, _ := e.start(healthStep(a), lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunSucceeded || steps[0].State != StepSucceeded || e.health.count() != 3 || len(e.lifecycle.snapshot()) != 1 {
			t.Fatalf("run = %+v after %d checks, want succeeded after 3 and the run continued", got, e.health.count())
		}
	})

	t.Run("timeout", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return 30 * time.Millisecond }
		e.health.fn = func(context.Context, int) (string, error) { return "offline", nil }
		run, _ := e.start(healthStep(a), lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepTimeout || steps[0].State != StepFailed || steps[1].State != StepPending {
			t.Fatalf("run = %+v, steps = %v; want a timed-out health step", got, stepStates(steps))
		}
		if want := "Timed out after 30ms waiting for the connector to report online (last status: offline)."; steps[0].Error != want {
			t.Fatalf("step error = %q, want %q", steps[0].Error, want)
		}
		if e.health.count() < 2 || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("health checks = %d, lifecycle calls = %d; want repeated polling and no later step", e.health.count(), len(e.lifecycle.snapshot()))
		}
	})

	t.Run("check error fails the step", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.health.fn = func(context.Context, int) (string, error) {
			return "", errors.New("load connector: resource not found")
		}
		run, _ := e.start(healthStep(a))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepFailed || steps[0].Error != "load connector: resource not found" {
			t.Fatalf("run = %+v, step = %+v", got, steps[0])
		}
	})

	t.Run("cancelled while polling", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.health.fn = func(context.Context, int) (string, error) { return "offline", nil }
		run, _ := e.start(healthStep(a), lifecycleStep(a, "restart"))
		eventually(t, "the health step to poll", func() bool { return e.health.count() >= 2 })
		if err := e.exec.Cancel(context.Background(), run.ID, e.starter); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		e.settle()
		polls := e.health.count()
		time.Sleep(20 * time.Millisecond)
		if e.health.count() != polls {
			t.Fatalf("polling continued after cancel: %d -> %d checks", polls, e.health.count())
		}
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepSkipped}) || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("run = %+v, steps = %v, lifecycle calls = %d", got, stepStates(steps), len(e.lifecycle.snapshot()))
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications on cancel = %+v, want none", notes)
		}
	})
}

func TestManualStepPausesUntilConfirmed(t *testing.T) {
	e := newEnv(t)
	a := e.connector()
	run, frozen := e.start(lifecycleStep(a, "stop"), manualStep("Check the rack"), lifecycleStep(a, "start"))
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunWaitingManual || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepWaiting, StepPending}) {
		t.Fatalf("run = %+v, steps = %v; want waiting on the manual step", got, stepStates(steps))
	}
	if steps[1].StartedAt == "" || steps[1].FinishedAt != "" || steps[1].StartedAt != got.UpdatedAt {
		t.Fatalf("waiting step = %+v, run updated %q; want the pause stamped once on both", steps[1], got.UpdatedAt)
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 1 {
		t.Fatalf("lifecycle calls while waiting = %d, want 1", len(calls))
	}
	notes := e.notes.snapshot()
	if len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunWaiting || notes[0].ConnectorID != "" ||
		!strings.Contains(notes[0].Title, got.RunbookTitle) || !strings.Contains(notes[0].Message, `Step 2 "Check the rack"`) {
		t.Fatalf("notifications = %+v, want one waiting notification naming the step", notes)
	}

	ctx := context.Background()
	// 409 cases: a step that is not waiting, and no acting user.
	if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("confirm a succeeded step = %v, want ErrConflict", err)
	}
	if err := e.exec.Confirm(ctx, run.ID, frozen[2].ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("confirm a pending step = %v, want ErrConflict", err)
	}
	if err := e.exec.Confirm(ctx, run.ID, frozen[1].ID, ""); !errors.Is(err, ErrNoActor) {
		t.Fatalf("confirm without a user = %v, want ErrNoActor", err)
	}
	if err := e.exec.Confirm(ctx, "missing", frozen[1].ID, e.starter); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("confirm on a missing run = %v, want ErrNotFound", err)
	}
	if got, _ := e.get(run.ID); got.State != RunWaitingManual {
		t.Fatalf("run after rejected confirmations = %+v, want still waiting", got)
	}

	// Another operator confirms; the run continues as the starter.
	other := e.operator(a)
	if err := e.exec.Confirm(ctx, run.ID, frozen[1].ID, other); err != nil {
		t.Fatalf("Confirm() error: %v", err)
	}
	e.settle()
	got, steps = e.get(run.ID)
	if got.State != RunSucceeded || steps[1].State != StepSucceeded || steps[1].ConfirmedBy != other || steps[1].FinishedAt == "" {
		t.Fatalf("run = %+v, manual step = %+v; want succeeded and confirmed by %s", got, steps[1], other)
	}
	calls := e.lifecycle.snapshot()
	if len(calls) != 2 || calls[1].Verb != "start" || calls[1].Actor.UserID != e.starter {
		t.Fatalf("lifecycle calls = %+v, want the last step run as the starter", calls)
	}
	if err := e.exec.Confirm(ctx, run.ID, frozen[1].ID, other); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("confirm twice = %v, want ErrConflict", err)
	}
	if notes := e.notes.snapshot(); len(notes) != 1 {
		t.Fatalf("notifications after success = %+v, want only the waiting one", notes)
	}
	events := describe(e.events.snapshot(), map[string]string{a: "A"})
	wantEvents := []string{
		"all: run running",
		"A: step 0 running (run running)", "A: step 0 succeeded (run running)",
		"all: step 1 waiting (run waiting_manual)", "all: run waiting_manual",
		"all: step 1 succeeded (run running)", "all: run running",
		"A: step 2 running (run running)", "A: step 2 succeeded (run succeeded)", "all: run succeeded",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("events = %q, want %q", events, wantEvents)
	}
}

func TestResumeRunsFromFirstStepNotSucceeded(t *testing.T) {
	e := newEnv(t)
	a := e.connector()
	e.lifecycle.fn = func(_ context.Context, n int, _ lifecycleCall) error {
		if n == 3 {
			return errors.New("boom")
		}
		return nil
	}
	run, frozen := e.start(lifecycleStep(a, "stop"), lifecycleStep(a, "restart"), lifecycleStep(a, "start"), lifecycleStep(a, "restart"))
	ctx := context.Background()
	resumer := e.operator(a)

	e.settle()
	failed, failedSteps := e.get(run.ID)
	if failed.State != RunFailed || !reflect.DeepEqual(stepStates(failedSteps), []string{StepSucceeded, StepSucceeded, StepFailed, StepPending}) {
		t.Fatalf("run = %+v, steps = %v; want failed on the third step", failed, stepStates(failedSteps))
	}
	if _, err := e.exec.Resume(ctx, run.ID, ""); !errors.Is(err, ErrNoActor) {
		t.Fatalf("resume without a user = %v, want ErrNoActor", err)
	}
	if _, err := e.exec.Resume(ctx, "missing", resumer); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("resume a missing run = %v, want ErrNotFound", err)
	}

	resumed, err := e.exec.Resume(ctx, run.ID, resumer)
	if err != nil {
		t.Fatalf("Resume() error: %v", err)
	}
	if resumed.State != RunRunning || resumed.Reason != "" || resumed.ResumedBy == nil || *resumed.ResumedBy != resumer {
		t.Fatalf("resumed run = %+v, want running and resumed by %s", resumed, resumer)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded, StepSucceeded, StepSucceeded}) {
		t.Fatalf("run = %+v, steps = %v; want succeeded", got, stepStates(steps))
	}
	// Steps one and two ran once; step three ran again, as the resuming user.
	calls := e.lifecycle.snapshot()
	var stepIDs []any
	for _, call := range calls {
		stepIDs = append(stepIDs, call.Audit["stepId"])
	}
	if want := []any{frozen[0].ID, frozen[1].ID, frozen[2].ID, frozen[2].ID, frozen[3].ID}; !reflect.DeepEqual(stepIDs, want) {
		t.Fatalf("executed steps = %v, want %v", stepIDs, want)
	}
	if calls[2].Actor.UserID != e.starter || calls[3].Actor.UserID != resumer || calls[4].Actor.UserID != resumer {
		t.Fatalf("actors = %q, %q, %q; want the starter then the resuming user", calls[2].Actor.UserID, calls[3].Actor.UserID, calls[4].Actor.UserID)
	}
	if steps[0].FinishedAt != failedSteps[0].FinishedAt || steps[1].FinishedAt != failedSteps[1].FinishedAt {
		t.Fatal("steps that had succeeded were touched by the resume")
	}
	if steps[2].Error != "" || steps[2].StartedAt <= failedSteps[2].FinishedAt {
		t.Fatalf("retried step = %+v, want a fresh start after %q and no error", steps[2], failedSteps[2].FinishedAt)
	}

	// 409: finished runs are not resumable.
	if _, err := e.exec.Resume(ctx, run.ID, resumer); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("resume a succeeded run = %v, want ErrConflict", err)
	}
}

func TestResumeRejectedOutsideFailedState(t *testing.T) {
	e := newEnv(t)
	a := e.connector()
	ctx := context.Background()

	// running
	g := newGate()
	e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
	running, _ := e.start(lifecycleStep(a, "restart"))
	g.waitEntered(t)
	if _, err := e.exec.Resume(ctx, running.ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("resume a running run = %v, want ErrConflict", err)
	}
	close(g.release)
	e.settle()

	// waiting_manual, then cancelled
	waiting, _ := e.start(manualStep("Check"))
	e.settle()
	if _, err := e.exec.Resume(ctx, waiting.ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("resume a waiting run = %v, want ErrConflict", err)
	}
	if err := e.exec.Cancel(ctx, waiting.ID, e.starter); err != nil {
		t.Fatal(err)
	}
	if _, err := e.exec.Resume(ctx, waiting.ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("resume a cancelled run = %v, want ErrConflict", err)
	}

	// expired
	expired, _ := e.start(manualStep("Check"))
	e.settle()
	old := time.Now().UTC().Add(-48 * time.Hour).Format("2006-01-02T15:04:05.000000000Z")
	if _, err := e.s.RawDB().ExecContext(ctx, `UPDATE runbook_runs SET updated_at = ? WHERE id = ?`, old, expired.ID); err != nil {
		t.Fatal(err)
	}
	if ids, err := e.s.ExpireOpenRunbookRuns(ctx, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339)); err != nil || len(ids) != 1 {
		t.Fatalf("ExpireOpenRunbookRuns() = %v, %v", ids, err)
	}
	if _, err := e.exec.Resume(ctx, expired.ID, e.starter); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("resume an expired run = %v, want ErrConflict", err)
	}
	if got, _ := e.get(expired.ID); got.State != RunExpired {
		t.Fatalf("expired run = %+v", got)
	}
}

func TestCancel(t *testing.T) {
	ctx := context.Background()

	t.Run("while waiting on a manual step", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.start(manualStep("Check"), lifecycleStep(a, "restart"))
		e.settle()
		canceller := e.operator(a)
		if err := e.exec.Cancel(ctx, run.ID, ""); !errors.Is(err, ErrNoActor) {
			t.Fatalf("cancel without a user = %v, want ErrNoActor", err)
		}
		if err := e.exec.Cancel(ctx, run.ID, canceller); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || got.CancelledBy == nil || *got.CancelledBy != canceller || !reflect.DeepEqual(stepStates(steps), []string{StepSkipped, StepSkipped}) {
			t.Fatalf("run = %+v, steps = %v; want cancelled with waiting and pending steps skipped", got, stepStates(steps))
		}
		events := describe(e.events.snapshot(), map[string]string{a: "A"})
		if tail := events[len(events)-3:]; !reflect.DeepEqual(tail, []string{"all: step 0 skipped (run cancelled)", "A: step 1 skipped (run cancelled)", "all: run cancelled"}) {
			t.Fatalf("cancel events = %q", tail)
		}
		// 409: a cancelled run cannot be cancelled, confirmed or resumed.
		if err := e.exec.Cancel(ctx, run.ID, canceller); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("cancel twice = %v, want ErrConflict", err)
		}
		if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, canceller); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("confirm a cancelled run = %v, want ErrConflict", err)
		}
		if _, err := e.exec.Resume(ctx, run.ID, canceller); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("resume a cancelled run = %v, want ErrConflict", err)
		}
		e.settle()
		if len(e.lifecycle.snapshot()) != 0 {
			t.Fatal("a step executed after cancel")
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunWaiting {
			t.Fatalf("notifications = %+v, want only the earlier waiting one", notes)
		}
	})

	t.Run("a failed run, keeping the failed step", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error { return errors.New("boom") }
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		e.settle()
		if err := e.exec.Cancel(ctx, run.ID, e.starter); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepFailed, StepSkipped}) || steps[0].Error != "boom" {
			t.Fatalf("run = %+v, steps = %v; want the failed step kept", got, stepStates(steps))
		}
	})

	t.Run("a finished or missing run", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		if err := e.exec.Cancel(ctx, run.ID, e.starter); !errors.Is(err, store.ErrConflict) {
			t.Fatalf("cancel a succeeded run = %v, want ErrConflict", err)
		}
		if err := e.exec.Cancel(ctx, "missing", e.starter); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("cancel a missing run = %v, want ErrNotFound", err)
		}
	})

	t.Run("mid-step discards a result that arrives afterwards", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		entered, release := make(chan struct{}), make(chan struct{})
		// This connector call ignores cancellation and reports success late.
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error {
			close(entered)
			<-release
			return nil
		}
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		<-entered
		if err := e.exec.Cancel(ctx, run.ID, e.starter); err != nil {
			t.Fatalf("Cancel() error: %v", err)
		}
		if _, steps := e.get(run.ID); !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepSkipped}) {
			t.Fatalf("steps right after cancel = %v, want the in-flight step unknown", stepStates(steps))
		}
		close(release)
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepSkipped}) {
			t.Fatalf("run = %+v, steps = %v; the late result must not rewrite a cancelled run", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want 1: no step after cancel", len(calls))
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications on cancel = %+v, want none", notes)
		}
	})
}

// fixedGrants is a Grants fake with a scripted answer.
type fixedGrants struct {
	actor connectors.LifecycleActor
	ok    bool
	err   error
}

func (g fixedGrants) Operator(context.Context, string, string) (connectors.LifecycleActor, bool, error) {
	return g.actor, g.ok, g.err
}

func TestGrantIsRecheckedBeforeEveryAutomatedStep(t *testing.T) {
	ctx := context.Background()

	t.Run("revoked mid-run fails the later step without calling the connector", func(t *testing.T) {
		e := newEnv(t)
		a, b := e.connector(), e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
		run, _ := e.start(lifecycleStep(a, "stop"), lifecycleStep(b, "restart"), lifecycleStep(a, "start"))
		g.waitEntered(t)
		// Revoked while the earlier step is running.
		if err := e.s.DeleteConnectorGrant(ctx, e.starter, b); err != nil {
			t.Fatal(err)
		}
		close(g.release)
		e.settle()

		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonPermissionDenied || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepFailed, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want failed on the revoked step", got, stepStates(steps))
		}
		if !strings.Contains(steps[1].Error, "operator grant") || steps[1].StartedAt == "" || steps[1].FinishedAt == "" {
			t.Fatalf("revoked step = %+v, want a permission reason and both times", steps[1])
		}
		calls := e.lifecycle.snapshot()
		if len(calls) != 1 || calls[0].ConnectorID != a {
			t.Fatalf("lifecycle calls = %+v, want only the first step: connector %s must not be touched", calls, b)
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].ConnectorID != b {
			t.Fatalf("notifications = %+v, want one for the revoked step", notes)
		}
	})

	for name, step := range map[string]func(string) *store.RunbookStepRecord{
		"sync step":   syncStep,
		"health step": healthStep,
	} {
		t.Run(name+" needs the grant too", func(t *testing.T) {
			e := newEnv(t)
			a := e.connector()
			if _, err := e.s.UpsertConnectorGrant(ctx, e.starter, a, "viewer"); err != nil {
				t.Fatal(err)
			}
			run, _ := e.start(step(a))
			e.settle()
			got, steps := e.get(run.ID)
			if got.State != RunFailed || got.Reason != ReasonPermissionDenied || steps[0].State != StepFailed {
				t.Fatalf("run = %+v, step = %+v; a viewer grant must not run the step", got, steps[0])
			}
			if e.sync.count() != 0 || e.health.count() != 0 {
				t.Fatalf("connector reached without an operator grant: sync %d, health %d", e.sync.count(), e.health.count())
			}
		})
	}

	t.Run("disabled user no longer acts", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		if err := e.s.UpdateUser(ctx, e.starter, map[string]any{"disabled": true}); err != nil {
			t.Fatal(err)
		}
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunFailed || got.Reason != ReasonPermissionDenied || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("run = %+v, lifecycle calls = %d; want a permission failure and no call", got, len(e.lifecycle.snapshot()))
		}
	})

	t.Run("instance admin flag reaches the audit actor", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		admin := apitest.NewUser(t, e.s, "operator")
		apitest.GrantConnectorRole(t, e.s, admin, a, "operator")
		book, saved := e.runbook(lifecycleStep(a, "restart"))
		if _, _, err := e.exec.Start(ctx, book.ID, admin, saved); err != nil {
			t.Fatal(err)
		}
		e.settle()
		calls := e.lifecycle.snapshot()
		if len(calls) != 1 || calls[0].Actor != (connectors.LifecycleActor{UserID: admin, InstanceAdmin: true}) {
			t.Fatalf("lifecycle calls = %+v, want the admin as actor", calls)
		}
	})

	// The lifecycle core does not check grants and would audit an empty actor
	// as the system, so none of these may reach it.
	for name, grants := range map[string]Grants{
		"empty actor":       fixedGrants{ok: true},
		"no grant":          fixedGrants{actor: connectors.LifecycleActor{UserID: "someone"}},
		"grant check error": fixedGrants{actor: connectors.LifecycleActor{UserID: "someone"}, ok: true, err: errors.New("db down")},
	} {
		t.Run(name+" never reaches the lifecycle core", func(t *testing.T) {
			e := newEnv(t)
			a := e.connector()
			e.exec.grants = grants
			run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
			e.settle()
			got, steps := e.get(run.ID)
			if got.State != RunFailed || got.Reason != ReasonPermissionDenied || !reflect.DeepEqual(stepStates(steps), []string{StepFailed, StepPending}) {
				t.Fatalf("run = %+v, steps = %v; want a permission failure", got, stepStates(steps))
			}
			if calls := e.lifecycle.snapshot(); len(calls) != 0 {
				t.Fatalf("lifecycle core was called: %+v", calls)
			}
			if strings.Contains(steps[0].Error, "db down") {
				t.Fatalf("step error leaks the internal error: %q", steps[0].Error)
			}
		})
	}

	t.Run("a run without an acting user never reaches the lifecycle core", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		book, saved := e.runbook(lifecycleStep(a, "restart"))
		if _, _, err := e.exec.Start(ctx, book.ID, "", saved); !errors.Is(err, ErrNoActor) {
			t.Fatalf("Start() without a user = %v, want ErrNoActor", err)
		}
		// A row written by something other than Start.
		run, _, err := e.s.CreateRunbookRun(ctx, book.ID, "", FreezeSteps(saved))
		if err != nil {
			t.Fatal(err)
		}
		e.exec.grants = fixedGrants{actor: connectors.LifecycleActor{UserID: "someone"}, ok: true}
		e.exec.spawn(run.ID)
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunFailed || got.Reason != ReasonPermissionDenied || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("run = %+v, lifecycle calls = %d; want a permission failure and no call", got, len(e.lifecycle.snapshot()))
		}
	})
}

func TestRunExecutesFrozenStepsAfterRunbookChanges(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	if _, err := e.s.RawDB().ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	a := e.connector()

	// Edited while the run waits: the run keeps the steps recorded at start.
	book, saved := e.runbook(manualStep("Check"), lifecycleStep(a, "restart"))
	run, frozen, err := e.exec.Start(ctx, book.ID, e.starter, saved)
	if err != nil {
		t.Fatal(err)
	}
	e.settle()
	if _, err := e.s.ReplaceRunbookSteps(ctx, book.ID, []*store.RunbookStepRecord{lifecycleStep(a, "stop"), lifecycleStep(a, "stop")}); err != nil {
		t.Fatal(err)
	}
	if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, e.starter); err != nil {
		t.Fatal(err)
	}
	e.settle()
	calls := e.lifecycle.snapshot()
	if got, steps := e.get(run.ID); got.State != RunSucceeded || len(steps) != 2 || len(calls) != 1 || calls[0].Verb != "restart" {
		t.Fatalf("run = %+v with %d steps, lifecycle calls = %+v; want the frozen restart only", got, len(steps), calls)
	}

	// Deleted with a finished run: the run stays readable by its id.
	if err := e.s.DeleteRunbook(ctx, book.ID); err != nil {
		t.Fatal(err)
	}
	kept, keptSteps := e.get(run.ID)
	if kept.RunbookID != nil || kept.RunbookTitle != book.Title || len(keptSteps) != 2 {
		t.Fatalf("run after runbook delete = %+v with %d steps; want the title and steps kept", kept, len(keptSteps))
	}

	// Deleted while a run waits: the run still finishes its frozen steps.
	other, otherSaved := e.runbook(manualStep("Check"), lifecycleStep(a, "start"))
	orphan, orphanSteps, err := e.exec.Start(ctx, other.ID, e.starter, otherSaved)
	if err != nil {
		t.Fatal(err)
	}
	e.settle()
	if err := e.s.DeleteRunbook(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.exec.Confirm(ctx, orphan.ID, orphanSteps[0].ID, e.starter); err != nil {
		t.Fatalf("Confirm() on a run of a deleted runbook: %v", err)
	}
	e.settle()
	got, _ := e.get(orphan.ID)
	calls = e.lifecycle.snapshot()
	if got.State != RunSucceeded || got.RunbookID != nil || len(calls) != 2 || calls[1].Verb != "start" {
		t.Fatalf("orphaned run = %+v, lifecycle calls = %+v; want it to finish its frozen steps", got, calls)
	}
	if want := map[string]any{"runId": orphan.ID, "stepId": orphanSteps[1].ID}; !reflect.DeepEqual(calls[1].Audit, want) {
		t.Fatalf("audit detail without a runbook = %v, want %v", calls[1].Audit, want)
	}
	events := e.events.snapshot()
	if last := events[len(events)-1].Event; last.RunID != orphan.ID || last.RunbookID != "" || last.State != RunSucceeded {
		t.Fatalf("last event = %+v, want the orphaned run succeeded without a runbook id", last)
	}
}

func TestOneActiveRunPerRunbook(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()
	g := newGate()
	e.lifecycle.fn = func(ctx context.Context, n int, _ lifecycleCall) error {
		if n == 1 {
			return g.block(ctx)
		}
		return errors.New("boom")
	}

	book, saved := e.runbook(lifecycleStep(a, "restart"))
	first, _, err := e.exec.Start(ctx, book.ID, e.starter, saved)
	if err != nil {
		t.Fatal(err)
	}
	g.waitEntered(t)

	// 409: a second run of the same runbook, naming the active one.
	_, _, err = e.exec.Start(ctx, book.ID, e.starter, saved)
	var conflict *store.RunbookRunConflictError
	if !errors.As(err, &conflict) || conflict.RunID != first.ID {
		t.Fatalf("second start = %v, want a conflict naming %s", err, first.ID)
	}
	if _, _, err := e.exec.Start(ctx, book.ID, e.starter, nil); !errors.Is(err, store.ErrRunbookRunStepCount) {
		t.Fatalf("start without steps = %v, want ErrRunbookRunStepCount", err)
	}
	if _, _, err := e.exec.Start(ctx, "missing", e.starter, saved); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("start a missing runbook = %v, want ErrNotFound", err)
	}

	// Another runbook runs at the same time, and fails.
	otherBook, otherSaved := e.runbook(lifecycleStep(a, "stop"))
	other, _, err := e.exec.Start(ctx, otherBook.ID, e.starter, otherSaved)
	if err != nil {
		t.Fatalf("concurrent run of another runbook: %v", err)
	}
	eventually(t, "the other run to fail", func() bool {
		got, _ := e.get(other.ID)
		return got.State == RunFailed
	})
	if got, _ := e.get(first.ID); got.State != RunRunning {
		t.Fatalf("first run = %+v, want still running", got)
	}
	// 409: a failed run still occupies its runbook.
	_, _, err = e.exec.Start(ctx, otherBook.ID, e.starter, otherSaved)
	if !errors.As(err, &conflict) || conflict.RunID != other.ID {
		t.Fatalf("start over a failed run = %v, want a conflict naming %s", err, other.ID)
	}

	close(g.release)
	e.settle()
	// A finished run frees the runbook.
	if _, _, err := e.exec.Start(ctx, book.ID, e.starter, saved); err != nil {
		t.Fatalf("start after the first run finished: %v", err)
	}
	e.settle()
}

func TestShutdownLeavesRunForRecovery(t *testing.T) {
	t.Run("an interrupted step is not recorded as failed", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		g.waitEntered(t)
		e.spawner.cancel()
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunRunning || !reflect.DeepEqual(stepStates(steps), []string{StepRunning, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want it left running for startup recovery", got, stepStates(steps))
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications at shutdown = %+v, want none", notes)
		}
	})

	t.Run("a step that completes during shutdown is recorded", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		entered, release := make(chan struct{}), make(chan struct{})
		e.lifecycle.fn = func(context.Context, int, lifecycleCall) error {
			close(entered)
			<-release
			return nil
		}
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		<-entered
		e.spawner.cancel()
		close(release)
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunRunning || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want the finished step kept and nothing further started", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want 1", len(calls))
		}
	})
}

// faultyStore fails selected step writes.
type faultyStore struct {
	*store.Store
	failStepState string
}

func (f *faultyStore) UpdateRunbookRunStep(ctx context.Context, runID, stepID, expectedState string, updates map[string]any) (*store.RunbookRunStepRecord, error) {
	if updates["state"] == f.failStepState {
		return nil, errors.New("database is unavailable")
	}
	return f.Store.UpdateRunbookRunStep(ctx, runID, stepID, expectedState, updates)
}

func TestStoreFailureStopsRunWithoutLeavingItRunning(t *testing.T) {
	t.Run("a result that cannot be recorded leaves the step unknown", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec = e.newExecutor(&faultyStore{Store: e.s, failStepState: StepSucceeded})
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want failed with the executed step unknown", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("lifecycle calls = %d, want 1", len(calls))
		}
		notes := e.notes.snapshot()
		if len(notes) != 1 || !strings.Contains(notes[0].Message, "outcome is unknown") {
			t.Fatalf("notifications = %+v, want one saying the outcome is unknown", notes)
		}
	})

	t.Run("a step that cannot be started fails the run before any call", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.exec = e.newExecutor(&faultyStore{Store: e.s, failStepState: StepRunning})
		run, _ := e.start(lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonInternalError || steps[0].State != StepPending || len(e.lifecycle.snapshot()) != 0 {
			t.Fatalf("run = %+v, steps = %v, lifecycle calls = %d", got, stepStates(steps), len(e.lifecycle.snapshot()))
		}
		notes := e.notes.snapshot()
		if len(notes) != 1 || notes[0].ConnectorID != "" || !strings.Contains(notes[0].Message, "internal error") {
			t.Fatalf("notifications = %+v, want one run-level failure", notes)
		}
	})
}

func TestStepHelpers(t *testing.T) {
	// Frozen steps drop the authored identity. A step without a kind is a
	// lifecycle step; only the wait kinds keep a timeout, and legacy rows hold
	// the column default of 300 on every kind.
	frozen := FreezeSteps([]*store.RunbookStepRecord{
		{ID: "authored", RunbookID: "book", Position: 7, Kind: KindLifecycle, Title: "Restart", ConnectorID: "c", Verb: "restart", EntityRef: "100", TimeoutSeconds: 300},
		nil,
		{Title: "Legacy", ConnectorID: "c", Verb: "stop", TimeoutSeconds: 300},
		{Kind: KindManual, Title: "Check", TimeoutSeconds: 300},
		{Kind: KindSyncAndWait, Title: "Sync", ConnectorID: "c", TimeoutSeconds: 60},
		{Kind: KindWaitUntilHealthy, Title: "Wait", ConnectorID: "c"},
		{Kind: "config_push", Title: "Configure", ConnectorID: "c", EntityRef: "100", FieldKey: "enabled", TargetValue: `false`},
		{Kind: "wait_for_entity", Title: "Observe", ConnectorID: "c", EntityRef: "100", Attribute: "status", Operator: "eq", ExpectedValue: `"running"`},
	})
	want := []*store.RunbookRunStepRecord{
		{Kind: KindLifecycle, Title: "Restart", ConnectorID: "c", Verb: "restart", EntityRef: "100"},
		nil,
		{Kind: KindLifecycle, Title: "Legacy", ConnectorID: "c", Verb: "stop"},
		{Kind: KindManual, Title: "Check"},
		{Kind: KindSyncAndWait, Title: "Sync", ConnectorID: "c", TimeoutSeconds: 60},
		{Kind: KindWaitUntilHealthy, Title: "Wait", ConnectorID: "c", TimeoutSeconds: 300},
		{Kind: "config_push", Title: "Configure", ConnectorID: "c", EntityRef: "100", FieldKey: "enabled", TargetValue: `false`},
		{Kind: "wait_for_entity", Title: "Observe", ConnectorID: "c", EntityRef: "100", Attribute: "status", Operator: "eq", ExpectedValue: `"running"`},
	}
	if !reflect.DeepEqual(frozen, want) {
		t.Fatalf("FreezeSteps() = %+v, want %+v", frozen, want)
	}

	for _, tc := range []struct {
		step *store.RunbookRunStepRecord
		want time.Duration
	}{
		{&store.RunbookRunStepRecord{Kind: KindSyncAndWait, TimeoutSeconds: 10}, 10 * time.Second},
		{&store.RunbookRunStepRecord{Kind: KindWaitUntilHealthy, TimeoutSeconds: 1800}, 30 * time.Minute},
		{&store.RunbookRunStepRecord{Kind: KindSyncAndWait}, 5 * time.Minute},
		{&store.RunbookRunStepRecord{Kind: KindWaitUntilHealthy}, 5 * time.Minute},
		{&store.RunbookRunStepRecord{Kind: KindLifecycle}, 0},
		{&store.RunbookRunStepRecord{Kind: KindLifecycle, TimeoutSeconds: 300}, 0},
		{&store.RunbookRunStepRecord{Kind: KindManual, TimeoutSeconds: 300}, 0},
	} {
		if got := StepTimeout(tc.step); got != tc.want {
			t.Errorf("StepTimeout(%s, %ds) = %s, want %s", tc.step.Kind, tc.step.TimeoutSeconds, got, tc.want)
		}
	}

	resumer := "resumer"
	empty := ""
	if got := ActingUser(&store.RunbookRunRecord{StartedBy: "starter"}); got != "starter" {
		t.Errorf("ActingUser(started) = %q", got)
	}
	if got := ActingUser(&store.RunbookRunRecord{StartedBy: "starter", ResumedBy: &empty}); got != "starter" {
		t.Errorf("ActingUser(empty resumer) = %q", got)
	}
	if got := ActingUser(&store.RunbookRunRecord{StartedBy: "starter", ResumedBy: &resumer}); got != "resumer" {
		t.Errorf("ActingUser(resumed) = %q", got)
	}
}

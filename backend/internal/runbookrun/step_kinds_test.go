package runbookrun

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/compliance"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

func TestConfigPushStep(t *testing.T) {
	ctx := context.Background()

	t.Run("writes the frozen value as the acting user and audits the run", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.start(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
		e.settle()

		got, steps := e.get(run.ID)
		if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) {
			t.Fatalf("run = %+v, steps = %v; want both succeeded", got, stepStates(steps))
		}
		calls := e.push.snapshot()
		if len(calls) != 1 {
			t.Fatalf("config push calls = %+v, want 1", calls)
		}
		call := calls[0]
		if call.ConnectorID != a || call.EntityRef != "100" || call.FieldKey != "memory" || call.Value != float64(4096) {
			t.Fatalf("call = %+v, want connector %s entity 100 memory=4096", call, a)
		}
		if call.Actor.UserID != e.starter {
			t.Fatalf("actor = %+v, want the starter %s", call.Actor, e.starter)
		}
		if call.Audit["runId"] != run.ID || call.Audit["stepId"] != frozen[0].ID || call.Audit["stepIndex"] != frozen[0].Position {
			t.Fatalf("audit detail = %v, want the run id, step id and step index", call.Audit)
		}
		if len(e.lifecycle.snapshot()) != 1 {
			t.Fatal("the step after the push did not run")
		}
	})

	t.Run("decodes every JSON value shape", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.start(configPushStep(a, "enabled", `true`), configPushStep(a, "mode", `"fast"`))
		e.settle()
		calls := e.push.snapshot()
		if len(calls) != 2 || calls[0].Value != true || calls[1].Value != "fast" {
			t.Fatalf("calls = %+v, want true then \"fast\"", calls)
		}
	})

	t.Run("a runbook edit after start changes nothing", func(t *testing.T) {
		e := newEnv(t)
		if _, err := e.s.RawDB().ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
			t.Fatal(err)
		}
		a := e.connector()
		book, saved := e.runbook(manualStep("Check"), configPushStep(a, "memory", `4096`))
		run, frozen, err := e.exec.Start(ctx, book.ID, e.starter, saved)
		if err != nil {
			t.Fatal(err)
		}
		e.settle()
		edited := configPushStep(a, "memory", `1024`)
		edited.EntityRef = "200"
		edited.FieldKey = "cores"
		if _, err := e.s.ReplaceRunbookSteps(ctx, book.ID, []*store.RunbookStepRecord{manualStep("Check"), edited}); err != nil {
			t.Fatal(err)
		}
		if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, e.starter); err != nil {
			t.Fatal(err)
		}
		e.settle()
		calls := e.push.snapshot()
		if got, _ := e.get(run.ID); got.State != RunSucceeded || len(calls) != 1 {
			t.Fatalf("run = %+v, calls = %+v", got, calls)
		}
		if calls[0].Value != float64(4096) || calls[0].EntityRef != "100" || calls[0].FieldKey != "memory" {
			t.Fatalf("call = %+v, want the values recorded at start", calls[0])
		}
	})

	t.Run("a core that finds the field already at target succeeds", func(t *testing.T) {
		// The core returns nil without writing or auditing; the step is a plain success.
		e := newEnv(t)
		a := e.connector()
		run, _ := e.start(configPushStep(a, "memory", `4096`))
		e.settle()
		if got, steps := e.get(run.ID); got.State != RunSucceeded || steps[0].State != StepSucceeded || steps[0].Error != "" {
			t.Fatalf("run = %+v, step = %+v", got, steps[0])
		}
	})

	t.Run("a verify mismatch fails the step and the run", func(t *testing.T) {
		for name, mismatch := range map[string]*connectors.ConfigPushMismatchError{
			"reverted":       {RevertAttempted: true},
			"previous value": {},
		} {
			t.Run(name, func(t *testing.T) {
				e := newEnv(t)
				a := e.connector()
				e.push.fn = func(context.Context, int, configPushCall) error { return mismatch }
				run, _ := e.start(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
				e.settle()
				got, steps := e.get(run.ID)
				if got.State != RunFailed || got.Reason != ReasonStepFailed || !reflect.DeepEqual(stepStates(steps), []string{StepFailed, StepPending}) {
					t.Fatalf("run = %+v, steps = %v; want the push and the run failed", got, stepStates(steps))
				}
				if !strings.Contains(steps[0].Error, "verify") {
					t.Fatalf("step error = %q, want the mismatch described", steps[0].Error)
				}
				if len(e.lifecycle.snapshot()) != 0 {
					t.Fatal("a step ran after the failed push")
				}
			})
		}
	})

	t.Run("a withdrawn field fails the step", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.push.fn = func(context.Context, int, configPushCall) error {
			return errors.New(`field "memory" is not writable for this connector`)
		}
		run, _ := e.start(configPushStep(a, "memory", `4096`))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || steps[0].State != StepFailed || !strings.Contains(steps[0].Error, "not writable") {
			t.Fatalf("run = %+v, step = %+v; want a failure naming the withdrawn field", got, steps[0])
		}
	})

	t.Run("a revoked grant fails the step without writing", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		book, saved := e.runbook(manualStep("Check"), configPushStep(a, "memory", `4096`))
		run, frozen, err := e.exec.Start(ctx, book.ID, e.starter, saved)
		if err != nil {
			t.Fatal(err)
		}
		e.settle()
		if err := e.s.DeleteConnectorGrant(ctx, e.starter, a); err != nil {
			t.Fatal(err)
		}
		if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, e.starter); err != nil {
			t.Fatal(err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonPermissionDenied || steps[1].State != StepFailed {
			t.Fatalf("run = %+v, steps = %v; want a permission failure", got, stepStates(steps))
		}
		if calls := e.push.snapshot(); len(calls) != 0 {
			t.Fatalf("config push calls = %+v, want none", calls)
		}
	})

	t.Run("resuming an unknown step runs it again", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		book, saved := e.runbook(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
		// State left by a process that stopped mid-push.
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
		if _, steps := e.get(run.ID); steps[0].State != StepUnknown {
			t.Fatalf("step = %s, want unknown", steps[0].State)
		}
		if _, err := e.exec.Resume(ctx, run.ID, e.starter); err != nil {
			t.Fatal(err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		calls := e.push.snapshot()
		if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) || len(calls) != 1 || calls[0].Value != float64(4096) {
			t.Fatalf("run = %+v, steps = %v, calls = %+v; want the push executed once more", got, stepStates(steps), calls)
		}
	})

	t.Run("a cancel after the write began does not abandon the core", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		seen := make(chan error, 1)
		e.push.fn = func(ctx context.Context, _ int, _ configPushCall) error {
			g.entered <- struct{}{}
			<-g.release
			seen <- ctx.Err()
			return nil
		}
		run, _ := e.start(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
		g.waitEntered(t)
		if err := e.exec.Cancel(ctx, run.ID, e.starter); err != nil {
			t.Fatal(err)
		}
		close(g.release)
		e.settle()
		select {
		case err := <-seen:
			if err != nil {
				t.Fatalf("the core's context ended with %v after the cancel; the write, verification and revert must finish", err)
			}
		case <-time.After(testWait):
			t.Fatal("the core did not finish")
		}
		if got, _ := e.get(run.ID); got.State != RunCancelled {
			t.Fatalf("run = %+v, want cancelled", got)
		}
		if len(e.lifecycle.snapshot()) != 0 {
			t.Fatal("a step ran after the cancel")
		}
	})

	t.Run("a push finishing during shutdown is recorded", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		entered, release := make(chan struct{}), make(chan struct{})
		seen := make(chan error, 1)
		e.push.fn = func(ctx context.Context, _ int, _ configPushCall) error {
			close(entered)
			<-release
			seen <- ctx.Err()
			return nil
		}
		run, _ := e.start(configPushStep(a, "memory", `4096`), lifecycleStep(a, "restart"))
		<-entered
		e.spawner.cancel()
		close(release)
		e.settle()
		if err := <-seen; err != nil {
			t.Fatalf("the core's context ended with %v at shutdown", err)
		}
		got, steps := e.get(run.ID)
		if got.State != RunRunning || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want the finished push kept and nothing further started", got, stepStates(steps))
		}
	})

	t.Run("a step cancelled before the push starts does not write", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		step := &store.RunbookRunStepRecord{Kind: KindConfigPush, ConnectorID: a, EntityRef: "100", FieldKey: "memory", TargetValue: `1`}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		err := e.exec.performConfigPush(cancelled, &store.RunbookRunRecord{ID: "run"}, step, connectors.LifecycleActor{UserID: "u"})
		if !errors.Is(err, context.Canceled) || len(e.push.snapshot()) != 0 {
			t.Fatalf("err = %v, calls = %+v; want a cancel and no write", err, e.push.snapshot())
		}
	})

	t.Run("a malformed frozen value fails the step", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, _ := e.start(configPushStep(a, "memory", `{not json`))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || steps[0].State != StepFailed || len(e.push.snapshot()) != 0 {
			t.Fatalf("run = %+v, step = %+v", got, steps[0])
		}
	})
}

// entityOf is a snapshot holding the entity "100" with one attribute.
func entityOf(attribute string, value any) *compliance.Snapshot {
	return &compliance.Snapshot{Entities: []compliance.Entity{
		{Kind: "vm", Name: "other", ExternalID: "200", Attributes: map[string]any{attribute: "other"}},
		{Kind: "vm", Name: "guest", ExternalID: "100", Attributes: map[string]any{attribute: value}},
	}}
}

// shortTimeout makes every step time out after d.
func shortTimeout(e *env, d time.Duration) {
	e.exec.stepTimeout = func(*store.RunbookRunStepRecord) time.Duration { return d }
}

func TestWaitForEntityStep(t *testing.T) {
	ctx := context.Background()

	t.Run("holds on the first sync", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("status", "running"), nil }
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`), lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) {
			t.Fatalf("run = %+v, steps = %v", got, stepStates(steps))
		}
		if e.sync.count() != 1 || e.entities.count() != 1 {
			t.Fatalf("syncs = %d, loads = %d; want one of each", e.sync.count(), e.entities.count())
		}
		if len(e.lifecycle.snapshot()) != 1 {
			t.Fatal("the step after the wait did not run")
		}
	})

	t.Run("holds on the third sync", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.entities.fn = func(_ context.Context, n int) (*compliance.Snapshot, error) {
			if n < 3 {
				return entityOf("status", "stopped"), nil
			}
			return entityOf("status", "running"), nil
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded {
			t.Fatalf("run = %+v, want succeeded", got)
		}
		if e.sync.count() != 3 {
			t.Fatalf("syncs = %d, want 3", e.sync.count())
		}
	})

	t.Run("an absent entity may appear later", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.entities.fn = func(_ context.Context, n int) (*compliance.Snapshot, error) {
			switch n {
			case 1:
				return nil, nil
			case 2:
				return &compliance.Snapshot{Entities: []compliance.Entity{{ExternalID: "200"}}}, nil
			default:
				return entityOf("status", "running"), nil
			}
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded || e.sync.count() != 3 {
			t.Fatalf("run = %+v, syncs = %d; want succeeded on the third", got, e.sync.count())
		}
	})

	t.Run("the supported operators use the compliance rules", func(t *testing.T) {
		for _, tc := range []struct {
			op       string
			expected string
			value    any
		}{
			{"eq", `2`, 2},
			{"neq", `"stopped"`, "running"},
			{"contains", `"run"`, "running"},
			{"regex", `"^run+ing$"`, "running"},
			{"gt", `1`, 2},
			{"lt", `3`, 2},
		} {
			e := newEnv(t)
			a := e.connector()
			e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("status", tc.value), nil }
			run, _ := e.start(waitEntityStep(a, "status", tc.op, tc.expected))
			e.settle()
			if got, _ := e.get(run.ID); got.State != RunSucceeded {
				t.Errorf("%s %s: run = %+v, want succeeded", tc.op, tc.expected, got)
			}
		}
	})

	t.Run("timeout reports the last value", func(t *testing.T) {
		e := newEnv(t)
		shortTimeout(e, 100*time.Millisecond)
		a := e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("status", "starting"), nil }
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`), lifecycleStep(a, "restart"))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepTimeout || !reflect.DeepEqual(stepStates(steps), []string{StepFailed, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want a timed out wait", got, stepStates(steps))
		}
		if !strings.Contains(steps[0].Error, `status = "starting"`) || !strings.Contains(steps[0].Error, "status eq running") {
			t.Fatalf("step error = %q, want the condition and the last value", steps[0].Error)
		}
	})

	t.Run("timeout with the entity never found", func(t *testing.T) {
		e := newEnv(t)
		shortTimeout(e, 100*time.Millisecond)
		a := e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) {
			return &compliance.Snapshot{Entities: []compliance.Entity{{ExternalID: "200"}}}, nil
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepTimeout || !strings.Contains(steps[0].Error, "entity not found") {
			t.Fatalf("run = %+v, step = %+v; want a timeout saying the entity was not found", got, steps[0])
		}
	})

	t.Run("timeout with the attribute missing", func(t *testing.T) {
		e := newEnv(t)
		shortTimeout(e, 100*time.Millisecond)
		a := e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("other", "x"), nil }
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if _, steps := e.get(run.ID); !strings.Contains(steps[0].Error, "attribute status not present") {
			t.Fatalf("step error = %q, want the missing attribute", steps[0].Error)
		}
	})

	t.Run("a failed sync is only not yet", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.sync.fn = func(_ context.Context, n int) (*syncengine.RunResult, error) {
			switch n {
			case 1:
				return entityResult("success"), nil
			case 2:
				return nil, errors.New("connector unreachable")
			case 3:
				return &syncengine.RunResult{Status: "failed", Error: "boom"}, nil
			default:
				return entityResult("success"), nil
			}
		}
		e.entities.fn = func(_ context.Context, n int) (*compliance.Snapshot, error) {
			if n < 2 {
				return entityOf("status", "stopped"), nil
			}
			return entityOf("status", "running"), nil
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded {
			t.Fatalf("run = %+v, want succeeded after the failed syncs", got)
		}
		if e.sync.count() != 4 || e.entities.count() != 2 {
			t.Fatalf("syncs = %d, loads = %d; want 4 and 2: a failed sync must not be evaluated", e.sync.count(), e.entities.count())
		}
	})

	t.Run("a failed entity read is only not yet", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		e.entities.fn = func(_ context.Context, n int) (*compliance.Snapshot, error) {
			if n == 1 {
				return nil, errors.New("database is busy")
			}
			return entityOf("status", "running"), nil
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded {
			t.Fatalf("run = %+v, want succeeded", got)
		}
	})

	t.Run("cancel stops the wait between syncs", func(t *testing.T) {
		e := newEnv(t)
		e.exec.entityPollInterval = time.Hour
		a := e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("status", "stopped"), nil }
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		eventually(t, "the first poll", func() bool { return e.entities.count() == 1 })
		if err := e.exec.Cancel(ctx, run.ID, e.starter); err != nil {
			t.Fatal(err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunCancelled || steps[0].State != StepUnknown {
			t.Fatalf("run = %+v, step = %+v; want cancelled", got, steps[0])
		}
		if e.sync.count() != 1 {
			t.Fatalf("syncs = %d, want no sync after the cancel", e.sync.count())
		}
	})

	t.Run("cancel stops a wait inside a sync", func(t *testing.T) {
		e := newEnv(t)
		g := newGate()
		e.sync.fn = func(ctx context.Context, _ int) (*syncengine.RunResult, error) { return nil, g.block(ctx) }
		a := e.connector()
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		g.waitEntered(t)
		if err := e.exec.Cancel(ctx, run.ID, e.starter); err != nil {
			t.Fatal(err)
		}
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunCancelled || e.sync.count() != 1 {
			t.Fatalf("run = %+v, syncs = %d", got, e.sync.count())
		}
	})

	t.Run("a revoked grant fails the wait without syncing", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		if err := e.s.DeleteConnectorGrant(ctx, e.starter, a); err != nil {
			t.Fatal(err)
		}
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunFailed || got.Reason != ReasonPermissionDenied || e.sync.count() != 0 {
			t.Fatalf("run = %+v, syncs = %d", got, e.sync.count())
		}
	})

	t.Run("a condition the wait does not support fails the step", func(t *testing.T) {
		for _, step := range []*store.RunbookStepRecord{
			waitEntityStep("", "status", "days_left_lt", `10`),
			waitEntityStep("", "status", "eq", `{bad`),
			waitEntityStep("", "", "eq", `"x"`),
		} {
			e := newEnv(t)
			step.ConnectorID = e.connector()
			run, _ := e.start(step)
			e.settle()
			got, steps := e.get(run.ID)
			if got.State != RunFailed || got.Reason != ReasonStepFailed || steps[0].State != StepFailed || e.sync.count() != 0 {
				t.Errorf("%s %q: run = %+v, step = %+v, syncs = %d", step.Operator, step.ExpectedValue, got, steps[0], e.sync.count())
			}
		}
	})

	t.Run("the observation is truncated and secrets are withheld", func(t *testing.T) {
		e := newEnv(t)
		shortTimeout(e, 50*time.Millisecond)
		a := e.connector()
		long := strings.Repeat("v", 500)
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("status", long), nil }
		run, _ := e.start(waitEntityStep(a, "status", "eq", `"running"`))
		e.settle()
		_, steps := e.get(run.ID)
		if strings.Contains(steps[0].Error, long) || !strings.Contains(steps[0].Error, "…") || len(steps[0].Error) > 400 {
			t.Fatalf("step error = %q, want the value truncated", steps[0].Error)
		}

		e = newEnv(t)
		shortTimeout(e, 50*time.Millisecond)
		a = e.connector()
		e.entities.fn = func(context.Context, int) (*compliance.Snapshot, error) { return entityOf("api_token", "hunter2"), nil }
		run, _ = e.start(waitEntityStep(a, "api_token", "eq", `"x"`))
		e.settle()
		_, steps = e.get(run.ID)
		if strings.Contains(steps[0].Error, "hunter2") || !strings.Contains(steps[0].Error, "<redacted>") {
			t.Fatalf("step error = %q, want the secret withheld", steps[0].Error)
		}
	})
}

func entityResult(status string) *syncengine.RunResult {
	return &syncengine.RunResult{Status: status}
}

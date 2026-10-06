package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// raceIterations is how often a scenario with more than one legal order runs,
// each time on a fresh environment, so both orders get exercised.
const raceIterations = 10

// repeat runs fn as raceIterations subtests.
func repeat(t *testing.T, fn func(t *testing.T)) {
	t.Helper()
	for i := 1; i <= raceIterations; i++ {
		t.Run(fmt.Sprintf("iteration %d", i), fn)
	}
}

// race runs every fn in its own goroutine, released together from a closed
// channel, and returns when all have returned. Results go through variables the
// fns write, which race's return orders before the caller's reads.
func race(fns ...func()) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			fn()
		}()
	}
	close(start)
	wg.Wait()
}

// outcomes counts how often each legal order was observed.
type outcomes map[string]int

func (o outcomes) log(t *testing.T) {
	t.Helper()
	t.Logf("observed outcomes over %d iterations: %v", raceIterations, map[string]int(o))
}

// assertNoOpenSteps fails when a step of a finished run is still pending,
// running or waiting.
func assertNoOpenSteps(t *testing.T, steps []*store.RunbookRunStepRecord) {
	t.Helper()
	for i, step := range steps {
		switch step.State {
		case StepPending, StepRunning, StepWaiting:
			t.Fatalf("step %d is %s in a finished run; steps = %v", i, step.State, stepStates(steps))
		}
	}
}

func assertNoFailureNotes(t *testing.T, e *env) {
	t.Helper()
	for _, n := range e.notes.snapshot() {
		if n.EventType == notifications.EventRunbookRunFailed {
			t.Fatalf("failure notification sent: %+v", n)
		}
	}
}

func verbs(calls []lifecycleCall) []string {
	out := make([]string, len(calls))
	for i, call := range calls {
		out[i] = call.Verb
	}
	return out
}

func TestSimultaneousStartHasExactlyOneWinner(t *testing.T) {
	const starters = 8
	repeat(t, func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
		book, saved := e.runbook(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))

		runs := make([]*store.RunbookRunRecord, starters)
		errs := make([]error, starters)
		fns := make([]func(), starters)
		for i := range fns {
			fns[i] = func() { runs[i], _, errs[i] = e.exec.Start(context.Background(), book.ID, e.starter, saved) }
		}
		race(fns...)
		// Every Start has returned while the winner's step is still held.

		var winner *store.RunbookRunRecord
		for i := range runs {
			if errs[i] != nil {
				continue
			}
			if winner != nil {
				t.Fatalf("two Start calls succeeded: %s and %s", winner.ID, runs[i].ID)
			}
			winner = runs[i]
		}
		if winner == nil {
			t.Fatalf("no Start succeeded: %v", errs)
		}
		for i, err := range errs {
			var conflict *store.RunbookRunConflictError
			if runs[i] == winner {
				continue
			}
			if !errors.As(err, &conflict) || conflict.RunID != winner.ID {
				t.Fatalf("losing Start %d = %v, want a conflict naming %s", i, err, winner.ID)
			}
		}

		close(g.release)
		e.settle()
		if got, _ := e.get(winner.ID); got.State != RunSucceeded {
			t.Fatalf("winner run = %+v, want succeeded", got)
		}
		if calls := e.lifecycle.snapshot(); !reflect.DeepEqual(verbs(calls), []string{"restart", "start"}) {
			t.Fatalf("connector calls = %v, want one per step", verbs(calls))
		}
		all, total, err := e.s.ListRunbookRuns(context.Background(), book.ID, 50, 0)
		if err != nil || total != 1 || len(all) != 1 || all[0].ID != winner.ID {
			t.Fatalf("runs of the runbook = %d (total %d), err %v; want only %s", len(all), total, err, winner.ID)
		}
	})
}

func TestSimultaneousResumeHasExactlyOneWinner(t *testing.T) {
	const resumers = 4
	repeat(t, func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, n int, _ lifecycleCall) error {
			switch n {
			case 1:
				return errors.New("boom")
			case 2:
				return g.block(ctx)
			}
			return nil
		}
		run, frozen := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunFailed {
			t.Fatalf("run = %+v, want failed", got)
		}

		users := make([]string, resumers)
		errs := make([]error, resumers)
		fns := make([]func(), resumers)
		for i := range fns {
			users[i] = e.operator(a)
			fns[i] = func() { _, errs[i] = e.exec.Resume(context.Background(), run.ID, users[i]) }
		}
		race(fns...)

		winner := ""
		for i, err := range errs {
			switch {
			case err == nil && winner != "":
				t.Fatalf("two Resume calls succeeded: %s and %s", winner, users[i])
			case err == nil:
				winner = users[i]
			case !errors.Is(err, store.ErrConflict):
				t.Fatalf("losing Resume %d = %v, want ErrConflict", i, err)
			}
		}
		if winner == "" {
			t.Fatalf("no Resume succeeded: %v", errs)
		}

		waitSignal(t, g.entered, "the resumed step to start")
		calls := e.lifecycle.snapshot()
		if len(calls) != 2 || calls[1].Actor.UserID != winner {
			t.Fatalf("calls = %+v, want the resumed call made as %s", calls, winner)
		}
		if got, _ := e.get(run.ID); got.State != RunRunning || got.ResumedBy == nil || *got.ResumedBy != winner {
			t.Fatalf("run = %+v, want running and resumed by %s", got, winner)
		}

		close(g.release)
		e.settle()
		got, _ := e.get(run.ID)
		if got.State != RunSucceeded || got.ResumedBy == nil || *got.ResumedBy != winner {
			t.Fatalf("run = %+v, want succeeded, resumed by %s", got, winner)
		}
		var stepIDs []any
		for _, call := range e.lifecycle.snapshot() {
			stepIDs = append(stepIDs, call.Audit["stepId"])
		}
		if want := []any{frozen[0].ID, frozen[0].ID, frozen[1].ID}; !reflect.DeepEqual(stepIDs, want) {
			t.Fatalf("executed steps = %v, want the failed one, then the retry, then the next: %v", stepIDs, want)
		}
	})
}

func TestConfirmRacingCancel(t *testing.T) {
	seen := outcomes{}
	repeat(t, func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		// Never released: the step ends only through its context.
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
		run, frozen := e.start(manualStep("Check"), lifecycleStep(a, "restart"))
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunWaitingManual {
			t.Fatalf("run = %+v, want waiting", got)
		}
		confirmer, canceller := e.operator(a), e.operator(a)

		var confirmErr, cancelErr error
		race(
			func() { confirmErr = e.exec.Confirm(context.Background(), run.ID, frozen[0].ID, confirmer) },
			func() { cancelErr = e.exec.Cancel(context.Background(), run.ID, canceller) },
		)
		e.settle()

		if cancelErr != nil {
			t.Fatalf("Cancel() = %v, want nil", cancelErr)
		}
		got, steps := e.get(run.ID)
		if got.State != RunCancelled {
			t.Fatalf("run = %+v, want cancelled", got)
		}
		assertNoOpenSteps(t, steps)
		calls := e.lifecycle.snapshot()
		switch {
		case errors.Is(confirmErr, store.ErrConflict):
			seen["cancel first"]++
			if !reflect.DeepEqual(stepStates(steps), []string{StepSkipped, StepSkipped}) || len(calls) != 0 {
				t.Fatalf("steps = %v, calls = %d; want both skipped and no call", stepStates(steps), len(calls))
			}
		case confirmErr == nil:
			seen["confirm first"]++
			if steps[0].State != StepSucceeded || steps[0].ConfirmedBy != confirmer {
				t.Fatalf("manual step = %+v, want succeeded and confirmed by %s", steps[0], confirmer)
			}
			if s := steps[1].State; s != StepSkipped && s != StepUnknown {
				t.Fatalf("lifecycle step is %s, want skipped or unknown", s)
			}
			if len(calls) > 1 {
				t.Fatalf("connector calls = %d, want at most 1", len(calls))
			}
		default:
			t.Fatalf("Confirm() = %v, want nil or ErrConflict", confirmErr)
		}
		assertNoFailureNotes(t, e)
	})
	seen.log(t)
}

func TestCancelRacingStepCompletion(t *testing.T) {
	seen := outcomes{}
	repeat(t, func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, n int, _ lifecycleCall) error {
			if n == 1 {
				return g.block(ctx)
			}
			return nil
		}
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		waitSignal(t, g.entered, "the first step to start")

		var cancelErr error
		race(
			func() { close(g.release) },
			func() { cancelErr = e.exec.Cancel(context.Background(), run.ID, e.starter) },
		)
		e.settle()

		got, steps := e.get(run.ID)
		assertNoOpenSteps(t, steps)
		switch {
		case cancelErr == nil:
			if got.State != RunCancelled {
				t.Fatalf("run = %+v, want cancelled", got)
			}
			if s := steps[0].State; s != StepSucceeded && s != StepUnknown {
				t.Fatalf("first step is %s, want succeeded or unknown", s)
			}
			if s := steps[1].State; s != StepSkipped && s != StepUnknown {
				t.Fatalf("second step is %s, want skipped or unknown", s)
			}
			seen[fmt.Sprintf("cancelled, steps %v", stepStates(steps))]++
		case errors.Is(cancelErr, store.ErrConflict):
			if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) {
				t.Fatalf("run = %+v, steps = %v; want everything succeeded after a lost cancel", got, stepStates(steps))
			}
			seen["step completion first"]++
		default:
			t.Fatalf("Cancel() = %v, want nil or ErrConflict", cancelErr)
		}
		calls := e.lifecycle.snapshot()
		if len(calls) > 2 || (len(calls) == 2 && calls[0].Verb == calls[1].Verb) {
			t.Fatalf("connector calls = %v, want at most one per step", verbs(calls))
		}
		if notes := e.notes.snapshot(); len(notes) != 0 {
			t.Fatalf("notifications = %+v, want none", notes)
		}
	})
	seen.log(t)
}

func TestDoubleSpawnRunsEachStepOnce(t *testing.T) {
	t.Run("spawned again while the run is active", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		waitSignal(t, g.entered, "the first step to start")
		for i := 0; i < 2; i++ {
			if !e.exec.spawn(run.ID) {
				t.Fatal("spawn() = false, want the work accepted")
			}
		}
		close(g.release)
		e.settle()
		if got, _ := e.get(run.ID); got.State != RunSucceeded {
			t.Fatalf("run = %+v, want succeeded", got)
		}
		if calls := e.lifecycle.snapshot(); !reflect.DeepEqual(verbs(calls), []string{"restart", "start"}) {
			t.Fatalf("connector calls = %v, want one per step", verbs(calls))
		}
	})

	t.Run("spawned together on a run nobody drives", func(t *testing.T) {
		repeat(t, func(t *testing.T) {
			e := newEnv(t)
			a := e.connector()
			run, _ := e.arrangeRun(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
			accepted := make([]bool, 4)
			fns := make([]func(), len(accepted))
			for i := range fns {
				fns[i] = func() { accepted[i] = e.exec.spawn(run.ID) }
			}
			race(fns...)
			if !reflect.DeepEqual(accepted, []bool{true, true, true, true}) {
				t.Fatalf("spawn results = %v, want all accepted", accepted)
			}
			e.settle()
			if got, _ := e.get(run.ID); got.State != RunSucceeded {
				t.Fatalf("run = %+v, want succeeded", got)
			}
			if calls := e.lifecycle.snapshot(); !reflect.DeepEqual(verbs(calls), []string{"restart", "start"}) {
				t.Fatalf("connector calls = %v, want one per step", verbs(calls))
			}
		})
	})

	t.Run("spawned again on a run that then fails", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		g := newGate()
		e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error {
			if err := g.block(ctx); err != nil {
				return err
			}
			return errors.New("boom")
		}
		run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
		waitSignal(t, g.entered, "the first step to start")
		for i := 0; i < 3; i++ {
			if !e.exec.spawn(run.ID) {
				t.Fatal("spawn() = false, want the work accepted")
			}
		}
		close(g.release)
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonStepFailed || !reflect.DeepEqual(stepStates(steps), []string{StepFailed, StepPending}) {
			t.Fatalf("run = %+v, steps = %v; want failed on the first step", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 1 {
			t.Fatalf("connector calls = %d, want exactly the one failing call", len(calls))
		}
		if notes := e.notes.snapshot(); len(notes) != 1 || notes[0].EventType != notifications.EventRunbookRunFailed {
			t.Fatalf("notifications = %+v, want exactly one failure", notes)
		}
	})
}

func TestStoreGrantsOperatorForADeletedUser(t *testing.T) {
	ctx := context.Background()

	t.Run("directly", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		user := e.operator(a)
		grants := StoreGrants{Store: e.s}
		if _, ok, err := grants.Operator(ctx, user, a); err != nil || !ok {
			t.Fatalf("Operator() before the delete = %v, %v; want ok", ok, err)
		}
		if err := e.s.DeleteUser(ctx, user); err != nil {
			t.Fatal(err)
		}
		actor, ok, err := grants.Operator(ctx, user, a)
		if err != nil || ok || actor != (connectors.LifecycleActor{}) {
			t.Fatalf("Operator() for a deleted user = %+v, %v, %v; want no actor, ok=false, no error", actor, ok, err)
		}
	})

	t.Run("a run whose starter was deleted", func(t *testing.T) {
		e := newEnv(t)
		a := e.connector()
		run, frozen := e.start(manualStep("Check"), lifecycleStep(a, "restart"))
		e.settle()
		other := e.operator(a)
		if err := e.s.DeleteUser(ctx, e.starter); err != nil {
			t.Fatal(err)
		}
		if err := e.exec.Confirm(ctx, run.ID, frozen[0].ID, other); err != nil {
			t.Fatalf("Confirm() error: %v", err)
		}
		e.settle()
		got, steps := e.get(run.ID)
		if got.State != RunFailed || got.Reason != ReasonPermissionDenied || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepFailed}) {
			t.Fatalf("run = %+v, steps = %v; want failed with permission_denied on the lifecycle step", got, stepStates(steps))
		}
		if calls := e.lifecycle.snapshot(); len(calls) != 0 {
			t.Fatalf("connector calls = %+v, want none", calls)
		}
	})
}

func TestLifecycleStepIgnoresAStoredTimeout(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()
	book, _ := e.runbook(lifecycleStep(a, "restart"))
	// FreezeSteps drops a lifecycle step's timeout, so the row is built by hand.
	run, frozen, err := e.s.CreateRunbookRun(ctx, book.ID, e.starter, []*store.RunbookRunStepRecord{
		{Kind: KindLifecycle, Title: "Restart", ConnectorID: a, Verb: "restart", EntityRef: "100", TimeoutSeconds: 300},
	})
	if err != nil {
		t.Fatalf("CreateRunbookRun() error: %v", err)
	}
	if _, steps := e.get(run.ID); len(steps) != 1 || steps[0].TimeoutSeconds != 300 || frozen[0].TimeoutSeconds != 300 {
		t.Fatalf("stored step = %+v, want timeout 300", steps)
	}

	hasDeadline := true
	e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error {
		_, hasDeadline = ctx.Deadline()
		return nil
	}
	if !e.exec.spawn(run.ID) {
		t.Fatal("spawn() = false")
	}
	e.settle()
	if hasDeadline {
		t.Fatal("the lifecycle call's context has a deadline, want none: only wait steps time out")
	}
	if got, _ := e.get(run.ID); got.State != RunSucceeded {
		t.Fatalf("run = %+v, want succeeded", got)
	}
}

// activeRuns is the number of runs with a goroutine holding their slot.
func activeRuns(e *env) int {
	e.exec.mu.Lock()
	defer e.exec.mu.Unlock()
	return len(e.exec.active)
}

// startQueued starts a run of two lifecycle steps whose first call returns only
// through its context, waits until that call is in flight and queues two more
// goroutines for the run behind the slot holder. Whether they have reached the
// slot wait yet is not observable; the assertions hold either way.
func startQueued(t *testing.T, e *env) *store.RunbookRunRecord {
	t.Helper()
	a := e.connector()
	g := newGate()
	// Never released.
	e.lifecycle.fn = func(ctx context.Context, _ int, _ lifecycleCall) error { return g.block(ctx) }
	run, _ := e.start(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
	waitSignal(t, g.entered, "the first step to start")
	for i := 0; i < 2; i++ {
		if !e.exec.spawn(run.ID) {
			t.Fatal("spawn() = false, want the work accepted")
		}
	}
	return run
}

func TestQueuedSpawnsStopOnShutdown(t *testing.T) {
	e := newEnv(t)
	run := startQueued(t, e)
	e.spawner.cancel()
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunRunning || !reflect.DeepEqual(stepStates(steps), []string{StepRunning, StepPending}) {
		t.Fatalf("run = %+v, steps = %v; want it left running for startup recovery", got, stepStates(steps))
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 1 {
		t.Fatalf("connector calls = %d, want 1", len(calls))
	}
	if notes := e.notes.snapshot(); len(notes) != 0 {
		t.Fatalf("notifications = %+v, want none", notes)
	}
	if n := activeRuns(e); n != 0 {
		t.Fatalf("registry holds %d runs, want none", n)
	}
}

func TestQueuedSpawnsStopOnCancel(t *testing.T) {
	e := newEnv(t)
	run := startQueued(t, e)
	if err := e.exec.Cancel(context.Background(), run.ID, e.starter); err != nil {
		t.Fatalf("Cancel() error: %v", err)
	}
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunCancelled || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepSkipped}) {
		t.Fatalf("run = %+v, steps = %v; want cancelled with the in-flight step unknown", got, stepStates(steps))
	}
	if calls := e.lifecycle.snapshot(); len(calls) != 1 {
		t.Fatalf("connector calls = %d, want 1", len(calls))
	}
	assertNoFailureNotes(t, e)
	if n := activeRuns(e); n != 0 {
		t.Fatalf("registry holds %d runs, want none", n)
	}
}

func TestAwaitStopped(t *testing.T) {
	e := newEnv(t)
	a := e.connector()
	blocker := newBlockingNotifier()
	t.Cleanup(blocker.unblock)
	e.exec.notifier = blocker
	e.lifecycle.fn = func(context.Context, int, lifecycleCall) error { return errors.New("boom") }
	// The goroutine fails the run, then is held inside its notification: it
	// still holds the run's slot.
	run, _ := e.start(lifecycleStep(a, "restart"))
	waitSignal(t, blocker.entered, "the failure notification")

	// await calls awaitStopped in a goroutine and returns a channel closed when
	// it returns.
	await := func(ctx context.Context) <-chan struct{} {
		returned := make(chan struct{})
		go func() {
			defer close(returned)
			e.exec.awaitStopped(ctx, run.ID)
		}()
		return returned
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	waitSignal(t, await(cancelled), "awaitStopped to give up on a done context")
	if n := activeRuns(e); n != 1 {
		t.Fatalf("registry holds %d runs, want the held goroutine's", n)
	}

	// The "not yet returned" check can only fail on a regression: it cannot fail
	// while the holder is held, but it also passes if the call is merely slow to
	// start. The wait after unblock is what proves it returns.
	patient := await(context.Background())
	select {
	case <-patient:
		t.Fatal("awaitStopped returned while the goroutine was still running")
	default:
	}
	blocker.unblock()
	waitSignal(t, patient, "awaitStopped to return once the goroutine stopped")
	e.settle()
	if n := activeRuns(e); n != 0 {
		t.Fatalf("registry holds %d runs, want none", n)
	}

	// No goroutine: it returns at once.
	waitSignal(t, await(context.Background()), "awaitStopped on a run without a goroutine")
	e.exec.awaitStopped(context.Background(), "unknown run")
}

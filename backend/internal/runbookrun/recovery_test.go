package runbookrun

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestRecoverInterruptsRunningRunsAndNeverContinuesThem(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()

	// State left by a process that stopped: one run mid-step, one between
	// steps and one waiting on a manual step.
	create := func(steps ...*store.RunbookStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
		t.Helper()
		book, saved := e.runbook(steps...)
		run, frozen, err := e.s.CreateRunbookRun(ctx, book.ID, e.starter, FreezeSteps(saved))
		if err != nil {
			t.Fatal(err)
		}
		return run, frozen
	}
	midStep, midSteps := create(lifecycleStep(a, "restart"), lifecycleStep(a, "start"))
	if _, err := e.s.UpdateRunbookRunStep(ctx, midStep.ID, midSteps[0].ID, StepPending, map[string]any{"state": StepRunning}); err != nil {
		t.Fatal(err)
	}
	between, betweenSteps := create(lifecycleStep(a, "stop"), lifecycleStep(a, "start"))
	for _, transition := range [][2]string{{StepPending, StepRunning}, {StepRunning, StepSucceeded}} {
		if _, err := e.s.UpdateRunbookRunStep(ctx, between.ID, betweenSteps[0].ID, transition[0], map[string]any{"state": transition[1]}); err != nil {
			t.Fatal(err)
		}
	}
	waiting, waitingSteps := create(manualStep("Check"), lifecycleStep(a, "restart"))
	if _, _, err := e.s.PauseRunbookRunOnManualStep(ctx, waiting.ID, waitingSteps[0].ID); err != nil {
		t.Fatal(err)
	}

	n, err := Recover(ctx, e.s, e.events, e.notes)
	if err != nil || n != 2 {
		t.Fatalf("Recover() = %d, %v; want the two running runs", n, err)
	}

	got, steps := e.get(midStep.ID)
	if got.State != RunFailed || got.Reason != ReasonInterrupted || !reflect.DeepEqual(stepStates(steps), []string{StepUnknown, StepPending}) {
		t.Fatalf("mid-step run = %+v, steps = %v; want failed/interrupted with the step unknown", got, stepStates(steps))
	}
	got, steps = e.get(between.ID)
	if got.State != RunFailed || got.Reason != ReasonInterrupted || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepPending}) {
		t.Fatalf("between-steps run = %+v, steps = %v; want failed/interrupted", got, stepStates(steps))
	}
	got, steps = e.get(waiting.ID)
	if got.State != RunWaitingManual || !reflect.DeepEqual(stepStates(steps), []string{StepWaiting, StepPending}) || got.UpdatedAt == "" {
		t.Fatalf("waiting run = %+v, steps = %v; want it untouched", got, stepStates(steps))
	}
	// Nothing executes until a user resumes.
	e.settle()
	if calls := e.lifecycle.snapshot(); len(calls) != 0 {
		t.Fatalf("recovery executed steps: %+v", calls)
	}

	notes := map[string]note{}
	for _, n := range e.notes.snapshot() {
		if n.EventType != notifications.EventRunbookRunFailed {
			t.Fatalf("notification = %+v, want only failures", n)
		}
		notes[n.ConnectorID] = n
	}
	if len(notes) != 2 || !strings.Contains(notes[a].Message, `Step 1 "restart"`) || !strings.Contains(notes[a].Message, "outcome is unknown") ||
		!strings.Contains(notes[""].Message, "interrupted by a backend restart") {
		t.Fatalf("notifications = %+v, want the interrupted step and the run-level interruption", notes)
	}
	var described []string
	for _, p := range e.events.snapshot() {
		if p.Event.RunID == midStep.ID {
			described = append(described, describe([]published{p}, map[string]string{a: "A"})[0]+" "+p.Event.Reason)
		}
	}
	if want := []string{"A: step 0 unknown (run failed) interrupted", "all: run failed interrupted"}; !reflect.DeepEqual(described, want) {
		t.Fatalf("events for the mid-step run = %q, want %q", described, want)
	}

	// A second pass finds nothing.
	if n, err := Recover(ctx, e.s, nil, nil); err != nil || n != 0 {
		t.Fatalf("second Recover() = %d, %v; want 0", n, err)
	}

	// The waiting run is still confirmable, and the interrupted one resumes
	// from the unknown step.
	if err := e.exec.Confirm(ctx, waiting.ID, waitingSteps[0].ID, e.starter); err != nil {
		t.Fatalf("Confirm() after recovery: %v", err)
	}
	e.settle()
	if got, _ := e.get(waiting.ID); got.State != RunSucceeded {
		t.Fatalf("confirmed run = %+v, want succeeded", got)
	}
	resumer := e.operator(a)
	if _, _, err := e.exec.Resume(ctx, midStep.ID, resumer, ResumeNone); err != nil {
		t.Fatalf("Resume() after recovery: %v", err)
	}
	e.settle()
	got, steps = e.get(midStep.ID)
	if got.State != RunSucceeded || !reflect.DeepEqual(stepStates(steps), []string{StepSucceeded, StepSucceeded}) {
		t.Fatalf("resumed run = %+v, steps = %v; want succeeded", got, stepStates(steps))
	}
	var resumed []string
	for _, call := range e.lifecycle.snapshot() {
		if call.Audit["runId"] == midStep.ID {
			resumed = append(resumed, call.Verb+" as "+call.Actor.UserID)
		}
	}
	if want := []string{"restart as " + resumer, "start as " + resumer}; !reflect.DeepEqual(resumed, want) {
		t.Fatalf("resumed steps = %q, want the unknown step executed again: %q", resumed, want)
	}
}

func TestRecoverResumesRunWhoseLastStepHadSucceeded(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()
	book, saved := e.runbook(lifecycleStep(a, "restart"))
	run, frozen, err := e.s.CreateRunbookRun(ctx, book.ID, e.starter, FreezeSteps(saved))
	if err != nil {
		t.Fatal(err)
	}
	// Written by code that updated the step and the run separately and
	// stopped in between.
	for _, transition := range [][2]string{{StepPending, StepRunning}, {StepRunning, StepSucceeded}} {
		if _, err := e.s.UpdateRunbookRunStep(ctx, run.ID, frozen[0].ID, transition[0], map[string]any{"state": transition[1]}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := Recover(ctx, e.s, e.events, e.notes); err != nil || n != 1 {
		t.Fatalf("Recover() = %d, %v", n, err)
	}
	if _, _, err := e.exec.Resume(ctx, run.ID, e.starter, ResumeNone); err != nil {
		t.Fatal(err)
	}
	e.settle()
	got, _ := e.get(run.ID)
	if got.State != RunSucceeded || got.FinishedAt == "" || len(e.lifecycle.snapshot()) != 0 {
		t.Fatalf("run = %+v, lifecycle calls = %d; want succeeded without executing anything again", got, len(e.lifecycle.snapshot()))
	}
}

func TestRecoverLeavesAwaitingApprovalRequestsOpen(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	connectorID := e.connector()
	e.operator(connectorID)
	book, authored := e.runbook(lifecycleStep(connectorID, "restart"))
	run, _, err := e.exec.Request(ctx, book.ID, e.starter, authored)
	if err != nil {
		t.Fatalf("Request() error: %v", err)
	}

	if n, err := Recover(ctx, e.s, e.events, e.notes); err != nil || n != 0 {
		t.Fatalf("Recover() = %d, %v; want no interrupted runs", n, err)
	}
	e.settle()
	got, steps := e.get(run.ID)
	if got.State != RunAwaitingApproval || !reflect.DeepEqual(stepStates(steps), []string{StepPending}) || got.FinishedAt != "" {
		t.Fatalf("awaiting run = %+v, steps = %v; want it untouched", got, stepStates(steps))
	}
	if starts := e.spawner.starts.Load(); starts != 0 || len(e.lifecycle.snapshot()) != 0 {
		t.Fatalf("recovery started %d executions, %d lifecycle calls; want none", starts, len(e.lifecycle.snapshot()))
	}
}

package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

// stepFailure is why a step did not succeed: the reason recorded on the run
// and the human-readable message recorded on the step.
type stepFailure struct {
	reason  string
	message string
}

// execute runs runID's steps in order until the run pauses, fails, finishes,
// is cancelled or the process shuts down. It re-reads the run before every
// step, so a cancel or a new resuming user takes effect at the next step.
func (e *Executor) execute(ctx context.Context, runID string) {
	for ctx.Err() == nil {
		run, steps, err := e.readRun(ctx, runID)
		if err != nil {
			slog.Error("runbook run: read run failed", "run", logsafe.Sanitize(runID), "error", err)
			e.failRun(ctx, runID, err)
			return
		}
		if run.State != RunRunning {
			return
		}
		step := firstUnfinished(steps)
		if step == nil {
			// Every step had already succeeded, e.g. a run interrupted after
			// its last step and then resumed.
			e.finishWithoutStep(ctx, run)
			return
		}
		if !e.runStep(ctx, run, steps, step) {
			return
		}
	}
}

// firstUnfinished returns the first step that has not succeeded.
func firstUnfinished(steps []*store.RunbookRunStepRecord) *store.RunbookRunStepRecord {
	for _, step := range steps {
		if step.State != StepSucceeded {
			return step
		}
	}
	return nil
}

// isLastUnfinished reports whether step is the only one left to succeed.
func isLastUnfinished(steps []*store.RunbookRunStepRecord, step *store.RunbookRunStepRecord) bool {
	for _, other := range steps {
		if other.ID != step.ID && other.State != StepSucceeded {
			return false
		}
	}
	return true
}

// runStep executes one step and records its outcome. It reports whether the
// run should go on to the next step.
func (e *Executor) runStep(ctx context.Context, run *store.RunbookRunRecord, steps []*store.RunbookRunStepRecord, step *store.RunbookRunStepRecord) bool {
	if step.Kind == KindManual {
		e.pause(ctx, run, step)
		return false
	}
	switch step.State {
	case StepPending, StepFailed, StepUnknown:
	default:
		// running, waiting or skipped: another goroutine owns the step or the
		// run is not executable. Never run a step twice.
		slog.Warn("runbook run: step is not startable", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "state", step.State)
		return false
	}

	dbCtx, cancel := detached(ctx)
	started, err := e.store.UpdateRunbookRunStep(dbCtx, run.ID, step.ID, step.State, map[string]any{"state": StepRunning})
	cancel()
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: start step failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
			e.failRun(ctx, run.ID, err)
		}
		return false
	}
	e.publishStep(run, started)

	failure := e.perform(ctx, run, started)
	if failure == nil {
		return e.recordSuccess(ctx, run, started, isLastUnfinished(steps, step))
	}
	if ctx.Err() != nil {
		// Cancelled or shutting down: the error says nothing about the
		// connector. Cancel has already marked the step unknown; after a
		// shutdown, startup recovery does.
		return false
	}
	e.recordFailure(ctx, run, started, StepFailed, failure)
	return false
}

// pause stops the run on a manual step until a user confirms it.
func (e *Executor) pause(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	paused, waiting, err := e.store.PauseRunbookRunOnManualStep(dbCtx, run.ID, step.ID)
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: pause on manual step failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
			e.failRun(ctx, run.ID, err)
		}
		return
	}
	e.publishStep(paused, waiting)
	e.publishRun(paused)
	e.notifyWaiting(dbCtx, paused, waiting)
}

// recordSuccess stores a succeeded step, together with the run when it was the
// last one. The write is attempted even when ctx is done: a cancelled run
// rejects it, and a shutdown should still keep an outcome that is known.
func (e *Executor) recordSuccess(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, last bool) bool {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	if last {
		finished, done, err := e.store.FinishRunbookRun(dbCtx, run.ID, step.ID)
		if err != nil {
			e.outcomeNotRecorded(ctx, run, step, err)
			return false
		}
		e.publishStep(finished, done)
		e.publishRun(finished)
		return false
	}
	done, err := e.store.UpdateRunbookRunStep(dbCtx, run.ID, step.ID, StepRunning, map[string]any{"state": StepSucceeded})
	if err != nil {
		e.outcomeNotRecorded(ctx, run, step, err)
		return false
	}
	e.publishStep(run, done)
	return true
}

// recordFailure stores the step outcome and the failed run in one transaction
// and notifies.
func (e *Executor) recordFailure(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, stepState string, failure *stepFailure) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	failed, ended, err := e.store.FailRunbookRunStep(dbCtx, run.ID, step.ID, stepState, failure.message, failure.reason)
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: record step failure failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
		}
		return
	}
	e.publishStep(failed, ended)
	e.publishRun(failed)
	e.notifyFailed(dbCtx, failed, ended)
}

// outcomeNotRecorded handles a succeeded step whose result could not be
// stored. A state conflict means the run was cancelled meanwhile and is
// already consistent. Anything else leaves a step that ran but is not
// recorded, so it is marked unknown and the run stops.
func (e *Executor) outcomeNotRecorded(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, err error) {
	if stateChanged(err) {
		return
	}
	slog.Error("runbook run: record step success failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
	e.recordFailure(ctx, run, step, StepUnknown, &stepFailure{
		reason:  ReasonInternalError,
		message: "The step ran but its result could not be recorded.",
	})
}

// finishWithoutStep completes a running run that has no step left to execute.
func (e *Executor) finishWithoutStep(ctx context.Context, run *store.RunbookRunRecord) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	finished, err := e.store.UpdateRunbookRun(dbCtx, run.ID, RunRunning, map[string]any{"state": RunSucceeded})
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: finish run failed", "run", logsafe.Sanitize(run.ID), "error", err)
		}
		return
	}
	e.publishRun(finished)
}

// failRun stops a run the executor can no longer drive because reading or
// writing its state failed between steps, when no step is in flight. Best
// effort: if this write fails too, the run stays running until it is
// cancelled or the next startup recovery.
func (e *Executor) failRun(ctx context.Context, runID string, cause error) {
	if ctx.Err() != nil || errors.Is(cause, store.ErrNotFound) {
		return
	}
	dbCtx, cancel := detached(ctx)
	defer cancel()
	failed, err := e.store.UpdateRunbookRun(dbCtx, runID, RunRunning, map[string]any{"state": RunFailed, "reason": ReasonInternalError})
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: fail run failed", "run", logsafe.Sanitize(runID), "error", err)
		}
		return
	}
	e.publishRun(failed)
	e.notifyFailed(dbCtx, failed, nil)
}

// perform authorizes and executes one automated step. It returns nil when the
// step succeeded.
func (e *Executor) perform(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) *stepFailure {
	actor, failure := e.authorize(ctx, run, step)
	if failure != nil {
		return failure
	}

	stepCtx := ctx
	timeout := e.stepTimeout(step)
	if timeout > 0 {
		var cancel context.CancelFunc
		stepCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var err error
	var waitingFor string
	switch step.Kind {
	case KindLifecycle:
		waitingFor = step.Verb + " to finish"
		err = e.lifecycle.MutateLifecycleOp(stepCtx, step.ConnectorID, step.Verb, step.EntityRef, actor, auditDetail(run, step))
	case KindSyncAndWait:
		waitingFor = "the sync to finish"
		err = e.syncAndWait(stepCtx, step.ConnectorID)
	case KindWaitUntilHealthy:
		var last string
		last, err = e.waitUntilHealthy(stepCtx, step.ConnectorID)
		waitingFor = "the connector to report online"
		if last != "" {
			waitingFor += " (last status: " + last + ")"
		}
	default:
		return &stepFailure{reason: ReasonStepFailed, message: fmt.Sprintf("Unsupported step kind %q.", step.Kind)}
	}
	if err == nil {
		return nil
	}
	if ctx.Err() == nil && (errors.Is(stepCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded)) {
		message := "Timed out waiting for " + waitingFor + "."
		if timeout > 0 {
			message = fmt.Sprintf("Timed out after %s waiting for %s.", timeout, waitingFor)
		}
		return &stepFailure{reason: ReasonStepTimeout, message: message}
	}
	return &stepFailure{reason: ReasonStepFailed, message: err.Error()}
}

// authorize re-checks, immediately before the step, that the user who started
// or last resumed the run still holds an operator grant on the step's
// connector. It fails closed, and never yields an empty actor.
func (e *Executor) authorize(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) (connectors.LifecycleActor, *stepFailure) {
	if step.ConnectorID == "" {
		return connectors.LifecycleActor{}, &stepFailure{reason: ReasonStepFailed, message: "The step has no connector."}
	}
	userID := ActingUser(run)
	if userID == "" {
		return connectors.LifecycleActor{}, &stepFailure{reason: ReasonPermissionDenied, message: "The run has no acting user."}
	}
	actor, ok, err := e.grants.Operator(ctx, userID, step.ConnectorID)
	if err != nil {
		slog.Error("runbook run: grant check failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
		return connectors.LifecycleActor{}, &stepFailure{reason: ReasonPermissionDenied, message: "The operator grant on the step's connector could not be verified."}
	}
	if !ok || actor.UserID == "" {
		return connectors.LifecycleActor{}, &stepFailure{
			reason:  ReasonPermissionDenied,
			message: "The user who started or last resumed the run no longer holds an operator grant on the step's connector.",
		}
	}
	return actor, nil
}

// ActingUser returns the user a run's automated steps execute as: the one who
// last resumed it, otherwise the one who started it.
func ActingUser(run *store.RunbookRunRecord) string {
	if run.ResumedBy != nil && *run.ResumedBy != "" {
		return *run.ResumedBy
	}
	return run.StartedBy
}

// auditDetail identifies the run and step in a lifecycle step's audit record.
func auditDetail(run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) map[string]any {
	detail := map[string]any{"runId": run.ID, "stepId": step.ID}
	if run.RunbookID != nil {
		detail["runbookId"] = *run.RunbookID
	}
	return detail
}

// syncAndWait syncs the connector and returns once that sync has finished
// successfully. While another sync holds the connector it waits and then runs
// its own, so the step always observes the state left by the steps before it.
func (e *Executor) syncAndWait(ctx context.Context, connectorID string) error {
	for {
		result, err := e.syncer.RunSyncFields(ctx, connectorID, uuid.NewString(), nil)
		switch {
		case errors.Is(err, syncengine.ErrAlreadyRunning):
			if err := sleep(ctx, e.syncBusyRetry); err != nil {
				return err
			}
		case err != nil:
			return err
		case result == nil:
			return errors.New("the sync did not run")
		case result.Status == "skipped":
			return errors.New("the sync was skipped because the connector is disabled")
		case result.Status != "success":
			if result.Error != "" {
				return errors.New(result.Error)
			}
			return fmt.Errorf("the sync ended with status %q", result.Status)
		default:
			return nil
		}
	}
}

// waitUntilHealthy checks the connector now and then every poll interval until
// a check reports online. It returns the last status seen with ctx's error
// when the wait is cut short.
func (e *Executor) waitUntilHealthy(ctx context.Context, connectorID string) (string, error) {
	last := ""
	for {
		status, err := e.health.CheckHealth(ctx, connectorID)
		if err != nil {
			return last, err
		}
		if status == healthStatusOnline {
			return status, nil
		}
		last = status
		if err := sleep(ctx, e.healthPollInterval); err != nil {
			return last, err
		}
	}
}

// readRun loads the run and its steps.
func (e *Executor) readRun(ctx context.Context, runID string) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	return e.store.GetRunbookRun(dbCtx, runID)
}

// stateChanged reports whether a guarded write was rejected because the run
// or step is no longer in the expected state (cancelled, expired, finished by
// another goroutine) or no longer exists. The stored state is then already
// consistent and the executor just stops.
func stateChanged(err error) bool {
	return errors.Is(err, store.ErrConflict) || errors.Is(err, store.ErrNotFound)
}

// detached returns a bounded context for a state read or write that survives
// cancellation of the run's context.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
}

// sleep waits for d or until ctx is done.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

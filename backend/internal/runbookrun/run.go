package runbookrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/compliance"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// stepFailure is why a step did not succeed: the reason recorded on the run
// and the human-readable message recorded on the step.
type stepFailure struct {
	reason  string
	message string
	// state is the step's state after the failure. Empty means StepFailed;
	// StepUnknown is for an outcome that could not be established.
	state string
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
		// running, waiting or skipped while the run is running. This goroutine
		// holds the run's slot, so nobody else owns the step: it was left
		// behind by an earlier failure. Never run a step twice, and never leave
		// the run running with nothing driving it.
		slog.Warn("runbook run: step is not startable", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "state", step.State)
		if step.State == StepRunning {
			// Whether the connector call happened is not known. The run fails
			// and the step becomes unknown, which a resume may start again.
			e.recordFailure(ctx, run, step, StepUnknown, &stepFailure{
				reason:  ReasonInternalError,
				message: "The step was left running by an earlier failure.",
			})
			return false
		}
		e.failRun(ctx, run.ID, fmt.Errorf("step %s is %s in a running run", step.ID, step.State))
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
		// Cancelled or shutting down. For the other kinds the error says
		// nothing about the connector. A config_push core does finish and
		// report after a cancel or shutdown (it has raised its own alert on a
		// mismatch), but the outcome is still not recorded here: cancel has
		// already marked the step unknown and, after a shutdown, startup
		// recovery does, and a resume re-reads the field before writing.
		return false
	}
	stepState := failure.state
	if stepState == "" {
		stepState = StepFailed
	}
	e.recordFailure(ctx, run, started, stepState, failure)
	return false
}

// pause stops the run on a manual step until a user confirms it. When the run
// cannot pause, a conflict means it was cancelled or expired meanwhile or the
// step is not pending: failRun fails the run if it is still running and does
// nothing otherwise.
func (e *Executor) pause(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	paused, waiting, err := e.store.PauseRunbookRunOnManualStep(dbCtx, run.ID, step.ID)
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: pause on manual step failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
		}
		e.failRun(ctx, run.ID, err)
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
// and notifies. When that write fails, failRun fails the run at run level if it
// is still running, so a failed write never leaves it running.
func (e *Executor) recordFailure(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, stepState string, failure *stepFailure) {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	failed, ended, err := e.store.FailRunbookRunStep(dbCtx, run.ID, step.ID, stepState, failure.message, failure.reason)
	if err != nil {
		if !stateChanged(err) {
			slog.Error("runbook run: record step failure failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
		}
		e.failRun(ctx, run.ID, err)
		return
	}
	e.publishStep(failed, ended)
	e.publishRun(failed)
	e.notifyFailed(dbCtx, failed, ended)
}

// outcomeNotRecorded handles a succeeded step whose result could not be
// stored. The step ran but is not recorded, so it is marked unknown and the
// run stops, also after a state conflict: the run may still be running. If it
// was cancelled meanwhile it is already consistent and that write conflicts too.
func (e *Executor) outcomeNotRecorded(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, err error) {
	if !stateChanged(err) {
		slog.Error("runbook run: record step success failed", "run", logsafe.Sanitize(run.ID), "step", logsafe.Sanitize(step.ID), "error", err)
	}
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

// failRun fails a run the executor can no longer drive, with reason
// internal_error, when no step is in flight. The write is guarded on the run
// still being running, so it does nothing for a run that was cancelled, expired
// or already failed, and it is skipped when ctx is done or cause is a missing
// run. Best effort: if this write fails too, the run stays running until it is
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
	var cond *entityCondition
	if step.Kind == KindWaitForEntity {
		var failure *stepFailure
		if cond, failure = parseEntityCondition(step); failure != nil {
			return failure
		}
	}
	switch step.Kind {
	case KindLifecycle:
		err = e.lifecycle.MutateRunbookLifecycleOp(stepCtx, step.ConnectorID, step.Verb, step.EntityRef, actor, auditDetail(run, step))
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
	case KindConnectorAction:
		return e.performConnectorAction(ctx, run, step, actor)
	case KindConfigPush:
		if e.configPush == nil {
			return &stepFailure{reason: ReasonStepFailed, message: "Config push is not configured."}
		}
		err = e.performConfigPush(ctx, run, step, actor)
	case KindWaitForEntity:
		if e.entities == nil {
			return &stepFailure{reason: ReasonStepFailed, message: "Entity loading is not configured."}
		}
		var last string
		last, err = e.waitForEntity(stepCtx, step, cond)
		if last == "" {
			last = noObservation
		}
		waitingFor = fmt.Sprintf("entity %s to satisfy %s (last observed: %s)", step.EntityRef, cond, last)
	default:
		return &stepFailure{reason: ReasonStepFailed, message: fmt.Sprintf("Unsupported step kind %q.", step.Kind)}
	}
	if err == nil {
		return nil
	}
	if timeout > 0 && ctx.Err() == nil && errors.Is(stepCtx.Err(), context.DeadlineExceeded) {
		// The step's own deadline expired. A deadline error from the operation
		// itself, such as a connector's HTTP timeout, is an ordinary failure.
		return &stepFailure{reason: ReasonStepTimeout, message: fmt.Sprintf("Timed out after %s waiting for %s.", timeout, waitingFor)}
	}
	return &stepFailure{reason: ReasonStepFailed, message: err.Error()}
}

// performConnectorAction sends the named action frozen at run start through the
// shared connector implementation, under the run's acting user. The action is
// refused, and nothing is sent, when its fingerprint changed since the run
// started. A request that was written with no status received leaves the step
// unknown. Any other failure leaves it failed. The response excerpt is never
// recorded: it can carry the service's own text, so only the status and the
// error are kept.
func (e *Executor) performConnectorAction(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, actor connectors.LifecycleActor) *stepFailure {
	if e.actions == nil {
		return &stepFailure{reason: ReasonStepFailed, message: "Connector actions are not configured."}
	}
	detail := auditDetail(run, step)
	detail["stepIndex"] = step.Position
	result, err := e.actions.MutateRunbookAction(ctx, step.ConnectorID, step.Action, step.EntityRef, step.ActionFingerprint, actor, detail)
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, connectors.ErrActionChanged):
		return &stepFailure{reason: ReasonStepFailed, message: "The action changed since the run started; nothing was sent."}
	case result.Written && result.Status == 0:
		return &stepFailure{reason: ReasonStepFailed, state: StepUnknown,
			message: "The request was sent but no response status was received: " + err.Error()}
	}
	return &stepFailure{reason: ReasonStepFailed, message: err.Error()}
}

// performConfigPush writes the field and value frozen at run start through the
// shared config-push core, under the run's acting user. The core's write,
// verification and revert run on a context detached from the run's
// cancellation, bounded by the executor's configPushTimeout (two minutes by
// default): a cancel or shutdown after the write began must not leave the field
// half-changed, or make the core skip the audit entry or raise a false mismatch
// alert. A step that is already cancelled when it gets here does not start.
func (e *Executor) performConfigPush(ctx context.Context, run *store.RunbookRunRecord, step *store.RunbookRunStepRecord, actor connectors.LifecycleActor) error {
	var value any
	if err := json.Unmarshal([]byte(step.TargetValue), &value); err != nil {
		return fmt.Errorf("the target value is not valid JSON: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	pushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.configPushTimeout)
	defer cancel()
	detail := auditDetail(run, step)
	detail["stepIndex"] = step.Position
	err := e.configPush.MutateRunbookConfigPush(pushCtx, step.ConnectorID, step.EntityRef, step.FieldKey, value, actor, detail)
	if err != nil && errors.Is(pushCtx.Err(), context.DeadlineExceeded) {
		// The bound cut the core short, so whether the write reached the
		// connector, and whether it was verified or reverted, is not known.
		return fmt.Errorf("the config push did not finish within %s, so the field may or may not have been written: %w", e.configPushTimeout, err)
	}
	return err
}

// entityCondition is a frozen wait_for_entity condition.
type entityCondition struct {
	compliance.Condition
}

// String renders the condition for a timeout reason.
func (c *entityCondition) String() string {
	return fmt.Sprintf("%s %s %v", c.Attribute, c.Op, c.Value)
}

// entityOperators are the operators wait_for_entity supports: the compliance
// operators without the days-left ones, which a wait does not offer.
var entityOperators = map[string]bool{"eq": true, "neq": true, "contains": true, "regex": true, "gt": true, "lt": true}

// parseEntityCondition decodes the condition frozen on a wait_for_entity step.
func parseEntityCondition(step *store.RunbookRunStepRecord) (*entityCondition, *stepFailure) {
	if step.EntityRef == "" || step.Attribute == "" {
		return nil, &stepFailure{reason: ReasonStepFailed, message: "The step has no entity or attribute to wait for."}
	}
	if !entityOperators[step.Operator] {
		return nil, &stepFailure{reason: ReasonStepFailed, message: fmt.Sprintf("Unsupported wait operator %q.", step.Operator)}
	}
	var expected any
	if err := json.Unmarshal([]byte(step.ExpectedValue), &expected); err != nil {
		return nil, &stepFailure{reason: ReasonStepFailed, message: "The expected value is not valid JSON."}
	}
	return &entityCondition{compliance.Condition{Attribute: step.Attribute, Op: step.Operator, Value: expected}}, nil
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
// successfully. The engine waits for a sync already running on the connector
// and then runs its own, so the step observes the state left by the steps
// before it. ctx, which carries the step timeout and the run's cancellation,
// bounds that wait too.
func (e *Executor) syncAndWait(ctx context.Context, connectorID string) error {
	result, err := e.syncer.RunSyncFieldsWhenFree(ctx, connectorID, uuid.NewString(), nil)
	switch {
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

// waitForEntity syncs the connector now and then every poll interval until the
// named entity satisfies cond. A failed sync or entity read counts as "not yet",
// so one flaky poll does not fail a restart still in progress. It returns a
// description of the last observation, truncated, with ctx's error when the wait
// is cut short.
func (e *Executor) waitForEntity(ctx context.Context, step *store.RunbookRunStepRecord, cond *entityCondition) (string, error) {
	last := ""
	for {
		if err := e.syncAndWait(ctx, step.ConnectorID); err != nil {
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			slog.Warn("runbook run: entity wait sync failed", "step", logsafe.Sanitize(step.ID), "connector", logsafe.Sanitize(step.ConnectorID), "error", logsafe.Err(err))
		} else {
			holds, observed, err := e.checkEntity(ctx, step, cond)
			switch {
			case err != nil:
				if ctx.Err() != nil {
					return last, ctx.Err()
				}
				slog.Warn("runbook run: entity wait read failed", "step", logsafe.Sanitize(step.ID), "connector", logsafe.Sanitize(step.ConnectorID), "error", logsafe.Err(err))
			case holds:
				return observed, nil
			default:
				last = observed
			}
		}
		if err := sleep(ctx, e.entityPollInterval); err != nil {
			return last, err
		}
	}
}

// checkEntity evaluates cond against the entity in the connector's latest
// snapshot. observed describes what it saw: the entity is absent, the attribute
// is absent, or the attribute's value.
func (e *Executor) checkEntity(ctx context.Context, step *store.RunbookRunStepRecord, cond *entityCondition) (holds bool, observed string, err error) {
	snapshot, err := e.entities.LatestEntities(ctx, step.ConnectorID)
	if err != nil {
		return false, "", err
	}
	if snapshot == nil {
		return false, "entity not found", nil
	}
	for _, entity := range snapshot.Entities {
		if entity.ExternalID != step.EntityRef && (entity.ExternalID != "" || entity.Name != step.EntityRef) {
			continue
		}
		value, present := entity.Attributes[cond.Attribute]
		return compliance.MatchesCondition(entity, cond.Condition), describeObservation(cond.Attribute, value, present), nil
	}
	return false, "entity not found", nil
}

// describeObservation formats an attribute's observed value for a timeout
// reason. The value comes from a connector, so it is truncated and withheld
// outright for attributes whose names look like secrets.
func describeObservation(attribute string, value any, present bool) string {
	if !present {
		return fmt.Sprintf("attribute %s not present", attribute)
	}
	if secretAttribute(attribute) {
		return attribute + " = <redacted>"
	}
	text := fmt.Sprint(value)
	if encoded, err := json.Marshal(value); err == nil {
		text = string(encoded)
	}
	if len(text) > lastObservationLimit {
		text = strings.ToValidUTF8(text[:lastObservationLimit], "") + "…"
	}
	return attribute + " = " + text
}

// secretAttribute reports whether an attribute name suggests a credential.
func secretAttribute(name string) bool {
	lower := strings.ToLower(name)
	for _, hint := range []string{"password", "passwd", "secret", "token", "key", "credential", "psk", "passphrase", "auth"} {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
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

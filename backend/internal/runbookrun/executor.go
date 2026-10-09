// Package runbookrun executes whole-runbook runs on the server: it owns the
// run state machine and drives it through narrow interfaces for the store, the
// lifecycle core, sync and health, so it runs without an HTTP request.
//
// The executor trusts its caller for who may start, resume, confirm or cancel
// a run and for the runbook.run elevation; it re-checks the operator grant of
// the acting user itself before every automated step.
package runbookrun

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/compliance"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/logsafe"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

// Run states.
const (
	RunRunning       = "running"
	RunWaitingManual = "waiting_manual"
	RunFailed        = "failed"
	RunSucceeded     = "succeeded"
	RunCancelled     = "cancelled"
	RunExpired       = "expired"
)

// Step states.
const (
	StepPending   = "pending"
	StepRunning   = "running"
	StepWaiting   = "waiting"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
	StepSkipped   = "skipped"
	StepUnknown   = "unknown"
)

// Step kinds.
const (
	KindLifecycle        = "lifecycle"
	KindSyncAndWait      = "sync_and_wait"
	KindWaitUntilHealthy = "wait_until_healthy"
	KindManual           = "manual"
	KindConfigPush       = "config_push"
	KindWaitForEntity    = "wait_for_entity"
	// KindConnectorAction runs a named action that a custom connector's recipe
	// declares, on the service or on one entity.
	KindConnectorAction = "connector_action"
)

// Reasons recorded on a failed run. The failed step carries the
// human-readable detail in its error.
const (
	// ReasonInterrupted: the backend restarted while the run was running.
	ReasonInterrupted = "interrupted"
	// ReasonStepFailed: a step's operation returned an error.
	ReasonStepFailed = "step_failed"
	// ReasonStepTimeout: a step exceeded its timeout.
	ReasonStepTimeout = "step_timeout"
	// ReasonPermissionDenied: the acting user no longer holds an operator
	// grant on the step's connector.
	ReasonPermissionDenied = "permission_denied"
	// ReasonInternalError: the executor could not read or record run state.
	ReasonInternalError = "internal_error"
)

const (
	// HealthPollInterval is how often a wait_until_healthy step checks its
	// connector.
	HealthPollInterval = 10 * time.Second
	// EntityPollInterval is how often a wait_for_entity step syncs its
	// connector and re-evaluates its condition. It is a floor: each sync also
	// waits behind one already running.
	EntityPollInterval = 30 * time.Second
	// DefaultStepTimeout applies to a sync_and_wait, wait_until_healthy or
	// wait_for_entity step without a timeout.
	DefaultStepTimeout = 5 * time.Minute
	// storeTimeout bounds one state read or write. Writes are detached from
	// the run's context so an outcome is still recorded while it is cancelled.
	storeTimeout = 15 * time.Second
	// configPushTimeout bounds one config_push step's write, verification and
	// revert, which run detached from the run's cancellation so they are not
	// abandoned halfway.
	configPushTimeout = 2 * time.Minute
	// lastObservationLimit truncates (in bytes) the last observed attribute value that a
	// wait_for_entity timeout reports.
	lastObservationLimit = 80
	// noObservation stands in for the last observation of a wait_for_entity
	// step that never got one: every sync or entity read failed, or none
	// finished.
	noObservation = "nothing: no sync result was available"
	// healthStatusOnline is the only status wait_until_healthy accepts.
	healthStatusOnline = "online"
)

// ErrNoActor rejects a start, resume, confirm or cancel without a user.
var ErrNoActor = errors.New("runbook run: an acting user is required")

// ErrActionUnavailable reports a run that cannot start because the fingerprint
// of a connector_action step's action cannot be computed, for example because
// the connector or the action is gone or no action service is configured.
// Nothing is stored when it is returned. The cause is not included: it may
// carry connector configuration.
var ErrActionUnavailable = errors.New("runbook run: a connector action cannot be prepared")

// ErrDecisionRequired rejects a resume of a run whose first unfinished step is
// an unknown connector_action step when no decision is given. Nothing changes.
var ErrDecisionRequired = errors.New("runbook run: the unknown connector action step needs a decision: resend or mark_done")

// ErrInvalidDecision rejects a resume decision that is not a ResumeDecision
// constant.
var ErrInvalidDecision = errors.New("runbook run: unknown resume decision")

// ErrDecisionFieldsRequired rejects a resume that gives a decision for an
// unknown connector_action step without the step id and the run revision the
// operator saw. Nothing changes.
var ErrDecisionFieldsRequired = errors.New("runbook run: a resume decision needs the step id and the run revision it was made on")

// ErrRunChanged rejects a resume whose expected run revision or first
// unfinished step no longer matches the run. Nothing changes. It matches
// store.ErrConflict too.
var ErrRunChanged = fmt.Errorf("runbook run: the run changed since the decision was made: %w", store.ErrConflict)

// ResumeExpectation ties a resume to what the caller saw. An empty field is not
// checked. UpdatedAt is the run's updatedAt as the caller received it and is
// compared inside the store transaction; StepID must be the run's first step
// that has not succeeded. Audit is written in the resume transaction when, and
// only when, a decision is applied to an unknown connector_action step.
type ResumeExpectation struct {
	StepID    string
	UpdatedAt string
	Audit     *store.AuditRecord
}

// ResumeDecision is the caller's choice for the first unfinished step of a
// failed run when that step is an unknown connector_action step.
type ResumeDecision string

// Resume decisions. ResumeNone is the decision of a resume that carries none.
const (
	ResumeNone     ResumeDecision = ""
	ResumeResend   ResumeDecision = "resend"
	ResumeMarkDone ResumeDecision = "mark_done"
)

// StepDecision reports a decision Resume applied to an unknown connector_action
// step. Resume returns nil when no such decision was applied.
type StepDecision struct {
	StepID   string
	Decision ResumeDecision
}

// ErrShuttingDown reports a start, resume or confirm that arrived after
// shutdown began. The run was recorded as failed with reason interrupted and
// can be resumed later.
var ErrShuttingDown = errors.New("runbook run: the server is shutting down")

// ShutdownError identifies work whose durable transition preceded a refused
// spawn. HTTP callers can still audit that transition while returning 503.
type ShutdownError struct {
	RunID string
}

func (e *ShutdownError) Error() string { return ErrShuttingDown.Error() }
func (e *ShutdownError) Unwrap() error { return ErrShuttingDown }

// Store is the run persistence the executor needs. *store.Store satisfies it.
type Store interface {
	CreateRunbookRun(ctx context.Context, runbookID, startedBy string, steps []*store.RunbookRunStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error)
	GetRunbookRun(ctx context.Context, id string) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error)
	UpdateRunbookRun(ctx context.Context, id, expectedState string, updates map[string]any) (*store.RunbookRunRecord, error)
	UpdateRunbookRunStep(ctx context.Context, runID, stepID, expectedState string, updates map[string]any) (*store.RunbookRunStepRecord, error)
	PauseRunbookRunOnManualStep(ctx context.Context, runID, stepID string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error)
	FailRunbookRunStep(ctx context.Context, runID, stepID, stepState, stepError, reason string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error)
	FinishRunbookRun(ctx context.Context, runID, stepID string) (*store.RunbookRunRecord, *store.RunbookRunStepRecord, error)
	ConfirmRunbookRunStep(ctx context.Context, runID, stepID, confirmedBy string) error
	ResumeRunbookRun(ctx context.Context, runID, expectedUpdatedAt, expectedStepID, userID string, audit *store.AuditRecord) (*store.RunbookRunRecord, error)
	// ResumeRunbookRunMarkingStepDone resumes a failed run and marks its first
	// unfinished unknown step succeeded, in one transaction.
	ResumeRunbookRunMarkingStepDone(ctx context.Context, runID, expectedUpdatedAt, stepID, userID string, audit *store.AuditRecord) (*store.RunbookRunRecord, error)
	CancelRunbookRun(ctx context.Context, id, cancelledBy string) error
}

// Lifecycle performs an already-authorized restart, start or stop with its
// failure alert and audit record. It does not check grants or elevation.
// *connectors.Handler satisfies it.
type Lifecycle interface {
	MutateRunbookLifecycleOp(ctx context.Context, connectorID, verb, entityRef string, actor connectors.LifecycleActor, extraAudit map[string]any) error
}

// ConfigPush performs an already-authorized config push with its verification,
// revert, alert and audit record. It does not check grants or elevation.
// *connectors.Handler satisfies it.
type ConfigPush interface {
	MutateRunbookConfigPush(ctx context.Context, connectorID, entityRef, fieldKey string, value any, actor connectors.LifecycleActor, extraAudit map[string]any) error
}

// Entities loads a connector's latest synced entities in the shape the
// compliance engine reads. A nil snapshot means there is none yet.
// StoreEntities satisfies it.
type Entities interface {
	LatestEntities(ctx context.Context, connectorID string) (*compliance.Snapshot, error)
}

// Syncer runs one connector sync and blocks until it ends. While another sync
// of the connector is in flight it waits for that one instead of failing, and
// it broadcasts nothing while waiting. *sync.Engine satisfies it.
type Syncer interface {
	RunSyncFieldsWhenFree(ctx context.Context, connectorID, jobID string, fields []string) (*syncengine.RunResult, error)
}

// HealthChecker runs one connector health check and returns its status.
type HealthChecker interface {
	CheckHealth(ctx context.Context, connectorID string) (status string, err error)
}

// Grants resolves the acting user of a step. ok is false when the user no
// longer exists, is disabled or lacks an operator grant on connectorID.
type Grants interface {
	Operator(ctx context.Context, userID, connectorID string) (actor connectors.LifecycleActor, ok bool, err error)
}

// Publisher delivers WebSocket events. *ws.Hub satisfies it.
type Publisher interface {
	Broadcast(eventType string, payload any)
	BroadcastConnector(connectorID, eventType string, payload any)
}

// Notifier dispatches run notifications. A non-empty actorID is notified even
// without a grant on connectorID. *notifications.Dispatcher satisfies it.
type Notifier interface {
	NotifyRunbookRun(ctx context.Context, eventType, severity, connectorID, actorID, title, message string)
}

// Spawner starts tracked background work under a context that is cancelled on
// shutdown. *sync.Engine satisfies it, so shutdown waits for running steps.
type Spawner interface {
	// TryGo runs work in the background. False means shutdown has begun and
	// work was not run.
	TryGo(work func(context.Context)) bool
}

// ConnectorActions performs recipe-declared named actions for connector_action
// steps. *connectors.Handler satisfies it, so a run and a direct request share
// one implementation, audit detail and failure alert.
type ConnectorActions interface {
	// ActionFingerprint returns the fingerprint of the action's definition as
	// the recipe declares it now. It sends nothing.
	ActionFingerprint(ctx context.Context, connectorID, name, entityRef string) (string, error)
	// MutateRunbookAction sends the action only when expectedFingerprint is not
	// empty and equals the action's current fingerprint. Otherwise it returns an
	// error wrapping connectors.ErrActionChanged and sends nothing.
	MutateRunbookAction(ctx context.Context, connectorID, name, entityRef, expectedFingerprint string, actor connectors.LifecycleActor, extraAudit map[string]any) (connector.ActionResult, error)
}

// Deps are the executor's collaborators. Events, Notifier and Actions may be
// nil; Actions is needed to start a run that has a connector_action step.
type Deps struct {
	Store      Store
	Lifecycle  Lifecycle
	ConfigPush ConfigPush
	Actions    ConnectorActions
	Entities   Entities
	Sync       Syncer
	Health     HealthChecker
	Grants     Grants
	Events     Publisher
	Notifier   Notifier
	Spawner    Spawner
}

// Executor runs runbook runs, one goroutine per active run. It executes in
// this process only: a run belongs to the replica that accepted it. At most one
// goroutine of this process drives a run at a time: a goroutine takes the
// run's slot in the registry before it executes and holds it until it stops. A
// run that a slot holder reads as running is therefore driven by nobody else
// here.
type Executor struct {
	store      Store
	lifecycle  Lifecycle
	configPush ConfigPush
	actions    ConnectorActions
	entities   Entities
	syncer     Syncer
	health     HealthChecker
	grants     Grants
	events     Publisher
	notifier   Notifier
	spawner    Spawner

	healthPollInterval time.Duration
	entityPollInterval time.Duration
	configPushTimeout  time.Duration
	stepTimeout        func(*store.RunbookRunStepRecord) time.Duration

	mu     sync.Mutex
	active map[string]*activeRun
}

// activeRun is the registry entry of the goroutine that holds a run's slot:
// its cancel handle, and a channel closed once that goroutine has stopped.
type activeRun struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// New builds an executor.
func New(deps Deps) *Executor {
	return &Executor{
		store:              deps.Store,
		lifecycle:          deps.Lifecycle,
		configPush:         deps.ConfigPush,
		actions:            deps.Actions,
		entities:           deps.Entities,
		syncer:             deps.Sync,
		health:             deps.Health,
		grants:             deps.Grants,
		events:             deps.Events,
		notifier:           deps.Notifier,
		spawner:            deps.Spawner,
		healthPollInterval: HealthPollInterval,
		entityPollInterval: EntityPollInterval,
		configPushTimeout:  configPushTimeout,
		stepTimeout:        StepTimeout,
		active:             make(map[string]*activeRun),
	}
}

// FreezeSteps copies authored steps into the form a run stores, so later
// edits to the runbook cannot change what the run executes, including the
// config_push target value. A step without a kind is a lifecycle step. Only
// the wait kinds (sync_and_wait, wait_until_healthy and wait_for_entity) keep a
// timeout: the value stored on
// any other step (legacy rows hold the column default) is dropped, and a wait
// step without one gets DefaultStepTimeout.
func FreezeSteps(steps []*store.RunbookStepRecord) []*store.RunbookRunStepRecord {
	frozen := make([]*store.RunbookRunStepRecord, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			frozen = append(frozen, nil)
			continue
		}
		kind := step.Kind
		if kind == "" {
			kind = KindLifecycle
		}
		timeout := 0
		if hasTimeout(kind) {
			timeout = step.TimeoutSeconds
			if timeout <= 0 {
				timeout = int(DefaultStepTimeout / time.Second)
			}
		}
		action := ""
		if kind == KindConnectorAction {
			action = step.Action
		}
		frozen = append(frozen, &store.RunbookRunStepRecord{
			Kind:           kind,
			Title:          step.Title,
			ConnectorID:    step.ConnectorID,
			Verb:           step.Verb,
			Action:         action,
			EntityRef:      step.EntityRef,
			FieldKey:       step.FieldKey,
			TargetValue:    step.TargetValue,
			Attribute:      step.Attribute,
			Operator:       step.Operator,
			ExpectedValue:  step.ExpectedValue,
			TimeoutSeconds: timeout,
		})
	}
	return frozen
}

// hasTimeout reports whether steps of kind wait and therefore time out.
func hasTimeout(kind string) bool {
	return kind == KindSyncAndWait || kind == KindWaitUntilHealthy || kind == KindWaitForEntity
}

// StepTimeout returns how long a step may run. Only the wait kinds have a
// timeout; a lifecycle step is bounded by the
// connector call itself (zero), whatever its row holds.
func StepTimeout(step *store.RunbookRunStepRecord) time.Duration {
	if !hasTimeout(step.Kind) {
		return 0
	}
	if step.TimeoutSeconds > 0 {
		return time.Duration(step.TimeoutSeconds) * time.Second
	}
	return DefaultStepTimeout
}

// Start creates a run of runbookID from the given authored steps, records
// userID as its starter and begins executing without waiting for it. The
// caller has already validated the runbook.run elevation and the operator
// grant on every connector of steps. A second active run for the runbook is a
// *store.RunbookRunConflictError; no steps is store.ErrRunbookRunStepCount.
// After shutdown began the run is recorded as failed (interrupted) and the
// error is ErrShuttingDown.
func (e *Executor) Start(ctx context.Context, runbookID, userID string, steps []*store.RunbookStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error) {
	if userID == "" {
		return nil, nil, ErrNoActor
	}
	frozenSteps := FreezeSteps(steps)
	if err := e.freezeActionFingerprints(ctx, frozenSteps); err != nil {
		return nil, nil, err
	}
	run, frozen, err := e.store.CreateRunbookRun(ctx, runbookID, userID, frozenSteps)
	if err != nil {
		return nil, nil, err
	}
	e.publishRun(run)
	if !e.spawn(run.ID) {
		return nil, nil, e.failShutdown(ctx, run.ID)
	}
	return run, frozen, nil
}

// freezeActionFingerprints stores on every connector_action step the
// fingerprint of its action as the recipe declares it now, so the step can
// refuse to send a changed action later. It fails before anything is stored
// when a fingerprint cannot be computed or no action service is configured.
func (e *Executor) freezeActionFingerprints(ctx context.Context, steps []*store.RunbookRunStepRecord) error {
	for _, step := range steps {
		if step == nil || step.Kind != KindConnectorAction {
			continue
		}
		if e.actions == nil {
			return fmt.Errorf("%w: step %q runs action %q", ErrActionUnavailable, step.Title, step.Action)
		}
		fingerprint, err := e.actions.ActionFingerprint(ctx, step.ConnectorID, step.Action, step.EntityRef)
		if err != nil {
			slog.Warn("runbook run: connector action unavailable", "connector", logsafe.Sanitize(step.ConnectorID), "action", logsafe.Sanitize(step.Action), "error", logsafe.Err(err))
			return fmt.Errorf("%w: step %q runs action %q", ErrActionUnavailable, step.Title, step.Action)
		}
		step.ActionFingerprint = fingerprint
	}
	return nil
}

// Confirm completes the waiting manual step stepID as userID and continues the
// run. It is store.ErrConflict when the step is not waiting. After shutdown
// began the step stays confirmed, the run is recorded as failed (interrupted)
// and the error is ErrShuttingDown; resuming continues after the step.
func (e *Executor) Confirm(ctx context.Context, runID, stepID, userID string) error {
	if userID == "" {
		return ErrNoActor
	}
	if err := e.store.ConfirmRunbookRunStep(ctx, runID, stepID, userID); err != nil {
		return err
	}
	e.awaitStopped(ctx, runID)
	if run, steps, err := e.store.GetRunbookRun(ctx, runID); err == nil {
		for _, step := range steps {
			if step.ID == stepID {
				e.publishStep(run, step)
			}
		}
		e.publishRun(run)
	}
	if !e.spawn(runID) {
		return e.failShutdown(ctx, runID)
	}
	return nil
}

// Resume restarts a failed run from its first step that has not succeeded and
// records userID as the acting user for the steps that follow. It is
// store.ErrConflict for a run in any other state. After shutdown began the run
// is recorded as failed (interrupted) again and the error is ErrShuttingDown.
//
// When that first step is a connector_action step in state unknown, decision
// is required: ErrDecisionRequired without a decision, which changes nothing.
// ResumeResend starts the step again as any unknown step is started.
// ResumeMarkDone marks the step succeeded without sending anything and
// continues with the next step. The returned StepDecision reports the decision
// applied to that step and is nil otherwise; a decision is ignored for any
// other first step. A durable decision is returned even when shutdown prevents
// execution, so the caller can still audit the choice.
func (e *Executor) Resume(ctx context.Context, runID, userID string, decision ResumeDecision) (*store.RunbookRunRecord, *StepDecision, error) {
	return e.resume(ctx, runID, userID, decision, ResumeExpectation{}, false)
}

// ResumeExpecting is Resume tied to expect. ErrRunChanged means the run's
// updatedAt or first unfinished step differs from the expectation; it changes
// nothing. Unlike Resume, applying a decision to an unknown connector_action
// step requires both expect.StepID and expect.UpdatedAt
// (ErrDecisionFieldsRequired): the HTTP handler uses this one.
func (e *Executor) ResumeExpecting(ctx context.Context, runID, userID string, decision ResumeDecision, expect ResumeExpectation) (*store.RunbookRunRecord, *StepDecision, error) {
	return e.resume(ctx, runID, userID, decision, expect, true)
}

func (e *Executor) resume(ctx context.Context, runID, userID string, decision ResumeDecision, expect ResumeExpectation, requireFields bool) (*store.RunbookRunRecord, *StepDecision, error) {
	if userID == "" {
		return nil, nil, ErrNoActor
	}
	switch decision {
	case ResumeNone, ResumeResend, ResumeMarkDone:
	default:
		return nil, nil, ErrInvalidDecision
	}
	run, steps, err := e.store.GetRunbookRun(ctx, runID)
	if err != nil {
		return nil, nil, err
	}
	if expect.UpdatedAt != "" && run.UpdatedAt != expect.UpdatedAt {
		return nil, nil, ErrRunChanged
	}
	if run.State != RunFailed {
		return nil, nil, store.ErrConflict
	}
	step := firstUnfinished(steps)
	if expect.StepID != "" && (step == nil || step.ID != expect.StepID) {
		return nil, nil, ErrRunChanged
	}
	if step == nil || step.Kind != KindConnectorAction || step.State != StepUnknown {
		return e.resumeFailed(ctx, run, userID, nil, expect)
	}
	if decision == ResumeNone {
		return nil, nil, ErrDecisionRequired
	}
	if requireFields && (expect.StepID == "" || expect.UpdatedAt == "") {
		return nil, nil, ErrDecisionFieldsRequired
	}
	applied := &StepDecision{StepID: step.ID, Decision: decision}
	if decision == ResumeMarkDone {
		return e.resumeMarkingDone(ctx, run, step.ID, userID, applied, expect)
	}
	return e.resumeFailed(ctx, run, userID, applied, expect)
}

// expectedUpdatedAt is the revision the store transaction compares: the one
// the caller saw when it gave one, else the one the executor just read.
func expectedUpdatedAt(observed *store.RunbookRunRecord, expect ResumeExpectation) string {
	if expect.UpdatedAt != "" {
		return expect.UpdatedAt
	}
	return observed.UpdatedAt
}

// resumeConflict reports a store conflict as ErrRunChanged when the caller gave
// an expectation, since the run then no longer matches what the caller saw.
func resumeConflict(err error, expect ResumeExpectation) error {
	if errors.Is(err, store.ErrConflict) && (expect.UpdatedAt != "" || expect.StepID != "") {
		return ErrRunChanged
	}
	return err
}

// resumeFailed moves a failed run to running as userID and starts its
// goroutine. applied is returned unchanged.
func (e *Executor) resumeFailed(ctx context.Context, observed *store.RunbookRunRecord, userID string, applied *StepDecision, expect ResumeExpectation) (*store.RunbookRunRecord, *StepDecision, error) {
	var audit *store.AuditRecord
	if applied != nil {
		audit = expect.Audit
	}
	run, err := e.store.ResumeRunbookRun(ctx, observed.ID, expectedUpdatedAt(observed, expect), expect.StepID, userID, audit)
	if err != nil {
		return nil, nil, resumeConflict(err, expect)
	}
	e.awaitStopped(ctx, run.ID)
	e.publishRun(run)
	if !e.spawn(run.ID) {
		shutdown := e.failShutdown(ctx, run.ID)
		if applied == nil {
			return nil, nil, shutdown
		}
		return run, applied, shutdown
	}
	return run, applied, nil
}

// resumeMarkingDone marks the unknown step stepID succeeded and resumes the
// run as userID in one store transaction, then publishes both and starts the
// goroutine, which continues with the next step.
func (e *Executor) resumeMarkingDone(ctx context.Context, observed *store.RunbookRunRecord, stepID, userID string, applied *StepDecision, expect ResumeExpectation) (*store.RunbookRunRecord, *StepDecision, error) {
	run, err := e.store.ResumeRunbookRunMarkingStepDone(ctx, observed.ID, expectedUpdatedAt(observed, expect), stepID, userID, expect.Audit)
	if err != nil {
		return nil, nil, resumeConflict(err, expect)
	}
	e.awaitStopped(ctx, run.ID)
	if _, steps, err := e.store.GetRunbookRun(ctx, run.ID); err == nil {
		for _, step := range steps {
			if step.ID == stepID {
				e.publishStep(run, step)
			}
		}
	}
	e.publishRun(run)
	if !e.spawn(run.ID) {
		return run, applied, e.failShutdown(ctx, run.ID)
	}
	return run, applied, nil
}

// Cancel ends a running, waiting or failed run as userID and stops its
// goroutine. A step in flight is left unknown by the store and its result, if
// one still arrives, is discarded. It is store.ErrConflict for a finished run.
func (e *Executor) Cancel(ctx context.Context, runID, userID string) error {
	if userID == "" {
		return ErrNoActor
	}
	if err := e.store.CancelRunbookRun(ctx, runID, userID); err != nil {
		return err
	}
	e.cancelActive(runID)
	if run, steps, err := e.store.GetRunbookRun(ctx, runID); err == nil {
		// The store stamps the run and every step it closed with one time.
		for _, step := range steps {
			if step.FinishedAt == run.FinishedAt && (step.State == StepSkipped || step.State == StepUnknown) {
				e.publishStep(run, step)
			}
		}
		e.publishRun(run)
	}
	return nil
}

// spawn starts the goroutine that executes runID from its first step that has
// not succeeded and reports whether the spawner accepted it; false means
// shutdown has begun. The goroutine first takes the run's slot in the registry:
// while another goroutine holds it, it waits for that one to stop, or returns
// without executing when shutdown comes first. Only the slot holder is
// registered, and it leaves the registry before its done channel closes. Every
// transition is also guarded by the stored state.
func (e *Executor) spawn(runID string) bool {
	return e.spawner.TryGo(func(base context.Context) {
		ctx, cancel := context.WithCancel(base)
		defer cancel()
		handle := &activeRun{cancel: cancel, done: make(chan struct{})}
		if !e.claim(ctx, runID, handle) {
			return
		}
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
			close(handle.done)
		}()
		e.execute(ctx, runID)
	})
}

// claim registers handle as the holder of runID's slot, waiting for the
// current holder to stop first. It reports false when ctx ends before the slot
// is free.
func (e *Executor) claim(ctx context.Context, runID string, handle *activeRun) bool {
	for {
		e.mu.Lock()
		holder := e.active[runID]
		if holder == nil {
			e.active[runID] = handle
			e.mu.Unlock()
			return true
		}
		e.mu.Unlock()
		select {
		case <-holder.done:
		case <-ctx.Done():
			return false
		}
	}
}

// awaitStopped waits until the goroutine holding runID's slot has stopped, or
// until ctx is done. A resume or confirm calls it before publishing, so the
// previous goroutine's last events reach clients first. That goroutine has
// already stored its final state and only has events and a notification left.
func (e *Executor) awaitStopped(ctx context.Context, runID string) {
	e.mu.Lock()
	holder := e.active[runID]
	e.mu.Unlock()
	if holder == nil {
		return
	}
	select {
	case <-holder.done:
	case <-ctx.Done():
	}
}

// failShutdown records a run whose goroutine the spawner refused as failed with
// reason interrupted and returns ErrShuttingDown. The write is detached from
// ctx so a dropped request still records it. If it fails, startup recovery
// marks the run left running as interrupted anyway. It sends no notification:
// the caller gets the error and the dispatcher may already be draining.
func (e *Executor) failShutdown(ctx context.Context, runID string) error {
	dbCtx, cancel := detached(ctx)
	defer cancel()
	failed, err := e.store.UpdateRunbookRun(dbCtx, runID, RunRunning, map[string]any{"state": RunFailed, "reason": ReasonInterrupted})
	switch {
	case err == nil:
		e.publishRun(failed)
	case !stateChanged(err):
		slog.Error("runbook run: record shutdown failed", "run", logsafe.Sanitize(runID), "error", err)
	}
	return &ShutdownError{RunID: runID}
}

// cancelActive cancels the context of the goroutine holding runID's slot, if
// this process has one.
func (e *Executor) cancelActive(runID string) {
	e.mu.Lock()
	handle := e.active[runID]
	e.mu.Unlock()
	if handle != nil {
		handle.cancel()
	}
}

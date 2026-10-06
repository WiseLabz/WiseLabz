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
	"sync"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/connectors"
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
	// DefaultStepTimeout applies to a sync_and_wait or wait_until_healthy step
	// frozen without a timeout.
	DefaultStepTimeout = 5 * time.Minute
	// syncBusyRetryInterval paces sync_and_wait while another sync holds the
	// connector.
	syncBusyRetryInterval = 2 * time.Second
	// storeTimeout bounds one state read or write. Writes are detached from
	// the run's context so an outcome is still recorded while it is cancelled.
	storeTimeout = 15 * time.Second
	// healthStatusOnline is the only status wait_until_healthy accepts.
	healthStatusOnline = "online"
)

// ErrNoActor rejects a start, resume, confirm or cancel without a user.
var ErrNoActor = errors.New("runbook run: an acting user is required")

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
	CancelRunbookRun(ctx context.Context, id, cancelledBy string) error
}

// Lifecycle performs an already-authorized restart, start or stop with its
// failure alert and audit record. It does not check grants or elevation.
// *connectors.Handler satisfies it.
type Lifecycle interface {
	MutateLifecycleOp(ctx context.Context, connectorID, verb, entityRef string, actor connectors.LifecycleActor, extraAudit map[string]any) error
}

// Syncer runs one connector sync and blocks until it ends. *sync.Engine
// satisfies it.
type Syncer interface {
	RunSyncFields(ctx context.Context, connectorID, jobID string, fields []string) (*syncengine.RunResult, error)
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

// Notifier dispatches run notifications. *notifications.Dispatcher satisfies
// it.
type Notifier interface {
	NotifyRunbookRun(ctx context.Context, eventType, severity, connectorID, title, message string)
}

// Spawner starts tracked background work under a context that is cancelled on
// shutdown. *sync.Engine satisfies it, so shutdown waits for running steps.
type Spawner interface {
	Go(work func(context.Context))
}

// Deps are the executor's collaborators. Events and Notifier may be nil.
type Deps struct {
	Store     Store
	Lifecycle Lifecycle
	Sync      Syncer
	Health    HealthChecker
	Grants    Grants
	Events    Publisher
	Notifier  Notifier
	Spawner   Spawner
}

// Executor runs runbook runs, one goroutine per active run. It executes in
// this process only: a run belongs to the replica that accepted it.
type Executor struct {
	store     Store
	lifecycle Lifecycle
	syncer    Syncer
	health    HealthChecker
	grants    Grants
	events    Publisher
	notifier  Notifier
	spawner   Spawner

	healthPollInterval time.Duration
	syncBusyRetry      time.Duration
	stepTimeout        func(*store.RunbookRunStepRecord) time.Duration

	mu     sync.Mutex
	active map[string]*activeRun
}

// activeRun is the cancel handle of one run's goroutine.
type activeRun struct {
	cancel context.CancelFunc
}

// New builds an executor.
func New(deps Deps) *Executor {
	return &Executor{
		store:              deps.Store,
		lifecycle:          deps.Lifecycle,
		syncer:             deps.Sync,
		health:             deps.Health,
		grants:             deps.Grants,
		events:             deps.Events,
		notifier:           deps.Notifier,
		spawner:            deps.Spawner,
		healthPollInterval: HealthPollInterval,
		syncBusyRetry:      syncBusyRetryInterval,
		stepTimeout:        StepTimeout,
		active:             make(map[string]*activeRun),
	}
}

// FreezeSteps copies authored steps into the form a run stores, so later
// edits to the runbook cannot change what the run executes.
func FreezeSteps(steps []*store.RunbookStepRecord) []*store.RunbookRunStepRecord {
	frozen := make([]*store.RunbookRunStepRecord, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			frozen = append(frozen, nil)
			continue
		}
		frozen = append(frozen, &store.RunbookRunStepRecord{
			Kind:           step.Kind,
			Title:          step.Title,
			ConnectorID:    step.ConnectorID,
			Verb:           step.Verb,
			EntityRef:      step.EntityRef,
			TimeoutSeconds: step.TimeoutSeconds,
		})
	}
	return frozen
}

// StepTimeout returns how long a step may run. A lifecycle step without a
// timeout is bounded only by the connector call itself (zero).
func StepTimeout(step *store.RunbookRunStepRecord) time.Duration {
	if step.TimeoutSeconds > 0 {
		return time.Duration(step.TimeoutSeconds) * time.Second
	}
	if step.Kind == KindSyncAndWait || step.Kind == KindWaitUntilHealthy {
		return DefaultStepTimeout
	}
	return 0
}

// Start creates a run of runbookID from the given authored steps, records
// userID as its starter and begins executing without waiting for it. The
// caller has already validated the runbook.run elevation and the operator
// grant on every connector of steps. A second active run for the runbook is a
// *store.RunbookRunConflictError; no steps is store.ErrRunbookRunStepCount.
func (e *Executor) Start(ctx context.Context, runbookID, userID string, steps []*store.RunbookStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord, error) {
	if userID == "" {
		return nil, nil, ErrNoActor
	}
	run, frozen, err := e.store.CreateRunbookRun(ctx, runbookID, userID, FreezeSteps(steps))
	if err != nil {
		return nil, nil, err
	}
	e.publishRun(run)
	e.spawn(run.ID)
	return run, frozen, nil
}

// Confirm completes the waiting manual step stepID as userID and continues the
// run. It is store.ErrConflict when the step is not waiting.
func (e *Executor) Confirm(ctx context.Context, runID, stepID, userID string) error {
	if userID == "" {
		return ErrNoActor
	}
	if err := e.store.ConfirmRunbookRunStep(ctx, runID, stepID, userID); err != nil {
		return err
	}
	if run, steps, err := e.store.GetRunbookRun(ctx, runID); err == nil {
		for _, step := range steps {
			if step.ID == stepID {
				e.publishStep(run, step)
			}
		}
		e.publishRun(run)
	}
	e.spawn(runID)
	return nil
}

// Resume restarts a failed run from its first step that has not succeeded and
// records userID as the acting user for the steps that follow. It is
// store.ErrConflict for a run in any other state.
func (e *Executor) Resume(ctx context.Context, runID, userID string) (*store.RunbookRunRecord, error) {
	if userID == "" {
		return nil, ErrNoActor
	}
	run, err := e.store.UpdateRunbookRun(ctx, runID, RunFailed, map[string]any{"state": RunRunning, "resumed_by": userID})
	if err != nil {
		return nil, err
	}
	e.publishRun(run)
	e.spawn(run.ID)
	return run, nil
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
// not succeeded. Every transition is guarded by the stored state, so a second
// goroutine for the same run stops at its first read or write.
func (e *Executor) spawn(runID string) {
	e.spawner.Go(func(base context.Context) {
		ctx, cancel := context.WithCancel(base)
		defer cancel()
		handle := &activeRun{cancel: cancel}
		e.mu.Lock()
		e.active[runID] = handle
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			if e.active[runID] == handle {
				delete(e.active, runID)
			}
			e.mu.Unlock()
		}()
		e.execute(ctx, runID)
	})
}

// cancelActive cancels the context of runID's goroutine, if it has one here.
func (e *Executor) cancelActive(runID string) {
	e.mu.Lock()
	handle := e.active[runID]
	e.mu.Unlock()
	if handle != nil {
		handle.cancel()
	}
}

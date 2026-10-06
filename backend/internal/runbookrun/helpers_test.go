package runbookrun

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

const testWait = 5 * time.Second

// env is an executor over a real SQLite store with fake collaborators.
type env struct {
	t         *testing.T
	s         *store.Store
	lifecycle *fakeLifecycle
	sync      *fakeSync
	health    *fakeHealth
	events    *eventRecorder
	notes     *noteRecorder
	spawner   *testSpawner
	exec      *Executor
	// starter holds operator on every connector made with env.connector.
	starter string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	s := apitest.NewStore(t)
	e := &env{
		t:         t,
		s:         s,
		lifecycle: &fakeLifecycle{},
		sync:      &fakeSync{},
		health:    &fakeHealth{},
		events:    &eventRecorder{},
		notes:     &noteRecorder{},
		spawner:   newTestSpawner(t),
		starter:   apitest.NewUser(t, s, "viewer"),
	}
	e.exec = e.newExecutor(s)
	return e
}

// newExecutor builds an executor over st with the env's fakes and short
// polling intervals.
func (e *env) newExecutor(st Store) *Executor {
	exec := New(Deps{
		Store:     st,
		Lifecycle: e.lifecycle,
		Sync:      e.sync,
		Health:    e.health,
		Grants:    StoreGrants{Store: e.s},
		Events:    e.events,
		Notifier:  e.notes,
		Spawner:   e.spawner,
	})
	exec.healthPollInterval = time.Millisecond
	return exec
}

// connector creates a connector and grants the starter operator on it.
func (e *env) connector() string {
	e.t.Helper()
	rec := &store.ConnectorRecord{
		Name:     "run-" + uuid.NewString(),
		Category: "virtualization",
		Type:     "proxmox",
		URL:      "https://example.com",
	}
	if err := e.s.CreateConnector(context.Background(), rec); err != nil {
		e.t.Fatalf("CreateConnector() error: %v", err)
	}
	apitest.GrantConnectorRole(e.t, e.s, e.starter, rec.ID, "operator")
	return rec.ID
}

// operator creates another user holding operator on the given connectors.
func (e *env) operator(connectorIDs ...string) string {
	e.t.Helper()
	id := apitest.NewUser(e.t, e.s, "viewer")
	for _, connectorID := range connectorIDs {
		apitest.GrantConnectorRole(e.t, e.s, id, connectorID, "operator")
	}
	return id
}

// runbook stores a runbook with the given authored steps.
func (e *env) runbook(steps ...*store.RunbookStepRecord) (*store.RunbookRecord, []*store.RunbookStepRecord) {
	e.t.Helper()
	suffix := uuid.NewString()
	book, saved, err := e.s.CreateRunbookWithSteps(context.Background(), &store.RunbookRecord{
		Title:       "Runbook " + suffix,
		TargetType:  "change_type",
		TargetValue: suffix,
	}, steps)
	if err != nil {
		e.t.Fatalf("CreateRunbookWithSteps() error: %v", err)
	}
	return book, saved
}

// start stores a runbook with steps and starts a run of it as the starter.
func (e *env) start(steps ...*store.RunbookStepRecord) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	e.t.Helper()
	book, saved := e.runbook(steps...)
	run, frozen, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		e.t.Fatalf("Start() error: %v", err)
	}
	return run, frozen
}

// settle waits until every executor goroutine has stopped: the runs have
// paused, failed, finished or been cancelled.
func (e *env) settle() {
	e.t.Helper()
	e.spawner.wait(e.t)
}

func (e *env) get(runID string) (*store.RunbookRunRecord, []*store.RunbookRunStepRecord) {
	e.t.Helper()
	run, steps, err := e.s.GetRunbookRun(context.Background(), runID)
	if err != nil {
		e.t.Fatalf("GetRunbookRun() error: %v", err)
	}
	return run, steps
}

func stepStates(steps []*store.RunbookRunStepRecord) []string {
	states := make([]string, len(steps))
	for i, step := range steps {
		states[i] = step.State
	}
	return states
}

func lifecycleStep(connectorID, verb string) *store.RunbookStepRecord {
	return &store.RunbookStepRecord{Kind: KindLifecycle, Title: verb, ConnectorID: connectorID, Verb: verb, EntityRef: "100"}
}

func syncStep(connectorID string) *store.RunbookStepRecord {
	return &store.RunbookStepRecord{Kind: KindSyncAndWait, Title: "Sync", ConnectorID: connectorID, TimeoutSeconds: 300}
}

func healthStep(connectorID string) *store.RunbookStepRecord {
	return &store.RunbookStepRecord{Kind: KindWaitUntilHealthy, Title: "Wait", ConnectorID: connectorID, TimeoutSeconds: 300}
}

func manualStep(title string) *store.RunbookStepRecord {
	return &store.RunbookStepRecord{Kind: KindManual, Title: title}
}

// testSpawner runs work in tracked goroutines under a context the test can
// cancel to simulate shutdown, and can refuse work like an engine that is
// shutting down.
type testSpawner struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	refused atomic.Bool
}

func newTestSpawner(t *testing.T) *testSpawner {
	ctx, cancel := context.WithCancel(context.Background())
	sp := &testSpawner{ctx: ctx, cancel: cancel}
	t.Cleanup(func() {
		cancel()
		sp.wg.Wait()
	})
	return sp
}

// refuse makes TryGo drop work until accept is called.
func (sp *testSpawner) refuse() { sp.refused.Store(true) }

func (sp *testSpawner) accept() { sp.refused.Store(false) }

func (sp *testSpawner) TryGo(work func(context.Context)) bool {
	if sp.refused.Load() {
		return false
	}
	sp.wg.Add(1)
	go func() {
		defer sp.wg.Done()
		work(sp.ctx)
	}()
	return true
}

func (sp *testSpawner) wait(t *testing.T) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		sp.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testWait):
		t.Fatal("executor goroutines did not stop")
	}
}

// gate blocks a fake call until the test releases it.
type gate struct {
	entered chan struct{}
	release chan struct{}
}

func newGate() *gate {
	return &gate{entered: make(chan struct{}, 16), release: make(chan struct{})}
}

// block signals that the call is in flight, then waits for the release or
// for ctx to end.
func (g *gate) block(ctx context.Context) error {
	g.entered <- struct{}{}
	select {
	case <-g.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *gate) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-g.entered:
	case <-time.After(testWait):
		t.Fatal("the step did not start")
	}
}

type lifecycleCall struct {
	ConnectorID string
	Verb        string
	EntityRef   string
	Actor       connectors.LifecycleActor
	Audit       map[string]any
}

type fakeLifecycle struct {
	mu    sync.Mutex
	calls []lifecycleCall
	// fn decides the outcome of the n-th call (from 1); nil succeeds.
	fn func(ctx context.Context, n int, call lifecycleCall) error
}

func (f *fakeLifecycle) MutateLifecycleOp(ctx context.Context, connectorID, verb, entityRef string, actor connectors.LifecycleActor, extraAudit map[string]any) error {
	call := lifecycleCall{ConnectorID: connectorID, Verb: verb, EntityRef: entityRef, Actor: actor, Audit: extraAudit}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	n := len(f.calls)
	fn := f.fn
	f.mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx, n, call)
}

func (f *fakeLifecycle) snapshot() []lifecycleCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]lifecycleCall(nil), f.calls...)
}

type fakeSync struct {
	mu    sync.Mutex
	calls []string
	// fn decides the outcome of the n-th call (from 1); nil succeeds.
	fn func(ctx context.Context, n int) (*syncengine.RunResult, error)
}

func (f *fakeSync) RunSyncFieldsWhenFree(ctx context.Context, connectorID, _ string, _ []string) (*syncengine.RunResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, connectorID)
	n := len(f.calls)
	fn := f.fn
	f.mu.Unlock()
	if fn == nil {
		return &syncengine.RunResult{ConnectorID: connectorID, Status: "success"}, nil
	}
	return fn(ctx, n)
}

func (f *fakeSync) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type fakeHealth struct {
	mu    sync.Mutex
	calls []string
	// fn decides the outcome of the n-th call (from 1); nil reports online.
	fn func(ctx context.Context, n int) (string, error)
}

func (f *fakeHealth) CheckHealth(ctx context.Context, connectorID string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, connectorID)
	n := len(f.calls)
	fn := f.fn
	f.mu.Unlock()
	if fn == nil {
		return "online", nil
	}
	return fn(ctx, n)
}

func (f *fakeHealth) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// published is one WebSocket event: global, or scoped to a connector.
type published struct {
	ConnectorID string
	Type        string
	Event       Event
}

type eventRecorder struct {
	mu     sync.Mutex
	events []published
}

func (r *eventRecorder) Broadcast(eventType string, payload any) {
	r.BroadcastConnector("", eventType, payload)
}

func (r *eventRecorder) BroadcastConnector(connectorID, eventType string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, published{ConnectorID: connectorID, Type: eventType, Event: payload.(Event)})
}

func (r *eventRecorder) snapshot() []published {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]published(nil), r.events...)
}

type note struct {
	EventType   string
	Severity    string
	ConnectorID string
	ActorID     string
	Title       string
	Message     string
}

type noteRecorder struct {
	mu    sync.Mutex
	notes []note
}

func (r *noteRecorder) NotifyRunbookRun(_ context.Context, eventType, severity, connectorID, actorID, title, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notes = append(r.notes, note{EventType: eventType, Severity: severity, ConnectorID: connectorID, ActorID: actorID, Title: title, Message: message})
}

func (r *noteRecorder) snapshot() []note {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]note(nil), r.notes...)
}

package runbookrun

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/api/connectors"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncengine "github.com/WiseLabz/wiselabz/internal/sync"
)

type mockOrphanConnector struct {
	mu           sync.Mutex
	restartCalls int
	startCalls   int
	stopCalls    int
	pushCalls    int
}

func (m *mockOrphanConnector) Name() string     { return "mock-orphan" }
func (m *mockOrphanConnector) Type() string     { return "mock_orphan" }
func (m *mockOrphanConnector) Category() string { return "virtualization" }
func (m *mockOrphanConnector) Validate(_ context.Context, _ map[string]any) error {
	return nil
}
func (m *mockOrphanConnector) Fetch(_ context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{ServiceName: "mock-orphan"}, nil
}
func (m *mockOrphanConnector) Restart(_ context.Context, _ map[string]any, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.restartCalls++
	return nil
}
func (m *mockOrphanConnector) Start(_ context.Context, _ map[string]any, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalls++
	return nil
}
func (m *mockOrphanConnector) Stop(_ context.Context, _ map[string]any, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalls++
	return nil
}
func (m *mockOrphanConnector) WritableFields() []connector.ConfigField {
	return []connector.ConfigField{{Key: "setting", Type: "string", EntityScope: true}}
}
func (m *mockOrphanConnector) ConfigPush(_ context.Context, _ map[string]any, _, _ string, _ any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pushCalls++
	return nil
}
func (m *mockOrphanConnector) ConfigRead(_ context.Context, _ map[string]any, _, _ string) (any, error) {
	return "current", nil
}

func (m *mockOrphanConnector) totalCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restartCalls + m.startCalls + m.stopCalls
}

func (m *mockOrphanConnector) totalPushes() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pushCalls
}

func orphanEnv(t *testing.T, mock *mockOrphanConnector) (*env, string) {
	t.Helper()
	connector.AllowLoopbackForTest(t)
	e := newEnv(t)
	cfg := &config.Config{Encryption: config.EncryptionSettings{Key: "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="}}
	jwt := auth.NewService("test-secret-test-secret-test-secret", time.Hour, time.Hour)
	engine := syncengine.NewEngine(e.s, nil, nil, nil, cfg.Encryption.Key)
	handler := connectors.NewHandler(e.s, engine, cfg, jwt, nil)

	typ := "mock_orphan/" + t.Name()
	connector.Register(connector.TypeSchema{Type: typ, Name: "Mock Orphan", Category: "virtualization"},
		func(map[string]any) (connector.Connector, error) { return mock, nil })

	rec := &store.ConnectorRecord{
		Name:       "orphan-" + strconv.Itoa(int(time.Now().UnixNano())),
		Category:   "virtualization",
		Type:       "custom",
		URL:        "https://example.com",
		ConfigData: "{}",
		Enabled:    false,
		ManagedBy:  store.ManagedByConfigOrphaned,
	}
	if err := e.s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if err := e.s.UpdateConnector(context.Background(), rec.ID, map[string]any{
		"type":       typ,
		"managed_by": store.ManagedByConfigOrphaned,
	}); err != nil {
		t.Fatal(err)
	}
	apitest.GrantConnectorRole(t, e.s, e.starter, rec.ID, "operator")

	e.exec = New(Deps{
		Store:      e.s,
		Lifecycle:  handler,
		ConfigPush: handler,
		Entities:   StoreEntities{Store: e.s},
		Sync:       engine,
		Health:     e.health,
		Grants:     StoreGrants{Store: e.s},
		Events:     e.events,
		Notifier:   e.notes,
		Spawner:    e.spawner,
	})
	return e, rec.ID
}

func TestOrphanedConnectorLifecycleStepFailsAtExecution(t *testing.T) {
	mock := &mockOrphanConnector{}
	e, connID := orphanEnv(t, mock)

	book, saved := e.runbook(lifecycleStep(connID, "restart"))
	run, _, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		t.Fatalf("Start() error = %v, want run to start successfully", err)
	}
	e.settle()

	gotRun, gotSteps := e.get(run.ID)
	if gotRun.State != RunFailed {
		t.Fatalf("run state = %q, want %q", gotRun.State, RunFailed)
	}
	if gotRun.Reason != ReasonStepFailed {
		t.Fatalf("run reason = %q, want %q", gotRun.Reason, ReasonStepFailed)
	}
	if len(gotSteps) != 1 {
		t.Fatalf("steps count = %d, want 1", len(gotSteps))
	}
	if gotSteps[0].State != StepFailed {
		t.Fatalf("step state = %q, want %q", gotSteps[0].State, StepFailed)
	}
	const wantErr = "This connector was removed from config.yaml. Delete it or release it to the UI first."
	if !strings.Contains(gotSteps[0].Error, wantErr) {
		t.Fatalf("step error = %q, want it to contain %q", gotSteps[0].Error, wantErr)
	}
	if calls := mock.totalCalls(); calls != 0 {
		t.Fatalf("lifecycle executor calls = %d, want 0", calls)
	}
}

func TestOrphanedConnectorConfigPushStepFailsAtExecution(t *testing.T) {
	mock := &mockOrphanConnector{}
	e, connID := orphanEnv(t, mock)

	book, saved := e.runbook(configPushStep(connID, "setting", `"new_val"`))
	run, _, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		t.Fatalf("Start() error = %v, want run to start successfully", err)
	}
	e.settle()

	gotRun, gotSteps := e.get(run.ID)
	if gotRun.State != RunFailed {
		t.Fatalf("run state = %q, want %q", gotRun.State, RunFailed)
	}
	if gotRun.Reason != ReasonStepFailed {
		t.Fatalf("run reason = %q, want %q", gotRun.Reason, ReasonStepFailed)
	}
	if len(gotSteps) != 1 {
		t.Fatalf("steps count = %d, want 1", len(gotSteps))
	}
	if gotSteps[0].State != StepFailed {
		t.Fatalf("step state = %q, want %q", gotSteps[0].State, StepFailed)
	}
	const wantErr = "This connector was removed from config.yaml. Delete it or release it to the UI first."
	if !strings.Contains(gotSteps[0].Error, wantErr) {
		t.Fatalf("step error = %q, want it to contain %q", gotSteps[0].Error, wantErr)
	}
	if pushes := mock.totalPushes(); pushes != 0 {
		t.Fatalf("config push calls = %d, want 0", pushes)
	}
}

func TestOrphanedConnectorRunStartAccepted(t *testing.T) {
	mock := &mockOrphanConnector{}
	e, connID := orphanEnv(t, mock)

	book, saved := e.runbook(
		manualStep("Manual review"),
		lifecycleStep(connID, "restart"),
	)
	run, frozen, err := e.exec.Start(context.Background(), book.ID, e.starter, saved)
	if err != nil {
		t.Fatalf("Start() error = %v, want run to start successfully", err)
	}
	e.settle()

	gotRun, gotSteps := e.get(run.ID)
	if gotRun.State != RunWaitingManual {
		t.Fatalf("run state = %q, want %q", gotRun.State, RunWaitingManual)
	}
	if len(gotSteps) != 2 {
		t.Fatalf("steps count = %d, want 2", len(gotSteps))
	}
	if gotSteps[0].State != StepWaiting {
		t.Fatalf("step 0 state = %q, want %q", gotSteps[0].State, StepWaiting)
	}
	if gotSteps[1].State != StepPending {
		t.Fatalf("step 1 state = %q, want %q", gotSteps[1].State, StepPending)
	}
	if mock.totalCalls() != 0 {
		t.Fatalf("mock calls = %d, want 0", mock.totalCalls())
	}

	// Confirm manual step, which continues execution to the orphaned lifecycle step
	if err := e.exec.Confirm(context.Background(), run.ID, frozen[0].ID, e.starter); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	e.settle()

	gotRun, gotSteps = e.get(run.ID)
	if gotRun.State != RunFailed {
		t.Fatalf("run state = %q, want %q", gotRun.State, RunFailed)
	}
	if gotRun.Reason != ReasonStepFailed {
		t.Fatalf("run reason = %q, want %q", gotRun.Reason, ReasonStepFailed)
	}
	if gotSteps[1].State != StepFailed {
		t.Fatalf("step 1 state = %q, want %q", gotSteps[1].State, StepFailed)
	}
	const wantErr = "This connector was removed from config.yaml. Delete it or release it to the UI first."
	if !strings.Contains(gotSteps[1].Error, wantErr) {
		t.Fatalf("step 1 error = %q, want it to contain %q", gotSteps[1].Error, wantErr)
	}
	if mock.totalCalls() != 0 {
		t.Fatalf("lifecycle executor calls = %d, want 0", mock.totalCalls())
	}
}

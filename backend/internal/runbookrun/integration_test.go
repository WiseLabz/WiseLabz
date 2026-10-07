package runbookrun

import (
	"context"
	"encoding/json"
	"strconv"
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

// stepKindsConnector is a config-push connector with a settable memory field
// and a VM whose status reads "running" only some fetches after a push.
type stepKindsConnector struct {
	mu             sync.Mutex
	memory         int
	pushed         bool
	fetchesAfter   int
	pushGate       *gate
	pushes         []int
	runningFetches int
}

func (c *stepKindsConnector) Name() string     { return "step-kinds" }
func (c *stepKindsConnector) Type() string     { return "step_kinds" }
func (c *stepKindsConnector) Category() string { return "networking" }
func (c *stepKindsConnector) Validate(context.Context, map[string]any) error {
	return nil
}

func (c *stepKindsConnector) Fetch(ctx context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	status := "stopped"
	if c.pushed {
		c.fetchesAfter++
		if c.fetchesAfter >= c.runningFetches {
			status = "running"
		}
	}
	memory, _ := json.Marshal(c.memory)
	return &connector.ServiceSnapshot{
		ServiceName: "step kinds",
		FetchedAt:   time.Now(),
		Sections:    []connector.SnapshotSection{{Title: "memory", Content: string(memory)}},
		Entities: []connector.SnapshotEntity{
			{Kind: "vm", Name: "guest", ExternalID: "100", Attributes: map[string]any{"status": status}},
		},
	}, nil
}

func (c *stepKindsConnector) WritableFields() []connector.ConfigField {
	return []connector.ConfigField{{Key: "memory", Type: "number", EntityScope: true}}
}

func (c *stepKindsConnector) ConfigPush(ctx context.Context, _ map[string]any, _, _ string, value any) error {
	if c.pushGate != nil {
		c.pushGate.entered <- struct{}{}
		<-c.pushGate.release
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	n := int(value.(float64))
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memory = n
	c.pushed = true
	c.pushes = append(c.pushes, n)
	return nil
}

func (c *stepKindsConnector) ConfigRead(context.Context, map[string]any, string, string) (any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.memory, nil
}

func (c *stepKindsConnector) current() (memory int, pushes []int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.memory, append([]int(nil), c.pushes...)
}

// stepKindsEnv wires the executor to the real connectors handler, sync engine
// and store around the fake connector.
func stepKindsEnv(t *testing.T, fake *stepKindsConnector) (*env, string) {
	t.Helper()
	connector.AllowLoopbackForTest(t)
	e := newEnv(t)
	cfg := &config.Config{Encryption: config.EncryptionSettings{Key: "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="}}
	jwt := auth.NewService("test-secret-test-secret-test-secret", time.Hour, time.Hour)
	engine := syncengine.NewEngine(e.s, nil, nil, nil, cfg.Encryption.Key)
	handler := connectors.NewHandler(e.s, engine, cfg, jwt, nil)

	typ := "step_kinds/" + t.Name()
	connector.Register(connector.TypeSchema{Type: typ, Name: "Step kinds", Category: "networking"},
		func(map[string]any) (connector.Connector, error) { return fake, nil })
	rec := &store.ConnectorRecord{Name: "run-" + strconv.Itoa(int(time.Now().UnixNano())), Category: "networking", Type: "custom", URL: "https://example.com", ConfigData: "{}", Enabled: true}
	if err := e.s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if err := e.s.UpdateConnector(context.Background(), rec.ID, map[string]any{"type": typ}); err != nil {
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
	e.exec.entityPollInterval = 5 * time.Millisecond
	return e, rec.ID
}

func auditDetails(t *testing.T, s *store.Store, action string) []map[string]any {
	t.Helper()
	rows, _, err := s.ListAuditRecords(context.Background(), action, "connector", "", "", 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	details := make([]map[string]any, len(rows))
	for i, row := range rows {
		if err := json.Unmarshal([]byte(row.Detail), &details[i]); err != nil {
			t.Fatal(err)
		}
	}
	return details
}

// TestConfigPushThenWaitForEntityRun runs a config_push followed by a
// wait_for_entity through the real push core, sync engine and entity loader
// against a fake connector.
func TestConfigPushThenWaitForEntityRun(t *testing.T) {
	fake := &stepKindsConnector{memory: 2048, runningFetches: 4}
	e, connectorID := stepKindsEnv(t, fake)

	run, frozen := e.start(
		configPushStep(connectorID, "memory", `4096`),
		waitEntityStep(connectorID, "status", "eq", `"running"`),
	)
	e.settle()

	got, steps := e.get(run.ID)
	if got.State != RunSucceeded {
		t.Fatalf("run = %+v, steps = %+v; want succeeded", got, steps)
	}
	if memory, pushes := fake.current(); memory != 4096 || len(pushes) != 1 {
		t.Fatalf("memory = %d after pushes %v, want 4096 written once", memory, pushes)
	}
	details := auditDetails(t, e.s, "connector.configPush")
	if len(details) != 1 || details[0]["runId"] != run.ID || details[0]["stepId"] != frozen[0].ID || details[0]["fieldKey"] != "memory" || details[0]["entityRef"] != "100" {
		t.Fatalf("config push audit = %v, want one entry carrying the run and step", details)
	}
	if alerts, _, err := e.s.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10); err != nil || len(alerts) != 0 {
		t.Fatalf("alerts = %v, err = %v; want none", alerts, err)
	}

	// A second run finds the field already at target: no write, no new audit.
	again, _ := e.start(configPushStep(connectorID, "memory", `4096`))
	e.settle()
	if got, _ := e.get(again.ID); got.State != RunSucceeded {
		t.Fatalf("second run = %+v, want succeeded", got)
	}
	if _, pushes := fake.current(); len(pushes) != 1 {
		t.Fatalf("pushes = %v, want no second write", pushes)
	}
	if details := auditDetails(t, e.s, "connector.configPush"); len(details) != 1 {
		t.Fatalf("config push audit = %v, want no entry for the skipped write", details)
	}
}

// TestCancelDuringConfigPushFinishesTheCore cancels a run while the connector
// write is in flight: the core must still verify and audit, and must not raise
// a mismatch alert for the abandoned context.
func TestCancelDuringConfigPushFinishesTheCore(t *testing.T) {
	fake := &stepKindsConnector{memory: 2048, runningFetches: 1, pushGate: newGate()}
	e, connectorID := stepKindsEnv(t, fake)

	run, frozen := e.start(configPushStep(connectorID, "memory", `4096`), waitEntityStep(connectorID, "status", "eq", `"running"`))
	fake.pushGate.waitEntered(t)
	if err := e.exec.Cancel(context.Background(), run.ID, e.starter); err != nil {
		t.Fatal(err)
	}
	close(fake.pushGate.release)
	e.settle()

	if got, _ := e.get(run.ID); got.State != RunCancelled {
		t.Fatalf("run = %+v, want cancelled", got)
	}
	if memory, _ := fake.current(); memory != 4096 {
		t.Fatalf("memory = %d, want the write kept", memory)
	}
	details := auditDetails(t, e.s, "connector.configPush")
	if len(details) != 1 || details[0]["runId"] != run.ID || details[0]["stepId"] != frozen[0].ID {
		t.Fatalf("config push audit = %v, want the completed write audited", details)
	}
	if alerts, _, err := e.s.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10); err != nil || len(alerts) != 0 {
		t.Fatalf("alerts = %v, err = %v; want no alert for a cancelled run", alerts, err)
	}
	if e.sync.count() != 0 {
		t.Fatal("the wait ran after the cancel")
	}
}

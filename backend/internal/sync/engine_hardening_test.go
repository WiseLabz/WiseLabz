package sync

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// sequentialConnector returns successive snapshots from a queue, one per Fetch call,
// and records the config map it was called with.
type sequentialConnector struct {
	snapshots  []*connector.ServiceSnapshot
	i          int
	lastConfig map[string]any
}

func (f *sequentialConnector) Name() string                                       { return "sequential" }
func (f *sequentialConnector) Type() string                                       { return "sync_test_sequential" }
func (f *sequentialConnector) Category() string                                   { return "test" }
func (f *sequentialConnector) Validate(_ context.Context, _ map[string]any) error { return nil }
func (f *sequentialConnector) Fetch(_ context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	f.lastConfig = config
	sn := f.snapshots[f.i]
	if f.i < len(f.snapshots)-1 {
		f.i++
	}
	return sn, nil
}

func TestRunSyncFlagsRepeatDriftAndBumpsSeverity(t *testing.T) {
	seq := &sequentialConnector{snapshots: []*connector.ServiceSnapshot{
		{ServiceName: "svc", Sections: []connector.SnapshotSection{{Title: "Foo", Content: "b"}}, FetchedAt: time.Now()},
		{ServiceName: "svc", Sections: []connector.SnapshotSection{{Title: "Foo", Content: "c"}}, FetchedAt: time.Now()},
	}}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_sequential", Category: "test", Name: "Sequential"},
		func(_ map[string]any) (connector.Connector, error) { return seq, nil },
	)

	s := newTestStore(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "svc", Category: "networking", Type: "sync_test_sequential", Enabled: true}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{
		ConnectorID: conn.ID,
		Data:        `{"serviceName":"svc","sections":[{"title":"Foo","content":"a"}]}`,
		FetchedAt:   time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	engine := NewEngine(s, nil, nil, nil)

	if _, err := engine.RunSync(ctx, conn.ID, "job1"); err != nil {
		t.Fatalf("RunSync (1st): %v", err)
	}
	if _, err := engine.RunSync(ctx, conn.ID, "job2"); err != nil {
		t.Fatalf("RunSync (2nd): %v", err)
	}

	changes, _, err := s.ListChanges(ctx, conn.ID, "", 0, 10)
	if err != nil {
		t.Fatalf("ListChanges: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(changes))
	}

	// detected_at has only second resolution, so the two changes can tie on
	// ORDER BY — find the repeat by content (it's the one that mentions
	// "recurring") rather than assuming list order.
	var repeat, first store.ChangeRecord
	for _, c := range changes {
		if strings.Contains(c.Summary, "recurring") {
			repeat = c
		} else {
			first = c
		}
	}

	if repeat.PatternID == "" || repeat.PatternID != first.PatternID {
		t.Fatalf("pattern IDs = %q, %q, want equal and non-empty", repeat.PatternID, first.PatternID)
	}
	if !strings.Contains(repeat.Summary, "recurring") {
		t.Errorf("repeat change summary = %q, want it to mention recurring", repeat.Summary)
	}
	if repeat.Severity != "warning" {
		t.Errorf("repeat change severity = %q, want warning (bumped from info)", repeat.Severity)
	}
	if first.Severity != "info" {
		t.Errorf("first change severity = %q, want info (unmodified, no prior pattern)", first.Severity)
	}
}

func TestRunSyncRefusesExpiredCredentialsWithoutRefresher(t *testing.T) {
	connector.Register(
		connector.TypeSchema{Type: "sync_test_expired_noop", Category: "test", Name: "Expired"},
		func(_ map[string]any) (connector.Connector, error) {
			return &fakeConnector{snapshot: &connector.ServiceSnapshot{ServiceName: "svc", FetchedAt: time.Now()}}, nil
		},
	)
	s := newTestStore(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{
		Name: "svc", Category: "networking", Type: "sync_test_expired_noop", Enabled: true,
		CredentialExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	result, err := NewEngine(s, nil, nil, nil).RunSync(ctx, conn.ID, "job1")
	if err == nil {
		t.Fatal("RunSync() = nil error, want auth error for expired credentials")
	}
	if !strings.Contains(err.Error(), "auth error") {
		t.Errorf("error = %v, want it to mention auth error", err)
	}
	if result.Status != "error" {
		t.Errorf("result.Status = %q, want error", result.Status)
	}

	got, err := s.GetConnector(ctx, conn.ID)
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.Status != "offline" {
		t.Errorf("connector status = %q, want offline", got.Status)
	}
}

// refreshableConnector implements both connector.Connector and
// connector.CredentialRefresher.
type refreshableConnector struct {
	fakeConnector
	newExpiry time.Time
}

func (r *refreshableConnector) RefreshCredentials(_ context.Context, config map[string]any) (map[string]any, time.Time, error) {
	refreshed := make(map[string]any, len(config)+1)
	for k, v := range config {
		refreshed[k] = v
	}
	refreshed["token"] = "refreshed-token"
	return refreshed, r.newExpiry, nil
}

func TestRunSyncRefreshesExpiredCredentials(t *testing.T) {
	newExpiry := time.Now().Add(24 * time.Hour)
	rc := &refreshableConnector{
		fakeConnector: fakeConnector{snapshot: &connector.ServiceSnapshot{ServiceName: "svc", FetchedAt: time.Now()}},
		newExpiry:     newExpiry,
	}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_refreshable", Category: "test", Name: "Refreshable"},
		func(_ map[string]any) (connector.Connector, error) { return rc, nil },
	)
	s := newTestStore(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{
		Name: "svc", Category: "networking", Type: "sync_test_refreshable", Enabled: true,
		CredentialExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
		ConfigData:          `{"token":"stale-token"}`,
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	result, err := NewEngine(s, nil, nil, nil).RunSync(ctx, conn.ID, "job1")
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("result.Status = %q, want success", result.Status)
	}

	got, err := s.GetConnector(ctx, conn.ID)
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.IsCredentialExpired(time.Now()) {
		t.Error("credentials still report expired after refresh")
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(got.ConfigData), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg["token"] != "refreshed-token" {
		t.Errorf("config_data token = %v, want refreshed-token (persisted from RefreshCredentials)", cfg["token"])
	}
}

func TestRunSyncFieldsPassesHintToConnector(t *testing.T) {
	seq := &sequentialConnector{snapshots: []*connector.ServiceSnapshot{{ServiceName: "svc", FetchedAt: time.Now()}}}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_fields_hint", Category: "test", Name: "FieldsHint"},
		func(_ map[string]any) (connector.Connector, error) { return seq, nil },
	)
	s := newTestStore(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "svc", Category: "networking", Type: "sync_test_fields_hint", Enabled: true}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	if _, err := NewEngine(s, nil, nil, nil).RunSyncFields(ctx, conn.ID, "job1", []string{"vms", "storage"}); err != nil {
		t.Fatalf("RunSyncFields: %v", err)
	}

	got := connector.RequestedFields(seq.lastConfig)
	if len(got) != 2 || got[0] != "vms" || got[1] != "storage" {
		t.Errorf("connector received fields = %v, want [vms storage]", got)
	}
}

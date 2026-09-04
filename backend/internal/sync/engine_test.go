package sync

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	_ "modernc.org/sqlite"
)

// fakeNotifier records NotifyAlertCreated calls.
type fakeNotifier struct {
	mu    sync.Mutex
	calls []string // alert IDs
}

func (f *fakeNotifier) NotifyAlertCreated(_ context.Context, alertID, _, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, alertID)
}

func (f *fakeNotifier) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// fakeConnector returns a fixed snapshot regardless of config.
type fakeConnector struct {
	snapshot *connector.ServiceSnapshot
}

func (f *fakeConnector) Name() string     { return "fake" }
func (f *fakeConnector) Type() string     { return "sync_test_fake" }
func (f *fakeConnector) Category() string { return "test" }
func (f *fakeConnector) Fetch(_ context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	return f.snapshot, nil
}
func (f *fakeConnector) Validate(_ context.Context, _ map[string]any) error { return nil }

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + dir + "/test.db?cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() }) //nolint:errcheck

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	if err := store.RunMigrations(db, "sqlite", logger); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return store.New(db, "sqlite")
}

// TestRunSync_NotifiesOnEligibleAlert verifies that RunSync calls the
// AlertNotifier exactly once per newly created (non-info) alert, and not at
// all when the only detected changes are info-severity.
func TestRunSync_NotifiesOnEligibleAlert(t *testing.T) {
	nextSnapshot := &connector.ServiceSnapshot{
		ServiceName: "svc",
		Sections:    []connector.SnapshotSection{}, // section removed vs prev -> "warning" alert
		FetchedAt:   time.Now(),
	}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_fake_a", Category: "test", Name: "Fake A"},
		func(_ map[string]any) (connector.Connector, error) {
			return &fakeConnector{snapshot: nextSnapshot}, nil
		},
	)

	s := newTestStore(t)
	ctx := context.Background()

	conn := &store.ConnectorRecord{
		Name: "svc", Category: "networking", Type: "sync_test_fake_a", Enabled: true,
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	prevSnapshot := `{"serviceName":"svc","sections":[{"title":"Ports","content":"22,80"}]}`
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{
		ConnectorID: conn.ID, Data: prevSnapshot, FetchedAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	notifier := &fakeNotifier{}
	engine := NewEngine(s, nil, notifier)

	result, err := engine.RunSync(ctx, conn.ID, "job1")
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if result.AlertsCount != 1 {
		t.Fatalf("expected 1 alert, got %d", result.AlertsCount)
	}
	if got := notifier.count(); got != 1 {
		t.Fatalf("expected notifier called once, got %d", got)
	}
}

// TestRunSync_NoNotifyOnInfoOnlyChange verifies info-severity changes (e.g. a
// newly added section) do not create an alert or notify, matching the
// existing "non-info" eligibility rule in RunSync.
func TestRunSync_NoNotifyOnInfoOnlyChange(t *testing.T) {
	nextSnapshot := &connector.ServiceSnapshot{
		ServiceName: "svc",
		Sections:    []connector.SnapshotSection{{Title: "Ports", Content: "22,80"}, {Title: "New", Content: "x"}},
		FetchedAt:   time.Now(),
	}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_fake_b", Category: "test", Name: "Fake B"},
		func(_ map[string]any) (connector.Connector, error) {
			return &fakeConnector{snapshot: nextSnapshot}, nil
		},
	)

	s := newTestStore(t)
	ctx := context.Background()

	conn := &store.ConnectorRecord{
		Name: "svc", Category: "networking", Type: "sync_test_fake_b", Enabled: true,
	}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatalf("create connector: %v", err)
	}

	prevSnapshot := `{"serviceName":"svc","sections":[{"title":"Ports","content":"22,80"}]}`
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{
		ConnectorID: conn.ID, Data: prevSnapshot, FetchedAt: time.Now().Add(-time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	notifier := &fakeNotifier{}
	engine := NewEngine(s, nil, notifier)

	result, err := engine.RunSync(ctx, conn.ID, "job1")
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if result.AlertsCount != 0 {
		t.Fatalf("expected 0 alerts for info-only change, got %d", result.AlertsCount)
	}
	if got := notifier.count(); got != 0 {
		t.Fatalf("expected notifier not called, got %d", got)
	}
}

package health

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/store/storetest"
)

type fakeConn struct {
	typ      string
	err      error
	hang     bool
	validate atomic.Int32
}

func (f *fakeConn) Name() string     { return "fake" }
func (f *fakeConn) Type() string     { return f.typ }
func (f *fakeConn) Category() string { return "test" }
func (f *fakeConn) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return nil, errors.New("unused")
}

func (f *fakeConn) Validate(ctx context.Context, _ map[string]any) error {
	f.validate.Add(1)
	if f.hang {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+storetest.MigratedSQLite(t)+"?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store.New(db, "sqlite")
}

func register(t *testing.T, f *fakeConn) string {
	t.Helper()
	f.typ = "health_pkg_fake/" + t.Name()
	connector.Register(connector.TypeSchema{Type: f.typ, Category: "test", Name: "Fake"},
		func(map[string]any) (connector.Connector, error) { return f, nil })
	return f.typ
}

func seed(t *testing.T, s *store.Store, typ, name string, enabled bool) *store.ConnectorRecord {
	t.Helper()
	c := &store.ConnectorRecord{Name: name, Category: "networking", Type: typ, URL: "https://example.com", Enabled: enabled}
	if err := s.CreateConnector(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	return c
}

func count(t *testing.T, s *store.Store, id string) int {
	t.Helper()
	now := time.Now().UTC()
	st, err := s.GetConnectorUptime(context.Background(), id, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return st.CheckCount
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRunHealthCheckRecordsAndUpdatesStatus(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{err: errors.New("boom")})
	c := seed(t, s, typ, "svc", true)

	res, err := RunHealthCheck(context.Background(), s, c, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "offline" || res.Message != "boom" {
		t.Fatalf("res = %+v", res)
	}
	got, _ := s.GetConnector(context.Background(), c.ID)
	if got.Status != "offline" {
		t.Errorf("persisted status = %q", got.Status)
	}
	if count(t, s, c.ID) != 1 {
		t.Error("expected one history row")
	}
}

func TestRunDueChecksSkipsDisabledAndMaintenance(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{})
	enabled := seed(t, s, typ, "enabled", true)
	disabled := seed(t, s, typ, "disabled", false)
	windowed := seed(t, s, typ, "windowed", true)
	now := time.Now().UTC()
	if err := s.CreateMaintenanceWindow(context.Background(), &store.MaintenanceWindowRecord{
		ConnectorID: windowed.ID, StartsAt: now.Add(-time.Minute).Format(time.RFC3339), EndsAt: now.Add(time.Hour).Format(time.RFC3339), CreatedBy: "u",
	}); err != nil {
		t.Fatal(err)
	}

	r := &Runner{Store: s}
	if err := r.RunDueChecks(context.Background(), discard()); err != nil {
		t.Fatal(err)
	}
	if count(t, s, enabled.ID) != 1 {
		t.Error("enabled connector should be checked")
	}
	if count(t, s, disabled.ID) != 0 {
		t.Error("disabled connector must not be checked")
	}
	if count(t, s, windowed.ID) != 0 {
		t.Error("connector in maintenance must not be checked")
	}
}

func TestRunDueChecksTimeoutIsolation(t *testing.T) {
	s := newStore(t)
	hangTyp := register(t, &fakeConn{hang: true})
	hang := seed(t, s, hangTyp, "hang", true)

	// Second type registered under a distinct name via a sub-test name.
	var okTyp string
	t.Run("ok", func(t *testing.T) { okTyp = register(t, &fakeConn{}) })
	ok := seed(t, s, okTyp, "ok", true)

	r := &Runner{Store: s, MaxConcurrency: 2, CheckTimeout: 100 * time.Millisecond}
	start := time.Now()
	if err := r.RunDueChecks(context.Background(), discard()); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("sweep was not bounded by the per-check timeout")
	}
	if count(t, s, ok.ID) != 1 {
		t.Error("healthy connector should still be checked")
	}
	// The hung check times out and is recorded as offline.
	if count(t, s, hang.ID) != 1 {
		t.Error("timed-out check should be recorded")
	}
}

func TestRunHealthCheckParentCancelWritesNothing(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{hang: true})
	c := seed(t, s, typ, "svc", true)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	_, err := RunHealthCheck(ctx, s, c, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if count(t, s, c.ID) != 0 {
		t.Error("cancelled check must not record a history row")
	}
	got, _ := s.GetConnector(context.Background(), c.ID)
	if got.Status != c.Status || got.StatusMessage != c.StatusMessage {
		t.Errorf("status changed to %q/%q on cancel", got.Status, got.StatusMessage)
	}
}

func TestRunHealthCheckTimeoutRecordsOffline(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{hang: true})
	c := seed(t, s, typ, "svc", true)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res, err := RunHealthCheck(ctx, s, c, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "offline" {
		t.Errorf("status = %q, want offline", res.Status)
	}
	if count(t, s, c.ID) != 1 {
		t.Error("timed-out check should be recorded")
	}
	got, _ := s.GetConnector(context.Background(), c.ID)
	if got.Status != "offline" {
		t.Errorf("persisted status = %q, want offline", got.Status)
	}
}

func TestRunHealthCheckSkipsUnchangedStatusWrite(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{err: errors.New("boom")})
	c := seed(t, s, typ, "svc", true)

	if _, err := RunHealthCheck(context.Background(), s, c, ""); err != nil {
		t.Fatal(err)
	}
	first, _ := s.GetConnector(context.Background(), c.ID)
	time.Sleep(1100 * time.Millisecond) // updated_at has 1s resolution
	if _, err := RunHealthCheck(context.Background(), s, first, ""); err != nil {
		t.Fatal(err)
	}
	second, _ := s.GetConnector(context.Background(), c.ID)
	if second.UpdatedAt != first.UpdatedAt {
		t.Errorf("updated_at bumped %q -> %q despite unchanged status", first.UpdatedAt, second.UpdatedAt)
	}
	if count(t, s, c.ID) != 2 {
		t.Error("history row must still be recorded")
	}
}

func TestRunHealthCheckKeepsSyncReportedOffline(t *testing.T) {
	s := newStore(t)
	typ := register(t, &fakeConn{})
	ctx := context.Background()
	rec := seed(t, s, typ, "probe", true)
	rec.Status, rec.StatusMessage = "offline", connector.AllTargetsUnreachable(2)
	if err := s.UpdateConnector(ctx, rec.ID, map[string]any{"status": rec.Status, "status_message": rec.StatusMessage}); err != nil {
		t.Fatal(err)
	}
	// Validate passes, but it cannot see that every target failed to answer.
	res, err := RunHealthCheck(ctx, s, rec, "")
	if err != nil || res.Status != "offline" || res.Message != "All 2 targets unreachable" {
		t.Fatalf("RunHealthCheck = %+v, %v; want the sync-reported offline kept", res, err)
	}
	// Any other offline status still recovers when Validate passes.
	rec.StatusMessage = "connection refused"
	if res, err = RunHealthCheck(ctx, s, rec, ""); err != nil || res.Status != "online" {
		t.Fatalf("RunHealthCheck = %+v, %v; want online", res, err)
	}
}

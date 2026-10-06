package sync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
	"github.com/gorilla/websocket"
)

type baseCtxKey struct{}

func TestTryGoReportsAcceptanceUntilShutdown(t *testing.T) {
	e := NewEngine(nil, nil, nil, nil, "")
	e.SetBaseContext(context.WithValue(context.Background(), baseCtxKey{}, "base"))

	got := make(chan any, 1)
	if !e.TryGo(func(ctx context.Context) { got <- ctx.Value(baseCtxKey{}) }) {
		t.Fatal("TryGo before Wait must accept the work")
	}
	select {
	case v := <-got:
		if v != "base" {
			t.Fatalf("work ran under %v, want the base context", v)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("accepted work did not run")
	}

	if err := e.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ran := make(chan struct{}, 2)
	if e.TryGo(func(context.Context) { ran <- struct{}{} }) {
		t.Fatal("TryGo after Wait must refuse the work")
	}
	e.Go(func(context.Context) { ran <- struct{}{} })
	select {
	case <-ran:
		t.Fatal("work offered after Wait must never run")
	default:
	}
	if !e.Idle() {
		t.Fatal("refused work must not leave the engine busy")
	}
}

// watchConnectorEvents connects a websocket client allowed to read every
// connector event and returns the hub plus the envelopes it receives.
func watchConnectorEvents(t *testing.T) (*ws.Hub, <-chan ws.Envelope) {
	t.Helper()
	hub := ws.NewHub()
	hub.SetConnectorAudience(func(context.Context, string) ([]string, error) { return []string{"watcher"}, nil })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go hub.Run(ctx)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = hub.UpgradeHandler(w, r, ws.Identity{UserID: "watcher", Role: "admin"})
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	// Registration is asynchronous to the dial; connector events are dropped
	// until the hub has the client.
	for deadline := time.Now().Add(5 * time.Second); hub.ClientCount() != 1; {
		if time.Now().After(deadline) {
			t.Fatal("websocket client did not register")
		}
		time.Sleep(time.Millisecond)
	}
	events := make(chan ws.Envelope, 256)
	go func() {
		defer close(events)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var env ws.Envelope
			if json.Unmarshal(data, &env) == nil {
				events <- env
			}
		}
	}()
	return hub, events
}

// eventsUntilMarker returns every envelope received before the named marker
// event, which the hub delivers in publish order.
func eventsUntilMarker(t *testing.T, events <-chan ws.Envelope, marker string) []ws.Envelope {
	t.Helper()
	var seen []ws.Envelope
	for {
		select {
		case env, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before marker %q", marker)
			}
			if env.Type == marker {
				return seen
			}
			seen = append(seen, env)
		case <-time.After(5 * time.Second):
			t.Fatalf("marker %q not delivered", marker)
		}
	}
}

func payloadOf(env ws.Envelope) map[string]any {
	p, _ := env.Payload.(map[string]any)
	return p
}

func newBlockedSync(t *testing.T, typ string, hub *ws.Hub) (*Engine, *store.ConnectorRecord, *blockingConnector, chan error) {
	t.Helper()
	s := newTestStore(t)
	c := &blockingConnector{entered: make(chan context.Context, 8), release: make(chan struct{})}
	connector.Register(connector.TypeSchema{Type: typ, Category: "networking"}, func(map[string]any) (connector.Connector, error) { return c, nil })
	rec := &store.ConnectorRecord{Name: "test", Type: typ, Category: "networking", Enabled: true}
	if err := s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	e := NewEngine(s, hub, nil, nil, "")
	first := make(chan error, 1)
	go func() { _, err := e.RunSync(context.Background(), rec.ID, "first"); first <- err }()
	select {
	case <-c.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first fetch did not start")
	}
	return e, rec, c, first
}

func TestRunSyncFieldsWhenFreeWaitsForInFlightSyncSilently(t *testing.T) {
	hub, events := watchConnectorEvents(t)
	e, rec, c, first := newBlockedSync(t, "when_free_wait", hub)

	type outcome struct {
		res *RunResult
		err error
	}
	waiter := make(chan outcome, 1)
	go func() {
		res, err := e.RunSyncFieldsWhenFree(context.Background(), rec.ID, "waiter", nil)
		waiter <- outcome{res, err}
	}()

	// The first sync still holds the slot, so the waiter can neither have
	// finished nor have started its own fetch.
	select {
	case o := <-waiter:
		t.Fatalf("waiter returned while a sync was in flight: %+v, %v", o.res, o.err)
	case <-c.entered:
		t.Fatal("waiter fetched while a sync was in flight")
	default:
	}

	hub.BroadcastConnector(rec.ID, "test.before-release", nil)
	close(c.release)

	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first sync = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first sync did not finish")
	}
	select {
	case o := <-waiter:
		if o.err != nil || o.res == nil || o.res.Status != "success" {
			t.Fatalf("waiter = %+v, %v; want its own successful sync", o.res, o.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("waiter did not run after the slot freed")
	}
	select {
	case <-c.entered:
	default:
		t.Fatal("waiter did not fetch on its own: connector fetched once, want twice")
	}
	select {
	case <-c.entered:
		t.Fatal("connector fetched more than twice")
	default:
	}

	hub.BroadcastConnector(rec.ID, "test.done", nil)
	before := eventsUntilMarker(t, events, "test.before-release")
	after := eventsUntilMarker(t, events, "test.done")
	for _, env := range append(before, after...) {
		if p := payloadOf(env); p["error"] != nil {
			t.Errorf("unexpected error event %s: %v", env.Type, p)
		}
	}
	// Nothing about the waiter's job may be broadcast until its sync starts,
	// which cannot happen before the first sync is released.
	for _, env := range before {
		if payloadOf(env)["jobId"] == "waiter" {
			t.Errorf("waiter broadcast while waiting: %s %v", env.Type, payloadOf(env))
		}
	}
	started := false
	for _, env := range after {
		if payloadOf(env)["jobId"] == "waiter" {
			started = true
		}
	}
	if !started {
		t.Error("waiter's own sync never broadcast progress")
	}
	if !e.Idle() {
		t.Error("engine must be idle once both syncs have returned")
	}
}

func TestRunSyncFieldsWhenFreeStopsWaitingWithContext(t *testing.T) {
	e, rec, c, first := newBlockedSync(t, "when_free_ctx", nil)

	cancelled, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := e.RunSyncFieldsWhenFree(cancelled, rec.ID, "cancelled", nil); done <- err }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled wait = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled wait did not return")
	}

	expiring, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	go func() { _, err := e.RunSyncFieldsWhenFree(expiring, rec.ID, "expiring", nil); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expired wait = %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("expired wait did not return")
	}

	// The in-flight sync is untouched and keeps the slot.
	select {
	case err := <-first:
		t.Fatalf("first sync ended early: %v", err)
	default:
	}
	if e.Idle() {
		t.Fatal("first sync must still hold the slot")
	}
	if _, err := e.RunSyncFields(context.Background(), rec.ID, "contender", nil); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("RunSyncFields during the first sync = %v, want ErrAlreadyRunning", err)
	}

	close(c.release)
	select {
	case err := <-first:
		if err != nil {
			t.Fatalf("first sync = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first sync did not finish")
	}
	if !e.Idle() {
		t.Fatal("slot leaked after the waiters gave up")
	}
	if _, err := e.RunSyncFields(context.Background(), rec.ID, "after", nil); err != nil {
		t.Fatalf("sync after the waiters gave up = %v", err)
	}
}

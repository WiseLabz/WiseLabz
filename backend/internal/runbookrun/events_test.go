package runbookrun

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/notifications"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// frame is a decoded runbook.run.updated envelope.
type frame struct {
	Type        string `json:"type"`
	ConnectorID string `json:"connectorId"`
	Payload     Event  `json:"payload"`
}

// dialHub connects one WebSocket client with the given identity.
func dialHub(t *testing.T, hub *ws.Hub, id ws.Identity) *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := hub.UpgradeHandler(w, r, id); err != nil {
			t.Logf("UpgradeHandler error: %v", err)
		}
	}))
	t.Cleanup(server.Close)
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial hub: %v", err)
	}
	t.Cleanup(func() { conn.Close() }) //nolint:errcheck
	return conn
}

// readRunFrames reads runbook.run.updated frames until the run-level event
// for wantState arrives.
func readRunFrames(t *testing.T, conn *websocket.Conn, wantState string) []frame {
	t.Helper()
	var frames []frame
	if err := conn.SetReadDeadline(time.Now().Add(testWait)); err != nil {
		t.Fatal(err)
	}
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read frame (got %d so far): %v", len(frames), err)
		}
		var f frame
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("decode frame %s: %v", data, err)
		}
		if f.Type != ws.EventRunbookRunUpdated {
			continue
		}
		frames = append(frames, f)
		if f.Payload.Step == nil && f.Payload.State == wantState {
			return frames
		}
	}
}

func describeFrames(frames []frame, aliases map[string]string) []string {
	events := make([]published, len(frames))
	for i, f := range frames {
		events[i] = published{ConnectorID: f.ConnectorID, Type: f.Type, Event: f.Payload}
	}
	return describe(events, aliases)
}

// TestHubDeliversStepEventsOnlyToConnectorReaders drives a run through the
// real hub: a step event reaches only clients allowed to read the step's
// connector, while run-level and manual-step events reach everyone.
func TestHubDeliversStepEventsOnlyToConnectorReaders(t *testing.T) {
	e := newEnv(t)
	a, other := e.connector(), e.connector()

	hub := ws.NewHub()
	hub.SetConnectorAudience(e.s.ConnectorReaderIDs)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go hub.Run(ctx)
	e.exec.events = hub

	viewer := apitest.NewUser(t, e.s, "viewer")
	apitest.GrantConnectorRole(t, e.s, viewer, a, "viewer")
	stranger := apitest.NewUser(t, e.s, "viewer")
	granted := dialHub(t, hub, ws.Identity{UserID: viewer})
	ungranted := dialHub(t, hub, ws.Identity{UserID: stranger})
	// The starter can read the connector, but this API key is restricted to
	// another one.
	restricted := dialHub(t, hub, ws.Identity{UserID: e.starter, APIKeyID: "key", ConnectorIDs: []string{other}})
	eventually(t, "the clients to register", func() bool { return hub.ClientCount() == 3 })

	run, frozen := e.start(lifecycleStep(a, "restart"), manualStep("Check"))
	e.settle()

	aliases := map[string]string{a: "A"}
	wantGranted := []string{
		"all: run running",
		"A: step 0 running (run running)", "A: step 0 succeeded (run running)",
		"all: step 1 waiting (run waiting_manual)", "all: run waiting_manual",
	}
	frames := readRunFrames(t, granted, RunWaitingManual)
	if got := describeFrames(frames, aliases); !reflect.DeepEqual(got, wantGranted) {
		t.Fatalf("frames for a connector reader = %q, want %q", got, wantGranted)
	}
	// The event identifies the run, the step and the new state.
	if f := frames[2]; f.Payload.RunID != run.ID || f.Payload.Step.ID != frozen[0].ID || f.Payload.Step.State != StepSucceeded || f.ConnectorID != a {
		t.Fatalf("step transition frame = %+v", f)
	}

	wantHidden := []string{"all: run running", "all: step 1 waiting (run waiting_manual)", "all: run waiting_manual"}
	for name, conn := range map[string]*websocket.Conn{"no grant": ungranted, "restricted API key": restricted} {
		frames := readRunFrames(t, conn, RunWaitingManual)
		if got := describeFrames(frames, aliases); !reflect.DeepEqual(got, wantHidden) {
			t.Fatalf("frames for a client with %s = %q, want %q", name, got, wantHidden)
		}
		for _, f := range frames {
			if f.ConnectorID != "" || (f.Payload.Step != nil && f.Payload.Step.ID == frozen[0].ID) {
				t.Fatalf("client with %s received a frame about the hidden step: %+v", name, f)
			}
		}
	}
}

// TestRunNotificationsThroughDispatcher runs the executor against the real
// dispatcher: a failed run and a waiting run notify, a succeeded and a
// cancelled run do not.
func TestRunNotificationsThroughDispatcher(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	a := e.connector()
	dispatcher := notifications.NewDispatcher(e.s, nil)
	e.exec.notifier = dispatcher
	stranger := apitest.NewUser(t, e.s, "viewer")

	count := func(userID string) map[string]int {
		t.Helper()
		dispatcher.Wait()
		rows, _, err := e.s.ListNotifications(ctx, userID, false, 0, 50)
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]int{}
		for _, row := range rows {
			counts[row.EventType]++
		}
		return counts
	}

	// Succeeded: nothing.
	e.start(lifecycleStep(a, "restart"))
	e.settle()
	if got := count(e.starter); len(got) != 0 {
		t.Fatalf("notifications after a succeeded run = %v, want none", got)
	}

	// Failed: the users who can view the failed step's connector.
	e.lifecycle.fn = func(context.Context, int, lifecycleCall) error { return errors.New("boom") }
	failed, _ := e.start(lifecycleStep(a, "restart"))
	e.settle()
	if got := count(e.starter); !reflect.DeepEqual(got, map[string]int{notifications.EventRunbookRunFailed: 1}) {
		t.Fatalf("starter notifications after a failed run = %v", got)
	}
	if got := count(stranger); len(got) != 0 {
		t.Fatalf("a user without a grant on the connector was notified of the failed step: %v", got)
	}

	// Cancelled: nothing more.
	if err := e.exec.Cancel(ctx, failed.ID, e.starter); err != nil {
		t.Fatal(err)
	}
	if got := count(e.starter); !reflect.DeepEqual(got, map[string]int{notifications.EventRunbookRunFailed: 1}) {
		t.Fatalf("starter notifications after cancel = %v, want no new one", got)
	}

	// Waiting on a manual step: everyone, as the step has no connector.
	waiting, _ := e.start(manualStep("Check"))
	e.settle()
	want := map[string]int{notifications.EventRunbookRunFailed: 1, notifications.EventRunbookRunWaiting: 1}
	if got := count(e.starter); !reflect.DeepEqual(got, want) {
		t.Fatalf("starter notifications after a waiting run = %v, want %v", got, want)
	}
	if got := count(stranger); !reflect.DeepEqual(got, map[string]int{notifications.EventRunbookRunWaiting: 1}) {
		t.Fatalf("stranger notifications after a waiting run = %v", got)
	}
	if err := e.exec.Cancel(ctx, waiting.ID, e.starter); err != nil {
		t.Fatal(err)
	}
	if got := count(e.starter); !reflect.DeepEqual(got, want) {
		t.Fatalf("starter notifications after cancelling the waiting run = %v, want %v", got, want)
	}
}

// TestRunEventCarriesNoStepDetails pins the WebSocket payload to identifiers
// and states. A step on a connector the viewer cannot see is redacted over
// REST, down to its kind, so an event must never carry a step's kind, title,
// connector, verb, entity or error.
func TestRunEventCarriesNoStepDetails(t *testing.T) {
	bookID := "book"
	run := &store.RunbookRunRecord{ID: "run", RunbookID: &bookID, RunbookTitle: "Restart proxy", State: RunFailed, Reason: ReasonStepFailed, StartedBy: "starter"}
	step := &store.RunbookRunStepRecord{
		ID: "step", RunID: "run", Position: 1, Kind: KindLifecycle, Title: "Restart", ConnectorID: "connector",
		Verb: "restart", EntityRef: "100", TimeoutSeconds: 30, State: StepFailed, Error: "connection refused", ConfirmedBy: "someone",
	}
	events := &eventRecorder{}
	publishStep(events, run, step)
	publishRun(events, run)

	var got []string
	for _, p := range events.snapshot() {
		data, err := json.Marshal(p.Event)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, p.ConnectorID+" "+string(data))
	}
	want := []string{
		`connector {"runId":"run","runbookId":"book","state":"failed","reason":"step_failed","step":{"id":"step","position":1,"state":"failed"}}`,
		` {"runId":"run","runbookId":"book","state":"failed","reason":"step_failed"}`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q, want %q", got, want)
	}
}

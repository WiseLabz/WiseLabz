package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

const lifecycleReviewConnectorType = "lifecycle_review"

type lifecycleReviewConnectorState struct {
	mu       sync.Mutex
	restarts int
	failure  error
	cancel   context.CancelFunc
	// operation, when set, replaces the default outcome of Restart. It runs
	// outside the state lock so it may block until the context ends.
	operation func(context.Context) error
}

type lifecycleReviewConnector struct {
	state *lifecycleReviewConnectorState
	typ   string
}

func (c *lifecycleReviewConnector) Name() string     { return "lifecycle-review" }
func (c *lifecycleReviewConnector) Type() string     { return c.typ }
func (c *lifecycleReviewConnector) Category() string { return "test" }
func (c *lifecycleReviewConnector) Validate(context.Context, map[string]any) error {
	return nil
}
func (c *lifecycleReviewConnector) Fetch(context.Context, map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{ServiceName: "lifecycle-review"}, nil
}
func (c *lifecycleReviewConnector) Restart(ctx context.Context, _ map[string]any, _ string) error {
	c.state.mu.Lock()
	c.state.restarts++
	cancel, failure, operation := c.state.cancel, c.state.failure, c.state.operation
	c.state.mu.Unlock()
	if operation != nil {
		return operation(ctx)
	}
	if cancel != nil {
		cancel()
	}
	return failure
}

func newLifecycleReviewConnector(t *testing.T, h *Handler) (string, *lifecycleReviewConnectorState) {
	t.Helper()
	connectorType := lifecycleReviewConnectorType + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	state := &lifecycleReviewConnectorState{}
	connector.Register(
		connector.TypeSchema{Type: connectorType, Category: "networking", Name: "Lifecycle Review"},
		func(map[string]any) (connector.Connector, error) {
			return &lifecycleReviewConnector{state: state, typ: connectorType}, nil
		},
	)

	url := "https://lifecycle-review-" + uuid.NewString() + ".example.com"
	record := &store.ConnectorRecord{
		Name:       "Lifecycle Review",
		Category:   "networking",
		Type:       connectorType,
		URL:        url,
		ConfigData: "{}",
		Enabled:    true,
		VerifyTLS:  true,
	}
	if err := h.Store.CreateConnector(context.Background(), record); err != nil {
		t.Fatalf("CreateConnector() error = %v", err)
	}
	return record.ID, state
}

func lifecycleReviewAudit(t *testing.T, h *Handler, connectorID string) []store.AuditRecord {
	t.Helper()
	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error = %v", err)
	}
	for _, record := range records {
		if record.TargetID != connectorID {
			t.Fatalf("unexpected audit target %q, want %q", record.TargetID, connectorID)
		}
	}
	return records
}

func lifecycleReviewAlerts(t *testing.T, h *Handler, connectorID string) []store.AlertRecord {
	t.Helper()
	alerts, _, err := h.Store.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
	return alerts
}

func TestRestartPreviewAuditActorFromAuthenticatedContext(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		role       string
		admin      bool
		wantRecord string
	}{
		{name: "normal user", role: "viewer", wantRecord: "user"},
		{name: "instance admin", role: "operator", admin: true, wantRecord: "admin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t)
			connectorID, _ := newLifecycleReviewConnector(t, h)
			userID := apitest.NewUser(t, h.Store, tc.role)
			apitest.GrantConnectorRole(t, h.Store, userID, connectorID, "operator")
			elevation, err := h.JWT.IssueElevation(userID, "connector.restart")
			if err != nil {
				t.Fatalf("IssueElevation() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/connectors/"+connectorID+"/restart", strings.NewReader(`{"entityRef":"vm-100"}`))
			req.SetPathValue("id", connectorID)
			req.Header.Set("X-Elevation-Token", elevation.Token)
			req = req.WithContext(auth.ContextWithUser(req.Context(), userID, tc.admin))
			rr := httptest.NewRecorder()
			h.RestartPreview(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("RestartPreview() status = %d, body = %s; want 200", rr.Code, rr.Body.String())
			}

			records := lifecycleReviewAudit(t, h, connectorID)
			if len(records) != 1 {
				t.Fatalf("restart audits = %+v, want one row", records)
			}
			if records[0].ActorUserID != userID || records[0].ActorRole != tc.wantRecord {
				t.Errorf("audit actor = %q/%q, want %q/%s", records[0].ActorUserID, records[0].ActorRole, userID, tc.wantRecord)
			}
		})
	}
}

func TestServeLifecycleOpAuditPreservesEntityRefAndExtraAudit(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID, _ := newLifecycleReviewConnector(t, h)
	userID := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, userID, connectorID, "operator")
	elevation, err := h.JWT.IssueElevation(userID, "connector.restart")
	if err != nil {
		t.Fatalf("IssueElevation() error = %v", err)
	}

	extraAudit := map[string]any{
		"runbookId": "runbook-4",
		"stepId":    "step-2",
		"entityRef": "forged-entity",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/runbooks/runbook-4/steps/step-2/execute", nil)
	req.Header.Set("X-Elevation-Token", elevation.Token)
	req = req.WithContext(auth.ContextWithUser(req.Context(), userID, false))
	rr := httptest.NewRecorder()
	h.ServeLifecycleOp(rr, req, connectorID, "restart", "vm-100", extraAudit)
	if rr.Code != http.StatusOK {
		t.Fatalf("ServeLifecycleOp() status = %d, body = %s; want 200", rr.Code, rr.Body.String())
	}

	records := lifecycleReviewAudit(t, h, connectorID)
	if len(records) != 1 {
		t.Fatalf("restart audits = %+v, want one row", records)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(records[0].Detail), &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	want := map[string]any{
		"entityRef": "vm-100",
		"runbookId": "runbook-4",
		"stepId":    "step-2",
	}
	if !reflect.DeepEqual(detail, want) {
		t.Errorf("audit detail = %+v, want %+v", detail, want)
	}
	if !reflect.DeepEqual(extraAudit, map[string]any{
		"entityRef": "forged-entity",
		"runbookId": "runbook-4",
		"stepId":    "step-2",
	}) {
		t.Errorf("extraAudit was mutated: %+v", extraAudit)
	}
}

func TestMutateLifecycleOpReviewValidationErrors(t *testing.T) {
	t.Parallel()
	t.Run("invalid entity ref stops before connector and side effects", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID, state := newLifecycleReviewConnector(t, h)
		err := h.MutateLifecycleOp(context.Background(), connectorID, "restart", "../invalid", LifecycleActor{}, nil)
		assertReviewLifecycleError(t, err, http.StatusBadRequest, "invalid_request")
		if state.restartCount() != 0 {
			t.Errorf("restart calls = %d, want 0", state.restartCount())
		}
		if alerts := lifecycleReviewAlerts(t, h, connectorID); len(alerts) != 0 {
			t.Errorf("alerts = %+v, want none", alerts)
		}
		if records := lifecycleReviewAudit(t, h, connectorID); len(records) != 0 {
			t.Errorf("audit records = %+v, want none", records)
		}
	})

	t.Run("unknown connector returns not found", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID, state := newLifecycleReviewConnector(t, h)
		err := h.MutateLifecycleOp(context.Background(), "missing-connector", "restart", "", LifecycleActor{}, nil)
		assertReviewLifecycleError(t, err, http.StatusNotFound, "not_found")
		assertNoReviewMutationSideEffects(t, h, connectorID, state)
	})

	for _, verb := range []string{"stop", "pause"} {
		t.Run("unsupported verb "+verb, func(t *testing.T) {
			h := newTestHandler(t)
			connectorID, state := newLifecycleReviewConnector(t, h)
			err := h.MutateLifecycleOp(context.Background(), connectorID, verb, "", LifecycleActor{}, nil)
			assertReviewLifecycleError(t, err, http.StatusBadRequest, "unsupported_operation")
			assertNoReviewMutationSideEffects(t, h, connectorID, state)
		})
	}
}

func TestMutateLifecycleOpFailureBroadcastsAlertPayload(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID, state := newLifecycleReviewConnector(t, h)
	state.mu.Lock()
	state.failure = errors.New("device offline")
	state.mu.Unlock()

	hub := ws.NewHub()
	hub.SetConnectorAudience(func(context.Context, string) ([]string, error) {
		return []string{"reader"}, nil
	})
	hubCtx, cancelHub := context.WithCancel(context.Background())
	hubDone := make(chan struct{})
	go func() {
		defer close(hubDone)
		hub.Run(hubCtx)
	}()
	h.WSHub = hub
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := hub.UpgradeHandler(w, r, ws.Identity{UserID: "reader"}); err != nil {
			t.Errorf("UpgradeHandler() error = %v", err)
		}
	}))
	wsConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		cancelHub()
		<-hubDone
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() {
		_ = wsConn.Close()
		deadline := time.Now().Add(2 * time.Second)
		for hub.ClientCount() != 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if hub.ClientCount() != 0 {
			t.Error("websocket client did not unregister")
		}
		cancelHub()
		<-hubDone
		server.Close()
	})
	deadline := time.Now().Add(time.Second)
	for hub.ClientCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatal("websocket client did not register")
	}

	err = h.MutateLifecycleOp(context.Background(), connectorID, "restart", "vm-100", LifecycleActor{}, nil)
	assertReviewLifecycleError(t, err, http.StatusBadGateway, "restart_failed")
	if err := wsConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}
	var event ws.Envelope
	if err := wsConn.ReadJSON(&event); err != nil {
		t.Fatalf("ReadJSON(alert event) error = %v", err)
	}
	if event.Type != ws.EventAlertCreated || event.ConnectorID != connectorID {
		t.Fatalf("event type/connector = %q/%q, want %q/%q", event.Type, event.ConnectorID, ws.EventAlertCreated, connectorID)
	}
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatalf("event payload = %#v, want object", event.Payload)
	}
	alerts := lifecycleReviewAlerts(t, h, connectorID)
	if len(alerts) != 1 {
		t.Fatalf("persisted alerts = %+v, want one", alerts)
	}
	wantPayload := map[string]any{
		"alertId":   alerts[0].ID,
		"serviceId": connectorID,
		"severity":  "critical",
		"title":     "Restart failed for Lifecycle Review",
	}
	if !reflect.DeepEqual(payload, wantPayload) {
		t.Errorf("alert payload = %+v, want %+v", payload, wantPayload)
	}
}

func TestMutateLifecycleOpPersistsSideEffectsAfterOperationCancelsContext(t *testing.T) {
	t.Parallel()
	t.Run("failure alert", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID, state := newLifecycleReviewConnector(t, h)
		state.mu.Lock()
		state.failure = errors.New("operation failed after cancellation")
		state.mu.Unlock()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		state.mu.Lock()
		state.cancel = cancel
		state.mu.Unlock()

		err := h.MutateLifecycleOp(ctx, connectorID, "restart", "vm-100", LifecycleActor{}, nil)
		assertReviewLifecycleError(t, err, http.StatusBadGateway, "restart_failed")
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("caller context error = %v, want context.Canceled", ctx.Err())
		}
		if alerts := lifecycleReviewAlerts(t, h, connectorID); len(alerts) != 1 {
			t.Errorf("alerts = %+v, want one persisted failure alert", alerts)
		}
		if records := lifecycleReviewAudit(t, h, connectorID); len(records) != 0 {
			t.Errorf("audit records = %+v, want none after failure", records)
		}
	})

	t.Run("success audit", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID, state := newLifecycleReviewConnector(t, h)
		userID := apitest.NewUser(t, h.Store, "viewer")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		state.mu.Lock()
		state.cancel = cancel
		state.mu.Unlock()

		if err := h.MutateLifecycleOp(ctx, connectorID, "restart", "vm-100", LifecycleActor{UserID: userID}, nil); err != nil {
			t.Fatalf("MutateLifecycleOp() error = %v", err)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("caller context error = %v, want context.Canceled", ctx.Err())
		}
		records := lifecycleReviewAudit(t, h, connectorID)
		if len(records) != 1 || records[0].ActorUserID != userID {
			t.Errorf("audit records = %+v, want one record for %q", records, userID)
		}
	})
}

func TestMutateLifecycleOpCancellationBeforeLookupStopsOperation(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID, state := newLifecycleReviewConnector(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()

	err := h.MutateLifecycleOp(ctx, connectorID, "restart", "vm-100", LifecycleActor{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("MutateLifecycleOp() error = %v, want context.Canceled", err)
	}
	if state.restartCount() != 0 {
		t.Errorf("restart calls = %d, want 0", state.restartCount())
	}
	if alerts := lifecycleReviewAlerts(t, h, connectorID); len(alerts) != 0 {
		t.Errorf("alerts = %+v, want none", alerts)
	}
	if records := lifecycleReviewAudit(t, h, connectorID); len(records) != 0 {
		t.Errorf("audit records = %+v, want none", records)
	}
}

func TestMutateLifecycleOpCallerContextEndSkipsFailureAlert(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		wrap func(error) error
	}{
		{name: "bare context error", wrap: func(err error) error { return err }},
		{name: "wrapped context error", wrap: func(err error) error { return fmt.Errorf("proxmox: %w", err) }},
	} {
		t.Run("cancel "+tc.name, func(t *testing.T) {
			t.Parallel()
			h := newTestHandler(t)
			connectorID, state := newLifecycleReviewConnector(t, h)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			started := make(chan struct{})
			state.mu.Lock()
			state.operation = func(ctx context.Context) error {
				close(started)
				<-ctx.Done()
				return tc.wrap(ctx.Err())
			}
			state.mu.Unlock()

			done := make(chan error, 1)
			go func() {
				done <- h.MutateLifecycleOp(ctx, connectorID, "restart", "vm-100", LifecycleActor{}, nil)
			}()
			<-started
			cancel()
			err := <-done

			assertReviewLifecycleError(t, err, http.StatusBadGateway, "restart_failed")
			if !errors.Is(err, context.Canceled) {
				t.Errorf("MutateLifecycleOp() error = %v, want it to wrap context.Canceled", err)
			}
			assertNoReviewFailureSideEffects(t, h, connectorID)
		})
	}

	t.Run("deadline exceeded", func(t *testing.T) {
		t.Parallel()
		h := newTestHandler(t)
		connectorID, state := newLifecycleReviewConnector(t, h)
		ctx := &deadlineContext{Context: context.Background(), done: make(chan struct{})}
		started := make(chan struct{})
		state.mu.Lock()
		state.operation = func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return fmt.Errorf("proxmox: %w", ctx.Err())
		}
		state.mu.Unlock()

		done := make(chan error, 1)
		go func() {
			done <- h.MutateLifecycleOp(ctx, connectorID, "restart", "vm-100", LifecycleActor{}, nil)
		}()
		<-started
		close(ctx.done)
		err := <-done

		assertReviewLifecycleError(t, err, http.StatusBadGateway, "restart_failed")
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("MutateLifecycleOp() error = %v, want it to wrap context.DeadlineExceeded", err)
		}
		assertNoReviewFailureSideEffects(t, h, connectorID)
	})
}

// deadlineContext is a context whose deadline "expires" when the test closes
// done, so a test decides when the deadline is reached instead of a timer.
type deadlineContext struct {
	context.Context
	done chan struct{}
}

func (c *deadlineContext) Done() <-chan struct{} { return c.done }

func (c *deadlineContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}

func (s *lifecycleReviewConnectorState) restartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restarts
}

func assertReviewLifecycleError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.status != status || lifecycleErr.code != code {
		t.Fatalf("lifecycle error = %v, want status %d code %q", err, status, code)
	}
}

// assertNoReviewFailureSideEffects checks that an abandoned operation left no
// alert (and therefore no alert broadcast, which only follows a created alert)
// and no audit record.
func assertNoReviewFailureSideEffects(t *testing.T, h *Handler, connectorID string) {
	t.Helper()
	if alerts := lifecycleReviewAlerts(t, h, connectorID); len(alerts) != 0 {
		t.Errorf("alerts = %+v, want none", alerts)
	}
	if records := lifecycleReviewAudit(t, h, connectorID); len(records) != 0 {
		t.Errorf("audit records = %+v, want none", records)
	}
}

func assertNoReviewMutationSideEffects(t *testing.T, h *Handler, connectorID string, state *lifecycleReviewConnectorState) {
	t.Helper()
	if state.restartCount() != 0 {
		t.Errorf("restart calls = %d, want 0", state.restartCount())
	}
	if alerts := lifecycleReviewAlerts(t, h, connectorID); len(alerts) != 0 {
		t.Errorf("alerts = %+v, want none", alerts)
	}
	if records := lifecycleReviewAudit(t, h, connectorID); len(records) != 0 {
		t.Errorf("audit records = %+v, want none", records)
	}
}

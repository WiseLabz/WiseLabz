package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func newLifecycleCoreConnector(t *testing.T, h *Handler, url string) string {
	t.Helper()
	registerBulkFakeConnector(t)
	return createBulkFakeConnector(t, h, url)
}

func TestPreviewLifecycleOpCore(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
	data := `{
		"serviceName":"switch",
		"dependencies":[{"kind":"upstream_service","name":"router","ref":"router-1"}],
		"entities":[{"kind":"port","name":"Switch / Port 2","externalId":"switch/port-2","attributes":{"connectedDevices":["Living Room AP","Porch Camera"]}}]
	}`
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: connectorID,
		Data:        data,
		FetchedAt:   "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	preview, err := h.PreviewLifecycleOp(context.Background(), connectorID, "restart", "switch/port-2")
	if err != nil {
		t.Fatalf("PreviewLifecycleOp() error = %v", err)
	}
	if preview.TargetService != "Switch / Port 2" {
		t.Errorf("TargetService = %q, want entity name", preview.TargetService)
	}
	if preview.EstimatedDowntimeSeconds != 30 {
		t.Errorf("restart downtime = %d, want 30", preview.EstimatedDowntimeSeconds)
	}
	wantAffected := []string{"Living Room AP", "Porch Camera"}
	if !reflect.DeepEqual(preview.AffectedEntities, wantAffected) {
		t.Errorf("AffectedEntities = %v, want %v", preview.AffectedEntities, wantAffected)
	}
	wantDependencies := []connector.ServiceDependency{{
		Kind: "upstream_service",
		Name: "router",
		Ref:  "router-1",
	}}
	if !reflect.DeepEqual(preview.DependentServices, wantDependencies) {
		t.Errorf("DependentServices = %+v, want %+v", preview.DependentServices, wantDependencies)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAlerts() after preview error = %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("preview alerts = %+v, want none", alerts)
	}
	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() after preview error = %v", err)
	}
	if len(records) != 0 {
		t.Errorf("preview audit records = %+v, want none", records)
	}

	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: connectorID,
		Data:        `{"serviceName":"switch"}`,
		FetchedAt:   "2026-01-02T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	// The fake connector only restarts: a dry-run of an unsupported verb is
	// rejected instead of previewing an operation that could never run.
	_, err = h.PreviewLifecycleOp(context.Background(), connectorID, "stop", "")
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.code != "unsupported_operation" || lifecycleErr.status != http.StatusBadRequest {
		t.Fatalf("PreviewLifecycleOp(stop) error = %v, want 400 unsupported_operation", err)
	}
}

func TestPreviewLifecycleOpSnapshotErrors(t *testing.T) {
	t.Parallel()
	t.Run("missing snapshot", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")

		_, err := h.PreviewLifecycleOp(context.Background(), connectorID, "restart", "")
		var lifecycleErr *lifecycleError
		if !errors.As(err, &lifecycleErr) || lifecycleErr.status != http.StatusNotFound || lifecycleErr.code != "not_found" {
			t.Fatalf("PreviewLifecycleOp() error = %v, want not-found lifecycle error", err)
		}
	})

	t.Run("invalid snapshot", func(t *testing.T) {
		h := newTestHandler(t)
		connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
		if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
			ConnectorID: connectorID,
			Data:        `{`,
			FetchedAt:   "2026-01-01T00:00:00Z",
		}); err != nil {
			t.Fatal(err)
		}

		_, err := h.PreviewLifecycleOp(context.Background(), connectorID, "restart", "")
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("PreviewLifecycleOp() error = %v, want JSON syntax error", err)
		}
	})
}

func TestMutateLifecycleOpFailureCreatesCriticalAlertWithoutAudit(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://broken.example.com")

	err := h.MutateLifecycleOp(context.Background(), connectorID, "restart", "vm-100", LifecycleActor{}, nil)
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) {
		t.Fatalf("MutateLifecycleOp() error = %v, want restart failure", err)
	}
	if lifecycleErr.status != http.StatusBadGateway || lifecycleErr.code != "restart_failed" {
		t.Fatalf("MutateLifecycleOp() error = %v, want restart failure", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("MutateLifecycleOp() error = %v, want original deadline error", err)
	}

	alerts, _, err := h.Store.ListAlerts(context.Background(), connectorID, "", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].Severity != "critical" {
		t.Fatalf("alerts = %+v, want one critical alert", alerts)
	}
	if alerts[0].ServiceID != connectorID {
		t.Errorf("alert ServiceID = %q, want %q", alerts[0].ServiceID, connectorID)
	}
	if alerts[0].Title != "Restart failed for Test" {
		t.Errorf("alert Title = %q, want %q", alerts[0].Title, "Restart failed for Test")
	}
	if alerts[0].Description != lifecycleErr.message {
		t.Errorf("alert Description = %q, want mutation error %q", alerts[0].Description, lifecycleErr.message)
	}

	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error = %v", err)
	}
	if len(records) != 0 {
		t.Errorf("audit records = %+v, want none after mutation failure", records)
	}
}

func TestMutateLifecycleOpWritesRunAuditWithExplicitActor(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
	actorID := apitest.NewUser(t, h.Store, "viewer")
	extraAudit := map[string]any{
		"runId":     "run-1",
		"stepId":    "step-2",
		"runbookId": "runbook-3",
	}

	if err := h.MutateLifecycleOp(
		context.Background(), connectorID, "restart", "vm-100",
		LifecycleActor{UserID: actorID}, extraAudit,
	); err != nil {
		t.Fatalf("MutateLifecycleOp() error = %v", err)
	}
	if !reflect.DeepEqual(extraAudit, map[string]any{
		"runId":     "run-1",
		"stepId":    "step-2",
		"runbookId": "runbook-3",
	}) {
		t.Errorf("extraAudit was mutated: %+v", extraAudit)
	}

	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("audit records = %+v, want one row", records)
	}
	record := records[0]
	if record.ActorUserID != actorID || record.ActorRole != "user" {
		t.Errorf("audit actor = %q/%q, want %q/user", record.ActorUserID, record.ActorRole, actorID)
	}
	if record.Action != "connector.restart" || record.TargetType != "connector" || record.TargetID != connectorID {
		t.Errorf("audit target = %+v, want restart for connector %q", record, connectorID)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(record.Detail), &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	wantDetail := map[string]any{
		"entityRef": "vm-100",
		"runId":     "run-1",
		"stepId":    "step-2",
		"runbookId": "runbook-3",
	}
	if !reflect.DeepEqual(detail, wantDetail) {
		t.Errorf("audit detail = %+v, want %+v", detail, wantDetail)
	}
}

func TestMutateLifecycleOpUsesExplicitActorOverAuthContext(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
	contextUserID := apitest.NewUser(t, h.Store, "viewer")
	explicitActorID := apitest.NewUser(t, h.Store, "operator")
	ctx := auth.ContextWithUser(context.Background(), contextUserID, false)

	if err := h.MutateLifecycleOp(
		ctx, connectorID, "restart", "vm-100",
		LifecycleActor{UserID: explicitActorID, InstanceAdmin: true}, nil,
	); err != nil {
		t.Fatalf("MutateLifecycleOp() error = %v", err)
	}

	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("audit records = %+v, want one row", records)
	}
	if records[0].ActorUserID != explicitActorID || records[0].ActorRole != "admin" {
		t.Errorf("audit actor = %q/%q, want explicit actor %q/admin (context user %q)",
			records[0].ActorUserID, records[0].ActorRole, explicitActorID, contextUserID)
	}
}

func TestMutateLifecycleOpAnonymousActorRole(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
	if err := h.MutateLifecycleOp(context.Background(), connectorID, "restart", "", LifecycleActor{}, nil); err != nil {
		t.Fatalf("MutateLifecycleOp() error = %v", err)
	}
	records, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil {
		t.Fatalf("ListAuditRecords() error = %v", err)
	}
	if len(records) != 1 || records[0].ActorUserID != "" || records[0].ActorRole != "" {
		t.Errorf("audit actor = %+v, want anonymous actor with empty user and role", records)
	}
}

func TestLifecycleWrapperChecksElevationBeforeEntityRef(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	connectorID := newLifecycleCoreConnector(t, h, "https://fake.example.com")
	path := "/api/connectors/" + connectorID + "/restart"
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"entityRef":"../invalid"}`))
	req.SetPathValue("id", connectorID)
	rr := httptest.NewRecorder()
	h.RestartPreview(rr, req)

	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "elevation_required") {
		t.Fatalf("status = %d, body = %s, want missing-elevation error", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "invalid entityRef") {
		t.Errorf("response exposed entityRef validation before elevation: %s", rr.Body.String())
	}
}

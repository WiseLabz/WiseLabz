package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func handlerActionRecipe() string {
	return `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: title
      external_id: id
      attributes:
        node: {path: node}
      actions:
        restart:
          method: PATCH
          path: /items/{external_id}/restart
          query: {node: "{attr.node}"}
          headers: {X-Mode: safe, X-Secret-Token: header-secret}
          body: {reason: recipe-defined}
          label: Restart item
          description: Restart this item
          downtime_seconds: 7
        rescan:
          method: POST
          path: /items/{external_id}/rescan
          query: {node: "{attr.node}"}
actions:
  rescan:
    method: POST
    path: /rescan
    query: {source: library}
    headers: {X-Mode: safe}
    body: {scope: all}
    label: Rescan library
    description: Refresh the library index
`
}

func seedRecipeActionConnector(t *testing.T, h *Handler, targetURL string) *store.ConnectorRecord {
	return seedRecipeActionConnectorWithRecipe(t, h, targetURL, handlerActionRecipe())
}

func seedRecipeActionConnectorWithRecipe(t *testing.T, h *Handler, targetURL, recipe string) *store.ConnectorRecord {
	t.Helper()
	configData, err := store.MarshalConnectorConfig("custom", map[string]any{
		"recipe":     recipe,
		"auth_token": "query-secret",
	}, h.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	record := &store.ConnectorRecord{
		Name:       "Action test",
		Type:       "custom",
		Category:   "other",
		URL:        targetURL,
		ConfigData: configData,
	}
	if err := h.Store.CreateConnector(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	return record
}

func seedRecipeActionSnapshot(t *testing.T, h *Handler, connectorID string) {
	t.Helper()
	data := `{"serviceName":"library","dependencies":[{"kind":"database","name":"db"}],"entities":[{"kind":"item","name":"Database item","externalId":"db|1","attributes":{"node":"node-1","connectedDevices":["worker-a"]}}]}`
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: connectorID,
		Data:        data,
		FetchedAt:   "2026-10-08T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
}

func actionHandlerRequest(id, name, path, body, user string, admin bool) *http.Request {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.SetPathValue("id", id)
	request.SetPathValue("name", name)
	return request.WithContext(auth.ContextWithUser(request.Context(), user, admin))
}

func issueTestElevation(t *testing.T, h *Handler, user, action, target string) string {
	t.Helper()
	token, err := h.JWT.IssueElevationBound(user, action, auth.ElevationBinding{Target: target})
	if err != nil {
		t.Fatal(err)
	}
	return token.Token
}

func TestRecipeActionPreviewResolvesAndRedactsWithoutSending(t *testing.T) {
	h := newTestHandler(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "unexpected action request")
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	seedRecipeActionSnapshot(t, h, record.ID)

	request := actionHandlerRequest(record.ID, "restart", "/?dryRun=true", `{"entityRef":"db|1","ignored":"operator value"}`, "operator", false)
	response := httptest.NewRecorder()
	h.ServeLifecycleOp(response, request, record.ID, "restart", "db|1", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("lifecycle preview status=%d body=%s", response.Code, response.Body.String())
	}
	var preview LifecyclePreview
	if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.UserDefined || preview.Label != "Restart item" || preview.Description != "Restart this item" || preview.EstimatedDowntimeSeconds != 7 {
		t.Fatalf("action metadata=%+v", preview)
	}
	if preview.TargetService != "Database item" || len(preview.AffectedEntities) != 1 || preview.AffectedEntities[0] != "worker-a" {
		t.Fatalf("snapshot fields=%+v", preview)
	}
	if preview.Request == nil || preview.Request.Method != http.MethodPatch {
		t.Fatalf("request preview=%+v", preview.Request)
	}
	if strings.Contains(preview.Request.URL, "query-secret") || !strings.Contains(preview.Request.URL, "node=node-1") {
		t.Fatalf("preview URL=%q", preview.Request.URL)
	}
	if preview.Request.Headers["X-Mode"] != "safe" || preview.Request.Headers["X-Secret-Token"] != "[redacted]" {
		t.Fatalf("preview headers=%v", preview.Request.Headers)
	}
	if calls.Load() != 0 {
		t.Fatalf("preview sent %d requests", calls.Load())
	}
}

func TestNamedEntityActionErrorsNameInvalidValuesAndPreserveSupportErrors(t *testing.T) {
	h := newTestHandler(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	data := `{"serviceName":"library","entities":[{"kind":"item","name":"Missing node","externalId":"missing-node","attributes":{}},{"kind":"item","name":"Whitespace ID","externalId":"web 1","attributes":{"node":"node-1"}},{"kind":"other","name":"Wrong kind","externalId":"wrong-kind","attributes":{"node":"node-1"}}]}`
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: record.ID, Data: data, FetchedAt: "2026-10-08T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		entityRef string
		code      string
		message   string
	}{
		{entityRef: "missing-node", code: "invalid_request", message: "node"},
		{entityRef: "web 1", code: "invalid_request", message: "web 1"},
		{entityRef: "wrong-kind", code: "unsupported_operation", message: "other"},
	} {
		request := actionHandlerRequest(record.ID, "rescan", "/?dryRun=true", `{"entityRef":"`+tc.entityRef+`"}`, "operator", false)
		response := httptest.NewRecorder()
		h.Action(response, request)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tc.code) || !strings.Contains(response.Body.String(), tc.message) {
			t.Errorf("entityRef %q status=%d body=%s", tc.entityRef, response.Code, response.Body.String())
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid action targets sent %d requests", calls.Load())
	}
}

func TestLifecycleRecipeActionResultAndAudit(t *testing.T) {
	h := newTestHandler(t)
	var seenMethod, seenPath, seenBody, seenToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenMethod = r.Method
		seenPath = r.URL.EscapedPath()
		seenToken = r.URL.Query().Get("api_token")
		body, _ := io.ReadAll(r.Body)
		seenBody = string(body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "restart accepted")
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	seedRecipeActionSnapshot(t, h, record.ID)
	elevation := issueTestElevation(t, h, "operator", "connector.restart", "")
	request := actionHandlerRequest(record.ID, "", "/", `{"entityRef":"db|1"}`, "operator", false)
	request.Header.Set("X-Elevation-Token", elevation)
	response := httptest.NewRecorder()
	h.RestartPreview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restart status=%d body=%s", response.Code, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "restarted" || result["statusCode"] != float64(http.StatusAccepted) || result["excerpt"] != "restart accepted" {
		t.Fatalf("restart result=%v", result)
	}
	if seenMethod != http.MethodPatch || seenPath != "/items/db%7C1/restart" || seenToken != "query-secret" || !strings.Contains(seenBody, "recipe-defined") {
		t.Fatalf("upstream request=%s %s?api_token=%q body=%s", seenMethod, seenPath, seenToken, seenBody)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 1 {
		t.Fatalf("restart audit=%+v err=%v", audits, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["entityRef"] != "db|1" || detail["method"] != http.MethodPatch || detail["url"] != server.URL+"/items/db%7C1/restart" || detail["status"] != float64(http.StatusAccepted) {
		t.Fatalf("restart audit detail=%v", detail)
	}
	if strings.Contains(audits[0].Detail, "query-secret") || strings.Contains(audits[0].Detail, "restart accepted") {
		t.Fatalf("restart audit leaked credentials or excerpt: %s", audits[0].Detail)
	}
}

func TestLifecycleActionFailureExcerptIsNotPersistedOrLogged(t *testing.T) {
	oldLogger := slog.Default()
	var logs strings.Builder
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(oldLogger)

	h := newTestHandler(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"error":"LIFECYCLE-BODY-SECRET"}`)
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	seedRecipeActionSnapshot(t, h, record.ID)
	elevation := issueTestElevation(t, h, "operator", "connector.restart", "")
	request := actionHandlerRequest(record.ID, "", "/", `{"entityRef":"db|1"}`, "operator", false)
	request.Header.Set("X-Elevation-Token", elevation)
	response := httptest.NewRecorder()
	h.RestartPreview(response, request)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), `"statusCode":409`) || !strings.Contains(response.Body.String(), "LIFECYCLE-BODY-SECRET") {
		t.Fatalf("lifecycle failure status=%d body=%s", response.Code, response.Body.String())
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), record.ID, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 || strings.Contains(alerts[0].Description, "LIFECYCLE-BODY-SECRET") {
		t.Fatalf("lifecycle alerts=%+v err=%v", alerts, err)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 0 {
		t.Fatalf("failed lifecycle audits=%+v err=%v", audits, err)
	}
	if strings.Contains(logs.String(), "LIFECYCLE-BODY-SECRET") {
		t.Fatalf("logs contain lifecycle response excerpt: %s", logs.String())
	}
}

func TestNamedRecipeActionElevationResultAndAudit(t *testing.T) {
	h := newTestHandler(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/rescan" || r.URL.Query().Get("api_token") != "query-secret" || r.URL.Query().Get("source") != "library" {
			t.Errorf("upstream request=%s %s query=%v", r.Method, r.URL.Path, r.URL.Query())
		}
		_, _ = io.WriteString(w, "rescan complete")
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	otherConnector := seedRecipeActionConnector(t, h, server.URL)
	seedRecipeActionSnapshot(t, h, record.ID)

	previewRequest := actionHandlerRequest(record.ID, "rescan", "/?dryRun=true", `{"ignored":"must not affect request"}`, "operator", false)
	preview := httptest.NewRecorder()
	h.Action(preview, previewRequest)
	if preview.Code != http.StatusOK || calls.Load() != 0 || !strings.Contains(preview.Body.String(), "Rescan library") || strings.Contains(preview.Body.String(), "query-secret") {
		t.Fatalf("preview status=%d calls=%d body=%s", preview.Code, calls.Load(), preview.Body.String())
	}

	for _, wrongBinding := range []string{record.ID + ":different", otherConnector.ID + ":rescan"} {
		wrongTarget := issueTestElevation(t, h, "operator", "connector.action", wrongBinding)
		wrongRequest := actionHandlerRequest(record.ID, "rescan", "/", `{}`, "operator", false)
		wrongRequest.Header.Set("X-Elevation-Token", wrongTarget)
		wrong := httptest.NewRecorder()
		h.Action(wrong, wrongRequest)
		if wrong.Code != http.StatusUnauthorized || calls.Load() != 0 {
			t.Fatalf("wrong binding %q status=%d calls=%d body=%s", wrongBinding, wrong.Code, calls.Load(), wrong.Body.String())
		}
	}

	elevation := issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan")
	unsupportedRequest := actionHandlerRequest(record.ID, "missing", "/", `{}`, "operator", false)
	unsupportedRequest.Header.Set("X-Elevation-Token", elevation)
	unsupported := httptest.NewRecorder()
	h.Action(unsupported, unsupportedRequest)
	if unsupported.Code != http.StatusBadRequest || !strings.Contains(unsupported.Body.String(), "unsupported_operation") || calls.Load() != 0 {
		t.Fatalf("unsupported status=%d calls=%d body=%s", unsupported.Code, calls.Load(), unsupported.Body.String())
	}

	request := actionHandlerRequest(record.ID, "rescan", "/", `{"ignored":"operator value"}`, "operator", false)
	request.Header.Set("X-Elevation-Token", elevation)
	response := httptest.NewRecorder()
	h.Action(response, request)
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("action status=%d calls=%d body=%s", response.Code, calls.Load(), response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != float64(http.StatusOK) || result["excerpt"] != "rescan complete" {
		t.Fatalf("action result=%v", result)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 1 {
		t.Fatalf("action audits=%+v err=%v", audits, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["action"] != "rescan" || detail["method"] != http.MethodPost || detail["url"] != server.URL+"/rescan" || detail["status"] != float64(http.StatusOK) {
		t.Fatalf("action audit detail=%v", detail)
	}
	if strings.Contains(audits[0].Detail, "query-secret") || strings.Contains(audits[0].Detail, "rescan complete") {
		t.Fatalf("action audit leaked secret or excerpt: %s", audits[0].Detail)
	}
}

func TestActionFailureReturnsExcerptButNeverPersistsOrLogsIt(t *testing.T) {
	oldLogger := slog.Default()
	var logs strings.Builder
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(oldLogger)

	h := newTestHandler(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"message":"BODY-SECRET"}`)
	}))
	defer server.Close()
	record := seedRecipeActionConnector(t, h, server.URL)
	elevation := issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan")
	request := actionHandlerRequest(record.ID, "rescan", "/", `{}`, "operator", false)
	request.Header.Set("X-Elevation-Token", elevation)
	response := httptest.NewRecorder()
	h.Action(response, request)
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), `"statusCode":409`) || !strings.Contains(response.Body.String(), "BODY-SECRET") {
		t.Fatalf("action error status=%d body=%s", response.Code, response.Body.String())
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), record.ID, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 || strings.Contains(alerts[0].Description, "BODY-SECRET") {
		t.Fatalf("alerts=%+v err=%v", alerts, err)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 0 {
		t.Fatalf("failed action audits=%+v err=%v", audits, err)
	}
	if strings.Contains(logs.String(), "BODY-SECRET") {
		t.Fatalf("logs contain response excerpt: %s", logs.String())
	}
}

func TestPreviewRejectsUnsupportedConnectorOperations(t *testing.T) {
	h := newTestHandler(t)
	unsupportedCustom := seedCoverageConnector(t, h, "without actions", "other")
	_, err := h.PreviewLifecycleOp(context.Background(), unsupportedCustom.ID, "restart", "")
	var lifecycleErr *lifecycleError
	if !errors.As(err, &lifecycleErr) || lifecycleErr.code != "unsupported_operation" {
		t.Fatalf("custom preview error=%v, want unsupported_operation", err)
	}
	builtin := &store.ConnectorRecord{Name: "TLS probe", Type: "tlsprobe", Category: "networking", ConfigData: "{}"}
	if err := h.Store.CreateConnector(context.Background(), builtin); err != nil {
		t.Fatal(err)
	}
	_, err = h.PreviewLifecycleOp(context.Background(), builtin.ID, "stop", "")
	if !errors.As(err, &lifecycleErr) || lifecycleErr.code != "unsupported_operation" {
		t.Fatalf("built-in preview error=%v, want unsupported_operation", err)
	}
}

func TestBulkRestartUsesOnlyCustomServiceRestartAndAuditsRequest(t *testing.T) {
	h := newTestHandler(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/service-restart" {
			t.Errorf("unexpected bulk restart request: %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, "restarted")
	}))
	defer server.Close()
	serviceRecipe := strings.Replace(handlerActionRecipe(), "actions:\n  rescan:", "actions:\n  restart: {method: POST, path: /service-restart}\n  rescan:", 1)
	service := seedRecipeActionConnectorWithRecipe(t, h, server.URL, serviceRecipe)
	entityOnly := seedRecipeActionConnector(t, h, server.URL)
	user := "bulk-operator"
	for _, record := range []*store.ConnectorRecord{service, entityOnly} {
		if _, err := h.Store.UpsertConnectorGrant(context.Background(), user, record.ID, "operator"); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/connectors/bulk-restart", strings.NewReader(`{"ids":["`+service.ID+`","`+entityOnly.ID+`"]}`))
	request = request.WithContext(auth.ContextWithUser(request.Context(), user, false))
	response := httptest.NewRecorder()
	h.BulkRestart(response, request)
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("bulk status=%d calls=%d body=%s", response.Code, calls.Load(), response.Body.String())
	}
	var body struct {
		Results []bulkItemResult `json:"results"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Results) != 2 || body.Results[0].Status != "success" || body.Results[1].Status != "error" {
		t.Fatalf("bulk results=%+v", body.Results)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.bulk_restart", "connector", "", "", 0, 10)
	if err != nil || len(audits) != 1 || audits[0].TargetID != service.ID {
		t.Fatalf("bulk audits=%+v err=%v", audits, err)
	}
	var detail map[string]any
	if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
		t.Fatal(err)
	}
	if detail["method"] != http.MethodPost || detail["url"] != server.URL+"/service-restart" || detail["status"] != float64(http.StatusOK) || strings.Contains(audits[0].Detail, "query-secret") || strings.Contains(audits[0].Detail, "restarted") {
		t.Fatalf("bulk audit detail=%v", detail)
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), entityOnly.ID, "", "", "", 0, 10)
	if err != nil || len(alerts) != 0 {
		t.Fatalf("entity-only bulk alerts=%+v err=%v", alerts, err)
	}
}

func TestListReportsSameCapabilitiesAndActionsAsGet(t *testing.T) {
	h := newTestHandler(t)
	user := apitest.NewUser(t, h.Store, "list-viewer")
	recipe := strings.Replace(handlerActionRecipe(), "actions:\n  rescan:", "actions:\n  restart: {method: POST, path: /service-restart}\n  rescan:", 1)
	custom := seedRecipeActionConnectorWithRecipe(t, h, "https://custom.example.com", recipe)
	builtIn := seedLifecyclePreviewConnector(t, h, "proxmox")
	for _, record := range []*store.ConnectorRecord{custom, builtIn} {
		apitest.GrantConnectorRole(t, h.Store, user, record.ID, "viewer")
	}
	withUser := func(r *http.Request) *http.Request {
		return r.WithContext(auth.ContextWithUser(r.Context(), user, false))
	}

	listRecorder := httptest.NewRecorder()
	h.List(listRecorder, withUser(httptest.NewRequest(http.MethodGet, "/api/connectors", nil)))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	for _, leaked := range []string{"configData", "query-secret", "header-secret", "preview-secret"} {
		if strings.Contains(listRecorder.Body.String(), leaked) {
			t.Fatalf("list response leaks %q: %s", leaked, listRecorder.Body.String())
		}
	}
	var rows []connectorWithRole
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("list rows=%d body=%s", len(rows), listRecorder.Body.String())
	}
	for _, row := range rows {
		request := httptest.NewRequest(http.MethodGet, "/api/connectors/"+row.ID, nil)
		request.SetPathValue("id", row.ID)
		getRecorder := httptest.NewRecorder()
		h.Get(getRecorder, withUser(request))
		if getRecorder.Code != http.StatusOK {
			t.Fatalf("get %s status=%d body=%s", row.ID, getRecorder.Code, getRecorder.Body.String())
		}
		var byID connectorWithRole
		if err := json.Unmarshal(getRecorder.Body.Bytes(), &byID); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(row.Capabilities, byID.Capabilities) || !reflect.DeepEqual(row.Actions, byID.Actions) {
			t.Errorf("%s: list caps=%+v actions=%+v, get caps=%+v actions=%+v", row.Name, row.Capabilities, row.Actions, byID.Capabilities, byID.Actions)
		}
		switch row.ID {
		case custom.ID:
			names := map[string]bool{}
			for _, action := range row.Actions {
				if !action.EntityScope {
					names[action.Name] = true
				}
			}
			if len(names) != 2 || !names["restart"] || !names["rescan"] || !row.Capabilities.Restart {
				t.Errorf("custom row caps=%+v actions=%+v, want restart and rescan", row.Capabilities, row.Actions)
			}
		case builtIn.ID:
			if !row.Capabilities.Restart {
				t.Errorf("built-in row caps=%+v, want restart", row.Capabilities)
			}
		}
	}
}

const noSendRecipe = `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: title
      external_id: id
      attributes:
        rack: {path: rack}
      actions:
        restart:
          method: PATCH
          path: /items/{external_id}/restart
          query: {slot: "{attr.rack}"}
        rescan:
          method: POST
          path: /items/{external_id}/rescan
          query: {slot: "{attr.rack}"}
`

func TestInvalidEntityTargetsSendNothingOnTheMutatingPath(t *testing.T) {
	h := newTestHandler(t)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	defer server.Close()
	record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, noSendRecipe)
	data := `{"serviceName":"library","entities":[` +
		`{"kind":"item","name":"Spaced","externalId":"web 1","attributes":{"rack":"r1"}},` +
		`{"kind":"item","name":"Traversal","externalId":"../admin","attributes":{"rack":"r1"}},` +
		`{"kind":"item","name":"No rack","externalId":"no-rack","attributes":{}}]}`
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: record.ID, Data: data, FetchedAt: "2026-10-08T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, entityRef string
		// namedMessage and restartMessage are substrings the 400 body must contain.
		namedMessage, restartMessage string
	}{
		{"not in the snapshot", "ghost", "ghost", ""},
		{"whitespace in the external id", "web 1", "web 1", ""},
		{"parent traversal in the external id", "../admin", "", ""},
		{"missing attribute", "no-rack", "rack", "rack"},
	} {
		for _, named := range []bool{true, false} {
			kind := "restart"
			message := tc.restartMessage
			if named {
				kind, message = "named", tc.namedMessage
			}
			t.Run(tc.name+"/"+kind, func(t *testing.T) {
				var request *http.Request
				response := httptest.NewRecorder()
				if named {
					request = actionHandlerRequest(record.ID, "rescan", "/", `{"entityRef":"`+tc.entityRef+`"}`, "operator", false)
					request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan"))
					h.Action(response, request)
				} else {
					request = actionHandlerRequest(record.ID, "", "/", `{"entityRef":"`+tc.entityRef+`"}`, "operator", false)
					request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.restart", ""))
					h.RestartPreview(response, request)
				}
				if response.Code != http.StatusBadRequest || (message != "" && !strings.Contains(response.Body.String(), message)) {
					t.Fatalf("status=%d body=%s, want 400 naming %q", response.Code, response.Body.String(), message)
				}
				if calls.Load() != 0 {
					t.Fatalf("service received %d requests", calls.Load())
				}
				for _, action := range []string{"connector.action", "connector.restart"} {
					if audits, _, err := h.Store.ListAuditRecords(context.Background(), action, "connector", "", "", 0, 10); err != nil || len(audits) != 0 {
						t.Fatalf("%s audits=%+v err=%v, want none", action, audits, err)
					}
				}
				if alerts, _, err := h.Store.ListAlerts(context.Background(), record.ID, "", "", "", 0, 10); err != nil || len(alerts) != 0 {
					t.Fatalf("alerts=%+v err=%v, want none", alerts, err)
				}
			})
		}
	}
}

func TestNamedEntityActionIgnoresExtraRequestFields(t *testing.T) {
	h := newTestHandler(t)
	var seen struct {
		method, escapedPath, rawQuery, body, evil string
		calls                                     int
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen.calls++
		seen.method, seen.escapedPath, seen.rawQuery, seen.body = r.Method, r.URL.EscapedPath(), r.URL.RawQuery, string(body)
		seen.evil = r.Header.Get("X-Evil")
		_, _ = io.WriteString(w, "done")
	}))
	defer server.Close()
	recipe := strings.Replace(handlerActionRecipe(),
		"          path: /items/{external_id}/rescan\n          query: {node: \"{attr.node}\"}\n",
		"          path: /items/{external_id}/rescan\n          query: {node: \"{attr.node}\"}\n          body: {scope: \"{external_id}\"}\n", 1)
	record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, recipe)
	seedRecipeActionSnapshot(t, h, record.ID)
	payload := `{"entityRef":"db|1","path":"/evil","method":"DELETE","query":{"x":"1"},"body":{"y":2},"headers":{"X-Evil":"1"}}`
	request := actionHandlerRequest(record.ID, "rescan", "/", payload, "operator", false)
	request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan"))
	response := httptest.NewRecorder()
	h.Action(response, request)
	if response.Code != http.StatusOK || seen.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, seen.calls, response.Body.String())
	}
	if seen.method != http.MethodPost || seen.escapedPath != "/items/db%7C1/rescan" ||
		seen.rawQuery != "api_token=query-secret&node=node-1" || seen.body != `{"scope":"db|1"}` || seen.evil != "" {
		t.Fatalf("upstream request = %s %s ?%s body=%q X-Evil=%q; want only what the recipe declares", seen.method, seen.escapedPath, seen.rawQuery, seen.body, seen.evil)
	}
}

func TestBulkRestartFailureKeepsServiceResponseOutOfResultAlertAndAudit(t *testing.T) {
	const sentinel = "BULK-SENTINEL-TEXT"
	h := newTestHandler(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, sentinel)
	}))
	defer server.Close()
	serviceRecipe := strings.Replace(handlerActionRecipe(), "actions:\n  rescan:", "actions:\n  restart: {method: POST, path: /service-restart}\n  rescan:", 1)
	service := seedRecipeActionConnectorWithRecipe(t, h, server.URL, serviceRecipe)
	user := "bulk-operator"
	if _, err := h.Store.UpsertConnectorGrant(context.Background(), user, service.ID, "operator"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/connectors/bulk-restart", strings.NewReader(`{"ids":["`+service.ID+`"]}`))
	request = request.WithContext(auth.ContextWithUser(request.Context(), user, false))
	response := httptest.NewRecorder()
	h.BulkRestart(response, request)
	var body struct {
		Results []bulkItemResult `json:"results"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("bulk body=%s: %v", response.Body.String(), err)
	}
	if len(body.Results) != 1 || body.Results[0].ID != service.ID || body.Results[0].Status != "error" {
		t.Fatalf("bulk results=%+v, want an error for %s", body.Results, service.ID)
	}
	if strings.Contains(response.Body.String(), sentinel) {
		t.Fatalf("bulk response carries the service response text: %s", response.Body.String())
	}
	alerts, _, err := h.Store.ListAlerts(context.Background(), service.ID, "", "", "", 0, 10)
	if err != nil || len(alerts) != 1 {
		t.Fatalf("alerts=%+v err=%v, want one", alerts, err)
	}
	audits, _, err := h.Store.ListAuditRecords(context.Background(), "", "", "", "", 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range []any{alerts[0].Title, alerts[0].Description, audits} {
		encoded, err := json.Marshal(stored)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("service response text was persisted: %s", encoded)
		}
	}
}

func TestNamedEntityActionKeepsIntegerPrecisionInPlaceholders(t *testing.T) {
	const serial = "9007199254740993" // 2^53 + 1: not representable as a float64
	recipe := `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: title
      external_id: id
      attributes:
        serial: {path: serial}
      actions:
        check:
          method: POST
          path: /items/{external_id}/serial/{attr.serial}
          query: {serial: "{attr.serial}"}
          body: {text: "serial={attr.serial}"}
`
	h := newTestHandler(t)
	var calls atomic.Int32
	var gotPath, gotSerial, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data, _ := io.ReadAll(r.Body)
		gotPath, gotSerial, gotBody = r.URL.Path, r.URL.Query().Get("serial"), string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, recipe)
	if err := h.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: record.ID,
		Data:        `{"serviceName":"library","entities":[{"kind":"item","name":"Serial item","externalId":"item-1","attributes":{"serial":` + serial + `}}]}`,
		FetchedAt:   "2026-10-08T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	preview := httptest.NewRecorder()
	h.Action(preview, actionHandlerRequest(record.ID, "check", "/?dryRun=true", `{"entityRef":"item-1"}`, "operator", false))
	if preview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var shown LifecyclePreview
	if err := json.Unmarshal(preview.Body.Bytes(), &shown); err != nil || shown.Request == nil {
		t.Fatalf("preview=%s err=%v", preview.Body.String(), err)
	}
	shownBody, _ := json.Marshal(shown.Request.Body)
	if !strings.Contains(shown.Request.URL, "/serial/"+serial) || !strings.Contains(shown.Request.URL, "serial="+serial) || !strings.Contains(string(shownBody), "serial="+serial) {
		t.Fatalf("preview lost precision: url=%q body=%s", shown.Request.URL, shownBody)
	}
	if calls.Load() != 0 {
		t.Fatalf("preview sent %d requests", calls.Load())
	}

	request := actionHandlerRequest(record.ID, "check", "/", `{"entityRef":"item-1"}`, "operator", false)
	request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":check"))
	response := httptest.NewRecorder()
	h.Action(response, request)
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("action status=%d calls=%d body=%s", response.Code, calls.Load(), response.Body.String())
	}
	if gotPath != "/items/item-1/serial/"+serial || gotSerial != serial || !strings.Contains(gotBody, "serial="+serial) {
		t.Fatalf("upstream received path=%q serial=%q body=%q, want %s everywhere", gotPath, gotSerial, gotBody, serial)
	}
}

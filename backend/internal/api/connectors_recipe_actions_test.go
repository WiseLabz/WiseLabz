package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestRecipeActionsAPIIntegration(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	app := newTestApp(t)
	operatorID, operatorToken := app.user(t, "operator")
	viewerID, viewerToken := app.user(t, "viewer")
	connectorOperatorID, connectorOperatorToken := app.user(t, "viewer")

	type receivedAction struct {
		method string
		path   string
		query  string
		body   string
	}
	var mu sync.Mutex
	var actions []receivedAction
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_token") != "integration-secret" {
			t.Errorf("request did not use saved query auth: %s %s query=%v", r.Method, r.URL.Path, r.URL.Query())
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/items":
			_, _ = fmt.Fprint(w, `[{"id":"db|1","title":"Database","node":"node-1"}]`)
		case r.Method == http.MethodPatch && r.URL.EscapedPath() == "/items/db%7C1/restart":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			actions = append(actions, receivedAction{method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery, body: string(body)})
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			_, _ = fmt.Fprint(w, "restart accepted")
		case r.Method == http.MethodPost && r.URL.Path == "/rescan":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			actions = append(actions, receivedAction{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: string(body)})
			mu.Unlock()
			_, _ = fmt.Fprint(w, "rescan complete")
		default:
			t.Errorf("unexpected recipe request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

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
        node: {path: node}
      actions:
        restart:
          method: PATCH
          path: /items/{external_id}/restart
          query: {node: "{attr.node}"}
          headers: {X-Secret-Token: static-header-secret}
          body: {reason: recipe-defined}
          label: Restart item
          description: Restart this item
          downtime_seconds: 7
actions:
  rescan:
    method: POST
    path: /rescan
    query: {source: library}
    body: {scope: all}
    label: Rescan library
    description: Refresh the library index
`
	create := app.reqElevated(t, http.MethodPost, "/api/connectors", map[string]any{
		"name": "Recipe actions",
		"type": "custom",
		"url":  server.URL,
		"config": map[string]any{
			"recipe":     recipe,
			"auth_token": "integration-secret",
		},
	}, operatorToken, "connector.recipeActions")
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		ID           string                         `json:"id"`
		Capabilities connector.CapabilityDescriptor `json:"capabilities"`
		Actions      []connector.ActionDescriptor   `json:"actions"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || !created.Capabilities.Restart || len(created.Actions) != 2 {
		t.Fatalf("created connector operations=%+v actions=%+v", created.Capabilities, created.Actions)
	}
	app.connectorGrant(t, operatorID, created.ID, "operator")
	app.connectorGrant(t, viewerID, created.ID, "viewer")
	app.connectorGrant(t, connectorOperatorID, created.ID, "operator")

	syncResponse := app.req(t, http.MethodPost, "/api/connectors/"+created.ID+"/sync", nil, operatorToken)
	if syncResponse.Code != http.StatusAccepted {
		t.Fatalf("sync status=%d body=%s", syncResponse.Code, syncResponse.Body.String())
	}
	waitForSyncRuns(t, app, created.ID, 1)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := app.Store.GetLatestSnapshot(context.Background(), created.ID); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	snapshot, err := app.Store.GetLatestSnapshot(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("sync did not store the latest snapshot: %v", err)
	}
	var synced connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(snapshot.Data), &synced); err != nil {
		t.Fatal(err)
	}
	if len(synced.Entities) != 1 || synced.Entities[0].ExternalID != "db|1" {
		t.Fatalf("synced entities=%+v", synced.Entities)
	}

	preview := app.req(t, http.MethodPost, "/api/connectors/"+created.ID+"/restart?dryRun=true", map[string]any{"entityRef": "db|1"}, operatorToken)
	if preview.Code != http.StatusOK {
		t.Fatalf("lifecycle preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	if !strings.Contains(preview.Body.String(), `"userDefined":true`) || !strings.Contains(preview.Body.String(), "Restart item") || strings.Contains(preview.Body.String(), "integration-secret") || strings.Contains(preview.Body.String(), "static-header-secret") {
		t.Fatalf("lifecycle preview metadata/redaction=%s", preview.Body.String())
	}
	mu.Lock()
	if len(actions) != 0 {
		t.Fatalf("dry run sent action requests: %+v", actions)
	}
	mu.Unlock()

	restart := app.reqElevated(t, http.MethodPost, "/api/connectors/"+created.ID+"/restart", map[string]any{"entityRef": "db|1"}, operatorToken, "connector.restart")
	if restart.Code != http.StatusOK || !strings.Contains(restart.Body.String(), `"status":"restarted"`) || !strings.Contains(restart.Body.String(), `"statusCode":202`) || !strings.Contains(restart.Body.String(), "restart accepted") {
		t.Fatalf("restart status=%d body=%s", restart.Code, restart.Body.String())
	}

	namedPreview := app.req(t, http.MethodPost, "/api/connectors/"+created.ID+"/actions/rescan?dryRun=true", map[string]any{}, operatorToken)
	if namedPreview.Code != http.StatusOK || !strings.Contains(namedPreview.Body.String(), "Rescan library") || strings.Contains(namedPreview.Body.String(), "integration-secret") {
		t.Fatalf("named preview status=%d body=%s", namedPreview.Code, namedPreview.Body.String())
	}
	named := app.reqElevated(t, http.MethodPost, "/api/connectors/"+created.ID+"/actions/rescan", map[string]any{"unexpected": "ignored"}, operatorToken, "connector.action", created.ID+":rescan")
	if named.Code != http.StatusOK || !strings.Contains(named.Body.String(), `"statusCode":200`) || !strings.Contains(named.Body.String(), "rescan complete") {
		t.Fatalf("named action status=%d body=%s", named.Code, named.Body.String())
	}
	viewer := app.req(t, http.MethodPost, "/api/connectors/"+created.ID+"/actions/rescan", nil, viewerToken)
	if viewer.Code != http.StatusForbidden {
		t.Fatalf("viewer action status=%d, want 403 body=%s", viewer.Code, viewer.Body.String())
	}
	scopedOperator := app.reqElevated(t, http.MethodPost, "/api/connectors/"+created.ID+"/actions/rescan", map[string]any{}, connectorOperatorToken, "connector.action", created.ID+":rescan")
	if scopedOperator.Code != http.StatusOK || !strings.Contains(scopedOperator.Body.String(), `"statusCode":200`) {
		t.Fatalf("connector operator action status=%d body=%s", scopedOperator.Code, scopedOperator.Body.String())
	}

	mu.Lock()
	gotActions := append([]receivedAction(nil), actions...)
	mu.Unlock()
	if len(gotActions) != 3 {
		t.Fatalf("upstream action requests=%+v", gotActions)
	}
	if gotActions[0].method != http.MethodPatch || gotActions[0].path != "/items/db%7C1/restart" || !strings.Contains(gotActions[0].query, "node=node-1") || !strings.Contains(gotActions[0].body, "recipe-defined") {
		t.Fatalf("lifecycle upstream request=%+v", gotActions[0])
	}
	for _, namedAction := range gotActions[1:] {
		if namedAction.method != http.MethodPost || namedAction.path != "/rescan" || !strings.Contains(namedAction.query, "source=library") || !strings.Contains(namedAction.body, "scope") {
			t.Fatalf("named upstream request=%+v", namedAction)
		}
	}

	restartAudits, _, err := app.Store.ListAuditRecords(context.Background(), "connector.restart", "connector", "", "", 0, 10)
	if err != nil || len(restartAudits) != 1 {
		t.Fatalf("lifecycle audits=%+v err=%v", restartAudits, err)
	}
	var restartDetail map[string]any
	if err := json.Unmarshal([]byte(restartAudits[0].Detail), &restartDetail); err != nil {
		t.Fatal(err)
	}
	if restartDetail["entityRef"] != "db|1" || restartDetail["method"] != http.MethodPatch || restartDetail["status"] != float64(http.StatusAccepted) || strings.Contains(restartAudits[0].Detail, "integration-secret") {
		t.Fatalf("lifecycle audit detail=%v", restartDetail)
	}
	actionAudits, _, err := app.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
	if err != nil || len(actionAudits) != 2 {
		t.Fatalf("named audits=%+v err=%v", actionAudits, err)
	}
	var actionDetail map[string]any
	if err := json.Unmarshal([]byte(actionAudits[0].Detail), &actionDetail); err != nil {
		t.Fatal(err)
	}
	if actionDetail["action"] != "rescan" || actionDetail["method"] != http.MethodPost || actionDetail["status"] != float64(http.StatusOK) || strings.Contains(actionAudits[0].Detail, "rescan complete") || strings.Contains(actionAudits[0].Detail, "integration-secret") {
		t.Fatalf("named audit detail=%v", actionDetail)
	}
	var scopedOperatorAudit bool
	for _, audit := range actionAudits {
		if audit.ActorUserID == connectorOperatorID && audit.ActorRole == "user" {
			scopedOperatorAudit = true
			break
		}
	}
	if !scopedOperatorAudit {
		t.Fatalf("connector-scoped operator action was not attributed to its operator grant: %+v", actionAudits)
	}
}

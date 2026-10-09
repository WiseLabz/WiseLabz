package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type actionReviewLogs struct {
	mu   sync.Mutex
	text strings.Builder
}

func (b *actionReviewLogs) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.Write(p)
}

func (b *actionReviewLogs) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.text.String()
}

const actionReviewRecipe = `version: 1
category: other
auth: {mode: header, name: x-client-id}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity:
      kind: item
      name: name
      external_id: id
      actions:
        restart:
          method: PATCH
          path: /items/{external_id}/restart
          headers: {X-Mode: public}
          body: {id: "{external_id}"}
        rescan:
          method: POST
          path: /items/{external_id}/rescan
          headers: {X-Mode: public}
          body: {id: "{external_id}"}
actions:
  restart: {method: POST, path: /restart}
  rescan: {method: POST, path: /rescan, label: Rescan library, description: Refresh the index, downtime_seconds: 5}
`

func assertActionReviewNoBody(t *testing.T, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "SENSITIVE-RESPONSE") {
		t.Fatalf("response body escaped its transient result: %s", encoded)
	}
}

// Read every column, including fields not exposed by record JSON, so adding a
// persistent writer cannot bypass the confidentiality check.
func assertActionReviewPersistentWriters(t *testing.T, app *testApp, userID string) {
	t.Helper()
	for _, table := range []string{"audit_log", "alerts", "runbook_runs", "runbook_run_steps", "journal_entries"} {
		rows, err := app.Store.DB().QueryContext(context.Background(), "SELECT * FROM "+table)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := rows.Columns()
		if err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			targets := make([]any, len(columns))
			for i := range values {
				targets[i] = &values[i]
			}
			if err := rows.Scan(targets...); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			for i, value := range values {
				if data, ok := value.([]byte); ok {
					value = string(data)
				}
				if strings.Contains(fmt.Sprint(value), "SENSITIVE-RESPONSE") {
					_ = rows.Close()
					t.Fatalf("response body leaked into %s.%s", table, columns[i])
				}
			}
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	timeline, _, _, err := app.Store.ListTimeline(context.Background(), store.TimelineFilter{UserID: userID, Admin: true}, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertActionReviewNoBody(t, timeline)
}

func TestRecipeActionRealPathsPreserveAuthorizationAuditAndConfidentiality(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	for _, runbook := range []bool{false, true} {
		for _, named := range []bool{false, true} {
			for _, failure := range []bool{false, true} {
				name := fmt.Sprintf("runbook=%t/named=%t/failure=%t", runbook, named, failure)
				t.Run(name, func(t *testing.T) {
					var logs actionReviewLogs
					oldLogger := slog.Default()
					slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
					t.Cleanup(func() { slog.SetDefault(oldLogger) })
					app := newTestApp(t)
					userID, token := app.user(t, "operator")
					app.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) {
						return auth.RuntimeSettings{StepUpForDestructive: true}, true
					})
					if !app.JWT.StepUpEnabled() {
						t.Fatal("authorization case must run with step-up enabled")
					}
					action, verb, method := "restart", "restart", http.MethodPatch
					auditAction, elevationAction := "connector.restart", "connector.restart"
					if named {
						action, verb, method = "rescan", "", http.MethodPost
						auditAction, elevationAction = "connector.action", "connector.action"
					}
					var sends, fetches atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method == http.MethodGet {
							fetches.Add(1)
							_, _ = io.WriteString(w, `[]`)
							return
						}
						sends.Add(1)
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Error(err)
						}
						if r.Method != method || r.URL.EscapedPath() != "/items/db%7C1/"+action ||
							r.Header.Get("X-Client-Id") != "credential-value" || r.Header.Get("X-License") != "legacy-value" ||
							!strings.Contains(string(body), `"db|1"`) {
							t.Errorf("wrong declared request: %s %s body=%s", r.Method, r.URL.EscapedPath(), body)
						}
						w.Header().Set("Content-Type", "text/plain")
						if failure {
							w.WriteHeader(http.StatusConflict)
						}
						_, _ = io.WriteString(w, "SENSITIVE-RESPONSE-must-stay-transient")
					}))
					defer server.Close()
					create := app.reqElevated(t, http.MethodPost, "/api/connectors", map[string]any{
						"name": "Reviewed recipe", "type": "custom", "url": server.URL,
						"config": map[string]any{"recipe": actionReviewRecipe, "auth_token": "credential-value",
							"headers": `{"x-license":"legacy-value"}`},
					}, token, "connector.recipeActions")
					if create.Code != http.StatusCreated {
						t.Fatalf("create=%d %s", create.Code, create.Body)
					}
					var created struct {
						ID      string                       `json:"id"`
						Actions []connector.ActionDescriptor `json:"actions"`
					}
					if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
						t.Fatal(err)
					}
					app.connectorGrant(t, userID, created.ID, "operator")
					if len(created.Actions) != 4 {
						t.Fatalf("declared metadata missing: %+v", created.Actions)
					}
					for _, descriptor := range created.Actions {
						if descriptor.Name == "restart" && (descriptor.Label != "restart" || descriptor.DowntimeSeconds != 30) {
							t.Fatalf("restart defaults=%+v", descriptor)
						}
						if descriptor.Name == "rescan" && descriptor.EntityScope &&
							(descriptor.Label != "rescan" || descriptor.DowntimeSeconds != 0 || descriptor.EntityKind != "item") {
							t.Fatalf("entity rescan defaults=%+v", descriptor)
						}
						if descriptor.Name == "rescan" && !descriptor.EntityScope &&
							(descriptor.Label != "Rescan library" || descriptor.Description != "Refresh the index" || descriptor.DowntimeSeconds != 5) {
							t.Fatalf("declared service metadata=%+v", descriptor)
						}
					}
					if err := app.Store.CreateSnapshot(context.Background(), &store.SnapshotRecord{
						ConnectorID: created.ID, FetchedAt: time.Now().UTC().Format(time.RFC3339),
						Data: `{"serviceName":"reviewed","entities":[{"kind":"item","name":"DB","externalId":"db|1"}]}`,
					}); err != nil {
						t.Fatal(err)
					}
					apiServer := httptest.NewServer(app.Router)
					defer apiServer.Close()
					frames := wsDial(t, app, apiServer, token)
					deadline := time.Now().Add(5 * time.Second)
					for app.WSHub.ClientCount() == 0 && time.Now().Before(deadline) {
						time.Sleep(time.Millisecond)
					}
					path := "/api/connectors/" + created.ID + "/" + action
					if named {
						path = "/api/connectors/" + created.ID + "/actions/" + action
					}
					body := map[string]any{"entityRef": "db|1"}
					preview := app.req(t, http.MethodPost, path+"?dryRun=true", body, token)
					if preview.Code != http.StatusOK || sends.Load() != 0 ||
						strings.Contains(preview.Body.String(), "credential-value") || strings.Contains(preview.Body.String(), "legacy-value") {
						t.Fatalf("unsafe preview=%d %s sends=%d", preview.Code, preview.Body, sends.Load())
					}
					var previewData struct {
						Label    string                  `json:"label"`
						Downtime int                     `json:"estimatedDowntimeSeconds"`
						Request  connector.ActionRequest `json:"request"`
					}
					if err := json.Unmarshal(preview.Body.Bytes(), &previewData); err != nil {
						t.Fatal(err)
					}
					wantDowntime := 30
					if named {
						wantDowntime = 0
					}
					if previewData.Label != action || previewData.Downtime != wantDowntime ||
						previewData.Request.Headers["X-Mode"] != "public" || previewData.Request.Headers["X-Client-Id"] != "[redacted]" {
						t.Fatalf("preview defaults/request=%+v", previewData)
					}
					var runID string
					if runbook {
						step := map[string]any{"kind": "lifecycle", "title": "Execute declared action", "connectorId": created.ID,
							"verb": verb, "entityRef": "db|1"}
						if named {
							step["kind"], step["action"] = "connector_action", action
						}
						author := app.req(t, http.MethodPost, "/api/runbooks", map[string]any{
							"title": "Recipe runtime", "targetType": "change_type", "targetValue": "review", "steps": []any{step},
						}, token)
						if author.Code != http.StatusCreated {
							t.Fatalf("author=%d %s", author.Code, author.Body)
						}
						var book struct {
							ID string `json:"id"`
						}
						if err := json.Unmarshal(author.Body.Bytes(), &book); err != nil {
							t.Fatal(err)
						}
						start := app.reqElevated(t, http.MethodPost, "/api/runbooks/"+book.ID+"/run", nil, token, "runbook.run", book.ID)
						if start.Code != http.StatusAccepted {
							t.Fatalf("start=%d %s", start.Code, start.Body)
						}
						var started struct {
							ID string `json:"id"`
						}
						if err := json.Unmarshal(start.Body.Bytes(), &started); err != nil {
							t.Fatal(err)
						}
						runID = started.ID
						deadline := time.Now().Add(10 * time.Second)
						for {
							run, steps, err := app.Store.GetRunbookRun(context.Background(), runID)
							if err != nil {
								t.Fatal(err)
							}
							if run.State == "succeeded" || run.State == "failed" {
								want := "succeeded"
								if failure {
									want = "failed"
								}
								if run.State != want || len(steps) != 1 || steps[0].State != want {
									t.Fatalf("unexpected run outcome=%+v steps=%+v", run, steps)
								}
								assertActionReviewNoBody(t, run)
								assertActionReviewNoBody(t, steps)
								break
							}
							if time.Now().After(deadline) {
								t.Fatal("run did not finish")
							}
							time.Sleep(time.Millisecond)
						}
					} else {
						missing := app.req(t, http.MethodPost, path, body, token)
						if missing.Code != http.StatusBadRequest || !strings.Contains(missing.Body.String(), "elevation_required") || sends.Load() != 0 {
							t.Fatalf("missing-token call=%d %s sends=%d", missing.Code, missing.Body, sends.Load())
						}
						targets := []string{}
						if named {
							targets = append(targets, created.ID+":"+action)
						}
						response := app.reqElevated(t, http.MethodPost, path, body, token, elevationAction, targets...)
						want := http.StatusOK
						if failure {
							want = http.StatusBadGateway
						}
						if response.Code != want || !strings.Contains(response.Body.String(), "SENSITIVE-RESPONSE") {
							t.Fatalf("action=%d %s", response.Code, response.Body)
						}
					}
					if sends.Load() != 1 || fetches.Load() != 0 {
						t.Fatalf("requests: sends=%d fetches=%d", sends.Load(), fetches.Load())
					}
					syncRows, err := app.Store.ListSyncRunsByConnector(context.Background(), created.ID, 10)
					if err != nil || len(syncRows) != 0 {
						t.Fatalf("action triggered sync: %+v %v", syncRows, err)
					}
					audits, _, err := app.Store.ListAuditRecords(context.Background(), auditAction, "connector", "", "", 0, 10)
					if err != nil {
						t.Fatal(err)
					}
					wantAudit := 1
					if failure {
						wantAudit = 0
					}
					if len(audits) != wantAudit {
						t.Fatalf("action audits=%+v", audits)
					}
					if !failure {
						var detail map[string]any
						if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
							t.Fatal(err)
						}
						if audits[0].ActorUserID != userID || audits[0].TargetID != created.ID || detail["entityRef"] != "db|1" ||
							detail["method"] != method || detail["url"] != server.URL+"/items/db%7C1/"+action || detail["status"] != float64(http.StatusOK) {
							t.Fatalf("wrong action audit=%+v detail=%v", audits[0], detail)
						}
						if named && detail["action"] != action {
							t.Fatalf("missing action name: %v", detail)
						}
						if runbook && (detail["runId"] != runID || detail["stepId"] == "") {
							t.Fatalf("missing run audit identifiers: %v", detail)
						}
					}
					alerts, _, err := app.Store.ListAlerts(context.Background(), created.ID, "", "", "", 0, 10)
					wantAlerts := 0
					if failure {
						wantAlerts = 1
					}
					if err != nil || len(alerts) != wantAlerts {
						t.Fatalf("alerts=%+v err=%v", alerts, err)
					}
					assertActionReviewPersistentWriters(t, app, userID)
					if strings.Contains(logs.String(), "SENSITIVE-RESPONSE") {
						t.Fatal("response body leaked to debug-enabled logs")
					}
					// A terminal run event and an alert event must have traversed the real hub.
					wantEvent := "review.barrier"
					if runbook {
						wantEvent = "runbook.run.updated"
					} else {
						app.WSHub.Broadcast(wantEvent, nil)
					}
					eventDeadline := time.After(5 * time.Second)
					sawAlert := false
					for {
						select {
						case frame := <-frames:
							assertActionReviewNoBody(t, frame)
							if frame.Type == "alert.created" {
								sawAlert = true
							}
							terminal := frame.Type == wantEvent
							if runbook {
								data, err := json.Marshal(frame.Payload)
								if err != nil {
									t.Fatal(err)
								}
								var event struct {
									RunID string `json:"runId"`
									State string `json:"state"`
								}
								if err := json.Unmarshal(data, &event); err != nil {
									t.Fatal(err)
								}
								terminal = terminal && event.RunID == runID && (event.State == "succeeded" || event.State == "failed")
							}
							if terminal {
								if failure && !sawAlert {
									t.Fatal("failure alert never traversed websocket hub")
								}
								if strings.Contains(logs.String(), "SENSITIVE-RESPONSE") {
									t.Fatal("response body leaked to debug-enabled logs after event delivery")
								}
								return
							}
						case <-eventDeadline:
							t.Fatal("expected action/run websocket delivery did not arrive")
						}
					}
				})
			}
		}
	}
}

func TestRecipeNamedActionExecutesWhenStepUpIsDisabled(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	app := newTestApp(t)
	userID, token := app.user(t, "operator")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	configData, err := store.MarshalConnectorConfig("custom", map[string]any{"recipe": actionReviewRecipe, "auth_token": "credential-value"}, app.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	record := &store.ConnectorRecord{Name: "No step-up", Category: "other", Type: "custom", URL: server.URL, ConfigData: configData}
	if err := app.Store.CreateConnector(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	app.connectorGrant(t, userID, record.ID, "operator")
	app.JWT.SetSettingsSource(func() (auth.RuntimeSettings, bool) { return auth.RuntimeSettings{StepUpForDestructive: false}, true })
	response := app.req(t, http.MethodPost, "/api/connectors/"+record.ID+"/actions/rescan", nil, token)
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("disabled step-up action=%d %s calls=%d", response.Code, response.Body, calls.Load())
	}
}

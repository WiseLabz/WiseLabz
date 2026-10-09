package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// runActionStepAgainst runs a one-step connector_action runbook for the
// recipe's service rescan action against server and returns the finished run
// and its step.
func runActionStepAgainst(t *testing.T, server *httptest.Server) (*testApp, *store.RunbookRunRecord, *store.RunbookRunStepRecord) {
	t.Helper()
	app := newTestApp(t)
	userID, token := app.user(t, "operator")
	configData, err := store.MarshalConnectorConfig("custom", map[string]any{"recipe": actionReviewRecipe, "auth_token": "credential-value"}, app.Config.Encryption.Key)
	if err != nil {
		t.Fatal(err)
	}
	record := &store.ConnectorRecord{Name: "Outcome recipe", Category: "other", Type: "custom", URL: server.URL, ConfigData: configData}
	if err := app.Store.CreateConnector(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	app.connectorGrant(t, userID, record.ID, "operator")
	author := app.req(t, http.MethodPost, "/api/runbooks", map[string]any{
		"title": "Outcome", "targetType": "change_type", "targetValue": "outcome",
		"steps": []any{map[string]any{"kind": "connector_action", "title": "Rescan", "connectorId": record.ID, "action": "rescan"}},
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
	deadline := time.Now().Add(10 * time.Second)
	for {
		run, steps, err := app.Store.GetRunbookRun(context.Background(), started.ID)
		if err != nil {
			t.Fatal(err)
		}
		if run.State == "succeeded" || run.State == "failed" {
			if len(steps) != 1 {
				t.Fatalf("steps=%+v", steps)
			}
			return app, run, steps[0]
		}
		if time.Now().After(deadline) {
			t.Fatal("run did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRunbookActionStepOutcomes(t *testing.T) {
	connector.AllowLoopbackForTest(t)

	t.Run("upstream 500 fails the step without recording the response text", func(t *testing.T) {
		const sentinel = "UPSTREAM-SENTINEL-TEXT"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, sentinel)
		}))
		defer server.Close()
		app, run, step := runActionStepAgainst(t, server)
		if step.State != "failed" || run.State != "failed" {
			t.Fatalf("run=%+v step=%+v, want both failed", run, step)
		}
		if !strings.Contains(step.Error, "500") || strings.Contains(step.Error, sentinel) {
			t.Fatalf("step error = %q, want the status and not the response text", step.Error)
		}
		encoded, err := json.Marshal([]any{run, step})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), sentinel) {
			t.Fatalf("response text escaped into the run: %s", encoded)
		}
		audits, _, err := app.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
		if err != nil || len(audits) != 0 {
			t.Fatalf("audits for a failed action = %+v, %v; want none", audits, err)
		}
	})

	t.Run("a connection closed without an answer leaves the step unknown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("Hijack() error = %v", err)
				return
			}
			_ = conn.Close()
		}))
		defer server.Close()
		app, run, step := runActionStepAgainst(t, server)
		if step.State != "unknown" || run.State != "failed" {
			t.Fatalf("run=%+v step=%+v, want an unknown step in a failed run", run, step)
		}
		audits, _, err := app.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
		if err != nil || len(audits) != 0 {
			t.Fatalf("audits for an unanswered action = %+v, %v; want none", audits, err)
		}
	})
}

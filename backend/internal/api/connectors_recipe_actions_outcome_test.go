package api_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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

	for _, tc := range []struct {
		name  string
		reset bool
	}{
		{"200 then a stalled body", false},
		{"200 then a connection reset mid body", true},
	} {
		t.Run(tc.name+" still succeeds exactly once", func(t *testing.T) {
			var calls atomic.Int32
			done := make(chan struct{})
			body := strings.Repeat("a", 600)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if tc.reset {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("Hijack() error = %v", err)
						return
					}
					_, _ = io.WriteString(conn, "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 5000\r\n\r\n"+body)
					_ = conn.Close()
					return
				}
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, body)
				w.(http.Flusher).Flush()
				select {
				case <-done:
				case <-r.Context().Done():
				}
			}))
			defer server.Close()
			defer close(done)
			app, run, step := runActionStepAgainst(t, server)
			if step.State != "succeeded" || run.State != "succeeded" || step.Error != "" {
				t.Fatalf("run=%+v step=%+v, want both succeeded with no error", run, step)
			}
			if calls.Load() != 1 {
				t.Errorf("upstream requests = %d, want 1", calls.Load())
			}
			alerts, _, err := app.Store.ListAlerts(context.Background(), "", "", "", "", 0, 10)
			if err != nil || len(alerts) != 0 {
				t.Errorf("alerts = %+v, %v; want none", alerts, err)
			}
			audits, _, err := app.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
			if err != nil || len(audits) != 1 {
				t.Fatalf("audits = %+v, %v; want exactly one", audits, err)
			}
			var detail map[string]any
			if err := json.Unmarshal([]byte(audits[0].Detail), &detail); err != nil {
				t.Fatal(err)
			}
			if detail["runId"] != run.ID || detail["stepId"] != step.ID {
				t.Errorf("audit detail = %v, want run %s and step %s", detail, run.ID, step.ID)
			}
		})
	}
}

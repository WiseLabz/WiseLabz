package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/httputil"
)

const outcomeSentinel = "LATE-BODY-SENTINEL"

// outcomeUpstream answers status and then misbehaves after the response head:
// a stall that lasts until the test ends, or a connection reset mid-body. The
// 600 byte body carries outcomeSentinel after byte 520, past the excerpt.
func outcomeUpstream(status int, reset bool, calls *atomic.Int32, done <-chan struct{}) http.Handler {
	body := strings.Repeat("a", 530) + outcomeSentinel + strings.Repeat("b", 600-530-len(outcomeSentinel))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if reset {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				return
			}
			_, _ = io.WriteString(conn, fmt.Sprintf("HTTP/1.1 %d %s\r\n", status, http.StatusText(status)))
			_, _ = io.WriteString(conn, "Content-Type: text/plain\r\nContent-Length: 5000\r\n\r\n"+body)
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		w.(http.Flusher).Flush()
		select {
		case <-done:
		case <-r.Context().Done():
		}
	})
}

func TestNamedActionOutcomeIgnoresBodyFailureAfterStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reset  bool
	}{
		{"200 then stall", http.StatusOK, false},
		{"200 then reset", http.StatusOK, true},
		{"500 then stall", http.StatusInternalServerError, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(t)
			var calls atomic.Int32
			done := make(chan struct{})
			server := httptest.NewServer(outcomeUpstream(tc.status, tc.reset, &calls, done))
			defer server.Close()
			defer close(done)
			record := seedRecipeActionConnector(t, h, server.URL)
			seedRecipeActionSnapshot(t, h, record.ID)

			request := actionHandlerRequest(record.ID, "rescan", "/", `{}`, "operator", false)
			request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", "connector.action", record.ID+":rescan"))
			response := httptest.NewRecorder()
			start := time.Now()
			h.Action(response, request)
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Errorf("handler took %v, want it to answer without waiting on the body", elapsed)
			}
			if calls.Load() != 1 {
				t.Errorf("upstream requests = %d, want 1", calls.Load())
			}

			audits, _, err := h.Store.ListAuditRecords(context.Background(), "connector.action", "connector", "", "", 0, 10)
			if err != nil {
				t.Fatal(err)
			}
			alerts, _, err := h.Store.ListAlerts(context.Background(), record.ID, "", "", "", 0, 10)
			if err != nil {
				t.Fatal(err)
			}
			for _, audit := range audits {
				if strings.Contains(audit.Detail, outcomeSentinel) {
					t.Errorf("audit leaked the body: %s", audit.Detail)
				}
			}
			for _, alert := range alerts {
				if strings.Contains(alert.Description, outcomeSentinel) || strings.Contains(alert.Title, outcomeSentinel) {
					t.Errorf("alert leaked the body: %+v", alert)
				}
			}

			if tc.status == http.StatusOK {
				var result map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if response.Code != http.StatusOK || result["status"] != float64(http.StatusOK) {
					t.Fatalf("status=%d body=%s, want 200 with status 200", response.Code, response.Body.String())
				}
				if len(audits) != 1 || len(alerts) != 0 {
					t.Errorf("audits=%d alerts=%d, want 1 audit and no alert", len(audits), len(alerts))
				}
				return
			}
			if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), `"statusCode":500`) {
				t.Fatalf("status=%d body=%s, want 502 with statusCode 500", response.Code, response.Body.String())
			}
			if len(audits) != 0 || len(alerts) == 0 {
				t.Errorf("audits=%d alerts=%d, want no audit and an alert", len(audits), len(alerts))
			}
		})
	}
}

func TestOversizedRequestBodyIsRefusedBeforeAnythingIsSent(t *testing.T) {
	pad := strings.Repeat("x", httputil.MaxJSONBodyBytes+1)
	bodies := map[string]string{
		"entityRef after the limit":  `{"pad":"` + pad + `","entityRef":"db|1"}`,
		"entityRef before the limit": `{"entityRef":"db|1","pad":"` + pad + `"}`,
	}
	routes := []struct {
		name   string
		action string
		target string
		serve  func(h *Handler, w http.ResponseWriter, r *http.Request)
	}{
		{"named action", "connector.action", ":rescan", func(h *Handler, w http.ResponseWriter, r *http.Request) { h.Action(w, r) }},
		{"restart", "connector.restart", "", func(h *Handler, w http.ResponseWriter, r *http.Request) { h.RestartPreview(w, r) }},
	}
	for bodyName, body := range bodies {
		for _, route := range routes {
			t.Run(route.name+"/"+bodyName, func(t *testing.T) {
				h := newTestHandler(t)
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusNoContent)
				}))
				defer server.Close()
				record := seedRecipeActionConnectorWithRecipe(t, h, server.URL, restartRecipe)
				seedRecipeActionSnapshot(t, h, record.ID)

				request := actionHandlerRequest(record.ID, "rescan", "/", body, "operator", false)
				request.Header.Set("X-Elevation-Token", issueTestElevation(t, h, "operator", route.action, record.ID+route.target))
				response := httptest.NewRecorder()
				route.serve(h, response, request)

				if response.Code != http.StatusRequestEntityTooLarge || !strings.Contains(response.Body.String(), "request_too_large") {
					t.Fatalf("status=%d body=%s, want 413 request_too_large", response.Code, response.Body.String())
				}
				if calls.Load() != 0 {
					t.Errorf("upstream requests = %d, want 0", calls.Load())
				}
				for _, action := range []string{"connector.action", "connector.restart"} {
					audits, _, err := h.Store.ListAuditRecords(context.Background(), action, "connector", "", "", 0, 10)
					if err != nil || len(audits) != 0 {
						t.Errorf("%s audits = %+v, %v; want none", action, audits, err)
					}
				}
			})
		}
	}
}

const restartRecipe = `version: 1
category: other
auth: {mode: query, name: api_token}
endpoints:
  - name: items
    path: /items
    method: GET
    items: '@this'
    entity: {kind: item, name: title, external_id: id}
actions:
  restart: {method: POST, path: /restart}
  rescan: {method: POST, path: /rescan}
`

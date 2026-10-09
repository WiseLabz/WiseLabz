package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func recipeForActionExecution(auth, actions string) string {
	return "version: 1\ncategory: media\nauth:\n" + auth + "\n" + actions + `
endpoints:
  - name: items
    path: /api/items
    method: GET
    items: items
    entity: {kind: item, name: name, external_id: id}
`
}

func serviceActionRecipe(method string) string {
	return recipeForActionExecution("  mode: none", fmt.Sprintf("actions:\n  operation:\n    method: %s\n    path: /api/operation\n", method))
}

func TestResolveActionUsesLatestEntitySnapshotAndRedactsPreview(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "unexpected request")
	}))
	defer server.Close()
	raw := `version: 1
category: media
auth:
  mode: query
  name: api_token
actions:
  rescan:
    method: POST
    path: /api/rescan
    query: {source: library, escaped: "{{literal}}"}
    body: {scope: all, escaped: "{{literal}}"}
endpoints:
  - name: containers
    path: /api/containers
    method: GET
    items: items
    entity:
      kind: container
      name: name
      external_id: id
      attributes:
        node: {path: node}
      actions:
        restart:
          method: PATCH
          path: /api/containers/{external_id}/restart
          query: {node: "{attr.node}"}
          headers: {X-Mode: fast, Authorization: "Bearer static-secret", X-Session: "Bearer value-secret"}
          body: {id: "{external_id}", node: "{attr.node}", escaped: "{{literal}}"}
          label: Restart container
          description: Restart the selected container
          downtime_seconds: 5
`
	config := map[string]any{"url": server.URL, "recipe": raw, "auth_token": "query-secret"}
	conn := &Connector{client: server.Client()}
	snapshot := &connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{
		Kind: "container", ExternalID: "db|1", Attributes: map[string]any{"node": "rack/1"},
	}}}
	resolved, err := conn.ResolveAction(config, "restart", "db|1", snapshot)
	if err != nil {
		t.Fatalf("ResolveAction() error = %v", err)
	}
	parsed, err := url.Parse(resolved.Request.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := parsed.EscapedPath(), "/api/containers/db%7C1/restart"; got != want {
		t.Errorf("escaped path = %q, want %q", got, want)
	}
	if parsed.Query().Get("node") != "rack/1" || parsed.Query().Get("api_token") != "query-secret" {
		t.Errorf("request query = %v", parsed.Query())
	}
	if resolved.Request.Method != http.MethodPatch || resolved.Request.Headers["X-Mode"] != "fast" {
		t.Errorf("request = %+v", resolved.Request)
	}
	if got := resolved.Descriptor; got.Name != "restart" || got.EntityKind != "container" || !got.EntityScope || got.Label != "Restart container" || got.DowntimeSeconds != 5 {
		t.Errorf("descriptor = %+v", got)
	}
	body, err := json.Marshal(resolved.Request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), `{"escaped":"{literal}","id":"db|1","node":"rack/1"}`; got != want {
		t.Errorf("resolved body = %s, want %s", got, want)
	}
	preview := RedactedActionRequest(config, resolved)
	previewURL, err := url.Parse(preview.URL)
	if err != nil {
		t.Fatal(err)
	}
	if previewURL.Query().Get("api_token") != "" || previewURL.Query().Get("node") != "rack/1" {
		t.Errorf("preview query = %v", previewURL.Query())
	}
	if preview.Headers["Authorization"] != "[redacted]" || preview.Headers["X-Session"] != "[redacted]" || preview.Headers["X-Mode"] != "fast" {
		t.Errorf("preview headers = %v", preview.Headers)
	}
	if calls.Load() != 0 {
		t.Fatalf("ResolveAction sent %d requests", calls.Load())
	}
	service, err := conn.ResolveAction(config, "rescan", "", nil)
	if err != nil {
		t.Fatalf("service ResolveAction() error = %v", err)
	}
	servicePreview := RedactedActionRequest(config, service)
	serviceURL, _ := url.Parse(servicePreview.URL)
	if serviceURL.Query().Get("source") != "library" || serviceURL.Query().Get("escaped") != "{literal}" || serviceURL.Query().Get("api_token") != "" {
		t.Errorf("service preview URL = %q", servicePreview.URL)
	}
	serviceBody, err := json.Marshal(servicePreview.Body)
	if err != nil || !strings.Contains(string(serviceBody), `"escaped":"{literal}"`) {
		t.Errorf("service action body = %s, error %v", serviceBody, err)
	}
}

func TestResolveActionRejectsInvalidTargetsBeforeSending(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, "unexpected request")
	}))
	defer server.Close()
	raw := `version: 1
category: media
auth: {mode: none}
endpoints:
  - name: containers
    path: /api/containers
    method: GET
    items: items
    entity:
      kind: container
      name: name
      external_id: id
      attributes:
        node: {path: node}
      actions:
        restart: {method: POST, path: "/api/containers/{external_id}/restart", query: {node: "{attr.node}"}}
`
	config := map[string]any{"url": server.URL, "recipe": raw}
	conn := &Connector{client: server.Client()}
	for _, id := range []string{"../admin", "web 1", "two..dots", "a/b", "a?b", "a#b", "a%b", "a;b", "a\x01b", "é"} {
		t.Run(fmt.Sprintf("id %q", id), func(t *testing.T) {
			snapshot := &connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "container", ExternalID: id, Attributes: map[string]any{"node": "node"}}}}
			_, err := conn.ResolveAction(config, "restart", id, snapshot)
			if err == nil {
				t.Fatalf("ResolveAction accepted invalid path value %q", id)
			}
			if id == "web 1" && !strings.Contains(err.Error(), id) {
				t.Errorf("error %q does not name rejected value", err)
			}
		})
	}
	snapshot := &connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "container", ExternalID: "ok"}}}
	if _, err := conn.ResolveAction(config, "restart", "missing", snapshot); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown entity error = %v", err)
	}
	if _, err := conn.ResolveAction(config, "restart", "ok", snapshot); err == nil || !strings.Contains(err.Error(), "node") {
		t.Errorf("missing attribute error = %v", err)
	}
	if _, err := conn.ResolveAction(config, "missing", "", snapshot); err == nil || !strings.Contains(err.Error(), "not declared") {
		t.Errorf("undeclared action error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid resolution sent %d requests", calls.Load())
	}
}

func TestSendActionSendsEachMethodOnce(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != method || r.URL.Path != "/api/operation" {
					t.Errorf("request = %s %s, want %s /api/operation", r.Method, r.URL.Path, method)
				}
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = io.WriteString(w, "OK")
			}))
			defer server.Close()
			config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(method)}
			conn := &Connector{client: server.Client()}
			action, err := conn.ResolveAction(config, "operation", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			result, err := conn.SendAction(context.Background(), config, action)
			if err != nil || result.Status != http.StatusOK || result.Excerpt != "OK" || !result.Written {
				t.Errorf("result/error = %+v / %v", result, err)
			}
			if calls.Load() != 1 {
				t.Errorf("request count = %d, want 1", calls.Load())
			}
			encoded, _ := json.Marshal(result)
			if strings.Contains(string(encoded), "written") {
				t.Errorf("JSON result exposed written: %s", encoded)
			}
		})
	}
}

func TestSendActionIncludesStatusButNeverBodyInErrors(t *testing.T) {
	const responseBody = `{"error":"response-secret"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, responseBody)
	}))
	defer server.Close()
	config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	conn := &Connector{client: server.Client()}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := conn.SendAction(context.Background(), config, action)
	if err == nil || !strings.Contains(err.Error(), "409") || strings.Contains(err.Error(), "response-secret") {
		t.Fatalf("SendAction() error = %v", err)
	}
	if result.Status != http.StatusConflict || result.Excerpt != responseBody || !result.Written {
		t.Errorf("result = %+v", result)
	}

	longServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, strings.Repeat("x", 2048))
	}))
	defer longServer.Close()
	longConfig := map[string]any{"url": longServer.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	longConn := &Connector{client: longServer.Client()}
	longAction, err := longConn.ResolveAction(longConfig, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	longResult, err := longConn.SendAction(context.Background(), longConfig, longAction)
	if err != nil || len(longResult.Excerpt) != 512 {
		t.Errorf("long result/error = %+v / %v", longResult, err)
	}
	if got := actionTextExcerpt("", []byte("a\x00b")); got != "ab" {
		t.Errorf("excerpt with controls = %q, want %q", got, "ab")
	}
	if got := actionTextExcerpt("", []byte("OK")); got != "OK" {
		t.Errorf("excerpt without content-type = %q", got)
	}
}

func TestSendActionDoesNotFollowRedirectAndOmitsBinaryExcerpt(t *testing.T) {
	var destinationCalls atomic.Int32
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		destinationCalls.Add(1)
		_, _ = io.WriteString(w, "followed")
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", destination.URL+"/other-origin")
		w.WriteHeader(http.StatusFound)
	}))
	defer redirect.Close()
	config := map[string]any{"url": redirect.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	conn := &Connector{client: redirect.Client()}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := conn.SendAction(context.Background(), config, action)
	if err == nil || result.Status != http.StatusFound || !result.Written || destinationCalls.Load() != 0 {
		t.Errorf("redirect result/error = %+v / %v; destination calls %d", result, err, destinationCalls.Load())
	}

	binary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0, 1, 2, 3, 255})
	}))
	defer binary.Close()
	binaryConfig := map[string]any{"url": binary.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	binaryConn := &Connector{client: binary.Client()}
	binaryAction, err := binaryConn.ResolveAction(binaryConfig, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	binaryResult, err := binaryConn.SendAction(context.Background(), binaryConfig, binaryAction)
	if err != nil || binaryResult.Status != http.StatusOK || binaryResult.Excerpt != "" {
		t.Errorf("binary result/error = %+v / %v", binaryResult, err)
	}
}

func TestSendActionOversizeAndDroppedConnection(t *testing.T) {
	oversized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, strings.Repeat("z", connector.MaxResponseBytes+1))
	}))
	defer oversized.Close()
	config := map[string]any{"url": oversized.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	conn := &Connector{client: oversized.Client()}
	action, err := conn.ResolveAction(config, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := conn.SendAction(context.Background(), config, action)
	if err != nil || result.Status != http.StatusOK || len(result.Excerpt) != 512 || !result.Written {
		t.Errorf("oversized result/error = %+v / %v, want success with a 512 byte excerpt", result, err)
	}

	dropped := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer dropped.Close()
	dropConfig := map[string]any{"url": dropped.URL, "recipe": serviceActionRecipe(http.MethodPost)}
	dropConn := &Connector{client: dropped.Client()}
	dropAction, err := dropConn.ResolveAction(dropConfig, "operation", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	dropResult, err := dropConn.SendAction(context.Background(), dropConfig, dropAction)
	if err == nil || !dropResult.Written || dropResult.Status != 0 {
		t.Errorf("dropped result/error = %+v / %v", dropResult, err)
	}
}

func TestCustomCapabilitiesArePerInstance(t *testing.T) {
	withConfig := map[string]any{"recipe": recipeForActionExecution("  mode: none", "actions:\n  restart: {method: POST, path: /restart}\n")}
	withoutConfig := map[string]any{"recipe": recipeForActionExecution("  mode: none", "actions: {}\n")}
	with, err := connector.Get(typeName, withConfig)
	if err != nil {
		t.Fatal(err)
	}
	without, err := connector.Get(typeName, withoutConfig)
	if err != nil {
		t.Fatal(err)
	}
	if !connector.Capabilities(with).Restart || connector.Capabilities(without).Restart {
		t.Fatalf("restart capability = %v / %v, want true / false", connector.Capabilities(with).Restart, connector.Capabilities(without).Restart)
	}
	if !connector.SupportsLifecycleVerb(typeName, "restart", withConfig) || connector.SupportsLifecycleVerb(typeName, "restart", withoutConfig) {
		t.Fatalf("registry lifecycle support did not vary by recipe")
	}
	if got := with.(connector.InstanceCapabilities).DeclaredActions(); len(got) != 1 || got[0].Name != "restart" {
		t.Fatalf("declared actions = %#v", got)
	}
}

func TestCustomLifecycleVerbsSendDeclaredRequestOnce(t *testing.T) {
	for _, verb := range []string{"restart", "start", "stop"} {
		t.Run(verb, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/api/"+verb {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			actions := fmt.Sprintf("actions:\n  %s: {method: POST, path: /api/%s}\n", verb, verb)
			config := map[string]any{"url": server.URL, "recipe": recipeForActionExecution("  mode: none", actions)}
			conn := &Connector{client: server.Client(), capabilityConfig: actionCapabilityConfig(config)}
			operation, ok := connector.LifecycleOp(conn, verb)
			if !ok {
				t.Fatalf("LifecycleOp(%q) unsupported", verb)
			}
			if err := operation(context.Background(), config, ""); err != nil {
				t.Fatalf("lifecycle operation error = %v", err)
			}
			if calls.Load() != 1 {
				t.Errorf("request count = %d, want 1", calls.Load())
			}
		})
	}
}

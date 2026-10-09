package custom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestActionPreviewRedactsConfiguredSecretHeaderNames(t *testing.T) {
	for _, legacy := range []any{`{"x-license":"legacy-value"}`, map[string]string{"x-license": "legacy-value"}} {
		t.Run("legacy headers", func(t *testing.T) {
			config := map[string]any{
				"url": "https://example.com", "headers": legacy,
				"recipe": recipeForActionExecution("  mode: header\n  name: x-client-id",
					"actions:\n  operation: {method: POST, path: /operate, headers: {X-Mode: public}}\n"),
				"auth_token": "credential-value",
			}
			conn := &Connector{}
			resolved, err := conn.ResolveAction(config, "operation", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			preview := RedactedActionRequest(config, resolved)
			if preview.Headers["X-Client-Id"] != "[redacted]" || preview.Headers["X-License"] != "[redacted]" {
				t.Fatalf("configured secret headers are visible: %v", preview.Headers)
			}
			if preview.Headers["X-Mode"] != "public" {
				t.Fatalf("static public header was lost: %v", preview.Headers)
			}
			if resolved.Request.Headers["X-Client-Id"] != "credential-value" || resolved.Request.Headers["X-License"] != "legacy-value" {
				t.Fatal("redaction mutated the executable request")
			}
		})
	}
}

func TestActionResponseReadFailuresKeepTypedErrorsAndStatus(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		t.Run(map[bool]string{false: "truncated body", true: "oversized body"}[oversized], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				if oversized {
					_, _ = io.WriteString(w, strings.Repeat("x", connector.MaxResponseBytes+1))
					return
				}
				w.Header().Set("Content-Length", "100")
				_, _ = io.WriteString(w, "short")
			}))
			defer server.Close()
			config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
			conn := &Connector{client: server.Client()}
			action, err := conn.ResolveAction(config, "operation", "", nil)
			if err != nil {
				t.Fatal(err)
			}
			result, err := conn.SendAction(context.Background(), config, action)
			var malformed *connector.MalformedResponseError
			if !errors.As(err, &malformed) || result.Status != http.StatusOK || !result.Written {
				t.Fatalf("read result=%+v error=%v; want malformed response with known 200", result, err)
			}
		})
	}

	t.Run("timeout after response headers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}))
		defer server.Close()
		config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
		conn := &Connector{client: server.Client()}
		action, err := conn.ResolveAction(config, "operation", "", nil)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		result, err := conn.SendAction(ctx, config, action)
		var timeout *connector.TimeoutError
		if !errors.As(err, &timeout) || result.Status != http.StatusOK || !result.Written {
			t.Fatalf("read result=%+v error=%v; want timeout with known 200", result, err)
		}
	})
}

func TestRecipeActionAcceptedMetadataBoundaries(t *testing.T) {
	for _, name := range []string{"a", strings.Repeat("a", 32)} {
		actions := "actions:\n  " + name + ":\n    method: POST\n    path: /operate\n" +
			"    label: " + strings.Repeat("x", 60) + "\n    description: " + strings.Repeat("x", 300) +
			"\n    downtime_seconds: 3600\n"
		if _, err := ParseRecipe(recipeForActionExecution("  mode: none", actions)); err != nil {
			t.Fatalf("valid name/metadata maximum rejected: %v", err)
		}
	}
}

func TestCanonicalActionsPreserveBodyTypesArrayOrderAndScope(t *testing.T) {
	base := recipeForActionExecution("  mode: none", "actions:\n  operation: {method: POST, path: /operate, body: [1, true]}\n")
	previous, err := CanonicalActions(map[string]any{"recipe": base})
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []string{`["1", true]`, `[1, "true"]`, `[true, 1]`} {
		next, err := CanonicalActions(map[string]any{"recipe": strings.Replace(base, "[1, true]", replacement, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if diff := DiffActions(previous, next); len(diff.Changed) != 1 {
			t.Fatalf("semantic body change %s not detected: %+v", replacement, diff)
		}
	}
	entity := recipeForActionExecution("  mode: none", "")
	entity = strings.Replace(entity, "    entity: {kind: item, name: name, external_id: id}",
		"    entity:\n      kind: item\n      name: name\n      external_id: id\n"+
			"      actions:\n        operation: {method: POST, path: /operate, body: [1, true]}", 1)
	next, err := CanonicalActions(map[string]any{"recipe": entity})
	if err != nil {
		t.Fatal(err)
	}
	if diff := DiffActions(previous, next); len(diff.Added) != 1 || len(diff.Removed) != 1 {
		t.Fatalf("service-to-entity move not detected: %+v", diff)
	}
}

func TestActionExcerptTruncatesAtUTF8Boundary(t *testing.T) {
	body := []byte(strings.Repeat("x", 511) + "é" + "tail")
	got := actionTextExcerpt("text/plain", body)
	if got != strings.Repeat("x", 511) || !utf8.ValidString(got) {
		t.Fatalf("excerpt is not the complete UTF8 prefix: %q", got)
	}
}

func TestSendActionNon2xxKeepsBodyInResultAndOutOfError(t *testing.T) {
	const sentinel = "UPSTREAM-BODY-SENTINEL"
	bodies := []struct {
		name        string
		contentType string
		body        string
	}{
		{"text body", "text/plain; charset=utf-8", "upstream said " + sentinel},
		{"json body", "application/json", `{"error":"` + sentinel + `"}`},
	}
	statuses := []int{
		http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict,
		http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable,
	}
	for _, status := range statuses {
		for _, tc := range bodies {
			t.Run(fmt.Sprintf("%d/%s", status, tc.name), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", tc.contentType)
					w.WriteHeader(status)
					_, _ = io.WriteString(w, tc.body)
				}))
				defer server.Close()
				config := map[string]any{"url": server.URL, "recipe": serviceActionRecipe(http.MethodPost)}
				conn := &Connector{client: server.Client()}
				action, err := conn.ResolveAction(config, "operation", "", nil)
				if err != nil {
					t.Fatal(err)
				}
				result, err := conn.SendAction(context.Background(), config, action)
				if err == nil {
					t.Fatalf("SendAction() succeeded for status %d", status)
				}
				if result.Status != status {
					t.Errorf("result.Status = %d, want %d", result.Status, status)
				}
				if !strings.Contains(result.Excerpt, sentinel) {
					t.Errorf("result.Excerpt = %q, want it to contain %q", result.Excerpt, sentinel)
				}
				if !strings.Contains(err.Error(), strconv.Itoa(status)) {
					t.Errorf("error %q does not name status %d", err, status)
				}
				if strings.Contains(err.Error(), sentinel) {
					t.Errorf("error leaks the response body: %v", err)
				}

				// The class must match what connector.CheckStatus gives the
				// same status, so callers branch on it identically.
				want := connector.CheckStatus(status, nil)
				var gotAuth, wantAuth *connector.AuthError
				var gotUnavailable, wantUnavailable *connector.ServiceUnavailableError
				if errors.As(err, &gotAuth) != errors.As(want, &wantAuth) {
					t.Errorf("auth error class = %v, want %v (err %v)", errors.As(err, &gotAuth), errors.As(want, &wantAuth), err)
				}
				if errors.As(err, &gotUnavailable) != errors.As(want, &wantUnavailable) {
					t.Errorf("service-unavailable class = %v, want %v (err %v)", errors.As(err, &gotUnavailable), errors.As(want, &wantUnavailable), err)
				}
				switch status {
				case http.StatusUnauthorized, http.StatusForbidden:
					if gotAuth == nil {
						t.Errorf("status %d: error %v is not an AuthError", status, err)
					}
				case http.StatusBadGateway, http.StatusServiceUnavailable:
					if gotUnavailable == nil {
						t.Errorf("status %d: error %v is not a ServiceUnavailableError", status, err)
					}
				}
			})
		}
	}
}

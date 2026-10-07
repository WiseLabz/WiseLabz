package custom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httpx"
)

func recipeForFetch(auth, endpoints string) string {
	return "version: 1\ncategory: media\nauth:\n" + auth + "\nendpoints:\n" + endpoints
}

const fetchEndpoint = `  - name: items
    path: /api/items
    method: GET
    items: items
    entity: {kind: media_item, name: name, external_id: id}
`

func TestFetchRecipeMapsEntitiesAndKeepsResponseBodyOutOfSections(t *testing.T) {
	const response = `{"items":[{"id":"42","name":"Example"}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("X-Legacy") != "legacy" || r.Header.Get("X-Recipe") != "recipe" {
			t.Errorf("request = %s headers %v", r.Method, r.Header)
		}
		_, _ = io.WriteString(w, response)
	}))
	defer server.Close()

	conn := &Connector{client: server.Client()}
	snapshot, err := conn.Fetch(context.Background(), map[string]any{
		"url":     server.URL,
		"headers": `{"X-Legacy":"legacy"}`,
		"recipe":  recipeForFetch("  mode: none", strings.Replace(fetchEndpoint, "    entity:", "    headers: {X-Recipe: recipe}\n    entity:", 1)),
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snapshot.Entities) != 1 || snapshot.Entities[0].ExternalID != "42" || snapshot.Entities[0].Name != "Example" {
		t.Fatalf("entities = %#v", snapshot.Entities)
	}
	if len(snapshot.Sections) != 1 || strings.Contains(snapshot.Sections[0].Content, response) {
		t.Fatalf("sections contain raw response: %#v", snapshot.Sections)
	}
}

func TestBuildRecipeRequestTreatsGETNullBodyAsAbsent(t *testing.T) {
	raw := strings.Replace(fetchEndpoint, "    method: GET", "    method: GET\n    body: null", 1)
	recipe, err := ParseRecipe(recipeForFetch("  mode: none", raw))
	if err != nil {
		t.Fatal(err)
	}
	baseURL, err := url.Parse("https://api.example.test")
	if err != nil {
		t.Fatal(err)
	}
	request, err := buildRecipeRequest(context.Background(), baseURL, recipe, recipe.Endpoints[0], nil)
	if err != nil {
		t.Fatal(err)
	}
	if request.Body != nil || request.ContentLength != 0 {
		t.Fatalf("GET null body request = Body %v, ContentLength %d; want no body", request.Body, request.ContentLength)
	}
}

func TestFetchRecipeUsesEachAuthMode(t *testing.T) {
	tests := []struct {
		name        string
		auth        string
		credentials map[string]any
		check       func(*http.Request) error
	}{
		{name: "none", auth: "  mode: none"},
		{
			name: "header", auth: "  mode: header\n  name: Authorization\n  prefix: 'Bearer '",
			credentials: map[string]any{"auth_token": "header-token"},
			check: func(r *http.Request) error {
				if got := r.Header.Get("Authorization"); got != "Bearer header-token" {
					return fmt.Errorf("Authorization = %q", got)
				}
				return nil
			},
		},
		{
			name: "basic", auth: "  mode: basic",
			credentials: map[string]any{"auth_username": "operator", "auth_password": "basic-secret"},
			check: func(r *http.Request) error {
				username, password, ok := r.BasicAuth()
				if !ok || username != "operator" || password != "basic-secret" {
					return fmt.Errorf("BasicAuth() = %q, %q, %t", username, password, ok)
				}
				return nil
			},
		},
		{
			name: "query", auth: "  mode: query\n  name: api_token",
			credentials: map[string]any{"auth_token": "query-secret"},
			check: func(r *http.Request) error {
				if got := r.URL.Query().Get("api_token"); got != "query-secret" {
					return fmt.Errorf("api_token = %q", got)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if test.check != nil {
					if err := test.check(r); err != nil {
						t.Error(err)
					}
				} else if r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected Authorization header %q", r.Header.Get("Authorization"))
				}
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			config := map[string]any{"url": server.URL, "recipe": recipeForFetch(test.auth, fetchEndpoint)}
			for key, value := range test.credentials {
				config[key] = value
			}
			if _, err := (&Connector{client: server.Client()}).Fetch(context.Background(), config); err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
		})
	}
}

func TestFetchRecipeAbortsOnLaterEndpointFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/second" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = io.WriteString(w, "upstream secret body")
			return
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"1","name":"first"}]}`)
	}))
	defer server.Close()
	endpoints := strings.Replace(fetchEndpoint, "/api/items", "/first", 1)
	endpoints += `  - name: second
    path: /second
    method: GET
    items: items
    entity: {kind: media_item, name: name, external_id: id}
`
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", endpoints),
	})
	var unavailable *connector.ServiceUnavailableError
	if snapshot != nil || !errors.As(err, &unavailable) || !strings.Contains(err.Error(), `endpoint "second"`) {
		t.Fatalf("Fetch() = %#v, %v; want nil snapshot and endpoint-scoped unavailable error", snapshot, err)
	}
	if strings.Contains(err.Error(), "upstream secret body") {
		t.Fatalf("error exposed response body: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("request count = %d, want one request per endpoint", calls.Load())
	}
}

func TestFetchRecipeItemsPathMustSelectList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":{}}`)
	}))
	defer server.Close()
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", fetchEndpoint),
	})
	var malformed *connector.MalformedResponseError
	if snapshot != nil || !errors.As(err, &malformed) || !strings.Contains(err.Error(), `endpoint "items"`) || !strings.Contains(err.Error(), "did not select a list") {
		t.Fatalf("Fetch() = %#v, %v; want endpoint-scoped malformed response", snapshot, err)
	}
}

func TestFetchRecipeUnauthorizedIsAuthErrorWithoutResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "denied private detail")
	}))
	defer server.Close()
	recipe := recipeForFetch("  mode: query\n  name: api_token", fetchEndpoint)
	_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipe, "auth_token": "private-query-token",
	})
	var auth *connector.AuthError
	if !errors.As(err, &auth) || strings.Contains(err.Error(), "private-query-token") || strings.Contains(err.Error(), "denied private detail") {
		t.Fatalf("Fetch() error = %v; want safe auth error", err)
	}
}

func TestFetchRecipeTransportErrorRedactsArbitraryQueryToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	serverURL := server.URL
	server.Close()
	_, err := (&Connector{client: &http.Client{}}).Fetch(context.Background(), map[string]any{
		"url":        serverURL,
		"recipe":     recipeForFetch("  mode: query\n  name: x_any_token", fetchEndpoint),
		"auth_token": "never-print-this",
	})
	if err == nil || strings.Contains(err.Error(), "never-print-this") || strings.Contains(err.Error(), "x_any_token") {
		t.Fatalf("Fetch() error = %v; want redacted transport failure", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestFetchRecipeSendsExactlyOneRequestEvenForRetryableGET(t *testing.T) {
	var calls atomic.Int32
	baseClient := &http.Client{Transport: httpx.RetryTransport(roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("unavailable")),
		}, nil
	}), httpx.RetryPolicy{MaxRetries: 2})}
	_, err := (&Connector{client: baseClient}).Fetch(context.Background(), map[string]any{
		"url": "http://api.example.test", "recipe": recipeForFetch("  mode: none", fetchEndpoint),
	})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("Fetch() error/calls = %v/%d, want error and one request", err, calls.Load())
	}
}

func TestFetchRecipeRejectsInvalidLaterEndpointBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer server.Close()
	endpoints := fetchEndpoint + `  - name: invalid
    path: /invalid
    method: GET
    items: items
    headers: {X-Bad: "line\nbreak"}
    entity: {kind: media_item, name: name, external_id: id}
`
	_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", endpoints),
	})
	if err == nil || calls.Load() != 0 || !strings.Contains(err.Error(), "endpoints[1].headers.X-Bad") {
		t.Fatalf("Fetch() error/calls = %v/%d, want validation error before request", err, calls.Load())
	}
}

func TestFetchRecipeRejectsURLUserInfoBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer server.Close()
	config := map[string]any{
		"url":    strings.Replace(server.URL, "http://", "http://user:password@", 1),
		"recipe": recipeForFetch("  mode: none", fetchEndpoint),
	}
	_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), config)
	if err == nil || calls.Load() != 0 || strings.Contains(err.Error(), "password") {
		t.Fatalf("Fetch() error/calls = %v/%d, want safe URL credential rejection", err, calls.Load())
	}
}

func TestFetchRecipeSanitizesCredentialInMappingErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"items":[{"id":"token-secret","name":"one"},{"id":"token-secret","name":"two"}]}`)
	}))
	defer server.Close()
	_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url":        server.URL,
		"recipe":     recipeForFetch("  mode: header\n  name: Authorization", fetchEndpoint),
		"auth_token": "token-secret",
	})
	var malformed *connector.MalformedResponseError
	if !errors.As(err, &malformed) || strings.Contains(err.Error(), "token-secret") {
		t.Fatalf("Fetch() error = %v; want classified error with secret redacted", err)
	}
}

func TestFetchRecipePOSTMergesQueryAndSendsStaticJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/items" {
			t.Errorf("path = %q, want /api/items", r.URL.Path)
		}
		query := r.URL.Query()
		for key, want := range map[string]string{
			"base":        "base-value",
			"from_path":   "path-value",
			"from_recipe": "recipe-value",
			"shared":      "endpoint-value",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("query %q = %q, want %q (all: %v)", key, got, want, query)
			}
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		var body struct {
			Query     string `json:"query"`
			Variables struct {
				Include bool `json:"include"`
			} `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		} else if body.Query != "query { items }" || !body.Variables.Include {
			t.Errorf("request body = %#v, want query and include=true", body)
		}
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer server.Close()

	endpoints := `  - name: items
    path: /api/items?from_path=path-value&shared=path-value
    method: POST
    query: {from_recipe: recipe-value, shared: endpoint-value}
    body: {query: "query { items }", variables: {include: true}}
    items: items
    entity: {kind: media_item, name: name, external_id: id}
`
	config := map[string]any{
		"url":    server.URL + "/base?base=base-value&shared=base-value",
		"recipe": recipeForFetch("  mode: none", endpoints),
	}
	if _, err := (&Connector{client: server.Client()}).Fetch(context.Background(), config); err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
}

func TestFetchRecipeRejectsAbsoluteAndSchemeRelativePathsBeforeRequest(t *testing.T) {
	for _, path := range []string{
		"https://outside.example/api/items",
		"//outside.example/api/items",
	} {
		t.Run(path, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			endpoints := strings.Replace(fetchEndpoint, "/api/items", path, 1)
			_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
				"url": server.URL, "recipe": recipeForFetch("  mode: none", endpoints),
			})
			if err == nil || calls.Load() != 0 {
				t.Fatalf("Fetch() error/calls = %v/%d, want validation failure before request", err, calls.Load())
			}
		})
	}
}

func TestValidateRecipeUsesOnlyFirstEndpointAndClassifiesStatus(t *testing.T) {
	tests := []struct {
		name            string
		status          int
		wantAuth        bool
		wantUnavailable bool
	}{
		{name: "success", status: http.StatusOK},
		{name: "unauthorized", status: http.StatusUnauthorized, wantAuth: true},
		{name: "unavailable", status: http.StatusServiceUnavailable, wantUnavailable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/first" {
					t.Errorf("path = %q, want only /first", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); got != "Bearer validation-token" {
					t.Errorf("Authorization = %q, want configured token", got)
				}
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `not required to be JSON`)
			}))
			defer server.Close()
			endpoints := `  - name: first
    path: /first
    method: GET
    items: items
    entity: {kind: media_item, name: name, external_id: id}
  - name: second
    path: /second
    method: GET
    items: items
    entity: {kind: media_item, name: name, external_id: id}
`
			config := map[string]any{
				"url":        server.URL,
				"recipe":     recipeForFetch("  mode: header\n  name: Authorization\n  prefix: 'Bearer '", endpoints),
				"auth_token": "validation-token",
			}
			err := (&Connector{client: server.Client()}).Validate(context.Background(), config)
			if calls.Load() != 1 {
				t.Fatalf("Validate() requests = %d, want exactly one", calls.Load())
			}
			if test.wantAuth {
				var auth *connector.AuthError
				if !errors.As(err, &auth) {
					t.Fatalf("Validate() error = %v, want authentication error", err)
				}
			} else if test.wantUnavailable {
				var unavailable *connector.ServiceUnavailableError
				if !errors.As(err, &unavailable) {
					t.Fatalf("Validate() error = %v, want service unavailable error", err)
				}
			} else if err != nil {
				t.Fatalf("Validate() error = %v, want success", err)
			}
		})
	}
}

func TestSameOriginComparesSchemeHostAndNormalizedPort(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		target string
		want   bool
	}{
		{name: "same origin", base: "https://api.example.test/base", target: "https://api.example.test/items", want: true},
		{name: "implicit and explicit default port", base: "http://api.example.test", target: "http://api.example.test:80/items", want: true},
		{name: "host case is equivalent", base: "https://API.example.test", target: "https://api.EXAMPLE.test/items", want: true},
		{name: "different scheme", base: "http://api.example.test", target: "https://api.example.test/items", want: false},
		{name: "different host", base: "https://api.example.test", target: "https://other.example.test/items", want: false},
		{name: "different port", base: "http://api.example.test:8080", target: "http://api.example.test:8081/items", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base, err := url.Parse(test.base)
			if err != nil {
				t.Fatal(err)
			}
			target, err := url.Parse(test.target)
			if err != nil {
				t.Fatal(err)
			}
			if got := sameOrigin(base, target); got != test.want {
				t.Fatalf("sameOrigin(%q, %q) = %t, want %t", test.base, test.target, got, test.want)
			}
		})
	}
}

func TestSameOriginURLNormalizesDefaultPorts(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"https://h", "https://H:443", true},
		{"http://h/a?x=1", "http://h:80/b", true},
		{"http://h", "https://h", false},
		{"https://h:8443", "https://h", false},
		{"https://h", "://bad", false},
		{"%zz", "https://h", false},
	}
	for _, test := range tests {
		if got := SameOrigin(test.a, test.b); got != test.want {
			t.Errorf("SameOrigin(%q, %q) = %t, want %t", test.a, test.b, got, test.want)
		}
	}
}

func TestFetchRecipeDoesNotFollowRedirects(t *testing.T) {
	var sourceCalls, targetCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/items" {
			sourceCalls.Add(1)
			w.Header().Set("Location", "/target")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/target" {
			targetCalls.Add(1)
			_, _ = io.WriteString(w, `{"items":[]}`)
		}
	}))
	defer server.Close()
	_, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", fetchEndpoint),
	})
	if err == nil || sourceCalls.Load() != 1 || targetCalls.Load() != 0 {
		t.Fatalf("Fetch() error/source/target calls = %v/%d/%d; want failure and 1/0", err, sourceCalls.Load(), targetCalls.Load())
	}
}

func TestFetchRecipeOversizedResponseReturnsNoSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := strings.Repeat("x", 1<<20)
		for written := 0; written <= connector.MaxResponseBytes; written += len(chunk) {
			if _, err := io.WriteString(w, chunk); err != nil {
				return
			}
		}
	}))
	defer server.Close()
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", fetchEndpoint),
	})
	var malformed *connector.MalformedResponseError
	if snapshot != nil || !errors.As(err, &malformed) || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("Fetch() = %#v, %v; want nil snapshot and malformed oversized-response error", snapshot, err)
	}
}

func TestFetchRecipeInvalidJSONNamesEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not JSON")
	}))
	defer server.Close()
	endpoints := strings.Replace(fetchEndpoint, "name: items", "name: malformed_response", 1)
	snapshot, err := (&Connector{client: server.Client()}).Fetch(context.Background(), map[string]any{
		"url": server.URL, "recipe": recipeForFetch("  mode: none", endpoints),
	})
	var malformed *connector.MalformedResponseError
	if snapshot != nil || !errors.As(err, &malformed) || !strings.Contains(err.Error(), `endpoint "malformed_response"`) {
		t.Fatalf("Fetch() = %#v, %v; want nil snapshot and endpoint-scoped malformed response", snapshot, err)
	}
}

func TestFetchRecipeConnectorCredentialsOverrideRecipeValues(t *testing.T) {
	endpoint := func(extra string) string {
		return strings.Replace(fetchEndpoint, "    entity:", extra+"    entity:", 1)
	}
	tests := []struct {
		name        string
		auth        string
		endpoints   string
		credentials map[string]any
		check       func(*http.Request) error
	}{
		{
			name: "header auth beats recipe headers of any case", auth: "  mode: header\n  name: X-Api-Key",
			endpoints:   endpoint("    headers: {X-Api-Key: from-recipe, x-api-key: also}\n"),
			credentials: map[string]any{"auth_token": "connector-token"},
			check: func(r *http.Request) error {
				if got := r.Header.Values("X-Api-Key"); len(got) != 1 || got[0] != "connector-token" {
					return fmt.Errorf("X-Api-Key = %q, want only the connector token", got)
				}
				return nil
			},
		},
		{
			name: "basic auth beats recipe Authorization header", auth: "  mode: basic",
			endpoints:   endpoint("    headers: {Authorization: recipe-value}\n"),
			credentials: map[string]any{"auth_username": "operator", "auth_password": "basic-secret"},
			check: func(r *http.Request) error {
				username, password, ok := r.BasicAuth()
				if !ok || username != "operator" || password != "basic-secret" {
					return fmt.Errorf("BasicAuth() = %q, %q, %t; Authorization = %q", username, password, ok, r.Header.Get("Authorization"))
				}
				return nil
			},
		},
		{
			name: "query auth beats recipe path and query values", auth: "  mode: query\n  name: apikey",
			endpoints:   strings.Replace(endpoint("    query: {apikey: fromRecipe}\n"), "/api/items", "/api/items?apikey=fromPath", 1),
			credentials: map[string]any{"auth_token": "connector-token"},
			check: func(r *http.Request) error {
				if got := r.URL.Query()["apikey"]; len(got) != 1 || got[0] != "connector-token" {
					return fmt.Errorf("apikey = %q, want only the connector token", got)
				}
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := test.check(r); err != nil {
					t.Error(err)
				}
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			config := map[string]any{"url": server.URL, "recipe": recipeForFetch(test.auth, test.endpoints)}
			for key, value := range test.credentials {
				config[key] = value
			}
			if _, err := (&Connector{client: server.Client()}).Fetch(context.Background(), config); err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
		})
	}
}

func TestFetchRecipeEndpointPathResolvesAgainstConnectorURLPrefix(t *testing.T) {
	tests := []struct {
		name         string
		endpointPath string
		wantPath     string
	}{
		{name: "relative path keeps the prefix", endpointPath: "api/items", wantPath: "/prefix/api/items"},
		{name: "rooted path replaces the prefix", endpointPath: "/api/items", wantPath: "/api/items"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = io.WriteString(w, `{"items":[]}`)
			}))
			defer server.Close()
			endpoints := strings.Replace(fetchEndpoint, "/api/items", test.endpointPath, 1)
			config := map[string]any{"url": server.URL + "/prefix/", "recipe": recipeForFetch("  mode: none", endpoints)}
			if _, err := (&Connector{client: server.Client()}).Fetch(context.Background(), config); err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if gotPath != test.wantPath {
				t.Fatalf("request path = %q, want %q", gotPath, test.wantPath)
			}
		})
	}
}

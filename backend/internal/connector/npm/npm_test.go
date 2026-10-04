package npm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const (
	testEmail    = "npm@example.test"
	testPassword = "secret-password"
	testToken    = "bearer-secret"
)

var testResources = map[string]string{
	"/api/nginx/proxy-hosts":       `[{"id":1,"domain_names":["app.example.test"],"forward_host":"10.0.0.3","forward_port":8080,"forward_scheme":"http","enabled":true}]`,
	"/api/nginx/redirection-hosts": `[{"id":2,"domain_names":["old.example.test"],"forward_http_code":301,"forward_scheme":"https","forward_domain_name":"app.example.test","enabled":true}]`,
	"/api/nginx/streams":           `[{"id":3,"incoming_port":2222,"forwarding_host":"10.0.0.4","forwarding_port":22,"tcp_forwarding":true,"enabled":true}]`,
	"/api/nginx/dead-hosts":        `[{"id":4,"domain_names":["gone.example.test"],"enabled":true}]`,
	"/api/nginx/certificates":      `[{"id":5,"provider":"letsencrypt","nice_name":"example.test","domain_names":["example.test"],"expires_on":"2030-01-01"}]`,
	"/api/nginx/access-lists":      `[{"id":6,"name":"internal","satisfy_any":false,"pass_auth":false,"proxy_host_count":1}]`,
}

func testServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(server.Close)
	return server
}

func testConnector(t *testing.T, config map[string]any) *Connector {
	t.Helper()
	connector.AllowLoopbackForTest(t)
	created, err := connector.Get(typeName, config)
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	conn, ok := created.(*Connector)
	if !ok {
		t.Fatalf("connector.Get returned %T", created)
	}
	return conn
}

func validConfig(url string) map[string]any {
	return map[string]any{"url": url, "email": testEmail, "password": testPassword}
}

func authServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			if r.Method != http.MethodPost {
				t.Errorf("token endpoint method = %s, want POST", r.Method)
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			var credentials map[string]string
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil || credentials["identity"] != testEmail || credentials["secret"] != testPassword {
				http.Error(w, "bad credentials", http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprintf(w, `{"token":%q,"expires":"2030-01-01T00:00:00Z"}`, testToken)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/nginx/") && r.Method != http.MethodGet {
			t.Errorf("resource %s method = %s, want GET", r.URL.Path, r.Method)
			http.Error(w, "resource requests must use GET", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+testToken {
			http.Error(w, "bearer rejected", http.StatusUnauthorized)
			return
		}
		handler(w, r)
	})
}

func TestRegistrationAndIdentity(t *testing.T) {
	created, err := connector.Get(typeName, nil)
	if err != nil {
		t.Fatalf("empty config factory: %v", err)
	}
	if created.Name() != "Nginx Proxy Manager" || created.Type() != "npm" || created.Category() != "networking" {
		t.Fatalf("identity = %s/%s/%s", created.Name(), created.Type(), created.Category())
	}
	schema, err := connector.GetTypeSchema(typeName)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	if schema.Name != "Nginx Proxy Manager" || schema.Category != "networking" {
		t.Fatalf("schema identity = %+v", schema)
	}
	fields := map[string]connector.SchemaField{}
	for _, field := range schema.Fields {
		fields[field.Key] = field
	}
	for _, key := range []string{"url", "email", "password"} {
		if !fields[key].Required {
			t.Errorf("%s should be required", key)
		}
	}
	if fields["password"].Type != "password" || fields["verify_tls"].Default != "true" {
		t.Errorf("password/verify_tls schema = %+v / %+v", fields["password"], fields["verify_tls"])
	}
	for _, phrase := range []string{"proxy hosts", "redirection hosts", "streams", "404 hosts", "certificates", "access lists", "admin", "Two-factor authentication must be disabled"} {
		if !strings.Contains(fields["email"].Description, phrase) {
			t.Errorf("email description %q does not mention %q", fields["email"].Description, phrase)
		}
	}
	if _, ok := connector.AttributeCatalog()[typeName]; !ok {
		t.Errorf("attribute catalog missing for %s", typeName)
	}
}

func TestValidateRequiresConfigurationAndAuthenticates(t *testing.T) {
	for _, config := range []map[string]any{
		nil,
		{"email": testEmail, "password": testPassword},
		{"url": "http://example.test", "password": testPassword},
		{"url": "http://example.test", "email": testEmail},
	} {
		if err := testConnector(t, config).Validate(context.Background(), nil); err == nil {
			t.Errorf("Validate(%v) succeeded without required fields", config)
		}
	}
	server := authServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	if err := testConnector(t, validConfig(server.URL)).Validate(context.Background(), nil); err != nil {
		t.Fatalf("valid Validate: %v", err)
	}
}

func TestFetchAuthenticatesEachTimeAndCollectsAllResources(t *testing.T) {
	tokens := 0
	gets := 0
	fetches := make([]*connector.ServiceSnapshot, 0, 2)
	proxyHosts := []string{
		`[{"id":1,"domain_names":["a.example.test"],"forward_host":"10.0.0.3","forward_port":8080,"forward_scheme":"http","enabled":true},{"id":2,"domain_names":["b.example.test"],"forward_host":"10.0.0.4","forward_port":8080,"forward_scheme":"http","enabled":true}]`,
		`[{"id":2,"domain_names":["b.example.test"],"forward_host":"10.0.0.4","forward_port":8080,"forward_scheme":"http","enabled":true},{"id":1,"domain_names":["a.example.test"],"forward_host":"10.0.0.3","forward_port":8080,"forward_scheme":"http","enabled":true}]`,
	}
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			tokens++
			var credentials map[string]string
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil || credentials["identity"] != testEmail || credentials["secret"] != testPassword {
				http.Error(w, "bad", http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprintf(w, `{"token":"token-%d"}`, tokens)
			return
		}
		gets++
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer token-%d", tokens) {
			http.Error(w, "bad token", http.StatusUnauthorized)
			return
		}
		body, ok := testResources[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/api/nginx/proxy-hosts" {
			body = proxyHosts[tokens%2]
		}
		_, _ = w.Write([]byte(body))
	})
	conn := testConnector(t, validConfig(server.URL))
	for i := 0; i < 2; i++ {
		snapshot, err := conn.Fetch(context.Background(), nil)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if len(snapshot.Sections) != len(testResources) || len(snapshot.Entities) != len(testResources)+1 {
			t.Errorf("snapshot sections/entities = %d/%d", len(snapshot.Sections), len(snapshot.Entities))
		}
		if snapshot.Metadata["npm_url"] != server.URL {
			t.Errorf("npm_url = %q", snapshot.Metadata["npm_url"])
		}
		fetches = append(fetches, snapshot)
		for _, key := range []string{"proxy_host_count", "redirection_host_count", "stream_count", "dead_host_count", "certificate_count", "access_list_count"} {
			want := "1"
			if key == "proxy_host_count" {
				want = "2"
			}
			if snapshot.Metadata[key] != want {
				t.Errorf("metadata[%q] = %q, want %q", key, snapshot.Metadata[key], want)
			}
		}
	}
	if tokens != 2 || gets != 2*len(testResources) {
		t.Errorf("token posts / gets = %d / %d", tokens, gets)
	}
	fetches[0].FetchedAt, fetches[1].FetchedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(fetches[0], fetches[1]) {
		t.Error("repeated fetches produced different snapshots")
	}
}

func TestFetchSelectiveFieldsAndPartialFailure(t *testing.T) {
	server := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nginx/streams" {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(testResources[r.URL.Path]))
	})
	snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), map[string]any{"fields": []string{"streams", "proxy_hosts"}})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(snapshot.Sections) != 2 || snapshot.Sections[0].Title != "Proxy Hosts" || snapshot.Sections[1].Title != "Streams" {
		t.Fatalf("selected sections = %+v", snapshot.Sections)
	}
	if snapshot.Sections[1].Error == "" || snapshot.Metadata["proxy_host_count"] != "1" {
		t.Fatalf("partial result did not preserve good and failed section: %+v", snapshot)
	}
	if _, ok := snapshot.Metadata["stream_count"]; ok {
		t.Errorf("failed resource count should be absent: %v", snapshot.Metadata)
	}
}

func TestAuthenticationAndResourceErrorsAreSafe(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		server := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, "%s %s", testPassword, testToken)
		})
		err := testConnector(t, validConfig(server.URL)).Validate(context.Background(), nil)
		var authErr *connector.AuthError
		if err == nil || !errors.As(err, &authErr) || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) {
			t.Errorf("Validate status %d error = %v", status, err)
		}
		server.Close()
	}

	server := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nginx/proxy-hosts" {
			w.WriteHeader(http.StatusFound)
			_, _ = fmt.Fprintf(w, "%s %s", testPassword, testToken)
			return
		}
		_, _ = w.Write([]byte(testResources[r.URL.Path]))
	})
	snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), map[string]any{"fields": []string{"proxy_hosts"}})
	if snapshot != nil || err == nil || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) {
		t.Errorf("redirect result = (%v, %v), want safe fatal error", snapshot, err)
	}
}

func TestResourceAuthErrorsAreSafe(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		server := authServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = fmt.Fprintf(w, "%s %s", testPassword, testToken)
		})
		snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), map[string]any{"fields": []string{"proxy_hosts"}})
		var authErr *connector.AuthError
		if snapshot != nil || err == nil || !errors.As(err, &authErr) || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) {
			t.Errorf("Fetch status %d = (%v, %v)", status, snapshot, err)
		}
		server.Close()
	}
}

func TestAuthenticationTwoFactorIsAuthError(t *testing.T) {
	challengeToken := "jwt-challenge-secret"
	resourceRequests := 0
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			if r.Method != http.MethodPost {
				t.Errorf("token endpoint method = %s, want POST", r.Method)
				http.Error(w, "wrong method", http.StatusMethodNotAllowed)
				return
			}
			_, _ = fmt.Fprintf(w, `{"requires_2fa":true,"challenge_token":%q}`, challengeToken)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/nginx/") {
			resourceRequests++
		}
		http.Error(w, "should not fetch without token", http.StatusInternalServerError)
	})
	snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), nil)
	var authErr *connector.AuthError
	if snapshot != nil || err == nil || !errors.As(err, &authErr) || !strings.Contains(err.Error(), "account has two-factor authentication enabled; use an account without 2FA") || strings.Contains(err.Error(), challengeToken) || strings.Contains(err.Error(), testPassword) || strings.Contains(fmt.Sprint(snapshot), challengeToken) {
		t.Errorf("2FA result = (%v, %v)", snapshot, err)
	}
	if resourceRequests != 0 {
		t.Errorf("2FA response was followed by %d resource requests", resourceRequests)
	}
}

func TestAuthenticationMissingTokenRemainsAuthError(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token endpoint method = %s, want POST", r.Method)
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(`{"message":"` + testPassword + `"}`))
	})
	err := testConnector(t, validConfig(server.URL)).Validate(context.Background(), nil)
	var authErr *connector.AuthError
	if err == nil || !errors.As(err, &authErr) || !strings.Contains(err.Error(), "npm authentication did not return a token") || strings.Contains(err.Error(), testPassword) {
		t.Errorf("missing-token error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportErrorsPreserveCauseAndRedactURL(t *testing.T) {
	conn := testConnector(t, validConfig("https://url-user:url-secret@example.test?token=query-secret"))
	wantCause := errors.New("x509: certificate signed by unknown authority")
	conn.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("TLS handshake: %w", wantCause)
	})}
	err := conn.Validate(context.Background(), nil)
	if !errors.Is(err, wantCause) {
		t.Errorf("transport error %v does not preserve TLS cause", err)
	}
	for _, secret := range []string{"url-user", "url-secret", "query-secret", testPassword, testToken} {
		if strings.Contains(fmt.Sprint(err), secret) {
			t.Errorf("transport error exposed %q: %v", secret, err)
		}
	}
	if !strings.Contains(fmt.Sprint(err), "x509: certificate signed by unknown authority") {
		t.Errorf("transport error %v omits useful TLS cause", err)
	}
}

func TestRequestStatusErrorsAreTypedAndSafe(t *testing.T) {
	for _, tc := range []struct {
		status int
		check  func(error) bool
	}{
		{http.StatusUnauthorized, func(err error) bool { var target *connector.AuthError; return errors.As(err, &target) }},
		{http.StatusForbidden, func(err error) bool { var target *connector.AuthError; return errors.As(err, &target) }},
		{http.StatusBadGateway, func(err error) bool { var target *connector.ServiceUnavailableError; return errors.As(err, &target) }},
		{http.StatusServiceUnavailable, func(err error) bool { var target *connector.ServiceUnavailableError; return errors.As(err, &target) }},
		{http.StatusGatewayTimeout, func(err error) bool { var target *connector.ServiceUnavailableError; return errors.As(err, &target) }},
		{599, func(err error) bool { return err != nil }},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			server := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprintf(w, "%s %s", testPassword, testToken)
			})
			_, err := testConnector(t, validConfig(server.URL)).request(context.Background(), http.MethodGet, "/api/nginx/proxy-hosts", testToken, nil)
			if !tc.check(err) || !strings.Contains(fmt.Sprint(err), fmt.Sprintf("API returned %d", tc.status)) {
				t.Errorf("status %d error = %v", tc.status, err)
			}
			if err == nil || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) || strings.HasSuffix(err.Error(), ": ") {
				t.Errorf("status %d error is unsafe or incomplete: %v", tc.status, err)
			}
		})
	}
}

func TestOversizedResponseUsesBoundedReaderError(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(testPassword + strings.Repeat("x", connector.MaxResponseBytes)))
	})
	_, err := testConnector(t, validConfig(server.URL)).request(context.Background(), http.MethodGet, "/api/nginx/proxy-hosts", testToken, nil)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("upstream response exceeds %d bytes", connector.MaxResponseBytes)) || strings.Contains(err.Error(), testPassword) {
		t.Errorf("oversized response error = %v", err)
	}
	var malformed *connector.MalformedResponseError
	if errors.As(err, &malformed) {
		t.Errorf("oversized response was classified as malformed: %v", err)
	}
}

func TestMalformedAuthenticationResponseIsSafe(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"token":[%q]}`, testPassword)
	})
	err := testConnector(t, validConfig(server.URL)).Validate(context.Background(), nil)
	var malformed *connector.MalformedResponseError
	if err == nil || !errors.As(err, &malformed) || strings.Contains(err.Error(), testPassword) {
		t.Errorf("malformed auth response error = %v", err)
	}
}

func TestRequestDeadlineIsClassifiedAndSafe(t *testing.T) {
	server := testServer(t, func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	err := testConnector(t, validConfig(server.URL)).Validate(ctx, nil)
	var timeoutErr *connector.TimeoutError
	if err == nil || !errors.As(err, &timeoutErr) || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) {
		t.Errorf("deadline error = %v", err)
	}
}

func TestVerifyTLSDefaultsToTrueAndCanBeDisabled(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			_, _ = w.Write([]byte(`{"token":"tls-token"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	connector.AllowLoopbackForTest(t)
	if err := testConnector(t, validConfig(server.URL)).Validate(context.Background(), nil); err == nil {
		t.Fatal("default TLS verification accepted a self-signed certificate")
	}
	config := validConfig(server.URL)
	config["verify_tls"] = false
	if err := testConnector(t, config).Validate(context.Background(), nil); err != nil {
		t.Fatalf("verify_tls=false Validate: %v", err)
	}
}

func TestMalformedResponsesDoNotLeakValues(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			_, _ = fmt.Fprintf(w, `{"token":%q}`, testPassword)
			return
		}
		_, _ = fmt.Fprintf(w, `[{"id":"%s"}]`, testToken)
	})
	conn := testConnector(t, validConfig(server.URL))
	if err := conn.Validate(context.Background(), nil); err != nil {
		t.Fatalf("token string response should decode: %v", err)
	}
	snapshot, err := conn.Fetch(context.Background(), map[string]any{"fields": []string{"proxy_hosts"}})
	if snapshot != nil || err == nil || strings.Contains(err.Error(), testPassword) || strings.Contains(err.Error(), testToken) {
		t.Errorf("malformed resource result = (%v, %v)", snapshot, err)
	}
	var malformed *connector.MalformedResponseError
	if !errors.As(err, &malformed) {
		t.Errorf("malformed response error type = %T, want *MalformedResponseError", err)
	}
}

func TestSnapshotOmitsCredentialBearingNPMFields(t *testing.T) {
	server := authServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/nginx/proxy-hosts" {
			_, _ = fmt.Fprintf(w, `[{"id":1,"domain_names":["app.example.test"],"forward_host":"10.0.0.2","forward_port":80,"meta":{"password":%q,"token":%q},"advanced_config":%q,"access_list":{"items":[{"username":"u","password":%q}]}}]`, testPassword, testToken, testToken, testPassword)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})
	snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), map[string]any{"fields": []string{"proxy_hosts"}})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	encoded, _ := json.Marshal(snapshot)
	if strings.Contains(string(encoded), testPassword) || strings.Contains(string(encoded), testToken) {
		t.Fatalf("snapshot included credential-bearing excluded NPM fields: %s", encoded)
	}
}

func TestFetchRequiresAuthentication(t *testing.T) {
	server := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
	snapshot, err := testConnector(t, validConfig(server.URL)).Fetch(context.Background(), nil)
	if snapshot != nil || err == nil {
		t.Fatalf("Fetch = (%v, %v), want fatal auth error", snapshot, err)
	}
	var authErr *connector.AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("Fetch error %T = %v, want AuthError", err, err)
	}
}

func TestSafeURL(t *testing.T) {
	if got := safeURL("https://user:secret@example.test/path?token=query-secret"); got != "https://example.test/path" {
		t.Errorf("safeURL = %q", got)
	}
}

func TestBaseURLAcceptsOptionalAPIPath(t *testing.T) {
	server := authServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`[]`)) })
	for _, baseURL := range []string{server.URL, server.URL + "/api", server.URL + "/api/"} {
		if err := testConnector(t, validConfig(baseURL)).Validate(context.Background(), nil); err != nil {
			t.Errorf("Validate with base URL %q: %v", baseURL, err)
		}
	}
}

package caddy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const configFixture = `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[{"match":[{"path":["/"],"host":["www.example.com","example.com"]}],"handle":[{"handler":"subroute","routes":[{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.8:8080"},{"dial":"api.internal:8080"},{"dial":"db.internal:5432"}]}]}]}],"terminal":true},{"match":[{"host":["api.example.com"]}],"handle":[{"handler":"subroute","routes":[{"match":[{"path":["/v1/*"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"api.internal:9000"}]}]}]}],"terminal":true},{"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"unix//run/app.sock"}]}],"terminal":true}]}}},"tls":{"automation":{"policies":[{"subjects":["www.example.com","example.com"]}]}}}}`

func TestValidateConfigJSON(t *testing.T) {
	c, _ := newConnector(map[string]any{"config_json": configFixture})
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRequiresExactlyOneMode(t *testing.T) {
	for _, config := range []map[string]any{{}, {"url": "http://localhost", "config_json": configFixture}, {"config_json": "{"}, {"url": "http://localhost", "basic_username": "u"}, {"url": "http://localhost", "bearer_token": "t", "basic_username": "u", "basic_password": "p"}} {
		c, _ := newConnector(config)
		if err := c.Validate(context.Background(), nil); err == nil {
			t.Errorf("Validate(%v) succeeded", config)
		}
	}
}

func TestFetchURLUsesOnlyConfigEndpointAndAuth(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config/" || r.Method != http.MethodGet {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(configFixture))
	}))
	defer server.Close()
	c, _ := newConnector(map[string]any{"url": server.URL, "bearer_token": "token"})
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Entities) != 4 {
		t.Fatalf("entity count = %d", len(snapshot.Entities))
	}
	if snapshot.Metadata["caddy_mode"] != "url" {
		t.Fatalf("metadata = %+v", snapshot.Metadata)
	}
	if len(snapshot.Dependencies) != 2 || snapshot.Dependencies[0].Name != "api.internal" || snapshot.Dependencies[1].Name != "db.internal" {
		t.Fatalf("dependencies = %+v", snapshot.Dependencies)
	}
}

func TestFetchPastedConfigAndHealthCheck(t *testing.T) {
	c, _ := newConnector(map[string]any{"config_json": configFixture})
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Metadata["caddy_mode"] != "config_json" || len(snapshot.Entities) != 4 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestEmptyConfigAndLiveNullAreHealthy(t *testing.T) {
	for _, raw := range []string{`{}`, `{"other":3}`, `{"apps":{"http":{"servers":[1]}}}`, `null`} {
		var parsed *parsedConfig
		var err error
		if raw == "null" {
			parsed, err = parseAdminConfig([]byte(raw))
		} else {
			parsed, err = parsePastedConfig([]byte(raw))
		}
		if err != nil || len(parsed.entities) != 0 || len(parsed.subjects) != 0 {
			t.Fatalf("parse %s = %+v, %v", raw, parsed, err)
		}
		serverTable, routeTable, tlsTable := tables(parsed)
		if serverTable == "" || routeTable == "" || tlsTable == "" {
			t.Fatalf("empty config tables = %q, %q, %q", serverTable, routeTable, tlsTable)
		}
	}
}

func TestLiveAdminNullFetchIsHealthy(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("null")) }))
	defer server.Close()
	c, _ := newConnector(map[string]any{"url": server.URL})
	snapshot, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Entities) != 0 || len(snapshot.Dependencies) != 0 {
		t.Fatalf("null config snapshot = %+v", snapshot)
	}
}

func TestBasicAuth(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "u" || password != "p" {
			t.Errorf("basic auth = %q %q %v", user, password, ok)
		}
		_, _ = w.Write([]byte(configFixture))
	}))
	defer server.Close()
	c, _ := newConnector(map[string]any{"url": server.URL, "basic_username": "u", "basic_password": "p"})
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSizeLimit(t *testing.T) {
	if _, err := parsePastedConfig([]byte(strings.Repeat(" ", maxConfigBytes+1))); err == nil {
		t.Fatal("oversize config accepted")
	}
}

func TestConfigSizeSchemaLimit(t *testing.T) {
	schema, err := connector.GetTypeSchema(typeName)
	if err != nil {
		t.Fatal(err)
	}
	if err := connector.ValidateConfig(*schema, map[string]any{"config_json": strings.Repeat("x", maxConfigBytes+1)}); err == nil {
		t.Fatal("schema accepted oversize pasted config")
	}
}

func TestURLSSRFBlocked(t *testing.T) {
	for _, rawURL := range []string{"http://127.0.0.1:2019", "http://169.254.169.254:2019"} {
		c, _ := newConnector(map[string]any{"url": rawURL})
		err := c.Validate(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "blocked") {
			t.Errorf("Validate(%s) error = %v, want guarded dial refusal", rawURL, err)
		}
	}
}

func TestVerifyTLSDefaultsOnAndCanBeDisabled(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	defaultConfig, _ := newConnector(map[string]any{"url": server.URL})
	if err := defaultConfig.Validate(context.Background(), nil); err == nil {
		t.Fatal("default TLS verification accepted the test server certificate")
	}
	insecureConfig, _ := newConnector(map[string]any{"url": server.URL, "verify_tls": false})
	if err := insecureConfig.Validate(context.Background(), nil); err != nil {
		t.Fatalf("verify_tls=false: %v", err)
	}
}

func TestURLStatusErrorsAreMapped(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusServiceUnavailable} {
		connector.AllowLoopbackForTest(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		c, _ := newConnector(map[string]any{"url": server.URL})
		err := c.Validate(context.Background(), nil)
		server.Close()
		switch status {
		case http.StatusUnauthorized:
			var target *connector.AuthError
			if !errors.As(err, &target) {
				t.Errorf("status %d error = %v", status, err)
			}
		case http.StatusServiceUnavailable:
			var target *connector.ServiceUnavailableError
			if !errors.As(err, &target) {
				t.Errorf("status %d error = %v", status, err)
			}
		}
	}
}

func TestVerifyTLSConfigFieldDefaultsTrue(t *testing.T) {
	schema, err := connector.GetTypeSchema(typeName)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range schema.Fields {
		if field.Key == "verify_tls" && field.Default == "true" {
			return
		}
	}
	t.Fatal("verify_tls schema field does not default to true")
}

func TestURLConfigCanExceedPastedLimit(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	body := "{" + `"padding":"` + strings.Repeat("x", maxConfigBytes+1) + `"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer server.Close()
	c, _ := newConnector(map[string]any{"url": server.URL})
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatalf("URL config over pasted limit: %v", err)
	}
}

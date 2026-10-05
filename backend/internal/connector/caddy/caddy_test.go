package caddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const configFixture = `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[{"match":[{"path":["/" ]},{"host":["www.example.com","example.com"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.8:8080"}]}]},{"match":[{"host":["api.example.com"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"api.internal:9000"}]}]}]}}},"tls":{"automation":{"policies":[{"subjects":["www.example.com","example.com"]}]}}}}`

func TestValidateConfigJSON(t *testing.T) {
	c, _ := newConnector(map[string]any{"config_json": configFixture})
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRequiresExactlyOneMode(t *testing.T) {
	for _, config := range []map[string]any{{}, {"url": "http://localhost", "config_json": configFixture}, {"config_json": "{"}} {
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
	if len(snapshot.Entities) != 3 {
		t.Fatalf("entity count = %d", len(snapshot.Entities))
	}
	if len(snapshot.Dependencies) != 1 || snapshot.Dependencies[0].Name != "api.internal" {
		t.Fatalf("dependencies = %+v", snapshot.Dependencies)
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
	if _, err := parseConfig([]byte(strings.Repeat(" ", maxConfigBytes+1))); err == nil {
		t.Fatal("oversize config accepted")
	}
}

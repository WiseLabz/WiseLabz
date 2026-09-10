package pfsense

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestValidateUsesBearerTokenAndSurfacesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" || r.URL.Path != "/api/v2/system/version" {
			t.Fatalf("request auth/path invalid")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
	err := c.Validate(context.Background(), nil)
	var authErr *connector.AuthError
	if !errors.As(err, &authErr) || !strings.Contains(err.Error(), "API returned 401: denied") {
		t.Fatalf("Validate() error = %v, want *connector.AuthError", err)
	}
}

func TestFetchSurfacesWANAndUpstreamDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/system/version":
			_, _ = w.Write([]byte(`{"data":{"platform":"pfSense","config_version":"2.7.2"}}`))
		case "/api/v2/interfaces":
			_, _ = w.Write([]byte(`{"data":[
				{"id":"wan","if":"em0","ipaddr":"203.0.113.5","status":"up","enable":true},
				{"id":"lan","if":"em1","ipaddr":"10.0.0.1","status":"up","enable":true}
			]}`))
		case "/api/v2/firewall/rules":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/v2/routing/gateways":
			_, _ = w.Write([]byte(`{"data":[{"name":"WAN_GW","gateway":"203.0.113.1","status":"online","monitor_rtt":"5ms","monitor_loss":"0%"}]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(snap.Sections) != 4 {
		t.Fatalf("Sections = %d, want 4", len(snap.Sections))
	}

	wantDeps := []connector.ServiceDependency{
		{Kind: "network", Name: "em0"},
		{Kind: "upstream_service", Name: "WAN_GW"},
	}
	for _, want := range wantDeps {
		found := false
		for _, got := range snap.Dependencies {
			if got.Kind == want.Kind && got.Name == want.Name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dependencies missing %+v, got %+v", want, snap.Dependencies)
		}
	}
}

func TestFetchSurfacesMalformedSystemResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/system/version":
			_, _ = w.Write([]byte(`not json`))
		case "/api/v2/interfaces", "/api/v2/firewall/rules", "/api/v2/routing/gateways":
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !strings.Contains(snap.Sections[0].Content, "malformed response") {
		t.Fatalf("System section = %q, want malformed response placeholder", snap.Sections[0].Content)
	}
}

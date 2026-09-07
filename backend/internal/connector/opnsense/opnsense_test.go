package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestValidateUsesBasicAuthAndSurfacesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "key" || password != "secret" || r.URL.Path != "/api/core/firmware/status" {
			t.Fatalf("request auth/path invalid")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	err := c.Validate(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "API returned 401: denied") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFetchSurfacesWANAndUpstreamDependencies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"24.1"}`))
		case "/api/diagnostics/interface/getInterfaces":
			_, _ = w.Write([]byte(`{"rows":[
				{"identifier":"wan","device":"igb0","ipaddr":"203.0.113.5","status":"up","media":"1000baseT"},
				{"identifier":"lan","device":"igb1","ipaddr":"10.0.0.1","status":"up","media":"1000baseT"}
			]}`))
		case "/api/firewall/filter/searchRule":
			_, _ = w.Write([]byte(`{"rows":[]}`))
		case "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{"items":[{"name":"WAN_GW","address":"203.0.113.1","status":"online","rtt":"5ms","loss":"0%"}]}`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(snap.Sections) != 4 {
		t.Fatalf("Sections = %d, want 4", len(snap.Sections))
	}

	wantDeps := []connector.ServiceDependency{
		{Kind: "network", Name: "igb0"},
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

package cloudflare

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

func TestFailureContract(t *testing.T) {
	connectortest.Run(t, func(serverURL string) (connector.Connector, map[string]any, error) {
		return &Connector{apiToken: "bad", accountID: "test", baseURL: serverURL, client: http.DefaultClient}, nil, nil
	}, false)
}

func TestConnector_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/zones":
			_, _ = w.Write([]byte(`{"result":[{"id":"zone1","name":"example.com"}]}`))
		case "/zones/zone1/dns_records":
			_, _ = w.Write([]byte(`{"result":[{"id":"r1","name":"app.example.com","type":"A","content":"203.0.113.5","proxied":true,"ttl":1}]}`))
		case "/accounts/acct1/cfd_tunnel":
			_, _ = w.Write([]byte(`{"result":[{"id":"t1","name":"prod-tunnel","status":"healthy","conns":1}]}`))
		case "/accounts/acct1/access/apps":
			_, _ = w.Write([]byte(`{"result":[{"id":"app1","name":"internal-dashboard","policies":[{"id":"pol1","name":"allow-eng","decision":"allow"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Connector{apiToken: "fake-token", accountID: "acct1", client: srv.Client(), baseURL: srv.URL}

	snap, err := c.Fetch(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if snap.ServiceName != "Cloudflare" {
		t.Errorf("ServiceName = %q, want %q", snap.ServiceName, "Cloudflare")
	}
	if len(snap.Sections) != 3 {
		t.Errorf("len(Sections) = %d, want 3", len(snap.Sections))
	}
	if len(snap.Entities) != 3 {
		t.Errorf("len(Entities) = %d, want 3 (1 dns_record + 1 tunnel + 1 policy)", len(snap.Entities))
	}
}

func TestConnector_Validate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := &Connector{apiToken: "bad-token", accountID: "acct1", client: srv.Client(), baseURL: srv.URL}
	if err := c.Validate(context.Background(), map[string]any{}); err == nil {
		t.Fatal("Validate() error = nil, want auth error")
	}
}

func TestConnector_ConfigPush(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := &Connector{apiToken: "fake-token", accountID: "acct1", client: srv.Client(), baseURL: srv.URL}

	if err := c.ConfigPush(context.Background(), map[string]any{}, "zone1/r1", "proxied", false); err != nil {
		t.Fatalf("ConfigPush(proxied) error = %v", err)
	}
	if gotPath != "/zones/zone1/dns_records/r1" {
		t.Errorf("ConfigPush(proxied) path = %q, want %q", gotPath, "/zones/zone1/dns_records/r1")
	}

	if err := c.ConfigPush(context.Background(), map[string]any{}, "app1/pol1", "enabled", true); err != nil {
		t.Fatalf("ConfigPush(enabled) error = %v", err)
	}
	if gotPath != "/accounts/acct1/access/apps/app1/policies/pol1" {
		t.Errorf("ConfigPush(enabled) path = %q, want %q", gotPath, "/accounts/acct1/access/apps/app1/policies/pol1")
	}
	if gotBody == "" {
		t.Fatal("ConfigPush(enabled) did not send a request body")
	}

	if err := c.ConfigPush(context.Background(), map[string]any{}, "badref", "proxied", true); err == nil {
		t.Fatal("ConfigPush() with malformed entityRef, want error")
	}
}

func TestConnector_ConfigRead(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/zones":
			_, _ = w.Write([]byte(`{"result":[{"id":"zone1","name":"example.com"}]}`))
		case "/zones/zone1/dns_records":
			_, _ = w.Write([]byte(`{"result":[{"id":"r0","name":"other.example.com","type":"A","content":"203.0.113.9","proxied":false,"ttl":1},{"id":"r1","name":"app.example.com","type":"A","content":"203.0.113.5","proxied":true,"ttl":1}]}`))
		case "/accounts/acct1/cfd_tunnel":
			_, _ = w.Write([]byte(`{"result":[]}`))
		case "/accounts/acct1/access/apps":
			_, _ = w.Write([]byte(`{"result":[{"id":"app1","name":"internal-dashboard","policies":[{"id":"pol0","name":"allow-ops","decision":"allow"},{"id":"pol1","name":"deny-eng","decision":"deny"},{"id":"pol2","name":"bypass-all","decision":"bypass"}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Connector{apiToken: "fake-token", accountID: "acct1", client: srv.Client(), baseURL: srv.URL}
	for _, tt := range []struct {
		entityRef string
		fieldKey  string
		want      any
	}{
		{entityRef: "zone1/r1", fieldKey: "proxied", want: true},
		{entityRef: "app1/pol1", fieldKey: "enabled", want: false},
		{entityRef: "app1/pol2", fieldKey: "enabled", want: nil}, // bypass has no allow/deny value to revert to
	} {
		got, err := c.ConfigRead(context.Background(), nil, tt.entityRef, tt.fieldKey)
		if err != nil {
			t.Fatalf("ConfigRead(%q, %q) error = %v", tt.entityRef, tt.fieldKey, err)
		}
		if got != tt.want {
			t.Errorf("ConfigRead(%q, %q) = %#v, want %#v", tt.entityRef, tt.fieldKey, got, tt.want)
		}
	}
	if requests == 0 {
		t.Fatal("ConfigRead() did not fetch a fresh snapshot")
	}
}

func TestConnector_ConfigReadRejectsInvalidRefAndField(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &Connector{client: srv.Client(), baseURL: srv.URL}

	for _, tt := range []struct {
		name      string
		entityRef string
		fieldKey  string
	}{
		{name: "empty ref", entityRef: "", fieldKey: "proxied"},
		{name: "extra ref segment", entityRef: "zone1/r1/extra", fieldKey: "proxied"},
		{name: "unsupported field", entityRef: "zone1/r1", fieldKey: "ttl"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := c.ConfigRead(context.Background(), nil, tt.entityRef, tt.fieldKey); err == nil {
				t.Fatal("ConfigRead() error = nil, want invalid input error")
			}
		})
	}
	if requests != 0 {
		t.Errorf("ConfigRead() made %d requests for invalid input, want 0", requests)
	}
}

package netbird

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnector_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/peers":
			_, _ = w.Write([]byte(`[{"id":"p1","name":"laptop","ip":"100.64.0.1","os":"linux","approval_required":false,"connected":true}]`))
		case "/api/routes":
			_, _ = w.Write([]byte(`[{"id":"r1","network":"10.0.0.0/24","enabled":true}]`))
		case "/api/policies":
			_, _ = w.Write([]byte(`[{"id":"pol1","name":"allow-eng","enabled":true}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Connector{url: srv.URL, apiToken: "fake-token", client: srv.Client()}

	snap, err := c.Fetch(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if snap.ServiceName != "Netbird" {
		t.Errorf("ServiceName = %q, want %q", snap.ServiceName, "Netbird")
	}
	if len(snap.Sections) != 3 {
		t.Errorf("len(Sections) = %d, want 3", len(snap.Sections))
	}
	if len(snap.Entities) != 2 {
		t.Errorf("len(Entities) = %d, want 2 (1 peer + 1 policy)", len(snap.Entities))
	}
}

func TestConnector_Validate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Connector{url: srv.URL, apiToken: "bad-token", client: srv.Client()}
	if err := c.Validate(context.Background(), map[string]any{}); err == nil {
		t.Fatal("Validate() error = nil, want auth error")
	}
}

func TestConnector_ConfigPush(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/api/peers/p1" {
			buf := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(buf)
			gotBody = string(buf)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Connector{url: srv.URL, apiToken: "fake-token", client: srv.Client()}
	if err := c.ConfigPush(context.Background(), map[string]any{}, "p1", "approved", true); err != nil {
		t.Fatalf("ConfigPush() error = %v", err)
	}
	if gotBody == "" {
		t.Fatal("ConfigPush() did not send a request body")
	}
}

func TestConnector_ConfigRead(t *testing.T) {
	var peerRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/peers":
			peerRequests++
			_, _ = w.Write([]byte(`[{"id":"p0","name":"phone","approval_required":true,"connected":true},{"id":"p1","name":"laptop","approval_required":false,"connected":true}]`))
		case "/api/routes", "/api/policies":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := &Connector{url: srv.URL, apiToken: "fake-token", client: srv.Client()}
	got, err := c.ConfigRead(context.Background(), nil, "p1", "approved")
	if err != nil {
		t.Fatalf("ConfigRead() error = %v", err)
	}
	if got != true {
		t.Errorf("ConfigRead() = %#v, want true", got)
	}
	if peerRequests != 1 {
		t.Errorf("peer fetch requests = %d, want 1 fresh read", peerRequests)
	}
}

func TestConnector_ConfigReadRejectsInvalidRefAndField(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	c := &Connector{url: srv.URL, client: srv.Client()}

	for _, tt := range []struct {
		name      string
		entityRef string
		fieldKey  string
	}{
		{name: "empty ref", entityRef: "", fieldKey: "approved"},
		{name: "path traversal ref", entityRef: "../peer", fieldKey: "approved"},
		{name: "unsupported field", entityRef: "p1", fieldKey: "connected"},
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

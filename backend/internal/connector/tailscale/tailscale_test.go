package tailscale

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newServer(t *testing.T, devices string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2/oauth/token":
			if u, p, ok := r.BasicAuth(); !ok || u != "cid" || p != "sec" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"minted"}`))
		case "/api/v2/tailnet/-/devices":
			if got := r.Header.Get("Authorization"); got != "Bearer minted" && got != "Bearer key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(devices))
		case "/api/v2/tailnet/-/acl":
			_, _ = w.Write([]byte(`{"acls":[{}, {}],"groups":{"group:a":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestConnector_Fetch(t *testing.T) {
	soon := time.Now().Add(5*24*time.Hour + time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	devices := fmt.Sprintf(`{"devices":[
		{"nodeId":"n1","name":"a.tail.ts.net","addresses":["100.64.0.1"],"os":"linux","authorized":true,"connectedToControl":true,"expires":%q,"tags":["tag:srv"]},
		{"nodeId":"n2","name":"b.tail.ts.net","addresses":["100.64.0.2"],"os":"macOS","authorized":true,"expires":%q},
		{"nodeId":"n3","name":"c.tail.ts.net","addresses":["100.64.0.3"],"keyExpiryDisabled":true,"expires":"0001-01-01T00:00:00Z"}]}`, soon, past)
	srv := newServer(t, devices)

	c := &Connector{url: srv.URL, tailnet: "-", apiKey: "key", client: srv.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snap.Sections) != 2 || len(snap.Entities) != 3 {
		t.Fatalf("sections=%d entities=%d, want 2/3", len(snap.Sections), len(snap.Entities))
	}
	a, b, d := snap.Entities[0], snap.Entities[1], snap.Entities[2]
	if a.IP != "100.64.0.1" || a.ExternalID != "n1" || a.Attributes["keyExpiresInDays"] != 5.0 || a.Attributes["keyExpired"] != false {
		t.Errorf("device a = %+v", a)
	}
	if b.Attributes["keyExpired"] != true {
		t.Errorf("device b should be expired: %+v", b.Attributes)
	}
	if _, ok := d.Attributes["keyExpiresInDays"]; ok || d.Attributes["keyExpired"] != false {
		t.Errorf("device c attrs = %+v", d.Attributes)
	}
	if !strings.Contains(snap.Sections[1].Content, "ACL rules | 2") {
		t.Errorf("policy summary = %q", snap.Sections[1].Content)
	}
}

func TestConnector_OAuth(t *testing.T) {
	srv := newServer(t, `{"devices":[]}`)
	c := &Connector{url: srv.URL, tailnet: "-", clientID: "cid", clientSecret: "sec", client: srv.Client()}
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	c.clientSecret = "wrong"
	if err := c.Validate(context.Background(), nil); err == nil {
		t.Fatal("Validate() error = nil, want auth error")
	}
}

func TestConnector_ValidateBadKey(t *testing.T) {
	srv := newServer(t, `{"devices":[]}`)
	c := &Connector{url: srv.URL, tailnet: "-", apiKey: "bad", client: srv.Client()}
	if err := c.Validate(context.Background(), nil); err == nil {
		t.Fatal("Validate() error = nil, want auth error")
	}
}

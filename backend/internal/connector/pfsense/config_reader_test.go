package pfsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestConfigRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/system/version":
			_, _ = w.Write([]byte(`{"data":{"platform":"pfSense","config_version":"2.7.2"}}`))
		case "/api/v2/interfaces":
			_, _ = w.Write([]byte(`{"data":[]}`))
		case "/api/v2/firewall/rules":
			_, _ = w.Write([]byte(`{"data":[{"id":0,"tracker":"rule-dns","descr":"DNS","disabled":true},{"id":1,"tracker":"rule-ssh","descr":"SSH","disabled":false}]}`))
		case "/api/v2/routing/gateways":
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", client: server.Client()}
	entityRef := connector.ScopedExternalID(typeName, server.URL, "tracker:rule-ssh")
	value, err := c.ConfigRead(context.Background(), nil, entityRef, "enabled")
	if err != nil || value != true {
		t.Fatalf("ConfigRead() = (%#v, %v), want (true, nil)", value, err)
	}
	if _, err := c.ConfigRead(context.Background(), nil, entityRef, "protocol"); err == nil {
		t.Error("ConfigRead() error = nil for unsupported field")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "tracker:rule-ssh", "enabled"); err == nil {
		t.Error("ConfigRead() error = nil for unscoped entity reference")
	}
	missingRef := connector.ScopedExternalID(typeName, server.URL, "tracker:missing")
	if _, err := c.ConfigRead(context.Background(), nil, missingRef, "enabled"); err == nil {
		t.Error("ConfigRead() error = nil for missing rule")
	}
}

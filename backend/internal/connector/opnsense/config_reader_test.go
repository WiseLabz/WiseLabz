package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/core/firmware/status":
			_, _ = w.Write([]byte(`{"product_name":"OPNsense","product_version":"24.1"}`))
		case "/api/diagnostics/interface/getInterfaces":
			_, _ = w.Write([]byte(`{"rows":[]}`))
		case "/api/firewall/filter/searchRule":
			_, _ = w.Write([]byte(`{"rows":[{"uuid":"rule-1","description":"SSH","action":"pass","protocol":"tcp","source_net":"any","destination_net":"any","enabled":"1"}]}`))
		case "/api/routes/gateway/status":
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	value, err := c.ConfigRead(context.Background(), nil, "rule-1", "enabled")
	if err != nil || value != true {
		t.Fatalf("ConfigRead() = (%#v, %v), want (true, nil)", value, err)
	}
	if _, err := c.ConfigRead(context.Background(), nil, "rule-1", "protocol"); err == nil {
		t.Error("ConfigRead() error = nil for unsupported field")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "rule/1", "enabled"); err == nil {
		t.Error("ConfigRead() error = nil for invalid entity reference")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "fallback:rule:abc", "enabled"); err == nil {
		t.Error("ConfigRead() error = nil for unsupported fallback reference")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "missing", "enabled"); err == nil {
		t.Error("ConfigRead() error = nil for missing rule")
	}
}

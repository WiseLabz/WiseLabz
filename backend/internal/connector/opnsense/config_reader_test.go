package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRead(t *testing.T) {
	var getRuleRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/firewall/filter/getRule/rule-1":
			getRuleRequests++
			_, _ = w.Write([]byte(`{"rule":{"enabled":"1"}}`))
		case "/api/firewall/filter/getRule/missing":
			getRuleRequests++
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: testClient(server)}
	t.Cleanup(func() { filterStates.Delete(filterKey(server.URL)) })
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
	if getRuleRequests != 2 {
		t.Errorf("ConfigRead() made %d getRule requests, want 2 and no search/fetch requests", getRuleRequests)
	}
}

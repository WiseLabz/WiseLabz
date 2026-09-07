package custom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateCustomURL(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.test", "http:///missing-host"} {
		if err := validateCustomURL(raw); err == nil {
			t.Errorf("validateCustomURL(%q) error = nil", raw)
		}
	}
	if err := validateCustomURL("https://example.test/status"); err != nil {
		t.Fatalf("validateCustomURL(valid): %v", err)
	}
}

func TestFetchUsesConfiguredMethodAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.Header.Get("X-Test") != "value" {
			t.Fatalf("request = %s %q", r.Method, r.Header.Get("X-Test"))
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()
	c := &Connector{client: server.Client()}
	snapshot, err := c.Fetch(context.Background(), map[string]any{
		"url": server.URL, "method": http.MethodPatch, "headers": `{"X-Test":"value"}`,
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snapshot.Metadata["status_code"] != "200" || !strings.Contains(snapshot.Sections[0].Content, `{"status":"ok"}`) {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

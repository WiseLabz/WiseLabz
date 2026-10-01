package truenas

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppActions(t *testing.T) {
	tests := []struct {
		name     string
		call     func(c *Connector) error
		wantPath string
	}{
		{"restart", func(c *Connector) error { return c.Restart(context.Background(), nil, "plex") }, pathApps + "/redeploy"},
		{"start", func(c *Connector) error { return c.Start(context.Background(), nil, "plex") }, pathApps + "/start"},
		{"stop", func(c *Connector) error { return c.Stop(context.Background(), nil, "plex") }, pathApps + "/stop"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotMethod, gotPath, gotBody, gotAuth string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
				b, _ := io.ReadAll(r.Body)
				gotBody = string(b)
				_, _ = w.Write([]byte("42"))
			}))
			defer server.Close()
			c := newTestConnector(t, map[string]any{"url": server.URL, "api_key": "key123"})

			if err := tt.call(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if gotMethod != http.MethodPost || gotPath != tt.wantPath {
				t.Errorf("request = %s %s, want POST %s", gotMethod, gotPath, tt.wantPath)
			}
			if gotBody != `"plex"` {
				t.Errorf("body = %s, want \"plex\"", gotBody)
			}
			if gotAuth != "Bearer key123" {
				t.Errorf("Authorization = %q", gotAuth)
			}
		})
	}
}

func TestAppActionRejectsBadRef(t *testing.T) {
	c := newTestConnector(t, map[string]any{"url": "http://127.0.0.1:1", "api_key": "k"})
	for _, ref := range []string{"", "../x", "a/b"} {
		if err := c.Stop(context.Background(), nil, ref); err == nil {
			t.Errorf("Stop(%q) succeeded, want error", ref)
		}
	}
}

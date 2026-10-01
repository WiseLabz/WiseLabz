package portainer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestStackActions(t *testing.T) {
	tests := []struct {
		name string
		call func(c *Connector) error
		want []string
	}{
		{"start", func(c *Connector) error { return c.Start(context.Background(), nil, "7") },
			[]string{"GET /api/stacks/7", "POST /api/stacks/7/start?endpointId=2"}},
		{"stop", func(c *Connector) error { return c.Stop(context.Background(), nil, "7") },
			[]string{"GET /api/stacks/7", "POST /api/stacks/7/stop?endpointId=2"}},
		{"restart", func(c *Connector) error { return c.Restart(context.Background(), nil, "7") },
			[]string{"GET /api/stacks/7", "POST /api/stacks/7/stop?endpointId=2", "POST /api/stacks/7/start?endpointId=2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = append(got, r.Method+" "+r.URL.RequestURI())
				if r.Header.Get("X-API-Key") != testAPIKey {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				_, _ = w.Write([]byte(`{"Id":7,"EndpointId":2}`))
			}))
			defer server.Close()
			c := newTestConnector(t, map[string]any{"url": server.URL, "api_key": testAPIKey})

			if err := tt.call(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("requests = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStackActionRejectsBadRef(t *testing.T) {
	c := newTestConnector(t, map[string]any{"url": "http://127.0.0.1:1", "api_key": testAPIKey})
	for _, ref := range []string{"", "../x", "7/start"} {
		if err := c.Stop(context.Background(), nil, ref); err == nil {
			t.Errorf("Stop(%q) succeeded, want error", ref)
		}
	}
}

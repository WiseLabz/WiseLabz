package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConfigRead(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/info":
			_, _ = w.Write([]byte(`{"Name":"docker","ServerVersion":"27.0","Containers":1,"ContainersRunning":1,"Images":1}`))
		case "/containers/json":
			if r.URL.Query().Get("all") != "true" {
				t.Fatalf("containers query = %q, want all=true", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`[{"Id":"container-1","Names":["/web"],"Image":"nginx","State":"running","HostConfig":{"NetworkMode":"bridge"}}]`))
		case "/containers/container-1/json":
			_, _ = w.Write([]byte(`{"HostConfig":{"RestartPolicy":{"Name":"unless-stopped"}}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{host: "tcp://docker", baseURL: server.URL, client: server.Client()}
	value, err := c.ConfigRead(context.Background(), nil, "container-1", "restartPolicy")
	if err != nil || value != "unless-stopped" {
		t.Fatalf("ConfigRead() = (%#v, %v), want (unless-stopped, nil)", value, err)
	}
	if _, err := c.ConfigRead(context.Background(), nil, "container-1", "image"); err == nil {
		t.Fatal("ConfigRead() error = nil for unsupported field")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "container/1", "restartPolicy"); err == nil {
		t.Fatal("ConfigRead() error = nil for invalid entity reference")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "missing", "restartPolicy"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ConfigRead() error = %v, want not-found error", err)
	}
	if requests == 0 {
		t.Fatal("ConfigRead() did not fetch current connector data")
	}
}

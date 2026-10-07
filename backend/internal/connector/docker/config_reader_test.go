package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRead(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/containers/container-1/json":
			_, _ = w.Write([]byte(`{"HostConfig":{"RestartPolicy":{"Name":"unless-stopped"}}}`))
		case "/containers/container-2/json":
			_, _ = w.Write([]byte(`{"HostConfig":{"RestartPolicy":{"Name":""}}}`))
		case "/containers/container-3/json":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"No such container: missing"}`))
		}
	}))
	defer server.Close()

	c := &Connector{host: "tcp://docker", baseURL: server.URL, client: server.Client()}
	value, err := c.ConfigRead(context.Background(), nil, "container-1", "restartPolicy")
	if err != nil || value != "unless-stopped" {
		t.Fatalf("ConfigRead() = (%#v, %v), want (unless-stopped, nil)", value, err)
	}
	if len(paths) != 1 || paths[0] != "/containers/container-1/json" {
		t.Fatalf("requests = %v, want only the container-1 inspect", paths)
	}
	value, err = c.ConfigRead(context.Background(), nil, "container-2", "restartPolicy")
	if err != nil || value != "no" {
		t.Fatalf("ConfigRead() empty policy = (%#v, %v), want (no, nil)", value, err)
	}
	if _, err := c.ConfigRead(context.Background(), nil, "container-3", "restartPolicy"); err == nil {
		t.Fatal("ConfigRead() error = nil for response without a restart policy")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "container-1", "image"); err == nil {
		t.Fatal("ConfigRead() error = nil for unsupported field")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "container/1", "restartPolicy"); err == nil {
		t.Fatal("ConfigRead() error = nil for invalid entity reference")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "missing", "restartPolicy"); err == nil {
		t.Fatal("ConfigRead() error = nil for a missing container")
	}
}

package docker

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestValidateHitsVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"Version":"24.0.0"}`))
	}))
	defer server.Close()

	c := &Connector{host: "tcp://" + server.Listener.Addr().String(), baseURL: server.URL, client: server.Client()}
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFetchBuildsSectionsFromEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_, _ = w.Write([]byte(`{"Name":"docker-host","ServerVersion":"24.0.0","Containers":1,"ContainersRunning":1,"Images":2,"NCPU":4,"MemTotal":1024}`))
		case "/containers/json":
			_, _ = w.Write([]byte(`[{"Names":["/web"],"Image":"nginx","State":"running","Status":"Up 2 hours"}]`))
		case "/images/json":
			_, _ = w.Write([]byte(`[{"RepoTags":["nginx:latest"],"Size":100}]`))
		case "/volumes":
			_, _ = w.Write([]byte(`{"Volumes":[{"Name":"data","Driver":"local","Mountpoint":"/var/lib/docker/volumes/data"}]}`))
		case "/networks":
			_, _ = w.Write([]byte(`[{"Name":"bridge","Driver":"bridge","Scope":"local"}]`))
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{host: "tcp://example", baseURL: server.URL, client: server.Client()}
	snap, err := c.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snap.Sections) != 5 {
		t.Fatalf("Sections = %d, want 5", len(snap.Sections))
	}
	if !strings.Contains(snap.Sections[0].Content, "docker-host") {
		t.Errorf("System section = %q", snap.Sections[0].Content)
	}
	wantDeps := []connector.ServiceDependency{{Kind: "host", Name: "tcp://example"}}
	if len(snap.Dependencies) != 1 || snap.Dependencies[0] != wantDeps[0] {
		t.Errorf("Dependencies = %+v, want %+v", snap.Dependencies, wantDeps)
	}
}

func TestFetchWithFieldsHintSkipsUnrequestedCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_, _ = w.Write([]byte(`{"Name":"docker-host"}`))
		case "/containers/json":
			_, _ = w.Write([]byte(`[{"Names":["/web"],"Image":"nginx","State":"running","Status":"Up"}]`))
		default:
			t.Fatalf("unexpected request path %s: selective fetch should only hit /info and /containers/json", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{host: "tcp://example", baseURL: server.URL, client: server.Client()}
	snap, err := c.Fetch(context.Background(), map[string]any{"fields": []any{"containers"}})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if len(snap.Sections) != 2 {
		t.Fatalf("Sections = %d, want 2 (System + Containers)", len(snap.Sections))
	}
}

func TestFetchToleratesEndpointFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			_, _ = w.Write([]byte(`{"Name":"docker-host"}`))
		case "/containers/json":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := &Connector{host: "tcp://example", baseURL: server.URL, client: server.Client()}
	snap, err := c.Fetch(context.Background(), map[string]any{"fields": []any{"containers"}})
	if err != nil {
		t.Fatalf("Fetch() error = %v, want nil (failures should be tolerated as placeholders)", err)
	}
	if !strings.Contains(snap.Sections[1].Content, "unavailable") {
		t.Errorf("Containers section = %q, want unavailable placeholder", snap.Sections[1].Content)
	}
}

func TestNewDockerClientRejectsUnsupportedScheme(t *testing.T) {
	if _, _, err := newDockerClient("ssh://user@host"); err == nil {
		t.Fatal("newDockerClient(ssh://...) error = nil, want rejection")
	}
}

func TestNewDockerClientDialsUnixSocket(t *testing.T) {
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "docker.sock")

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("net.Listen(unix): %v", err)
	}
	defer func() { _ = listener.Close() }()

	go func() {
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/version" {
				return
			}
			_, _ = w.Write([]byte(`{"Version":"24.0.0"}`))
		})}
		_ = srv.Serve(listener)
	}()

	client, baseURL, err := newDockerClient("unix://" + socketPath)
	if err != nil {
		t.Fatalf("newDockerClient(unix://...) error = %v", err)
	}
	if baseURL != "http://unix" {
		t.Fatalf("baseURL = %q, want http://unix", baseURL)
	}

	c := &Connector{host: "unix://" + socketPath, baseURL: baseURL, client: client}
	if err := c.Validate(context.Background(), nil); err != nil {
		t.Fatalf("Validate() over unix socket error = %v", err)
	}
}

func TestFetchSurfacesMalformedSystemResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/info" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	c := &Connector{host: "tcp://example", baseURL: server.URL, client: server.Client()}
	snap, err := c.Fetch(context.Background(), map[string]any{"fields": []any{"none"}})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if !strings.Contains(snap.Sections[0].Content, "malformed response") {
		t.Fatalf("System section = %q, want malformed response placeholder", snap.Sections[0].Content)
	}
}

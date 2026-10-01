package homeassistant

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestart(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()
	c := newTestConnector(t, map[string]any{"url": server.URL, "access_token": testToken})

	if err := c.Restart(context.Background(), nil, ""); err != nil {
		t.Fatalf("Restart: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != pathRestartService {
		t.Errorf("request = %s %s, want POST %s", gotMethod, gotPath, pathRestartService)
	}
	if gotAuth != "Bearer "+testToken {
		t.Errorf("Authorization = %q", gotAuth)
	}
}

func TestRestartReportsUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	c := newTestConnector(t, map[string]any{"url": server.URL, "access_token": "bad"})

	if err := c.Restart(context.Background(), nil, ""); err == nil {
		t.Fatal("Restart succeeded against a 401, want error")
	}
}

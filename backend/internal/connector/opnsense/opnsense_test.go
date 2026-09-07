package opnsense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateUsesBasicAuthAndSurfacesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, password, ok := r.BasicAuth()
		if !ok || user != "key" || password != "secret" || r.URL.Path != "/api/core/firmware/status" {
			t.Fatalf("request auth/path invalid")
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, apiKey: "key", apiSecret: "secret", client: server.Client()}
	err := c.Validate(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "API returned 401: denied") {
		t.Fatalf("Validate() error = %v", err)
	}
}

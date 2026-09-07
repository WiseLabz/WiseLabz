package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateUsesTokenAndSurfacesStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=user@pam!token=secret" || r.URL.Path != "/nodes" {
			t.Fatalf("request auth/path invalid")
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer server.Close()
	c := &Connector{url: server.URL, tokenID: "user@pam!token", tokenSecret: "secret", client: server.Client()}
	err := c.Validate(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "API returned 403: denied") {
		t.Fatalf("Validate() error = %v", err)
	}
}

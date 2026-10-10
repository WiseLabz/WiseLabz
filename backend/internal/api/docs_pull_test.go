package api_test

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDocsPullRoutesRequireAdminAndElevation(t *testing.T) {
	app := newTestApp(t)
	app.Config.Attachments.ImportDir = t.TempDir()
	stepUp(app, true)
	_, userToken := app.user(t, "viewer")
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		rec := app.req(t, method, "/api/docs/import/pull", nil, userToken)
		if rec.Code != 403 {
			t.Errorf("%s non-admin = %d %s", method, rec.Code, rec.Body)
		}
	}
	_, adminToken := app.user(t, "operator")
	body := map[string]any{"source": "bookstack", "url": "http://127.0.0.1:9", "tokenId": "id", "tokenSecret": "secret"}
	rec := app.req(t, http.MethodPost, "/api/docs/import/pull", body, adminToken)
	if rec.Code != 400 || decodeErr(t, rec).Code != "elevation_required" {
		t.Fatalf("bare = %d %s", rec.Code, rec.Body)
	}
	rec = app.reqElevated(t, http.MethodPost, "/api/docs/import/pull", body, adminToken, "docs.import.pull")
	if rec.Code != 202 {
		t.Fatalf("elevated = %d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status := app.req(t, http.MethodGet, "/api/docs/import/pull", nil, adminToken)
		if strings.Contains(status.Body.String(), `"state":"failed"`) {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if strings.Contains(rec.Body.String(), "tokenSecret") {
		t.Fatal("response exposes credential")
	}
}

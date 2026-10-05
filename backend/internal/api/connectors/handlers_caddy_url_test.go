package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

const caddyPastedJSON = `{"apps":{"http":{"servers":{"srv0":{"listen":[":443"],"routes":[{"match":[{"host":["app.lab.test"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.5:8080"}]}]}]}}}}}`

func caddyBody(t *testing.T, url string, cfg map[string]any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"name": "Caddy", "category": "networking", "type": "caddy", "url": url, "config": cfg})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func postConnector(h *Handler, body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.Create(rr, httptest.NewRequest(http.MethodPost, "/api/connectors", strings.NewReader(body)))
	return rr
}

func TestCreateURLRequirementFollowsTypeSchema(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)

	t.Run("caddy pasted json without url creates and syncs", func(t *testing.T) {
		rr := postConnector(h, caddyBody(t, "", map[string]any{"config_json": caddyPastedJSON}))
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
		}
		var created map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &created)
		id, _ := created["id"].(string)
		res, err := h.SyncEngine.RunSync(context.Background(), id, "")
		if err != nil {
			t.Fatalf("RunSync: %v", err)
		}
		if res == nil || res.Error != "" {
			t.Fatalf("sync result = %+v", res)
		}
	})

	t.Run("caddy with url and config_json is rejected clearly", func(t *testing.T) {
		rr := postConnector(h, caddyBody(t, "http://caddy.example.com:2019", map[string]any{"config_json": caddyPastedJSON}))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "only one") {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("caddy with neither is rejected clearly", func(t *testing.T) {
		rr := postConnector(h, caddyBody(t, "", map[string]any{}))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "either") {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("url-requiring type without url is still 400", func(t *testing.T) {
		rr := postConnector(h, `{"name":"x","category":"virtualization","type":"custom","url":""}`)
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `"url"`) {
			t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestUpdateURLRequirementParity(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	put := func(id, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/connectors/"+id, strings.NewReader(body))
		req.SetPathValue("id", id)
		req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
		rr := httptest.NewRecorder()
		h.Update(rr, req)
		return rr
	}
	idOf := func(rr *httptest.ResponseRecorder) string {
		var m map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &m)
		id, _ := m["id"].(string)
		return id
	}

	custom := idOf(postConnector(h, `{"name":"x","category":"virtualization","type":"custom","url":"https://a.example.com"}`))
	if rr := put(custom, `{"url":""}`); rr.Code != http.StatusBadRequest {
		t.Fatalf("clearing url on url-requiring type: status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}

	caddy := idOf(postConnector(h, caddyBody(t, "", map[string]any{"config_json": caddyPastedJSON})))
	if rr := put(caddy, `{"url":"","name":"renamed"}`); rr.Code != http.StatusOK {
		t.Fatalf("caddy empty url: status = %d; body=%s", rr.Code, rr.Body.String())
	}
	body := `{"url":"http://caddy.example.com:2019","config":{"config_json":` + mustJSON(caddyPastedJSON) + `}}`
	if rr := put(caddy, body); rr.Code != http.StatusBadRequest {
		t.Fatalf("caddy both modes: status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func mustJSON(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestTestConnectionPastedCaddy(t *testing.T) {
	t.Parallel()
	h := newTestHandler(t)
	var created map[string]any
	_ = json.Unmarshal(postConnector(h, caddyBody(t, "", map[string]any{"config_json": caddyPastedJSON})).Body.Bytes(), &created)
	id, _ := created["id"].(string)
	req := httptest.NewRequest(http.MethodPost, "/api/connectors/"+id+"/test", nil)
	req.SetPathValue("id", id)
	req = req.WithContext(auth.ContextWithUser(req.Context(), "admin", true))
	rr := httptest.NewRecorder()
	h.Test(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
}

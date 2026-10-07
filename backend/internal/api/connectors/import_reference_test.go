package connectors

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type importRefEnv struct {
	h       *Handler
	user    string
	traefik *store.ConnectorRecord
	npm     *store.ConnectorRecord
	hidden  *store.ConnectorRecord
}

func newImportRefEnv(t *testing.T) *importRefEnv {
	t.Helper()
	h := newTestHandler(t)
	env := &importRefEnv{h: h, user: apitest.NewUser(t, h.Store, "viewer")}
	for _, c := range []struct {
		dst       **store.ConnectorRecord
		name, typ string
		granted   bool
		url       string
	}{
		{&env.traefik, "traefik", "traefik", true, "http://traefik.lab:8080"},
		{&env.npm, "npm", "npm", true, "http://npm.lab:81"},
		{&env.hidden, "hidden traefik", "traefik", false, "http://other.lab:8080"},
	} {
		rec := &store.ConnectorRecord{Name: c.name, Category: "networking", Type: c.typ, URL: c.url, Enabled: true, ConfigData: "{}"}
		if err := h.Store.CreateConnector(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
		if c.granted {
			apitest.GrantConnectorRole(t, h.Store, env.user, rec.ID, "viewer")
		}
		*c.dst = rec
	}
	return env
}

func (e *importRefEnv) call(t *testing.T, method, path, body string, handle http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetPathValue("id", strings.TrimPrefix(path, "/api/connectors/"))
	req = req.WithContext(auth.ContextWithUser(req.Context(), e.user, true))
	rr := httptest.NewRecorder()
	handle(rr, req)
	return rr
}

func probeBody(name, importID string) string {
	return `{"name":"` + name + `","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443","import_connector_id":"` + importID + `"}}`
}

func requireImportFieldError(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Details []httputil.FieldError `json:"details"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Details {
		if d.Field == "config.import_connector_id" {
			return
		}
	}
	t.Errorf("no field error on config.import_connector_id: %+v", resp.Details)
}

func TestCreateTLSProbeImportReference(t *testing.T) {
	env := newImportRefEnv(t)
	create := func(importID string) *httptest.ResponseRecorder {
		return env.call(t, http.MethodPost, "/api/connectors", probeBody("probe", importID), env.h.Create)
	}

	if rr := create(env.traefik.ID); rr.Code != http.StatusCreated {
		t.Fatalf("viewable Traefik: status %d body=%s", rr.Code, rr.Body.String())
	}
	if rr := env.call(t, http.MethodPost, "/api/connectors",
		`{"name":"plain","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443"}}`, env.h.Create); rr.Code != http.StatusCreated {
		t.Fatalf("no reference, no url: status %d body=%s", rr.Code, rr.Body.String())
	}

	hidden := create(env.hidden.ID)
	missing := create("no-such-connector")
	for name, rr := range map[string]*httptest.ResponseRecorder{"not viewable": hidden, "missing": missing} {
		if rr.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403; body=%s", name, rr.Code, rr.Body.String())
		}
	}
	if hidden.Body.String() != missing.Body.String() {
		t.Errorf("a missing and a hidden connector are distinguishable:\n%s\n%s", hidden.Body.String(), missing.Body.String())
	}

	requireImportFieldError(t, create(env.npm.ID))
}

func TestCreateTLSProbeStillRejectsMalformedTargets(t *testing.T) {
	env := newImportRefEnv(t)
	rr := env.call(t, http.MethodPost, "/api/connectors",
		`{"name":"bad","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab"}}`, env.h.Create)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), "config.targets") {
		t.Fatalf("status %d body=%s", rr.Code, rr.Body.String())
	}
}

func TestUpdateTLSProbeImportReference(t *testing.T) {
	env := newImportRefEnv(t)
	rr := env.call(t, http.MethodPost, "/api/connectors",
		`{"name":"probe","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443"}}`, env.h.Create)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &created)
	update := func(importID string) *httptest.ResponseRecorder {
		body := `{"config":{"targets":"nas.lab:443","import_connector_id":"` + importID + `"}}`
		return env.call(t, http.MethodPut, "/api/connectors/"+created.ID, body, env.h.Update)
	}

	if rr := update(env.hidden.ID); rr.Code != http.StatusForbidden {
		t.Errorf("not viewable: status %d, want 403; body=%s", rr.Code, rr.Body.String())
	}
	if rr := update("no-such-connector"); rr.Code != http.StatusForbidden {
		t.Errorf("missing: status %d, want 403", rr.Code)
	}
	requireImportFieldError(t, update(env.npm.ID))
	if rr := update(env.traefik.ID); rr.Code != http.StatusOK {
		t.Errorf("viewable Traefik: status %d body=%s", rr.Code, rr.Body.String())
	}
}

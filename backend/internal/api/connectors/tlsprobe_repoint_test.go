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
)

// A TLS probe's settings decide where the server dials, so only an instance
// admin may change them; an operator can still save them unchanged.
func TestUpdateTLSProbeEndpointSettingsRequireInstanceAdmin(t *testing.T) {
	env := newImportRefEnv(t)
	rr := env.call(t, http.MethodPost, "/api/connectors",
		`{"name":"probe","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443","import_connector_id":"`+env.traefik.ID+`","import_port":8443}}`, env.h.Create)
	requireStatus(t, rr, http.StatusCreated)
	var created struct{ ID string }
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	probeID := created.ID
	operator := apitest.NewUser(t, env.h.Store, "viewer")
	apitest.GrantConnectorRole(t, env.h.Store, operator, probeID, "operator")
	apitest.GrantConnectorRole(t, env.h.Store, operator, env.traefik.ID, "viewer")

	put := func(admin bool, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/connectors/"+probeID, strings.NewReader(body))
		req.SetPathValue("id", probeID)
		req = req.WithContext(auth.ContextWithUser(req.Context(), operator, admin))
		rr := httptest.NewRecorder()
		env.h.Update(rr, req)
		return rr
	}
	stored := func() string {
		c, err := env.h.Store.GetConnector(context.Background(), probeID)
		if err != nil {
			t.Fatal(err)
		}
		return c.ConfigData
	}
	before := stored()

	for name, body := range map[string]string{
		"targets":             `{"config":{"targets":"other.lab:443"}}`,
		"import_port":         `{"config":{"import_port":9443}}`,
		"import_connector_id": `{"config":{"import_connector_id":"` + env.hidden.ID + `"}}`,
	} {
		rr := put(false, body)
		requireStatus(t, rr, http.StatusForbidden)
		if !strings.Contains(rr.Body.String(), "instance admin") {
			t.Errorf("%s: body %s does not say an instance admin is required", name, rr.Body.String())
		}
	}
	if after := stored(); after != before {
		t.Errorf("operator changed the stored config:\n%s\n%s", before, after)
	}

	requireStatus(t, put(false, `{"config":{}}`), http.StatusOK)
	if after := stored(); after != before {
		t.Errorf("omitting tlsprobe endpoint keys changed the stored config:\n%s\n%s", before, after)
	}
	requireStatus(t, put(true, `{"config":{"import_port":9443}}`), http.StatusOK)
	if after := stored(); after == before || !strings.Contains(after, `"import_port":9443`) || !strings.Contains(after, env.traefik.ID) || !strings.Contains(after, "nas.lab:443") {
		t.Errorf("instance admin's change was not stored with omitted endpoint keys preserved: %s", after)
	}
}

func TestTLSProbeConnectorResponseExposesOnlySafeConfig(t *testing.T) {
	env := newImportRefEnv(t)
	body := `{"name":"probe","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443","import_connector_id":"` + env.traefik.ID + `","import_port":8443,"token":"private-value"}}`
	created := env.call(t, http.MethodPost, "/api/connectors", body, env.h.Create)
	requireStatus(t, created, http.StatusCreated)
	var response struct {
		ID     string            `json:"id"`
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Config) != 3 || response.Config["targets"] != "nas.lab:443" || response.Config["import_connector_id"] != env.traefik.ID || response.Config["import_port"] != "8443" {
		t.Errorf("created config = %#v, want only targets, import_connector_id and import_port", response.Config)
	}
	if strings.Contains(created.Body.String(), "private-value") || strings.Contains(created.Body.String(), env.traefik.Name) || strings.Contains(created.Body.String(), env.traefik.URL) {
		t.Errorf("response leaked a secret or source connector details: %s", created.Body.String())
	}

	viewer := apitest.NewUser(t, env.h.Store, "probe viewer")
	apitest.GrantConnectorRole(t, env.h.Store, viewer, response.ID, "viewer")
	getReq := httptest.NewRequest(http.MethodGet, "/api/connectors/"+response.ID, nil)
	getReq.SetPathValue("id", response.ID)
	getReq = getReq.WithContext(auth.ContextWithUser(getReq.Context(), viewer, false))
	get := httptest.NewRecorder()
	env.h.Get(get, getReq)
	requireStatus(t, get, http.StatusOK)
	var viewed struct {
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &viewed); err != nil {
		t.Fatal(err)
	}
	if len(viewed.Config) != 3 || viewed.Config["targets"] != "nas.lab:443" || viewed.Config["import_connector_id"] != env.traefik.ID || viewed.Config["import_port"] != "8443" {
		t.Errorf("viewer config = %#v, want all three safe values", viewed.Config)
	}
	if strings.Contains(get.Body.String(), "private-value") || strings.Contains(get.Body.String(), env.traefik.Name) || strings.Contains(get.Body.String(), env.traefik.URL) {
		t.Errorf("GET response leaked a secret or source connector details: %s", get.Body.String())
	}
}

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
		`{"name":"probe","category":"monitoring","type":"tlsprobe","config":{"targets":"nas.lab:443","import_port":8443}}`, env.h.Create)
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
		"targets":             `{"config":{"targets":"other.lab:443","import_port":8443}}`,
		"import_port":         `{"config":{"targets":"nas.lab:443","import_port":9443}}`,
		"import_connector_id": `{"config":{"targets":"nas.lab:443","import_port":8443,"import_connector_id":"` + env.traefik.ID + `"}}`,
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

	requireStatus(t, put(false, `{"config":{"targets":"nas.lab:443","import_port":8443}}`), http.StatusOK)
	requireStatus(t, put(true, `{"config":{"targets":"other.lab:443","import_port":8443}}`), http.StatusOK)
	if stored() == before {
		t.Error("instance admin's change was not stored")
	}
}

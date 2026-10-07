package connectors

import (
	"errors"
	"net/http"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const (
	tlsProbeType = "tlsprobe"
	traefikType  = "traefik"
)

// authorizeImportReference checks the Traefik connector a TLS probe imports
// hosts from. The caller must be able to view it, because the probe's viewers
// will see the imported host names; it must exist and be a Traefik connector.
// A missing connector and one the caller cannot view get the same 403, so the
// check does not reveal which connector IDs exist. It writes the error response
// and reports false when the save must not proceed.
func (h *Handler) authorizeImportReference(w http.ResponseWriter, r *http.Request, typ string, config map[string]any) bool {
	if typ != tlsProbeType {
		return true
	}
	id, _ := config["import_connector_id"].(string)
	id = strings.TrimSpace(id)
	if id == "" {
		return true
	}
	rec, err := h.Store.GetConnector(r.Context(), id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		httputil.Errorf(w, err)
		return false
	}
	role := ""
	if err == nil {
		if role, err = h.Store.GetUserConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), id); err != nil {
			httputil.Errorf(w, err)
			return false
		}
	}
	if role == "" {
		httputil.Error(w, http.StatusForbidden, "forbidden", "You do not have access to the connector named in config.import_connector_id")
		return false
	}
	if rec.Type != traefikType {
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", "Request validation failed",
			[]httputil.FieldError{{Field: "config.import_connector_id", Msg: "must be a Traefik connector"}})
		return false
	}
	return true
}

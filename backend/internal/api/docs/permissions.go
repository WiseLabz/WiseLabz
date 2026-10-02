package docs

import (
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// requireDocOperator 403s unless the caller may mutate a doc scoped to
// connectorID. Lab-wide docs (connectorID == "", e.g. the Lab Topology doc)
// have no connector to check against, so they're gated on instance-admin
// instead of a per-connector grant.
func (h *Handler) requireDocOperator(w http.ResponseWriter, r *http.Request, connectorID string) bool {
	if connectorID == "" {
		if !auth.InstanceAdminFromContext(r.Context()) {
			httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
			return false
		}
		return true
	}
	ok, err := h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), connectorID, "operator")
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if !ok {
		httputil.Error(w, http.StatusForbidden, "forbidden", "insufficient permissions")
		return false
	}
	return true
}

// requireDocViewer 404s (not 403, to avoid confirming existence) unless the
// caller may view a doc scoped to connectorID. Human lab notes are visible
// to authenticated users; generated lab inventory is instance-admin only.
func (h *Handler) requireDocViewer(w http.ResponseWriter, r *http.Request, connectorID string, origin ...string) bool {
	if connectorID == "" {
		if len(origin) > 0 && origin[0] == store.DocOriginHuman {
			return true
		}
		if !auth.InstanceAdminFromContext(r.Context()) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
			return false
		}
		return true
	}
	ok, err := h.Store.UserHasConnectorRole(r.Context(), auth.UserIDFromContext(r.Context()), connectorID, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return false
	}
	return true
}

// requireDocEditor checks view access before edit access, so callers who
// cannot see the doc get the same 404 as for a missing one, and only callers
// who can see it but not change it get 403.
func (h *Handler) requireDocEditor(w http.ResponseWriter, r *http.Request, d *store.DocRecord) bool {
	return h.requireDocViewer(w, r, d.ServiceID, d.Origin) && h.requireDocOperator(w, r, d.ServiceID)
}

// loadDocForViewer loads the doc and enforces viewer access on its connector,
// writing a 404 and returning false if it is missing or not viewable.
func (h *Handler) loadDocForViewer(w http.ResponseWriter, r *http.Request, id string) bool {
	d, err := h.Store.GetDoc(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Doc not found")
		return false
	}
	if err != nil {
		httputil.Errorf(w, err)
		return false
	}
	return h.requireDocViewer(w, r, d.ServiceID, d.Origin)
}

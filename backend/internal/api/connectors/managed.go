package connectors

import (
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// RequireManagedBy lets a request through only when the connector in the {id}
// path value is in one of the allowed management states (#500). A connector
// declared in config.yaml takes its settings from that file, so changing them
// here would be undone at the next start; an orphaned one may only be deleted
// or released. Anything else is answered with 409.
func (h *Handler) RequireManagedBy(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := h.Store.GetConnector(r.Context(), r.PathValue("id"))
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
					return
				}
				httputil.Errorf(w, err)
				return
			}
			if !slices.Contains(allowed, c.ManagedBy) {
				writeManagedConflict(w, c.ManagedBy)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeManagedConflict(w http.ResponseWriter, managedBy string) {
	code, message := managedConflict(managedBy)
	httputil.Error(w, http.StatusConflict, code, message)
}

// managedConflictError is the 409 writeManagedConflict answers, as an error for
// in-process callers that bypass RequireManagedBy.
func managedConflictError(managedBy string) error {
	code, message := managedConflict(managedBy)
	return &lifecycleError{status: http.StatusConflict, code: code, message: message}
}

func managedConflict(managedBy string) (code, message string) {
	if managedBy == store.ManagedByConfigOrphaned {
		return "connector_orphaned", "This connector was removed from config.yaml. Delete it or release it to the UI first."
	}
	return "connector_managed", "This connector is managed by config.yaml. Change it there and restart the server."
}

// Release handles POST /api/connectors/{id}/release: it returns an orphaned
// connector to UI management and drops its config-sourced grants. The connector stays disabled; whoever releases
// it decides whether to enable it again.
func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateConnector(r.Context(), id, map[string]any{"managed_by": store.ManagedByUI}); err != nil {
		httputil.Errorf(w, err)
		return
	}
	// Grants declared in config.yaml can only be changed there; once the
	// connector is released nothing would ever remove them.
	if _, err := h.Store.SyncConfigConnectorGrants(r.Context(), id, nil); err != nil {
		httputil.Errorf(w, err)
		return
	}
	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.release", "connector", id, nil); err != nil {
		slog.Error("failed to record audit", "action", "connector.release", "error", err)
	}
	c, _ := h.Store.GetConnector(r.Context(), id)
	httputil.JSON(w, http.StatusOK, viewOf(c))
}

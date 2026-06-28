// Package changes provides API handlers for infrastructure change records.
package changes

import (
	"errors"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for change endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new change handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// List handles GET /api/changes.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	serviceID := r.URL.Query().Get("serviceId")
	severity := r.URL.Query().Get("severity")

	changes, total, err := h.Store.ListChanges(r.Context(), serviceID, severity, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, changes, page, pageSize, total)
}

// Get handles GET /api/changes/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.Store.GetChange(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Change not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, c)
}

// Acknowledge handles POST /api/changes/{id}/acknowledge.
func (h *Handler) Acknowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateChangeStatus(r.Context(), id, "acknowledged"); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Change not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

// Dismiss handles POST /api/changes/{id}/dismiss.
func (h *Handler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateChangeStatus(r.Context(), id, "dismissed"); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Change not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

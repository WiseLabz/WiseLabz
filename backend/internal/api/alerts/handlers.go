package alerts

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for alert endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new alert handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// List handles GET /api/alerts.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize, offset := httputil.Paginate(r)
	serviceID := r.URL.Query().Get("serviceId")
	severity := r.URL.Query().Get("severity")
	status := r.URL.Query().Get("status")

	alerts, total, err := h.Store.ListAlerts(r.Context(), serviceID, severity, status, offset, pageSize)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.WritePaginated(w, alerts, page, pageSize, total)
}

// Get handles GET /api/alerts/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a, err := h.Store.GetAlert(r.Context(), id)
	if err == store.ErrNotFound {
		httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, a)
}

// Resolve handles POST /api/alerts/{id}/resolve.
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateAlertStatus(r.Context(), id, "resolved", ""); err != nil {
		if err == store.ErrNotFound {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

// Dismiss handles POST /api/alerts/{id}/dismiss.
func (h *Handler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateAlertStatus(r.Context(), id, "dismissed", ""); err != nil {
		if err == store.ErrNotFound {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

// Snooze handles POST /api/alerts/{id}/snooze.
func (h *Handler) Snooze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		DurationMinutes int `json:"durationMinutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 60 // default 1 hour
	}

	snoozedUntil := time.Now().UTC().Add(time.Duration(req.DurationMinutes) * time.Minute).Format(time.RFC3339)
	if err := h.Store.UpdateAlertStatus(r.Context(), id, "snoozed", snoozedUntil); err != nil {
		if err == store.ErrNotFound {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.NoContent(w)
}

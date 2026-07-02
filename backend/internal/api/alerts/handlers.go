// Package alerts provides API handlers for alert lifecycle management.
package alerts

import (
	"context"
	"encoding/json"
	"errors"
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

// alertResponse builds the spec's Alert response for an alert record.
func (h *Handler) alertResponse(ctx context.Context, id string) (map[string]any, error) {
	a, err := h.Store.GetAlert(ctx, id)
	if err != nil {
		return nil, err
	}

	serviceName := ""
	if conn, err := h.Store.GetConnector(ctx, a.ServiceID); err == nil {
		serviceName = conn.Name
	}

	var changeID, snoozedUntil any
	if a.ChangeID != "" {
		changeID = a.ChangeID
	}
	if a.SnoozedUntil != "" {
		snoozedUntil = a.SnoozedUntil
	}

	return map[string]any{
		"id":           a.ID,
		"changeId":     changeID,
		"serviceId":    a.ServiceID,
		"serviceName":  serviceName,
		"severity":     a.Severity,
		"title":        a.Title,
		"description":  a.Description,
		"status":       a.Status,
		"createdAt":    a.CreatedAt,
		"snoozedUntil": snoozedUntil,
	}, nil
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
	if errors.Is(err, store.ErrNotFound) {
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
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	resp, err := h.alertResponse(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// Dismiss handles POST /api/alerts/{id}/dismiss.
func (h *Handler) Dismiss(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.Store.UpdateAlertStatus(r.Context(), id, "dismissed", ""); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	resp, err := h.alertResponse(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

// Snooze handles POST /api/alerts/{id}/snooze.
func (h *Handler) Snooze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Until string `json:"until"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Until == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "until is required")
		return
	}
	if _, err := time.Parse(time.RFC3339, req.Until); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "until must be an RFC3339 timestamp")
		return
	}

	if err := h.Store.UpdateAlertStatus(r.Context(), id, "snoozed", req.Until); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}
	resp, err := h.alertResponse(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, resp)
}

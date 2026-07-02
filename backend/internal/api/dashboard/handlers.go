// Package dashboard provides the dashboard overview API handler.
package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for dashboard endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new dashboard handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// Overview handles GET /api/dashboard/overview.
// Returns aggregated dashboard data.
func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	statusCounts, _ := h.Store.CountConnectorsByStatus(ctx)
	pendingAlerts, _ := h.Store.CountAlertsPending(ctx)
	latestChanges, _ := h.Store.GetLatestChanges(ctx, 5)
	lastSync, _ := h.Store.GetLastSyncTimestamp(ctx)

	recentChanges := make([]map[string]any, len(latestChanges))
	for i, c := range latestChanges {
		serviceName := ""
		if conn, err := h.Store.GetConnector(ctx, c.ServiceID); err == nil {
			serviceName = conn.Name
		}
		recentChanges[i] = map[string]any{
			"id":            c.ID,
			"serviceId":     c.ServiceID,
			"serviceName":   serviceName,
			"changeType":    c.ChangeType,
			"severity":      c.Severity,
			"summary":       c.Summary,
			"willTriggerAi": false,
			"detectedAt":    c.DetectedAt,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"statusCounts":  statusCounts,
		"pendingAlerts": pendingAlerts,
		"recentChanges": recentChanges,
		"lastSyncAt":    lastSync,
	})
}

// GetLayout handles GET /api/dashboard/layout.
func (h *Handler) GetLayout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	// Query dashboard_layouts table
	var widgets string
	err := h.Store.DB().QueryRowContext(r.Context(),
		`SELECT widgets FROM dashboard_layouts WHERE user_id = ?`, userID,
	).Scan(&widgets)

	if err != nil {
		// Return empty layout if not found
		httputil.JSON(w, http.StatusOK, map[string]any{
			"widgets": json.RawMessage("[]"),
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"widgets": json.RawMessage(widgets),
	})
}

// SaveLayout handles PUT /api/dashboard/layout.
func (h *Handler) SaveLayout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req struct {
		Widgets json.RawMessage `json:"widgets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	widgets := string(req.Widgets)
	if widgets == "" || widgets == "null" {
		widgets = "[]"
	}

	// Upsert layout
	_, err := h.Store.DB().ExecContext(r.Context(), `
		INSERT INTO dashboard_layouts (user_id, widgets) VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET widgets = ?
	`, userID, widgets, widgets)
	if err != nil {
		// SQLite doesn't support ON CONFLICT — use INSERT OR REPLACE
		_, err = h.Store.DB().ExecContext(r.Context(), `
			INSERT OR REPLACE INTO dashboard_layouts (user_id, widgets) VALUES (?, ?)
		`, userID, widgets)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"widgets": json.RawMessage(widgets),
	})
}

// Package connectors provides connector management API handlers.
package connectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for connector endpoints.
type Handler struct {
	Store *store.Store
}

// NewHandler creates a new connector handler.
func NewHandler(s *store.Store) *Handler {
	return &Handler{Store: s}
}

// List handles GET /api/connectors.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	_, pageSize, offset := httputil.Paginate(r)
	category := r.URL.Query().Get("category")

	var connectors []store.ConnectorRecord
	var err error

	if category != "" {
		connectors, _, err = h.Store.ListConnectorsByCategory(r.Context(), category, offset, pageSize)
	} else {
		connectors, _, err = h.Store.ListConnectors(r.Context(), offset, pageSize)
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if connectors == nil {
		connectors = []store.ConnectorRecord{}
	}

	// Spec: GET /connectors returns a bare Connector[] (see openapi.yaml).
	httputil.JSON(w, http.StatusOK, connectors)
}

// Create handles POST /api/connectors.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string         `json:"name"`
		Category  string         `json:"category"`
		Type      string         `json:"type"`
		URL       string         `json:"url"`
		VerifyTLS *bool          `json:"verifyTls"`
		Config    map[string]any `json:"config"`
		Enabled   *bool          `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}
	if req.Name == "" || req.Category == "" || req.Type == "" || req.URL == "" {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "name, category, type, and url are required")
		return
	}

	verifyTLS := true
	if req.VerifyTLS != nil {
		verifyTLS = *req.VerifyTLS
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	configData := "{}"
	if req.Config != nil {
		data, err := json.Marshal(req.Config)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid config JSON")
			return
		}
		configData = string(data)
	}

	c := &store.ConnectorRecord{
		Name:       req.Name,
		Category:   req.Category,
		Type:       req.Type,
		URL:        req.URL,
		VerifyTLS:  verifyTLS,
		ConfigData: configData,
		Enabled:    enabled,
	}

	if err := h.Store.CreateConnector(r.Context(), c); err != nil {
		httputil.Errorf(w, err)
		return
	}

	httputil.JSON(w, http.StatusCreated, c)
}

// Get handles GET /api/connectors/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, c)
}

// Update handles PATCH /api/connectors/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name      *string        `json:"name"`
		URL       *string        `json:"url"`
		VerifyTLS *bool          `json:"verifyTls"`
		Config    map[string]any `json:"config"`
		Enabled   *bool          `json:"enabled"`
		Category  *string        `json:"category"`
		Type      *string        `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.VerifyTLS != nil {
		updates["verify_tls"] = *req.VerifyTLS
	}
	if req.Config != nil {
		data, err := json.Marshal(req.Config)
		if err != nil {
			httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid config JSON")
			return
		}
		updates["config_data"] = string(data)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Category != nil {
		updates["category"] = *req.Category
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}

	if len(updates) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	if err := h.Store.UpdateConnector(r.Context(), id, updates); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	c, _ := h.Store.GetConnector(r.Context(), id)
	httputil.JSON(w, http.StatusOK, c)
}

// Delete handles DELETE /api/connectors/{id}.
// If auth config requires step-up, X-Elevation-Token must be validated.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check for elevation token (step-up auth)
	elevationToken := r.Header.Get("X-Elevation-Token")
	if elevationToken == "" {
		httputil.Error(w, http.StatusBadRequest, "elevation_required", "X-Elevation-Token header required for destructive action")
		return
	}

	// Elevation validation will be done in Phase 6 when sync engine is built
	// For now, accept the token as a placeholder
	_ = elevationToken

	if err := h.Store.DeleteConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	httputil.NoContent(w)
}

// Test handles POST /api/connectors/test.
// Validates a connector's configuration without saving it.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type   string         `json:"type"`
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	c, err := connector.Get(req.Type, req.Config)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Failed to create connector: %v", err))
		return
	}

	if err := c.Validate(r.Context(), req.Config); err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Connection successful",
	})
}

// RemovalImpact handles GET /api/connectors/{id}/removal-impact.
// Returns counts of dependent resources before deletion.
func (h *Handler) RemovalImpact(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	snapshots, _ := h.Store.CountSnapshotsByConnector(r.Context(), id)
	docs, _ := h.Store.CountDocsByConnector(r.Context(), id)
	changes, _ := h.Store.CountChangesByConnector(r.Context(), id)
	alerts, _ := h.Store.CountAlertsByConnector(r.Context(), id)

	httputil.JSON(w, http.StatusOK, map[string]any{
		"snapshots": snapshots,
		"docs":      docs,
		"changes":   changes,
		"alerts":    alerts,
		"hasData":   snapshots+docs+changes+alerts > 0,
	})
}

// Schema handles GET /api/connectors/schema.
func (h *Handler) Schema(w http.ResponseWriter, r *http.Request) {
	typ := r.URL.Query().Get("type")
	if typ != "" {
		s, err := connector.GetTypeSchema(typ)
		if err != nil {
			httputil.Error(w, http.StatusNotFound, "not_found", "Unknown connector type")
			return
		}
		httputil.JSON(w, http.StatusOK, s)
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{
		"schemas": connector.ListSchemas(),
	})
}

// ToggleEnabled handles PUT /api/connectors/{id}/enabled.
func (h *Handler) ToggleEnabled(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if err := h.Store.UpdateConnector(r.Context(), id, map[string]any{
		"enabled": req.Enabled,
	}); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	c, _ := h.Store.GetConnector(r.Context(), id)
	httputil.JSON(w, http.StatusOK, c)
}

// Sync handles POST /api/connectors/{id}/sync.
// Triggers a sync for a single connector. Returns 202 with job info.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	c, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	// Sync will be implemented in Phase 6
	slog.Info("sync triggered for connector", "id", id, "name", c.Name)

	httputil.JSON(w, http.StatusAccepted, map[string]any{
		"message":     "Sync queued",
		"connectorId": id,
	})
}

// SyncAll handles POST /api/sync.
// Triggers a global sync of all enabled connectors.
func (h *Handler) SyncAll(w http.ResponseWriter, _ *http.Request) {
	// Sync will be implemented in Phase 6
	slog.Info("global sync triggered")

	httputil.JSON(w, http.StatusAccepted, map[string]any{
		"message": "Global sync queued",
	})
}

// RequireElevation checks for a valid elevation token for destructive actions.
// This is called by the router middleware before destructive endpoints.
func RequireElevation(jwtSvc *auth.Service, action string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("X-Elevation-Token")
			if token == "" {
				httputil.Error(w, http.StatusBadRequest, "elevation_required",
					fmt.Sprintf("X-Elevation-Token header required for %s", action))
				return
			}
			_, err := jwtSvc.ValidateElevation(token, action)
			if err != nil {
				httputil.Error(w, http.StatusUnauthorized, "unauthorized",
					fmt.Sprintf("Invalid elevation token: %v", err))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Package connectors provides connector management API handlers.
package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
)

// Handler holds dependencies for connector endpoints.
type Handler struct {
	Store      *store.Store
	SyncEngine *sync.Engine
}

// NewHandler creates a new connector handler.
func NewHandler(s *store.Store, e *sync.Engine) *Handler {
	return &Handler{Store: s, SyncEngine: e}
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

// Test handles POST /api/connectors/{id}/test.
// Validates the connector's saved configuration by attempting a connection.
func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	rec, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	cfg, err := store.ParseConnectorConfig(rec.ConfigData)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	cfg["url"] = rec.URL
	cfg["verify_tls"] = rec.VerifyTLS

	c, err := connector.Get(rec.Type, cfg)
	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": fmt.Sprintf("Failed to create connector: %v", err),
		})
		return
	}

	start := time.Now()
	validateErr := c.Validate(r.Context(), cfg)
	latencyMs := int(time.Since(start).Milliseconds())

	if validateErr != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"ok":        false,
			"message":   validateErr.Error(),
			"latencyMs": latencyMs,
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"message":   "Connection successful",
		"latencyMs": latencyMs,
	})
}

// Data handles GET /api/connectors/{id}/data.
// Returns the latest fetched service snapshot for the connector.
func (h *Handler) Data(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.Store.GetConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	sn, err := h.Store.GetLatestSnapshot(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// No snapshot yet — return empty data, not an error
			httputil.JSON(w, http.StatusOK, map[string]any{
				"serviceName": "",
				"type":        "",
				"sections":    []map[string]any{},
				"metadata":    map[string]string{},
				"fetchedAt":   "",
			})
			return
		}
		httputil.Errorf(w, err)
		return
	}

	var snap connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(sn.Data), &snap); err != nil {
		httputil.Errorf(w, err)
		return
	}

	sections := make([]map[string]any, len(snap.Sections))
	for i, sec := range snap.Sections {
		sections[i] = map[string]any{
			"title":   sec.Title,
			"content": sec.Content,
			"order":   i,
		}
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"serviceName": snap.ServiceName,
		"type":        snap.Type,
		"sections":    sections,
		"metadata":    snap.Metadata,
		"fetchedAt":   snap.FetchedAt.Format(time.RFC3339),
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
	docSections, _ := h.Store.CountDocsByConnector(r.Context(), id)

	docs, err := h.Store.ListDocsByService(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	items := make([]map[string]string, 0, len(docs))
	for _, d := range docs {
		items = append(items, map[string]string{"type": "doc", "name": d.Title})
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"trackedServices": 1,
		"docSections":     docSections,
		"snapshots":       snapshots,
		"items":           items,
	})
}

// Schema handles GET /api/connectors/schema.
func (h *Handler) Schema(w http.ResponseWriter, _ *http.Request) {
	httputil.JSON(w, http.StatusOK, connector.ListSchemas())
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
// Triggers a sync for a single connector. Returns 202 with job info; the sync
// itself runs asynchronously and its progress streams over /ws.
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if _, err := h.Store.GetConnector(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
			return
		}
		httputil.Errorf(w, err)
		return
	}

	jobID := uuid.New().String()
	go func() {
		if _, err := h.SyncEngine.RunSync(context.Background(), id, jobID); err != nil {
			slog.Error("sync failed", "connector", id, "job", jobID, "error", err)
		}
	}()

	httputil.JSON(w, http.StatusAccepted, map[string]any{
		"jobId":     jobID,
		"serviceId": id,
	})
}

// SyncAll handles POST /api/sync.
// Triggers a global sync of all enabled connectors. Returns 202 with job info;
// the sync itself runs asynchronously.
func (h *Handler) SyncAll(w http.ResponseWriter, _ *http.Request) {
	jobID := uuid.New().String()
	go func() {
		if _, err := h.SyncEngine.RunSyncAll(context.Background(), jobID); err != nil {
			slog.Error("global sync failed", "job", jobID, "error", err)
		}
	}()

	httputil.JSON(w, http.StatusAccepted, map[string]any{
		"jobId":     jobID,
		"serviceId": nil,
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

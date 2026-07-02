// Package changes provides API handlers for infrastructure change records.
package changes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api/settings"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/sync"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Handler holds dependencies for change endpoints.
type Handler struct {
	Store    *store.Store
	Settings *settings.Handler
	AI       *ai.Registry
	WSHub    *ws.Hub
}

// NewHandler creates a new change handler.
func NewHandler(s *store.Store, settingsH *settings.Handler, aiRegistry *ai.Registry, hub *ws.Hub) *Handler {
	return &Handler{Store: s, Settings: settingsH, AI: aiRegistry, WSHub: hub}
}

// diffToSpec converts the stored []sync.DiffPatch JSON into the spec's Diff{format,hunks} shape.
func diffToSpec(raw string) map[string]any {
	var patches []sync.DiffPatch
	_ = json.Unmarshal([]byte(raw), &patches)

	hunks := make([]map[string]any, 0, len(patches))
	for _, p := range patches {
		var before, after any
		if p.Old != "" {
			before = p.Old
		}
		if p.New != "" {
			after = p.New
		}
		hunks = append(hunks, map[string]any{
			"path":   p.Section,
			"before": before,
			"after":  after,
		})
	}
	return map[string]any{
		"format": "infra",
		"hunks":  hunks,
	}
}

// changeDetail builds the spec's ChangeDetail response for a change record.
func (h *Handler) changeDetail(ctx context.Context, id string) (map[string]any, error) {
	c, err := h.Store.GetChange(ctx, id)
	if err != nil {
		return nil, err
	}

	serviceName := ""
	if conn, err := h.Store.GetConnector(ctx, c.ServiceID); err == nil {
		serviceName = conn.Name
	}

	var affectedDocIDs []string
	_ = json.Unmarshal([]byte(c.AffectedDocIDs), &affectedDocIDs)
	if affectedDocIDs == nil {
		affectedDocIDs = []string{}
	}

	return map[string]any{
		"id":             c.ID,
		"serviceId":      c.ServiceID,
		"serviceName":    serviceName,
		"changeType":     c.ChangeType,
		"severity":       c.Severity,
		"summary":        c.Summary,
		"willTriggerAi":  false,
		"detectedAt":     c.DetectedAt,
		"status":         c.Status,
		"diff":           diffToSpec(c.Diff),
		"affectedDocIds": affectedDocIDs,
	}, nil
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
	detail, err := h.changeDetail(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Change not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

// Acknowledge handles POST /api/changes/{id}/ack.
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
	detail, err := h.changeDetail(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
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
	detail, err := h.changeDetail(r.Context(), id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, detail)
}

// AIUpdate handles POST /api/changes/{id}/ai-update.
// Batched (non-streaming) suggestion: the full result is delivered over the
// doc.ai_suggestion WS event, correlated by the returned requestId.
func (h *Handler) AIUpdate(w http.ResponseWriter, r *http.Request) {
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

	cfg := h.Settings.LoadAIConfig(r.Context())
	if !cfg.Enabled {
		httputil.Error(w, http.StatusConflict, "ai_disabled", "AI module is not enabled")
		return
	}

	provider, err := h.AI.Get(cfg.Provider, map[string]any{
		"apiKey": cfg.APIKey, "model": cfg.Model, "baseUrl": cfg.BaseURL,
	})
	if err != nil {
		httputil.Error(w, http.StatusConflict, "ai_disabled", fmt.Sprintf("AI provider unavailable: %v", err))
		return
	}

	var affectedDocIDs []string
	_ = json.Unmarshal([]byte(c.AffectedDocIDs), &affectedDocIDs)
	docID := ""
	if len(affectedDocIDs) > 0 {
		docID = affectedDocIDs[0]
	}

	userID := auth.UserIDFromContext(r.Context())
	requestID := uuid.New().String()

	go func() {
		content, err := provider.Suggest(context.Background(), &ai.SuggestRequest{
			SystemPrompt: "Summarize this infrastructure change and suggest an updated documentation snippet.",
			UserPrompt:   fmt.Sprintf("Change summary: %s\n\nDiff:\n%s", c.Summary, c.Diff),
		})
		payload := map[string]any{"requestId": requestID}
		if docID != "" {
			payload["docId"] = docID
		}
		if err != nil {
			payload["status"] = "error"
			payload["error"] = err.Error()
		} else {
			payload["status"] = "complete"
			payload["fullContent"] = content
		}
		if h.WSHub != nil {
			h.WSHub.BroadcastToUser(userID, ws.EventDocAISuggestion, payload)
		}
	}()

	httputil.JSON(w, http.StatusAccepted, map[string]any{"requestId": requestID})
}

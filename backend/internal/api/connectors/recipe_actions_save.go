package connectors

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector/custom"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func recipeActionsFor(typ string, cfg map[string]any) (map[string]custom.RecipeAction, error) {
	if typ != "custom" {
		return map[string]custom.RecipeAction{}, nil
	}
	return custom.CanonicalActions(cfg)
}

func (h *Handler) authorizeRecipeActions(
	w http.ResponseWriter, r *http.Request, target, oldType string, oldConfig map[string]any,
	newType string, newConfig map[string]any,
) (custom.ActionDiff, bool) {
	previous, err := recipeActionsFor(oldType, oldConfig)
	if err != nil {
		httputil.Errorf(w, err)
		return custom.ActionDiff{}, false
	}
	next, err := recipeActionsFor(newType, newConfig)
	if err != nil {
		writeConfigRejection(w, err)
		return custom.ActionDiff{}, false
	}
	diff := custom.DiffActions(previous, next)
	if len(diff.Added)+len(diff.Changed)+len(diff.Removed) == 0 {
		return diff, true
	}
	if !auth.InstanceAdminFromContext(r.Context()) {
		httputil.Error(w, http.StatusForbidden, "forbidden", "Changing recipe actions requires an instance admin")
		return diff, false
	}
	if len(next) > 0 {
		if err := auth.ValidateElevationHeaderFor(h.JWT, h.Store, "connector.recipeActions", target, r); err != nil {
			auth.WriteElevationError(w, err)
			return diff, false
		}
	}
	return diff, true
}

func (h *Handler) authorizeUpdatedRecipeActions(w http.ResponseWriter, r *http.Request, id string, updates map[string]any) (custom.ActionDiff, bool) {
	rec, err := h.Store.GetConnector(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Connector not found")
		return custom.ActionDiff{}, false
	}
	if err != nil {
		httputil.Errorf(w, err)
		return custom.ActionDiff{}, false
	}
	typ := rec.Type
	if value, ok := updates["type"].(string); ok {
		typ = value
	}
	oldConfig := map[string]any{}
	if err := json.Unmarshal([]byte(rec.ConfigData), &oldConfig); err != nil {
		httputil.Errorf(w, err)
		return custom.ActionDiff{}, false
	}
	newConfig := oldConfig
	if value, ok := updates["config_data"].(string); ok {
		newConfig = map[string]any{}
		if err := json.Unmarshal([]byte(value), &newConfig); err != nil {
			httputil.Errorf(w, err)
			return custom.ActionDiff{}, false
		}
	}
	return h.authorizeRecipeActions(w, r, id, rec.Type, oldConfig, typ, newConfig)
}

func (h *Handler) recordRecipeActionsAudit(r *http.Request, id string, diff custom.ActionDiff) {
	if len(diff.Added)+len(diff.Changed)+len(diff.Removed) == 0 {
		return
	}
	detail := map[string]any{"added": diff.Added, "changed": diff.Changed, "removed": diff.Removed}
	if err := h.Store.RecordAuditFromContext(r.Context(), "connector.recipe_actions_changed", "connector", id, detail); err != nil {
		slog.Error("failed to record audit", "action", "connector.recipe_actions_changed", "error", err)
	}
}

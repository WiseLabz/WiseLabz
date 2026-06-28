// Package settings provides API handlers for auth, AI, and notification configuration.
package settings

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for settings endpoints.
type Handler struct {
	Store  *store.Store
	Config *config.Config
}

// NewHandler creates a new settings handler.
func NewHandler(s *store.Store, cfg *config.Config) *Handler {
	return &Handler{Store: s, Config: cfg}
}

// GetAuth handles GET /api/settings/auth.
func (h *Handler) GetAuth(w http.ResponseWriter, r *http.Request) {
	var rec struct {
		LocalEnabled         int
		AccessTokenTTL       int
		RefreshTokenTTL      int
		StepUpForDestructive int
	}
	err := h.Store.DB().QueryRowContext(r.Context(), `
		SELECT local_enabled, access_token_ttl, refresh_token_ttl, step_up_for_destructive
		FROM auth_config WHERE id = 1
	`).Scan(&rec.LocalEnabled, &rec.AccessTokenTTL, &rec.RefreshTokenTTL, &rec.StepUpForDestructive)
	if err != nil {
		// Return defaults from config
		httputil.JSON(w, http.StatusOK, map[string]any{
			"localEnabled":         true,
			"accessTokenTTL":       h.Config.Auth.AccessTokenTTLDuration().Seconds(),
			"refreshTokenTTL":      h.Config.Auth.RefreshTokenTTLDuration().Seconds(),
			"stepUpForDestructive": h.Config.Auth.StepUpForDestructive,
			"oidcProviders":        h.Config.Auth.OIDC,
		})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"localEnabled":         rec.LocalEnabled != 0,
		"accessTokenTTL":       rec.AccessTokenTTL,
		"refreshTokenTTL":      rec.RefreshTokenTTL,
		"stepUpForDestructive": rec.StepUpForDestructive != 0,
		"oidcProviders":        h.Config.Auth.OIDC,
	})
}

// UpdateAuth handles PUT /api/settings/auth.
func (h *Handler) UpdateAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LocalEnabled         *bool `json:"localEnabled"`
		AccessTokenTTL       *int  `json:"accessTokenTTL"`
		RefreshTokenTTL      *int  `json:"refreshTokenTTL"`
		StepUpForDestructive *bool `json:"stepUpForDestructive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	var args []any
	var parts []string

	if req.LocalEnabled != nil {
		parts = append(parts, "local_enabled = ?")
		args = append(args, boolToInt(*req.LocalEnabled))
	}
	if req.AccessTokenTTL != nil {
		parts = append(parts, "access_token_ttl = ?")
		args = append(args, *req.AccessTokenTTL)
	}
	if req.RefreshTokenTTL != nil {
		parts = append(parts, "refresh_token_ttl = ?")
		args = append(args, *req.RefreshTokenTTL)
	}
	if req.StepUpForDestructive != nil {
		parts = append(parts, "step_up_for_destructive = ?")
		args = append(args, boolToInt(*req.StepUpForDestructive))
	}

	if len(parts) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	query := "UPDATE auth_config SET " + strings.Join(parts, ", ") + " WHERE id = 1"

	_, err := h.Store.DB().ExecContext(r.Context(), query, args...)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	h.GetAuth(w, r)
}

// GetAI handles GET /api/settings/ai.
func (h *Handler) GetAI(w http.ResponseWriter, r *http.Request) {
	var rec struct {
		Enabled         int
		Provider        *string
		Model           *string
		APIKeyEncrypted string
		BaseURL         *string
		Mode            string
	}
	err := h.Store.DB().QueryRowContext(r.Context(), `
		SELECT enabled, provider, model, api_key_encrypted, base_url, mode
		FROM ai_config WHERE id = 1
	`).Scan(&rec.Enabled, &rec.Provider, &rec.Model, &rec.APIKeyEncrypted, &rec.BaseURL, &rec.Mode)
	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{
			"enabled":   h.Config.AI.Enabled,
			"provider":  h.Config.AI.Provider,
			"model":     h.Config.AI.Model,
			"hasApiKey": h.Config.AI.APIKey != "",
			"baseUrl":   h.Config.AI.BaseURL,
			"mode":      h.Config.AI.Mode,
		})
		return
	}

	prov := ""
	if rec.Provider != nil {
		prov = *rec.Provider
	}
	model := ""
	if rec.Model != nil {
		model = *rec.Model
	}
	baseURL := ""
	if rec.BaseURL != nil {
		baseURL = *rec.BaseURL
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"enabled":   rec.Enabled != 0,
		"provider":  prov,
		"model":     model,
		"hasApiKey": rec.APIKeyEncrypted != "",
		"baseUrl":   baseURL,
		"mode":      rec.Mode,
	})
}

// UpdateAI handles PUT /api/settings/ai.
func (h *Handler) UpdateAI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled  *bool   `json:"enabled"`
		Provider *string `json:"provider"`
		Model    *string `json:"model"`
		APIKey   *string `json:"apiKey"`
		BaseURL  *string `json:"baseUrl"`
		Mode     *string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	var args []any
	var parts []string

	if req.Enabled != nil {
		parts = append(parts, "enabled = ?")
		args = append(args, boolToInt(*req.Enabled))
	}
	if req.Provider != nil {
		parts = append(parts, "provider = ?")
		args = append(args, *req.Provider)
	}
	if req.Model != nil {
		parts = append(parts, "model = ?")
		args = append(args, *req.Model)
	}
	if req.APIKey != nil {
		key := crypto.DeriveKey(h.Config.Auth.Secret)
		encrypted, err := crypto.Encrypt(*req.APIKey, key)
		if err != nil {
			slog.Error("Failed to encrypt API key", "error", err)
			httputil.Error(w, http.StatusInternalServerError, "internal_error", "Failed to encrypt API key")
			return
		}
		parts = append(parts, "api_key_encrypted = ?")
		args = append(args, encrypted)
	}
	if req.BaseURL != nil {
		parts = append(parts, "base_url = ?")
		args = append(args, *req.BaseURL)
	}
	if req.Mode != nil {
		parts = append(parts, "mode = ?")
		args = append(args, *req.Mode)
	}

	if len(parts) == 0 {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	query := "UPDATE ai_config SET " + strings.Join(parts, ", ") + " WHERE id = 1"

	_, err := h.Store.DB().ExecContext(r.Context(), query, args...)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}

	h.GetAI(w, r)
}

// GetDecryptedAPIKey reads the stored encrypted API key and returns it decrypted.
// Returns an empty string if no key is stored or if decryption fails.
func (h *Handler) GetDecryptedAPIKey() string {
	var encrypted string
	err := h.Store.DB().QueryRow(`SELECT api_key_encrypted FROM ai_config WHERE id = 1`).Scan(&encrypted)
	if err != nil || encrypted == "" {
		return ""
	}

	key := crypto.DeriveKey(h.Config.Auth.Secret)
	plaintext, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		slog.Error("Failed to decrypt stored API key", "error", err)
		return ""
	}
	return plaintext
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

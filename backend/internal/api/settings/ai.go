package settings

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/aicfg"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// AIConfigValues is the resolved AI configuration; see aicfg.Values.
type AIConfigValues = aicfg.Values

// LoadAIConfig reads the current AI configuration, decrypting the stored API key.
func (h *Handler) LoadAIConfig(ctx context.Context) AIConfigValues {
	return h.AIConfig.Load(ctx)
}

// GetAIConfig handles GET /api/ai/config.
func (h *Handler) GetAIConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.LoadAIConfig(r.Context())
	httputil.JSON(w, http.StatusOK, map[string]any{
		"enabled":       cfg.Enabled,
		"provider":      cfg.Provider,
		"model":         cfg.Model,
		"baseUrl":       cfg.BaseURL,
		"mode":          cfg.Mode,
		"embedProvider": cfg.EmbedProvider,
		"embedModel":    cfg.EmbedModel,
		"embedBaseUrl":  cfg.EmbedBaseURL,
	})
}

// encryptAIKey encrypts an AI provider API key with the instance encryption
// key. On failure it logs (encryptLogMsg for the encrypt step), writes a 500
// response and returns ok=false.
func (h *Handler) encryptAIKey(w http.ResponseWriter, plaintext, encryptLogMsg string) (string, bool) {
	key, err := crypto.DecodeKey(h.Config.Encryption.Key)
	if err != nil {
		slog.Error("Failed to load encryption key", "error", err)
		httputil.Error(w, http.StatusInternalServerError, "internal_error", "Failed to encrypt API key")
		return "", false
	}
	encrypted, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		slog.Error(encryptLogMsg, "error", err)
		httputil.Error(w, http.StatusInternalServerError, "internal_error", "Failed to encrypt API key")
		return "", false
	}
	return encrypted, true
}

// UpdateAIConfig handles PUT /api/ai/config.
func (h *Handler) UpdateAIConfig(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[struct {
		Enabled       *bool   `json:"enabled"`
		Provider      *string `json:"provider"`
		Model         *string `json:"model"`
		APIKey        *string `json:"apiKey"`
		BaseURL       *string `json:"baseUrl"`
		Mode          *string `json:"mode"`
		EmbedProvider *string `json:"embedProvider"`
		EmbedModel    *string `json:"embedModel"`
		EmbedAPIKey   *string `json:"embedApiKey"`
		EmbedBaseURL  *string `json:"embedBaseUrl"`
	}](w, r)
	if !ok {
		return
	}

	update := store.AIConfigUpdate{
		Enabled: req.Enabled, Provider: req.Provider, Model: req.Model, BaseURL: req.BaseURL, Mode: req.Mode,
		EmbedProvider: req.EmbedProvider, EmbedModel: req.EmbedModel, EmbedBaseURL: req.EmbedBaseURL,
	}
	if req.APIKey != nil {
		encrypted, ok := h.encryptAIKey(w, *req.APIKey, "Failed to encrypt API key")
		if !ok {
			return
		}
		update.APIKeyEncrypted = &encrypted
	}
	if req.EmbedAPIKey != nil {
		encrypted, ok := h.encryptAIKey(w, *req.EmbedAPIKey, "Failed to encrypt embed API key")
		if !ok {
			return
		}
		update.EmbedAPIKeyEncrypted = &encrypted
	}

	changed, err := h.Store.UpdateAIConfig(r.Context(), update)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !changed {
		httputil.Error(w, http.StatusBadRequest, "invalid_request", "No fields to update")
		return
	}

	h.GetAIConfig(w, r)
}

// TestAIConfig handles POST /api/ai/config/test.
func (h *Handler) TestAIConfig(w http.ResponseWriter, r *http.Request) {
	cfg := h.LoadAIConfig(r.Context())
	if !cfg.Enabled || cfg.Provider == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": "AI module is not enabled"})
		return
	}

	provider, err := h.AI.Get(cfg.Provider, map[string]any{
		"apiKey": cfg.APIKey, "model": cfg.Model, "baseUrl": cfg.BaseURL,
	})
	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}

	start := time.Now()
	_, err = provider.Suggest(r.Context(), &ai.SuggestRequest{
		SystemPrompt: "Reply with the single word OK.",
		UserPrompt:   "ping",
		MaxTokens:    8,
	})
	latencyMs := int(time.Since(start).Milliseconds())
	if err != nil {
		httputil.JSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error(), "latencyMs": latencyMs})
		return
	}

	httputil.JSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Connection successful", "latencyMs": latencyMs})
}

// GetDecryptedAPIKey reads the stored encrypted API key and returns it decrypted.
// Returns an empty string if no key is stored or if decryption fails.
func (h *Handler) GetDecryptedAPIKey() string {
	return h.AIConfig.DecryptedAPIKey(context.Background())
}

// GetDecryptedEmbedAPIKey reads the stored encrypted embedding-provider API
// key and returns it decrypted. Returns an empty string if no key is stored
// or if decryption fails.
func (h *Handler) GetDecryptedEmbedAPIKey() string {
	return h.AIConfig.DecryptedEmbedAPIKey(context.Background())
}

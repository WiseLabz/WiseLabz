// Package aicfg resolves the effective AI configuration (DB row falling back
// to static config, with API keys decrypted). It sits below the API layer so
// services such as mcp and chat don't depend on api/settings.
package aicfg

import (
	"context"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/crypto"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Values is the resolved AI configuration (DB row, falling back to
// static config), including the decrypted API key for provider construction.
type Values struct {
	Enabled  bool
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
	Mode     string

	// Embedding backend for "ask your lab" chat retrieval, independent of
	// Provider/Model/APIKey/BaseURL above.
	EmbedProvider string
	EmbedModel    string
	EmbedAPIKey   string
	EmbedBaseURL  string

	// Providers is the ordered fallback chain: the primary provider above
	// (if configured) followed by any rows in ai_config_providers, in
	// priority order. Callers pass this to ai.SuggestWithFallback.
	Providers []ai.ProviderConfig
}

// Loader resolves Values from the store and static config.
type Loader struct {
	Store  *store.Store
	Config *config.Config
}

// New creates a Loader.
func New(s *store.Store, cfg *config.Config) *Loader {
	return &Loader{Store: s, Config: cfg}
}

// Load reads the current AI configuration, decrypting the stored API key.
func (l *Loader) Load(ctx context.Context) Values {
	rec, err := l.Store.GetAIConfigRecord(ctx)
	if err != nil {
		cfg := Values{
			Enabled: l.Config.AI.Enabled, Provider: l.Config.AI.Provider, Model: l.Config.AI.Model,
			BaseURL: l.Config.AI.BaseURL, APIKey: l.Config.AI.APIKey, Mode: l.Config.AI.Mode,
			EmbedProvider: l.Config.AI.EmbedProvider, EmbedModel: l.Config.AI.EmbedModel,
			EmbedAPIKey: l.Config.AI.EmbedAPIKey, EmbedBaseURL: l.Config.AI.EmbedBaseURL,
		}
		cfg.Providers = primaryProviderConfig(cfg)
		return cfg
	}
	embedProvider := rec.EmbedProvider
	if embedProvider == "" {
		embedProvider = l.Config.AI.EmbedProvider
	}
	embedModel := rec.EmbedModel
	if embedModel == "" {
		embedModel = l.Config.AI.EmbedModel
	}
	cfg := Values{
		Enabled: rec.Enabled, Provider: rec.Provider, Model: rec.Model,
		BaseURL: rec.BaseURL, APIKey: l.decryptAPIKey(rec.APIKeyEncrypted), Mode: rec.Mode,
		EmbedProvider: embedProvider, EmbedModel: embedModel,
		EmbedAPIKey: l.decryptEmbedAPIKey(rec.EmbedAPIKeyEncrypted), EmbedBaseURL: rec.EmbedBaseURL,
	}
	cfg.Providers = append(primaryProviderConfig(cfg), l.FallbackProviders(ctx)...)
	return cfg
}

// DecryptedAPIKey returns the stored primary API key, decrypted. Returns an
// empty string if no key is stored or if decryption fails.
func (l *Loader) DecryptedAPIKey(ctx context.Context) string {
	rec, err := l.Store.GetAIConfigRecord(ctx)
	if err != nil {
		return ""
	}
	return l.decryptAPIKey(rec.APIKeyEncrypted)
}

// DecryptedEmbedAPIKey returns the stored embedding-provider API key,
// decrypted. Returns an empty string if no key is stored or if decryption
// fails.
func (l *Loader) DecryptedEmbedAPIKey(ctx context.Context) string {
	rec, err := l.Store.GetAIConfigRecord(ctx)
	if err != nil {
		return ""
	}
	return l.decryptEmbedAPIKey(rec.EmbedAPIKeyEncrypted)
}

func (l *Loader) decryptAPIKey(encrypted string) string {
	if encrypted == "" {
		return ""
	}
	key, err := crypto.DecodeKey(l.Config.Encryption.Key)
	if err != nil {
		slog.Error("Failed to load encryption key", "error", err)
		return ""
	}
	plaintext, _, err := crypto.DecryptFor(crypto.PurposeAI, "api-key", encrypted, key)
	if err != nil {
		// Legacy path: the row may predate WISELABZ_ENCRYPTION_KEY, when the
		// key was derived from the JWT signing secret instead.
		//lint:ignore SA1019 intentional legacy fallback for rows encrypted before WISELABZ_ENCRYPTION_KEY
		legacyKey := crypto.DeriveKey(l.Config.Auth.Secret) //nolint:staticcheck // intentional legacy fallback for rows encrypted before WISELABZ_ENCRYPTION_KEY
		plaintext, err = crypto.Decrypt(encrypted, legacyKey)
		if err != nil {
			slog.Error("Failed to decrypt stored API key", "error", err)
			return ""
		}
		slog.Warn("Decrypted API key using legacy derived key; re-save the AI config to migrate it to WISELABZ_ENCRYPTION_KEY")
	}
	return plaintext
}

func (l *Loader) decryptEmbedAPIKey(encrypted string) string {
	if encrypted == "" {
		return ""
	}
	key, err := crypto.DecodeKey(l.Config.Encryption.Key)
	if err != nil {
		slog.Error("Failed to load encryption key", "error", err)
		return ""
	}
	plaintext, _, err := crypto.DecryptFor(crypto.PurposeAI, "api-key", encrypted, key)
	if err != nil {
		slog.Error("Failed to decrypt stored embed API key", "error", err)
		return ""
	}
	return plaintext
}

// primaryProviderConfig returns cfg's primary provider as a single-entry
// fallback chain, or none if no provider is configured.
func primaryProviderConfig(cfg Values) []ai.ProviderConfig {
	if cfg.Provider == "" {
		return nil
	}
	return []ai.ProviderConfig{{Name: cfg.Provider, Model: cfg.Model, APIKey: cfg.APIKey, BaseURL: cfg.BaseURL}}
}

// FallbackProviders reads the ai_config_providers table (priority 2+),
// decrypting each row's API key.
func (l *Loader) FallbackProviders(ctx context.Context) []ai.ProviderConfig {
	rows, err := l.Store.ListAIFallbackProviders(ctx)
	if err != nil {
		slog.Error("failed to load fallback AI providers", "error", err)
		return nil
	}

	key, keyErr := crypto.DecodeKey(l.Config.Encryption.Key)
	var out []ai.ProviderConfig
	for _, r := range rows {
		apiKey := ""
		if r.APIKeyEncrypted != "" && keyErr == nil {
			if plaintext, _, err := crypto.DecryptFor(crypto.PurposeAI, "api-key", r.APIKeyEncrypted, key); err == nil {
				apiKey = plaintext
			} else {
				slog.Error("failed to decrypt fallback AI provider API key", "error", err)
			}
		}
		out = append(out, ai.ProviderConfig{Name: r.Provider, Model: r.Model, APIKey: apiKey, BaseURL: r.BaseURL})
	}
	return out
}

// Package settings provides API handlers for auth, AI, and notification configuration.
package settings

import (
	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/aicfg"
	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for settings endpoints.
type Handler struct {
	Store  *store.Store
	Config *config.Config
	AI     *ai.Registry

	// AIConfig resolves the effective AI configuration (shared with chat/mcp).
	AIConfig *aicfg.Loader
}

// NewHandler creates a new settings handler.
func NewHandler(s *store.Store, cfg *config.Config, aiRegistry *ai.Registry) *Handler {
	return &Handler{Store: s, Config: cfg, AI: aiRegistry, AIConfig: aicfg.New(s, cfg)}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

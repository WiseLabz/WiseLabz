// Package system provides system-level API handlers (health, version, status).
package system

import (
	"net/http"
	"runtime/debug"

	"github.com/WiseLabz/wiselabz/internal/config"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Handler holds dependencies for system endpoints.
type Handler struct {
	DB     store.DBTX
	Config *config.Config
	Store  *store.Store
}

// NewHandler creates a new system handler.
func NewHandler(db store.DBTX, cfg *config.Config, s *store.Store) *Handler {
	return &Handler{DB: db, Config: cfg, Store: s}
}

// Health responds with the server health status.
// GET /api/health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := h.DB.PingContext(r.Context()); err != nil {
		dbStatus = "down"
	}

	status := "ok"
	if dbStatus != "ok" {
		status = "degraded"
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"healthy": dbStatus == "ok",
		"components": []map[string]any{
			{"name": "database", "status": dbStatus},
		},
	})
}

// Info responds with instance info (version, sync schedule, integrations).
// GET /api/system/info
func (h *Handler) Info(w http.ResponseWriter, _ *http.Request) {
	version := "dev"
	if buildInfo, ok := debug.ReadBuildInfo(); ok && buildInfo.Main.Version != "" {
		version = buildInfo.Main.Version
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"version":      version,
		"syncSchedule": h.Config.Sync.Schedule,
		"integrations": []map[string]any{},
	})
}

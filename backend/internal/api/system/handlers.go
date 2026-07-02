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
}

// NewHandler creates a new system handler.
func NewHandler(db store.DBTX, cfg *config.Config) *Handler {
	return &Handler{DB: db, Config: cfg}
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
		"status": status,
		"components": []map[string]any{
			{"name": "database", "status": dbStatus},
		},
	})
}

// Version responds with build version information.
// GET /api/version
func (h *Handler) Version(w http.ResponseWriter, _ *http.Request) {
	info := map[string]string{}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		info["go_version"] = buildInfo.GoVersion
		info["version"] = buildInfo.Main.Version
		for _, s := range buildInfo.Settings {
			switch s.Key {
			case "vcs.revision":
				info["commit"] = s.Value
			case "vcs.time":
				info["build_time"] = s.Value
			}
		}
	}

	if _, ok := info["version"]; !ok {
		info["version"] = "dev"
	}

	httputil.JSON(w, http.StatusOK, info)
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

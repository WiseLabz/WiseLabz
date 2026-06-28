// Package system provides system-level API handlers (health, version, status).
package system

import (
	"database/sql"
	"net/http"
	"runtime/debug"

	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// Handler holds dependencies for system endpoints.
type Handler struct {
	DB *sql.DB
}

// NewHandler creates a new system handler.
func NewHandler(db *sql.DB) *Handler {
	return &Handler{DB: db}
}

// Health responds with the server health status.
// GET /api/health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	healthy := true
	if err := h.DB.PingContext(r.Context()); err != nil {
		healthy = false
	}

	status := "ok"
	if !healthy {
		status = "degraded"
	}

	httputil.JSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"healthy": healthy,
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

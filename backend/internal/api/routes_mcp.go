package api

import (
	"github.com/go-chi/chi/v5"
)

// mountMCPRoutes mounts the read-only MCP server (issue #277) at /mcp. It is
// mounted in its own authenticated group (see mountAPIRoutes) rather than
// the shared one, so auth.TreatAsSafeMethod can run ahead of AuthMiddleware
// for this route only. By the time a tool handler runs, AuthMiddleware has
// already populated userID and the API-key restriction in the request
// context — see internal/mcp.NewHTTPHandler.
func mountMCPRoutes(r chi.Router, d routerDeps) {
	r.Mount("/mcp", d.mcpH)
}

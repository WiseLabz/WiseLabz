package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// mountDiscoveryRoutes registers the network discovery tree. It must be called
// on an already-instance-admin group. Starting a scan needs step-up when the
// instance has it enabled; reading and cancelling do not.
func mountDiscoveryRoutes(r chi.Router, d routerDeps) {
	r.Get("/discovery/suggestions", d.discoveryH.Suggestions)
	r.With(auth.RequireElevation(d.cfg.JWT, d.cfg.Store, "discovery.scan")).Post("/discovery/scan", d.discoveryH.Start)
	r.Get("/discovery/scan", d.discoveryH.Get)
	r.Delete("/discovery/scan", d.discoveryH.Cancel)
}

// Package topology provides authorized live topology HTTP endpoints.
package topology

import (
	"net/http"
	"slices"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/topology"
)

// Handler serves topology graph and path requests.
type Handler struct{ Store *store.Store }

// Path handles GET /api/topology/path.
func (h *Handler) Path(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	if from == "" || len(from) > 256 || len(to) > 256 {
		httpErr(w, 400, "from and to must be at most 256 characters; from is required")
		return
	}
	all, err := h.Store.ListConnectorIDs(r.Context())
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	allowed, err := h.Store.FilterConnectorIDsByGrant(r.Context(), auth.UserIDFromContext(r.Context()), all, "viewer")
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	restriction := auth.APIKeyRestrictionFromContext(r.Context())
	if len(restriction.ConnectorIDs) > 0 {
		allowed = slices.DeleteFunc(allowed, func(id string) bool { return !slices.Contains(restriction.ConnectorIDs, id) })
	}
	edges, err := h.Store.ListTopologyEdges(r.Context(), allowed)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	var path []topology.Step
	if to != "" {
		path = topology.ShortestPath(edges, from, to, false)
	} else {
		path = topology.FollowDirected(edges, from)
	}
	if path == nil {
		path = []topology.Step{}
	}
	for i := range path {
		c, err := h.Store.GetConnector(r.Context(), path[i].ConnectorID)
		if err == nil {
			path[i].ConnectorName = c.Name
		}
	}
	if to == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{"found": len(path) > 0, "path": path})
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"found": len(path) > 0, "hops": max(0, len(path)-1), "path": path})
}

func httpErr(w http.ResponseWriter, status int, message string) {
	httputil.Error(w, status, "invalid_request", message)
}

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

// Server-side caps on what the endpoints return. A response that hit a cap
// carries "truncated": true. Graph edges are kept in a stable order (edge
// kind, names, ID), so a capped response is deterministic.
const (
	maxGraphNodes = 2000
	maxGraphEdges = 5000
	// maxPathSteps caps the directed walk of GET /api/topology/path without `to`.
	maxPathSteps = 1000
)

// Handler serves topology graph and path requests.
type Handler struct{ Store *store.Store }

// visibleConnectors returns the connector IDs the caller may traverse (viewer
// grants, narrowed by any API-key connector restriction) and every connector's
// name, from a single listing. Hidden connectors never enter the result.
func (h *Handler) visibleConnectors(r *http.Request) (allowed []string, names map[string]string, err error) {
	list, err := h.Store.ListConnectorNames(r.Context())
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, len(list))
	names = make(map[string]string, len(list))
	for i, c := range list {
		ids[i] = c.ID
		names[c.ID] = c.Name
	}
	allowed, err = h.Store.FilterConnectorIDsByGrant(r.Context(), auth.UserIDFromContext(r.Context()), ids, "viewer")
	if err != nil {
		return nil, nil, err
	}
	if restriction := auth.APIKeyRestrictionFromContext(r.Context()); len(restriction.ConnectorIDs) > 0 {
		allowed = slices.DeleteFunc(allowed, func(id string) bool { return !slices.Contains(restriction.ConnectorIDs, id) })
	}
	return allowed, names, nil
}

// Path handles GET /api/topology/path.
func (h *Handler) Path(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, to := strings.TrimSpace(q.Get("from")), strings.TrimSpace(q.Get("to"))
	if from == "" || len(from) > 256 || len(to) > 256 {
		httpErr(w, 400, "from and to must be at most 256 characters; from is required")
		return
	}
	allowed, names, err := h.visibleConnectors(r)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	edges, err := h.Store.ListTopologyEdges(r.Context(), allowed)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	var path []topology.Step
	truncated := false
	if to != "" {
		path = topology.ShortestPath(edges, from, to, false)
	} else {
		path, truncated = topology.FollowDirected(edges, from, maxPathSteps)
	}
	if path == nil {
		path = []topology.Step{}
	}
	if len(path) > 0 {
		members, err := h.activeMembers(r, allowed)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
		for i := range path {
			path[i].ConnectorName = names[path[i].ConnectorID]
			path[i].NodeID = path[i].GraphNodeKey
			connectorServiceKey := path[i].ConnectorID + "\x00service\x00" + path[i].ConnectorID
			isConnectorService := path[i].Kind == "service" && path[i].GraphNodeKey == connectorServiceKey
			if !isConnectorService {
				if member, ok := members[path[i].GraphNodeKey]; ok {
					path[i].NodeID = member.EntityID
				}
			}
		}
	}
	if to == "" {
		httputil.JSON(w, http.StatusOK, map[string]any{"found": len(path) > 0, "truncated": truncated, "path": path})
		return
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"found": len(path) > 0, "hops": max(0, len(path)-1), "truncated": false, "path": path})
}

func httpErr(w http.ResponseWriter, status int, message string) {
	httputil.Error(w, status, "invalid_request", message)
}

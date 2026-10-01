package mcp

import (
	"context"
	"strings"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// topoNode identifies one node of the topology graph.
type topoNode struct {
	ConnectorID string
	Kind        string
	Name        string
	Ref         string
}

func (n topoNode) key() string {
	ref := n.Ref
	if ref == "" {
		ref = n.Name
	}
	return n.ConnectorID + "\x00" + n.Kind + "\x00" + ref
}

func (n topoNode) matches(query string) bool {
	return strings.EqualFold(n.Name, query) || (n.Ref != "" && strings.EqualFold(n.Ref, query))
}

type topoHop struct {
	prev     string // key of the previous node on the path
	edgeKind string
	source   string
}

// topologyStep is one entity on a returned path. EdgeKind/Source describe
// the edge that led here from the previous step (empty for the first step).
type topologyStep struct {
	ConnectorID   string `json:"connectorId"`
	ConnectorName string `json:"connectorName"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	EdgeKind      string `json:"edgeKind,omitempty"`
	EdgeSource    string `json:"edgeSource,omitempty"`
}

// shortestTopologyPath runs a breadth-first search over edges (treated as
// undirected) from every node matching from to the nearest node matching to.
// It only ever sees the edges passed in, so callers that pass grant-filtered
// edges never traverse hidden nodes. It returns the nodes (with the edge
// leading into each) in order, or nil when no path exists.
func shortestTopologyPath(edges []store.TopologyEdge, from, to string) []topologyStep {
	nodes := map[string]topoNode{}
	adj := map[string][]struct {
		to     string
		kind   string
		source string
	}{}
	order := []string{}
	touch := func(n topoNode) string {
		k := n.key()
		if _, ok := nodes[k]; !ok {
			nodes[k] = n
			order = append(order, k)
		}
		return k
	}
	for _, e := range edges {
		a := touch(topoNode{e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef})
		b := touch(topoNode{e.DstConnectorID, e.DstKind, e.DstName, e.DstRef})
		adj[a] = append(adj[a], struct {
			to     string
			kind   string
			source string
		}{b, e.Kind, e.Source})
		adj[b] = append(adj[b], struct {
			to     string
			kind   string
			source string
		}{a, e.Kind, e.Source})
	}

	visited := map[string]topoHop{}
	var queue []string
	for _, k := range order {
		if nodes[k].matches(from) {
			visited[k] = topoHop{}
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if nodes[cur].matches(to) {
			var rev []string
			for k := cur; ; {
				rev = append(rev, k)
				h := visited[k]
				if h.prev == "" {
					break
				}
				k = h.prev
			}
			steps := make([]topologyStep, 0, len(rev))
			for i := len(rev) - 1; i >= 0; i-- {
				n := nodes[rev[i]]
				h := visited[rev[i]]
				steps = append(steps, topologyStep{
					ConnectorID: n.ConnectorID, Kind: n.Kind, Name: n.Name,
					EdgeKind: h.edgeKind, EdgeSource: h.source,
				})
			}
			return steps
		}
		for _, nb := range adj[cur] {
			if _, seen := visited[nb.to]; seen {
				continue
			}
			visited[nb.to] = topoHop{prev: cur, edgeKind: nb.kind, source: nb.source}
			queue = append(queue, nb.to)
		}
	}
	return nil
}

// registerTopologyPath adds the topology_path tool: the shortest path between
// two entities over the persisted topology edges. Edges are loaded only for
// connectors the caller may view (grants plus any API-key connector
// restriction), so entities of other connectors are neither returned nor
// traversed, and "no path" reveals nothing about them.
func registerTopologyPath(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("topology_path",
		mcpsdk.WithDescription("Find the shortest path between two entities (VMs, containers, hosts, networks, services) in the lab topology. Edges come from declared dependencies and heuristic matches (external ID, IP, hostname) recorded at sync time; each step reports how it was linked."),
		mcpsdk.WithString("from", mcpsdk.Required(), mcpsdk.Description("Name or external ID of the starting entity.")),
		mcpsdk.WithString("to", mcpsdk.Required(), mcpsdk.Description("Name or external ID of the destination entity.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		from, err := req.RequireString("from")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		to, err := req.RequireString("to")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		userID := auth.UserIDFromContext(ctx)

		all, err := d.Store.ListConnectorIDs(ctx)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list connectors", err), nil
		}
		allowed, err := d.Store.FilterConnectorIDsByGrant(ctx, userID, all, "viewer")
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("check connector access", err), nil
		}
		edges, err := d.Store.ListTopologyEdges(ctx, allowed)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("load topology", err), nil
		}

		path := shortestTopologyPath(edges, from, to)
		if path == nil {
			return jsonResult(struct {
				Found bool           `json:"found"`
				Path  []topologyStep `json:"path"`
			}{false, []topologyStep{}})
		}

		names := map[string]string{}
		for i := range path {
			id := path[i].ConnectorID
			name, ok := names[id]
			if !ok {
				if c, err := d.Store.GetConnector(ctx, id); err == nil {
					name = c.Name
				}
				names[id] = name
			}
			path[i].ConnectorName = name
		}
		return jsonResult(struct {
			Found bool           `json:"found"`
			Hops  int            `json:"hops"`
			Path  []topologyStep `json:"path"`
		}{true, len(path) - 1, path})
	})
}

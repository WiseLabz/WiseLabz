package mcp

import (
	"context"
	"slices"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/topology"
)

type topologyStep = topology.Step

func shortestTopologyPath(edges []store.TopologyEdge, from, to string) []topologyStep {
	return topology.ShortestPath(edges, from, to, false)
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
		restriction := auth.APIKeyRestrictionFromContext(ctx)
		if len(restriction.ConnectorIDs) > 0 {
			allowed = slices.DeleteFunc(allowed, func(id string) bool { return !slices.Contains(restriction.ConnectorIDs, id) })
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

package mcp

import (
	"context"
	"encoding/json"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// connectorSummary is the MCP-facing projection of a connector: no
// config_data (it may hold encrypted credentials) and no internal sync
// bookkeeping the caller has no use for.
type connectorSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Type     string `json:"type"`
	Status   string `json:"status"`
	Enabled  bool   `json:"enabled"`
	MyRole   string `json:"myRole"`
}

// registerListConnectors adds the list_connectors tool: connectors the
// caller holds at least a viewer grant on, same default-deny scoping as GET
// /api/connectors (Store.ListConnectorsForUser already applies the request's
// API-key ConnectorIDs restriction internally, via apiKeyConnectorFilter).
func registerListConnectors(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("list_connectors",
		mcpsdk.WithDescription("List connectors visible to the caller (viewer role or above), optionally filtered by category."),
		mcpsdk.WithString("category", mcpsdk.Description("Optional connector category filter.")),
		withPagination(),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		userID := auth.UserIDFromContext(ctx)
		category := req.GetString("category", "")
		offset := offsetArg(req)

		rows, total, err := d.Store.ListConnectorsForUser(ctx, userID, category, offset, defaultPageSize)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list connectors", err), nil
		}

		out := make([]connectorSummary, 0, len(rows))
		for _, c := range rows {
			out = append(out, connectorSummary{
				ID: c.ID, Name: c.Name, Category: c.Category, Type: c.Type,
				Status: c.Status, Enabled: c.Enabled, MyRole: c.Role,
			})
		}
		return jsonResult(struct {
			Connectors []connectorSummary `json:"connectors"`
			Total      int                `json:"total"`
			Offset     int                `json:"offset"`
		}{out, total, offset})
	})
}

// jsonResult marshals v into a text tool result, shared by every tool here.
func jsonResult(v any) (*mcpsdk.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return mcpsdk.NewToolResultErrorFromErr("encode result", err), nil
	}
	return mcpsdk.NewToolResultText(string(b)), nil
}

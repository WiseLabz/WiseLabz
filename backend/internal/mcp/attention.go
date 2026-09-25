package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// registerListAttentionItems adds the list_attention_items tool.
// Store.MergedAttentionItems already narrows to connectors the caller holds
// a viewer+ grant on and already applies the request's API-key ConnectorIDs
// restriction internally (apiKeyConnectorFilter), so no extra filtering pass
// is needed here.
func registerListAttentionItems(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("list_attention_items",
		mcpsdk.WithDescription("List the merged attention queue: pending alerts and open quality findings on connectors the caller can view, newest/most-severe first."),
		mcpsdk.WithString("since", mcpsdk.Description("Optional RFC3339 timestamp; only items detected at or after this time are returned.")),
		withPagination(),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		userID := auth.UserIDFromContext(ctx)
		offset := offsetArg(req)

		items, total, err := d.Store.MergedAttentionItems(ctx, userID, req.GetString("since", ""), offset, defaultPageSize)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list attention items", err), nil
		}
		if items == nil {
			items = []store.AttentionItem{}
		}
		return jsonResult(struct {
			Items  []store.AttentionItem `json:"items"`
			Total  int                   `json:"total"`
			Offset int                   `json:"offset"`
		}{items, total, offset})
	})
}

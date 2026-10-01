package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// searchLimit bounds how many hits the search tool returns.
const searchLimit = 20

// registerSearch adds the search tool: keyword full-text search over docs and
// runbooks. Unlike search_docs it needs no AI or embedding backend. Docs are
// narrowed to what the caller may view (grants plus any API-key connector
// restriction) inside Store.SearchContent.
func registerSearch(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("search",
		mcpsdk.WithDescription("Keyword full-text search over lab documentation and runbooks. Works without any AI provider configured. Common stopwords are ignored, the last word matches as a prefix, and a query with no searchable words returns no hits. Docs and runbooks are interleaved by rank; each hit carries a 0-1 relevance score relative to the best hit of its type. Returns hits with a short snippet."),
		mcpsdk.WithString("query", mcpsdk.Required(), mcpsdk.Description("Keywords to search for.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		hits, err := d.Store.SearchContent(ctx, auth.UserIDFromContext(ctx), query, searchLimit)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("search", err), nil
		}
		if hits == nil {
			hits = []store.SearchHit{}
		}
		return jsonResult(struct {
			Hits []store.SearchHit `json:"hits"`
		}{hits})
	})
}

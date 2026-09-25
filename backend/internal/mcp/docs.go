package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/chat"
)

// searchDocsTopN bounds how many ranked sections search_docs returns, same
// value the "ask your lab" chat handler feeds its prompt with.
const searchDocsTopN = 5

// registerSearchDocs adds the search_docs tool: semantic search over doc
// section embeddings, scoped to the caller (chat.Retrieve calls
// store.ListDocSectionEmbeddings(ctx, userID, scopeDocID) under the hood, so
// it only ever searches docs the caller can already read).
func registerSearchDocs(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("search_docs",
		mcpsdk.WithDescription("Semantic search over generated lab documentation. Returns the most relevant doc sections for a question."),
		mcpsdk.WithString("question", mcpsdk.Required(), mcpsdk.Description("Natural-language question to search documentation for.")),
		mcpsdk.WithString("scopeDocId", mcpsdk.Description("Optional doc ID to restrict the search to a single doc's sections.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		question, err := req.RequireString("question")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		scopeDocID := req.GetString("scopeDocId", "")
		userID := auth.UserIDFromContext(ctx)

		cfg := d.Settings.LoadAIConfig(ctx)
		if !cfg.Enabled || cfg.EmbedProvider == "" {
			return mcpsdk.NewToolResultError("AI module is not enabled or has no embedding backend configured"), nil
		}
		embedder, err := d.Embed.Get(cfg.EmbedProvider, map[string]any{
			"apiKey": cfg.EmbedAPIKey, "model": cfg.EmbedModel, "baseUrl": cfg.EmbedBaseURL,
		})
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("embedding backend unavailable", err), nil
		}

		matches, err := chat.Retrieve(ctx, d.Store, embedder, question, userID, scopeDocID, searchDocsTopN)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("search docs", err), nil
		}
		if matches == nil {
			matches = []chat.Match{}
		}
		return jsonResult(struct {
			Matches []chat.Match `json:"matches"`
		}{matches})
	})
}

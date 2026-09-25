// Package mcp exposes a read-only Model Context Protocol server over the
// existing store/RBAC layer, for a local agent on the same host (issue
// #277). It never mounts a mutating tool — see the package-level tools in
// connectors.go, docs.go, findings.go, changes.go and attention.go.
package mcp

import (
	"context"
	"net/http"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/ai"
	"github.com/WiseLabz/wiselabz/internal/api/settings"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// defaultPageSize is the fixed page size every list tool falls back to when
// the caller doesn't request otherwise; pagination is offset-based, mirroring
// the (offset, limit) store signatures these tools wrap.
const defaultPageSize = 30

// Deps carries what the tool handlers need: the store for every read, plus
// the AI settings/embedding registry search_docs uses for retrieval (the
// same wiring internal/api/chat.Handler uses for PostMessage).
type Deps struct {
	Store    *store.Store
	Settings *settings.Handler
	Embed    *ai.EmbedRegistry
}

// newMCPServer builds the MCP server with all five read-only tools
// registered. Split out from NewHTTPHandler so tests can drive it directly
// through an in-process client instead of a real HTTP round trip.
func newMCPServer(d Deps) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer("wiselabz-lab", "1.0.0",
		mcpserver.WithToolCapabilities(false),
	)

	registerListConnectors(s, d)
	registerSearchDocs(s, d)
	registerListFindings(s, d)
	registerListChanges(s, d)
	registerListAttentionItems(s, d)

	return s
}

// NewHTTPHandler builds the MCP server and wraps it in mcp-go's
// StreamableHTTPServer, returning a plain http.Handler ready to mount behind
// the existing AuthMiddleware. The request's context (userID +
// APIKeyRestriction, set by AuthMiddleware) is threaded straight into every
// tool call via WithHTTPContextFunc.
func NewHTTPHandler(d Deps) http.Handler {
	s := newMCPServer(d)

	return mcpserver.NewStreamableHTTPServer(s,
		mcpserver.WithHTTPContextFunc(func(_ context.Context, r *http.Request) context.Context {
			return r.Context()
		}),
	)
}

// offsetArg reads the tool's optional "offset" number argument, defaulting
// to 0 (the first page) for anything missing or negative.
func offsetArg(req mcpsdk.CallToolRequest) int {
	offset := req.GetInt("offset", 0)
	if offset < 0 {
		return 0
	}
	return offset
}

// withPagination adds the shared "offset" input parameter every list tool
// exposes, at the fixed defaultPageSize page size.
func withPagination() mcpsdk.ToolOption {
	return mcpsdk.WithNumber("offset",
		mcpsdk.Description("Zero-based offset into the result set; page size is fixed at 30."),
	)
}

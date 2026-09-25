package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// findingSummary is the MCP-facing projection of a quality finding.
type findingSummary struct {
	ID          string `json:"id"`
	ConnectorID string `json:"connectorId"`
	CheckType   string `json:"checkType"`
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	LastSeenAt  string `json:"lastSeenAt"`
}

// registerListFindings adds the list_findings tool. Store.ListQualityFindings
// itself does no RBAC filtering, so the handler narrows the page down to
// connectors the caller holds at least a viewer grant on afterward — the
// same FilterConnectorIDsByGrant pass GET /api/findings uses, which also
// honors the calling API key's ConnectorIDs restriction (ClampConnectorRole
// reads it internally).
func registerListFindings(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("list_findings",
		mcpsdk.WithDescription("List documentation quality findings, optionally filtered by connector, check type, status, or since a timestamp."),
		mcpsdk.WithString("connectorId", mcpsdk.Description("Optional connector ID filter.")),
		mcpsdk.WithString("checkType", mcpsdk.Description("Optional check type filter.")),
		mcpsdk.WithString("status", mcpsdk.Description("Optional status filter (e.g. open, resolved).")),
		mcpsdk.WithString("since", mcpsdk.Description("Optional RFC3339 timestamp; only findings last seen at or after this time are returned.")),
		withPagination(),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		userID := auth.UserIDFromContext(ctx)
		offset := offsetArg(req)

		findings, total, err := d.Store.ListQualityFindings(ctx,
			req.GetString("connectorId", ""),
			req.GetString("checkType", ""),
			req.GetString("status", ""),
			req.GetString("since", ""),
			offset, defaultPageSize,
		)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list findings", err), nil
		}

		allowed, err := connectorAllowSet(ctx, d.Store, userID, findingConnectorIDs(findings))
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("check connector access", err), nil
		}

		out := make([]findingSummary, 0, len(findings))
		for _, f := range findings {
			if !allowed[f.ConnectorID] {
				continue
			}
			out = append(out, findingSummary{
				ID: f.ID, ConnectorID: f.ConnectorID, CheckType: f.CheckType,
				Severity: f.Severity, Title: f.Title, Description: f.Description,
				Status: f.Status, LastSeenAt: f.LastSeenAt,
			})
		}
		return jsonResult(struct {
			Findings []findingSummary `json:"findings"`
			Total    int              `json:"total"`
			Offset   int              `json:"offset"`
		}{out, total, offset})
	})
}

func findingConnectorIDs(findings []store.QualityFindingRecord) []string {
	ids := make([]string, len(findings))
	for i, f := range findings {
		ids[i] = f.ConnectorID
	}
	return ids
}

// connectorAllowSet resolves ids down to the ones userID holds at least a
// viewer grant on (via Store.FilterConnectorIDsByGrant, which also applies
// the request's API-key ConnectorIDs restriction), as a lookup set.
func connectorAllowSet(ctx context.Context, s *store.Store, userID string, ids []string) (map[string]bool, error) {
	allowed, err := s.FilterConnectorIDsByGrant(ctx, userID, ids, "viewer")
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(allowed))
	for _, id := range allowed {
		set[id] = true
	}
	return set, nil
}

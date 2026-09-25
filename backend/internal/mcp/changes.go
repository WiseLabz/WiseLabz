package mcp

import (
	"context"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// changeSummary is the MCP-facing projection of an infrastructure change.
type changeSummary struct {
	ID         string `json:"id"`
	ServiceID  string `json:"serviceId"`
	ChangeType string `json:"changeType"`
	Severity   string `json:"severity"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	DetectedAt string `json:"detectedAt"`
}

// registerListChanges adds the list_changes tool. Like list_findings,
// Store.ListChanges does no RBAC filtering itself, so the handler narrows
// the page to connectors (services) the caller can view, same as GET
// /api/changes.
func registerListChanges(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("list_changes",
		mcpsdk.WithDescription("List detected infrastructure changes, optionally filtered by service (connector) or severity."),
		mcpsdk.WithString("serviceId", mcpsdk.Description("Optional service (connector) ID filter.")),
		mcpsdk.WithString("severity", mcpsdk.Description("Optional severity filter.")),
		withPagination(),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		userID := auth.UserIDFromContext(ctx)
		offset := offsetArg(req)

		changes, total, err := d.Store.ListChanges(ctx,
			req.GetString("serviceId", ""),
			req.GetString("severity", ""),
			offset, defaultPageSize,
		)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list changes", err), nil
		}

		allowed, err := connectorAllowSet(ctx, d.Store, userID, changeServiceIDs(changes))
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("check connector access", err), nil
		}

		out := make([]changeSummary, 0, len(changes))
		for _, c := range changes {
			if !allowed[c.ServiceID] {
				continue
			}
			out = append(out, changeSummary{
				ID: c.ID, ServiceID: c.ServiceID, ChangeType: c.ChangeType,
				Severity: c.Severity, Summary: c.Summary, Status: c.Status,
				DetectedAt: c.DetectedAt,
			})
		}
		return jsonResult(struct {
			Changes []changeSummary `json:"changes"`
			Total   int             `json:"total"`
			Offset  int             `json:"offset"`
		}{out, total, offset})
	})
}

func changeServiceIDs(changes []store.ChangeRecord) []string {
	ids := make([]string, len(changes))
	for i, c := range changes {
		ids[i] = c.ServiceID
	}
	return ids
}

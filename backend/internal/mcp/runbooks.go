package mcp

import (
	"context"
	"errors"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// runbookSummary is the MCP-facing list projection of a runbook.
type runbookSummary struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	TargetType  string `json:"targetType"`
	TargetValue string `json:"targetValue"`
	StepCount   int    `json:"stepCount"`
}

// runbookStepView is a runbook step as MCP returns it. Steps on connectors
// the caller cannot view are redacted down to position and a generic title:
// no connector, verb or entity reference leaks.
type runbookStepView struct {
	Position      int    `json:"position"`
	Title         string `json:"title"`
	ConnectorID   string `json:"connectorId,omitempty"`
	ConnectorName string `json:"connectorName,omitempty"`
	Verb          string `json:"verb,omitempty"`
	EntityRef     string `json:"entityRef,omitempty"`
	Redacted      bool   `json:"redacted,omitempty"`
}

func runbookStepConnectorIDs(steps []*store.RunbookStepRecord) []string {
	ids := make([]string, 0, len(steps))
	for _, st := range steps {
		ids = append(ids, st.ConnectorID)
	}
	return ids
}

// registerListRunbooks adds list_runbooks. Runbooks themselves are global
// (like GET /api/runbooks); only their connector-bound steps are sensitive,
// and those are redacted in get_runbook.
func registerListRunbooks(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("list_runbooks",
		mcpsdk.WithDescription("List runbooks (operator guidance bound to a change type, alert severity or finding check type)."),
		withPagination(),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		offset := offsetArg(req)
		all, err := d.Store.ListRunbooks(ctx)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list runbooks", err), nil
		}
		total := len(all)
		if offset > total {
			offset = total
		}
		page := all[offset:min(offset+defaultPageSize, total)]

		ids := make([]string, len(page))
		for i, r := range page {
			ids[i] = r.ID
		}
		steps, err := d.Store.ListRunbookSteps(ctx, ids)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list runbook steps", err), nil
		}

		out := make([]runbookSummary, 0, len(page))
		for _, r := range page {
			out = append(out, runbookSummary{
				ID: r.ID, Title: r.Title, TargetType: r.TargetType, TargetValue: r.TargetValue,
				StepCount: len(steps[r.ID]),
			})
		}
		return jsonResult(struct {
			Runbooks []runbookSummary `json:"runbooks"`
			Total    int              `json:"total"`
			Offset   int              `json:"offset"`
		}{out, total, offset})
	})
}

// registerGetRunbook adds get_runbook: the runbook body plus its steps, with
// steps on connectors the caller cannot view (no grant, or outside the API
// key's connector restriction) redacted, and the linked doc id dropped when
// the doc is not viewable.
func registerGetRunbook(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("get_runbook",
		mcpsdk.WithDescription("Get one runbook by ID, including its steps. Steps on connectors you cannot access are redacted."),
		mcpsdk.WithString("id", mcpsdk.Required(), mcpsdk.Description("Runbook ID.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		userID := auth.UserIDFromContext(ctx)

		rb, err := d.Store.GetRunbook(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			return mcpsdk.NewToolResultError("runbook not found"), nil
		}
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("get runbook", err), nil
		}
		steps, err := d.Store.ListRunbookStepsFor(ctx, id)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("list runbook steps", err), nil
		}
		allowed, err := connectorAllowSet(ctx, d.Store, userID, runbookStepConnectorIDs(steps))
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("check connector access", err), nil
		}

		views := make([]runbookStepView, 0, len(steps))
		for _, st := range steps {
			if !allowed[st.ConnectorID] {
				views = append(views, runbookStepView{Position: st.Position, Title: "Restricted step", Redacted: true})
				continue
			}
			name := ""
			if c, err := d.Store.GetConnector(ctx, st.ConnectorID); err == nil {
				name = c.Name
			}
			views = append(views, runbookStepView{
				Position: st.Position, Title: st.Title, ConnectorID: st.ConnectorID,
				ConnectorName: name, Verb: st.Verb, EntityRef: st.EntityRef,
			})
		}

		var docID string
		if rb.DocID != nil {
			if ok, err := docViewable(ctx, d.Store, userID, *rb.DocID); err == nil && ok {
				docID = *rb.DocID
			}
		}

		return jsonResult(struct {
			ID          string            `json:"id"`
			Title       string            `json:"title"`
			Body        string            `json:"body"`
			TargetType  string            `json:"targetType"`
			TargetValue string            `json:"targetValue"`
			DocID       string            `json:"docId,omitempty"`
			Steps       []runbookStepView `json:"steps"`
		}{rb.ID, rb.Title, rb.Body, rb.TargetType, rb.TargetValue, docID, views})
	})
}

// docViewable reports whether the caller may view the doc: a viewer grant on
// its connector (API-key restrictions apply), or instance admin for lab-wide
// docs - the same rule as the docs REST endpoints.
func docViewable(ctx context.Context, s *store.Store, userID, docID string) (bool, error) {
	doc, err := s.GetDoc(ctx, docID)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if doc.ServiceID == "" {
		return doc.Origin == store.DocOriginHuman || auth.InstanceAdminFromContext(ctx), nil
	}
	return s.UserHasConnectorRole(ctx, userID, doc.ServiceID, "viewer")
}

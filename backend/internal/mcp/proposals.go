package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// maxProposalContentBytes caps a proposed doc body.
const maxProposalContentBytes = 256 * 1024

// maxProposalSummaryChars caps the free-text rationale on a proposal.
const maxProposalSummaryChars = 500

// registerProposeDocEdit adds propose_doc_edit, the one MCP tool that writes.
// It only ever stores a pending doc_edit_proposal: the doc itself is never
// modified here - a human doc operator approves or rejects the proposal over
// REST (see internal/api/docs). Because /mcp is mounted behind
// auth.TreatAsSafeMethod (every MCP call is a POST), the read-only API-key
// gate in AuthMiddleware does not protect this tool, so the handler enforces
// key scope and operator access itself.
func registerProposeDocEdit(s *mcpserver.MCPServer, d Deps) {
	tool := mcpsdk.NewTool("propose_doc_edit",
		mcpsdk.WithDescription("Propose a replacement body for a doc. The doc is NOT changed: a doc operator must review and approve the proposal. Requires a full-scope API key and operator access to the doc (instance admin for lab-wide docs; connector-limited keys cannot touch lab-wide docs). A new proposal for the same doc replaces your earlier pending one; at most 25 pending proposals per user."),
		mcpsdk.WithString("docId", mcpsdk.Required(), mcpsdk.Description("ID of the doc to propose an edit for.")),
		mcpsdk.WithString("content", mcpsdk.Required(), mcpsdk.Description("The complete proposed doc body (Markdown).")),
		mcpsdk.WithNumber("baseVersion", mcpsdk.Description("Doc version the proposal was written against. Defaults to the doc's current version; approval fails if the doc has changed since.")),
		mcpsdk.WithString("summary", mcpsdk.Description("Short explanation of what the edit changes and why.")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if auth.APIKeyRestrictionFromContext(ctx).ReadOnly {
			return mcpsdk.NewToolResultError("propose_doc_edit requires a full-scope API key; this key is read-only"), nil
		}
		docID, err := req.RequireString("docId")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		content, err := req.RequireString("content")
		if err != nil {
			return mcpsdk.NewToolResultError(err.Error()), nil
		}
		if content == "" || len(content) > maxProposalContentBytes {
			return mcpsdk.NewToolResultError("content must be non-empty and at most 256 KiB"), nil
		}
		summary := req.GetString("summary", "")
		if r := []rune(summary); len(r) > maxProposalSummaryChars {
			summary = string(r[:maxProposalSummaryChars])
		}
		userID := auth.UserIDFromContext(ctx)

		doc, err := d.Store.GetDoc(ctx, docID)
		if errors.Is(err, store.ErrNotFound) {
			return mcpsdk.NewToolResultError("doc not found"), nil
		}
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("get doc", err), nil
		}

		ok, err := canOperateDoc(ctx, d.Store, userID, doc)
		if err != nil {
			return mcpsdk.NewToolResultErrorFromErr("check access", err), nil
		}
		if !ok {
			// Same message as a missing doc: don't confirm existence.
			return mcpsdk.NewToolResultError("doc not found"), nil
		}

		// A stale base is allowed (approval then reports a conflict), but a
		// base that never existed - below 1 or ahead of the doc - is a client
		// bug and is rejected up front.
		base := req.GetInt("baseVersion", doc.CurrentVersion)
		if base < 1 || base > doc.CurrentVersion {
			return mcpsdk.NewToolResultError(fmt.Sprintf("baseVersion must be between 1 and the doc's current version (%d)", doc.CurrentVersion)), nil
		}
		p := &store.DocEditProposal{
			DocID: docID, BaseVersion: base, Content: content, Summary: summary, AuthorID: userID,
		}
		if err := d.Store.CreateDocEditProposal(ctx, p); err != nil {
			if errors.Is(err, store.ErrProposalLimit) {
				return mcpsdk.NewToolResultError(err.Error()), nil
			}
			return mcpsdk.NewToolResultErrorFromErr("create proposal", err), nil
		}
		if err := d.Store.RecordAuditScopedFromContext(ctx, "doc.edit_proposed", "doc", docID, map[string]any{
			"proposalId": p.ID, "baseVersion": base, "via": "mcp",
		}, []string{doc.ServiceID}); err != nil {
			// Best-effort, like every other audited write: the proposal is
			// already stored and stays reviewable, so a failed audit row is
			// logged rather than turned into a tool error.
			slog.Error("failed to record audit", "action", "doc.edit_proposed", "error", err)
		}

		return jsonResult(struct {
			ProposalID  string `json:"proposalId"`
			DocID       string `json:"docId"`
			BaseVersion int    `json:"baseVersion"`
			Status      string `json:"status"`
		}{p.ID, docID, base, p.Status})
	})
}

// canOperateDoc reports whether the caller holds operator access to doc:
// operator grant on its connector (API-key restrictions apply), or instance
// admin for lab-wide docs - the same rule as saving a doc over REST. A
// connector-restricted API key is never instance admin (see
// auth.InstanceAdminFromContext), so it cannot operate lab-wide docs.
func canOperateDoc(ctx context.Context, s *store.Store, userID string, doc *store.DocRecord) (bool, error) {
	if doc.ServiceID == "" {
		return auth.InstanceAdminFromContext(ctx), nil
	}
	return s.UserHasConnectorRole(ctx, userID, doc.ServiceID, "operator")
}

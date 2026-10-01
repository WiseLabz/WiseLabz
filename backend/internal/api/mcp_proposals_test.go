package api_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// newMCPClient connects a real streamable-HTTP MCP client to app using token.
func newMCPClient(t *testing.T, app *testApp, token string) *mcpclient.Client {
	t.Helper()
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)
	client, err := mcpclient.NewStreamableHttpClient(server.URL+"/api/mcp",
		mcptransport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + token}),
	)
	if err != nil {
		t.Fatalf("new streamable http client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Initialize(context.Background(), mcpsdk.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return client
}

func callMCP(t *testing.T, c *mcpclient.Client, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	res, err := c.CallTool(context.Background(), mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{Name: name, Arguments: args},
	})
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

// TestMCPKnowledgeToolsAdvertised checks the new tools are mounted.
func TestMCPKnowledgeToolsAdvertised(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	userID, _ := app.user(t, "viewer")
	client := newMCPClient(t, app, mintAPIKey(t, app, userID, "full", nil))

	tools, err := client.ListTools(context.Background(), mcpsdk.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	want := map[string]bool{"search": false, "topology_path": false, "list_runbooks": false, "get_runbook": false, "propose_doc_edit": false}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %s not advertised", name)
		}
	}
}

// TestMCPProposeDocEdit covers the single MCP write tool across key scopes:
// full-scope operator (creates a pending proposal, doc untouched, audited),
// read-only key (rejected even though /mcp bypasses the read-only method
// gate), and a full key whose owner is only a viewer (rejected).
func TestMCPProposeDocEdit(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	ctx := context.Background()

	connectorID := "svc-1"
	if err := app.Store.CreateConnector(ctx, &store.ConnectorRecord{
		ID: connectorID, Name: "svc-1", Category: "networking", Type: "test", Enabled: true,
	}); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	if err := app.Store.CreateDoc(ctx, &store.DocRecord{ID: "doc-1", Title: "Doc", Kind: "service", ServiceID: connectorID, Content: "original"}); err != nil {
		t.Fatalf("create doc: %v", err)
	}

	operatorID, _ := app.user(t, "operator")
	app.connectorGrant(t, operatorID, connectorID, "operator")
	viewerID, _ := app.user(t, "viewer")
	app.connectorGrant(t, viewerID, connectorID, "viewer")

	args := map[string]any{"docId": "doc-1", "content": "proposed", "summary": "fix typo"}

	pendingCount := func() int {
		list, err := app.Store.ListDocEditProposals(ctx, store.ProposalPending)
		if err != nil {
			t.Fatalf("list proposals: %v", err)
		}
		return len(list)
	}

	t.Run("read-only key is rejected", func(t *testing.T) {
		client := newMCPClient(t, app, mintAPIKey(t, app, operatorID, "read", nil))
		res := callMCP(t, client, "propose_doc_edit", args)
		if !res.IsError {
			t.Fatalf("expected error result, got %s", mcpsdk.GetTextFromContent(res.Content[0]))
		}
		if n := pendingCount(); n != 0 {
			t.Fatalf("pending proposals = %d, want 0", n)
		}
	})

	t.Run("viewer-owned full key is rejected", func(t *testing.T) {
		client := newMCPClient(t, app, mintAPIKey(t, app, viewerID, "full", nil))
		res := callMCP(t, client, "propose_doc_edit", args)
		if !res.IsError {
			t.Fatalf("expected error result, got %s", mcpsdk.GetTextFromContent(res.Content[0]))
		}
		if n := pendingCount(); n != 0 {
			t.Fatalf("pending proposals = %d, want 0", n)
		}
	})

	t.Run("operator full key creates a pending proposal", func(t *testing.T) {
		client := newMCPClient(t, app, mintAPIKey(t, app, operatorID, "full", nil))
		res := callMCP(t, client, "propose_doc_edit", args)
		if res.IsError {
			t.Fatalf("unexpected error: %s", mcpsdk.GetTextFromContent(res.Content[0]))
		}
		var out struct {
			ProposalID string `json:"proposalId"`
			Status     string `json:"status"`
		}
		if err := json.Unmarshal([]byte(mcpsdk.GetTextFromContent(res.Content[0])), &out); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if out.ProposalID == "" || out.Status != store.ProposalPending {
			t.Fatalf("out = %+v", out)
		}
		doc, _ := app.Store.GetDoc(ctx, "doc-1")
		if doc.Content != "original" || doc.CurrentVersion != 1 {
			t.Fatalf("doc was modified by MCP: %+v", doc)
		}
		p, err := app.Store.GetDocEditProposal(ctx, out.ProposalID)
		if err != nil || p.AuthorID != operatorID || p.BaseVersion != 1 || p.Content != "proposed" {
			t.Fatalf("proposal = %+v, %v", p, err)
		}
		audits, _, err := app.Store.ListAuditRecords(ctx, "doc.edit_proposed", "", "", "", 0, 10)
		if err != nil || len(audits) != 1 || audits[0].TargetID != "doc-1" {
			t.Fatalf("audit = %+v, %v", audits, err)
		}
	})
}

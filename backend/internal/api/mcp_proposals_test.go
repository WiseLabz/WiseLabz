package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
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
		_, total, err := app.Store.ListDocEditProposals(ctx, store.ProposalPending, store.ProposalScope{All: true}, 1, 0)
		if err != nil {
			t.Fatalf("list proposals: %v", err)
		}
		return total
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

func textOf(res *mcpsdk.CallToolResult) string { return mcpsdk.GetTextFromContent(res.Content[0]) }

// TestMCPProposeDocEditScopeAndLimits covers the access edges of
// propose_doc_edit that the basic test does not: connector-limited keys,
// lab-wide docs, baseVersion validation, the content size limit and the
// per-author pending cap.
func TestMCPProposeDocEditScopeAndLimits(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	ctx := context.Background()

	for _, id := range []string{"svc-1", "svc-2"} {
		if err := app.Store.CreateConnector(ctx, &store.ConnectorRecord{
			ID: id, Name: id, Category: "networking", Type: "test", Enabled: true,
		}); err != nil {
			t.Fatalf("create connector: %v", err)
		}
	}
	mkDoc := func(id, svc string) {
		t.Helper()
		if err := app.Store.CreateDoc(ctx, &store.DocRecord{ID: id, Title: id, Kind: "service", ServiceID: svc, Content: "original"}); err != nil {
			t.Fatalf("create doc %s: %v", id, err)
		}
	}
	mkDoc("doc-1", "svc-1")
	mkDoc("lab-1", "")

	// operator: instance admin with an operator grant on svc-1 and svc-2.
	adminID, _ := app.user(t, "operator")
	app.connectorGrant(t, adminID, "svc-1", "operator")
	app.connectorGrant(t, adminID, "svc-2", "operator")
	// plain: not instance admin, operator on svc-1 only.
	plainID, _ := app.user(t, "viewer")
	app.connectorGrant(t, plainID, "svc-1", "operator")

	// mintAPIKey derives the raw token from (user, scope), so keys for the
	// same user with different connector restrictions need distinct tokens.
	mintKey := func(userID string, connectorIDs []string) string {
		t.Helper()
		raw := "wlz_test_" + userID + "_" + strings.Join(connectorIDs, "_")
		if err := app.Store.CreateAPIKey(ctx, &store.APIKey{
			UserID: userID, Name: "scoped-key", TokenHash: store.HashToken(raw),
			Role: "viewer", Scope: "full", ConnectorIDs: connectorIDs,
		}); err != nil {
			t.Fatalf("create key: %v", err)
		}
		return raw
	}
	propose := func(token, docID, content string, extra map[string]any) *mcpsdk.CallToolResult {
		t.Helper()
		args := map[string]any{"docId": docID, "content": content}
		for k, v := range extra {
			args[k] = v
		}
		return callMCP(t, newMCPClient(t, app, token), "propose_doc_edit", args)
	}
	pendingFor := func(docID string) int {
		t.Helper()
		items, _, err := app.Store.ListDocEditProposals(ctx, store.ProposalPending, store.ProposalScope{All: true}, 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, it := range items {
			if it.DocID == docID {
				n++
			}
		}
		return n
	}

	t.Run("connector-limited key excluding the doc's connector is rejected", func(t *testing.T) {
		res := propose(mintKey(adminID, []string{"svc-2"}), "doc-1", "x", nil)
		if !res.IsError || textOf(res) != "doc not found" {
			t.Fatalf("res = %v %s, want 'doc not found'", res.IsError, textOf(res))
		}
		if n := pendingFor("doc-1"); n != 0 {
			t.Fatalf("pending = %d", n)
		}
	})

	t.Run("connector-limited key including the connector succeeds", func(t *testing.T) {
		res := propose(mintKey(plainID, []string{"svc-1"}), "doc-1", "x", nil)
		if res.IsError {
			t.Fatalf("unexpected error: %s", textOf(res))
		}
	})

	t.Run("lab-wide doc: non-admin rejected", func(t *testing.T) {
		res := propose(mintKey(plainID, nil), "lab-1", "x", nil)
		if !res.IsError || textOf(res) != "doc not found" {
			t.Fatalf("res = %v %s", res.IsError, textOf(res))
		}
	})

	t.Run("lab-wide doc: connector-limited key of an instance admin rejected", func(t *testing.T) {
		res := propose(mintKey(adminID, []string{"svc-1", "svc-2"}), "lab-1", "x", nil)
		if !res.IsError {
			t.Fatalf("restricted admin key proposed on a lab-wide doc: %s", textOf(res))
		}
		if n := pendingFor("lab-1"); n != 0 {
			t.Fatalf("pending = %d", n)
		}
	})

	t.Run("lab-wide doc: unrestricted admin key accepted", func(t *testing.T) {
		res := propose(mintKey(adminID, nil), "lab-1", "x", nil)
		if res.IsError {
			t.Fatalf("unexpected error: %s", textOf(res))
		}
	})

	t.Run("baseVersion validation", func(t *testing.T) {
		tok := mintKey(plainID, nil)
		for _, bad := range []int{0, -1, 2, 99} {
			if res := propose(tok, "doc-1", "x", map[string]any{"baseVersion": bad}); !res.IsError {
				t.Errorf("baseVersion %d accepted, want rejected", bad)
			}
		}
		if res := propose(tok, "doc-1", "x", map[string]any{"baseVersion": 1}); res.IsError {
			t.Errorf("baseVersion 1 (current) rejected: %s", textOf(res))
		}
	})

	t.Run("stale baseVersion is stored and fails at approval", func(t *testing.T) {
		mkDoc("doc-stale", "svc-1")
		if err := app.Store.UpdateDoc(ctx, "doc-stale", "changed", nil); err != nil {
			t.Fatal(err)
		}
		d, _ := app.Store.GetDoc(ctx, "doc-stale")
		if d.CurrentVersion < 2 {
			t.Fatalf("doc version = %d, want >= 2", d.CurrentVersion)
		}
		res := propose(mintKey(plainID, nil), "doc-stale", "x", map[string]any{"baseVersion": 1})
		if res.IsError {
			t.Fatalf("stale base rejected: %s", textOf(res))
		}
		var out struct {
			ProposalID string `json:"proposalId"`
		}
		if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
			t.Fatal(err)
		}
		if _, err := app.Store.ApproveDocEditProposal(ctx, out.ProposalID, adminID); !errors.Is(err, store.ErrVersionConflict) {
			t.Fatalf("approve err = %v, want ErrVersionConflict", err)
		}
	})

	t.Run("content size limit is inclusive at 256 KiB", func(t *testing.T) {
		mkDoc("doc-size", "svc-1")
		tok := mintKey(plainID, nil)
		const limit = 256 * 1024
		if res := propose(tok, "doc-size", strings.Repeat("a", limit), nil); res.IsError {
			t.Fatalf("content at limit rejected: %s", textOf(res))
		}
		if res := propose(tok, "doc-size", strings.Repeat("a", limit+1), nil); !res.IsError {
			t.Fatal("content over limit accepted")
		}
		if res := propose(tok, "doc-size", "", nil); !res.IsError {
			t.Fatal("empty content accepted")
		}
		// Multi-byte content is measured in bytes, not characters.
		if res := propose(tok, "doc-size", strings.Repeat("é", limit/2+1), nil); !res.IsError {
			t.Fatal("multi-byte content over the byte limit accepted")
		}
	})

	t.Run("a new proposal replaces the author's older pending one on the same doc", func(t *testing.T) {
		mkDoc("doc-repl", "svc-1")
		tok := mintKey(plainID, nil)
		propose(tok, "doc-repl", "first", nil)
		propose(tok, "doc-repl", "second", nil)
		if n := pendingFor("doc-repl"); n != 1 {
			t.Fatalf("pending = %d, want 1 after replacement", n)
		}
	})

	t.Run("per-author pending cap", func(t *testing.T) {
		capID, _ := app.user(t, "viewer")
		app.connectorGrant(t, capID, "svc-1", "operator")
		tok := mintKey(capID, nil)
		for i := 0; i < store.MaxPendingProposalsPerAuthor; i++ {
			id := fmt.Sprintf("doc-cap-%d", i)
			mkDoc(id, "svc-1")
			if res := propose(tok, id, "x", nil); res.IsError {
				t.Fatalf("proposal %d rejected: %s", i, textOf(res))
			}
		}
		mkDoc("doc-cap-over", "svc-1")
		res := propose(tok, "doc-cap-over", "x", nil)
		if !res.IsError || !strings.Contains(textOf(res), "too many pending") {
			t.Fatalf("over-cap res = %v %s", res.IsError, textOf(res))
		}
		if res := propose(tok, "doc-cap-0", "replacement", nil); res.IsError {
			t.Fatalf("replacing at the cap rejected: %s", textOf(res))
		}
	})
}

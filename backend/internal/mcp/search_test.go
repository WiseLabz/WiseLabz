package mcp

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestSearchTool(t *testing.T) {
	h := newTestHarness(t)
	userID := createUser(t, h.Store)
	connID := createConnector(t, h.Store, "pve", "virtualization")
	otherID := createConnector(t, h.Store, "nas", "networking")
	if _, err := h.Store.UpsertConnectorGrant(context.Background(), userID, connID, "viewer"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*store.DocRecord{
		{ID: "d1", Title: "Proxmox cluster", Kind: "service", ServiceID: connID, Content: "quorum and fencing details"},
		{ID: "d2", Title: "NAS pools", Kind: "service", ServiceID: otherID, Content: "quorum of disks"},
	} {
		if err := h.Store.CreateDoc(context.Background(), d); err != nil {
			t.Fatal(err)
		}
	}

	var out struct {
		Hits []store.SearchHit `json:"hits"`
	}
	// AI is not configured in this harness: search must still work.
	h.callTool(userCtx(userID), t, "search", map[string]any{"query": "quorum"}, &out)
	if len(out.Hits) != 1 || out.Hits[0].ID != "d1" {
		t.Fatalf("hits = %+v, want only d1", out.Hits)
	}

	out.Hits = nil
	h.callTool(restrictedCtx(userID, []string{otherID}), t, "search", map[string]any{"query": "quorum"}, &out)
	if len(out.Hits) != 0 {
		t.Fatalf("restricted key hits = %+v, want none", out.Hits)
	}
}

func searchIDs(ctx context.Context, t *testing.T, h *testHarness, query string) map[string]bool {
	t.Helper()
	var out struct {
		Hits []store.SearchHit `json:"hits"`
	}
	h.callTool(ctx, t, "search", map[string]any{"query": query}, &out)
	ids := map[string]bool{}
	for _, hit := range out.Hits {
		ids[hit.Type+":"+hit.ID] = true
	}
	return ids
}

func TestSearchToolQueryShapes(t *testing.T) {
	h := newTestHarness(t)
	userID := createUser(t, h.Store)
	for _, d := range []*store.DocRecord{
		{ID: "d1", Title: "Gateway", Kind: "lab", Content: "the router and firewall"},
		{ID: "d2", Title: "Café Münchën", Kind: "lab", Content: "naïve 東京 résumé"},
	} {
		if err := h.Store.CreateDoc(context.Background(), d); err != nil {
			t.Fatal(err)
		}
	}
	ctx := adminCtx(userID)

	// Syntax-looking and empty inputs must neither error nor match noise.
	for _, q := range []string{`"`, `*`, `NEAR(`, `OR`, `-`, `( )`, `!!!`, `the`, `the of and`, `"unterminated`, ""} {
		if ids := searchIDs(ctx, t, h, q); len(ids) != 0 {
			t.Errorf("query %q matched %v, want no hits", q, ids)
		}
	}
	if ids := searchIDs(ctx, t, h, `"router" OR * ( )`); !ids["doc:d1"] {
		t.Errorf("syntax-laden query should still match on its words: %v", ids)
	}
	if ids := searchIDs(ctx, t, h, "-gateway"); !ids["doc:d1"] {
		t.Errorf("leading dash is punctuation, not NOT: %v", ids)
	}
	for _, q := range []string{"café", "東京", "naïve", "munchen"} {
		if ids := searchIDs(ctx, t, h, q); !ids["doc:d2"] {
			t.Errorf("non-ASCII query %q: %v", q, ids)
		}
	}
}

func TestSearchToolLabWideVisibility(t *testing.T) {
	h := newTestHarness(t)
	userID := createUser(t, h.Store)
	connID := createConnector(t, h.Store, "pve", "virtualization")
	if _, err := h.Store.UpsertConnectorGrant(context.Background(), userID, connID, "viewer"); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*store.DocRecord{
		{ID: "lab", Title: "Lab overview", Kind: "lab", Content: "datacenter topology overview"},
		{ID: "svc", Title: "PVE", Kind: "service", ServiceID: connID, Content: "datacenter node list"},
	} {
		if err := h.Store.CreateDoc(context.Background(), d); err != nil {
			t.Fatal(err)
		}
	}

	if ids := searchIDs(adminCtx(userID), t, h, "datacenter"); !ids["doc:lab"] || !ids["doc:svc"] {
		t.Errorf("instance admin should see lab-wide and granted docs: %v", ids)
	}
	if ids := searchIDs(userCtx(userID), t, h, "datacenter"); ids["doc:lab"] || !ids["doc:svc"] {
		t.Errorf("non-admin must not see lab-wide docs: %v", ids)
	}
	// A restricted key never sees lab-wide docs, even when its owner is admin.
	if ids := searchIDs(restrictedAdminCtx(userID, []string{connID}), t, h, "datacenter"); ids["doc:lab"] || !ids["doc:svc"] {
		t.Errorf("restricted key leaked a lab-wide doc: %v", ids)
	}
}

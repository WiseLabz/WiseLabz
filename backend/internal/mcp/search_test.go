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

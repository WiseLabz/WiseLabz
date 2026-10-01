package mcp

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

type pathOut struct {
	Found bool           `json:"found"`
	Hops  int            `json:"hops"`
	Path  []topologyStep `json:"path"`
}

func TestTopologyPathTool(t *testing.T) {
	ctx := context.Background()
	h := newTestHarness(t)
	a := createConnector(t, h.Store, "pve", "virtualization")
	b := createConnector(t, h.Store, "docker", "containers_paas")
	c := createConnector(t, h.Store, "opn", "networking")

	// vm-1 (pve) == ct-1 (docker) -> depends on network lan (opn).
	if err := h.Store.ReplaceTopologyEdgesForConnector(ctx, b, []store.TopologyEdge{
		{SrcConnectorID: b, SrcKind: "container", SrcName: "ct-1", DstConnectorID: a, DstKind: "vm", DstName: "vm-1", DstRef: "100", Kind: store.TopologyEdgeSameAs, Source: "IP address"},
		{SrcConnectorID: b, SrcKind: "container", SrcName: "ct-1", DstConnectorID: c, DstKind: "network", DstName: "lan", Kind: store.TopologyEdgeDependency, Source: "network"},
	}); err != nil {
		t.Fatal(err)
	}

	full := createUser(t, h.Store)
	partial := createUser(t, h.Store)
	for _, cid := range []string{a, b, c} {
		if _, err := h.Store.UpsertConnectorGrant(ctx, full, cid, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	for _, cid := range []string{a, c} { // no grant on the intermediate docker connector
		if _, err := h.Store.UpsertConnectorGrant(ctx, partial, cid, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	args := map[string]any{"from": "vm-1", "to": "LAN"}

	t.Run("path found through visible entities", func(t *testing.T) {
		var out pathOut
		h.callTool(userCtx(full), t, "topology_path", args, &out)
		if !out.Found || out.Hops != 2 || len(out.Path) != 3 {
			t.Fatalf("out = %+v, want 3-step path", out)
		}
		if out.Path[0].Name != "vm-1" || out.Path[1].Name != "ct-1" || out.Path[2].Name != "lan" {
			t.Fatalf("path = %+v", out.Path)
		}
		if out.Path[1].EdgeKind != store.TopologyEdgeSameAs || out.Path[1].EdgeSource != "IP address" || out.Path[0].ConnectorName != "pve" {
			t.Fatalf("path details = %+v", out.Path)
		}
	})

	t.Run("hidden intermediate reports no path", func(t *testing.T) {
		var out pathOut
		h.callTool(userCtx(partial), t, "topology_path", args, &out)
		if out.Found || len(out.Path) != 0 {
			t.Fatalf("out = %+v, want no path", out)
		}
	})

	t.Run("connector-limited key", func(t *testing.T) {
		var out pathOut
		h.callTool(restrictedCtx(full, []string{a}), t, "topology_path", args, &out)
		if out.Found {
			t.Fatalf("out = %+v, want no path for key limited to one connector", out)
		}
	})
}

func TestShortestTopologyPathPrefersFewestHops(t *testing.T) {
	edges := []store.TopologyEdge{
		{SrcConnectorID: "c", SrcKind: "x", SrcName: "a", DstConnectorID: "c", DstKind: "x", DstName: "b", Kind: "k"},
		{SrcConnectorID: "c", SrcKind: "x", SrcName: "b", DstConnectorID: "c", DstKind: "x", DstName: "d", Kind: "k"},
		{SrcConnectorID: "c", SrcKind: "x", SrcName: "a", DstConnectorID: "c", DstKind: "x", DstName: "d", Kind: "k"},
	}
	if p := shortestTopologyPath(edges, "a", "d"); len(p) != 2 {
		t.Fatalf("path = %+v, want direct 2-node path", p)
	}
	if p := shortestTopologyPath(edges, "a", "zzz"); p != nil {
		t.Fatalf("path = %+v, want nil", p)
	}
}

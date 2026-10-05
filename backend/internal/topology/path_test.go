package topology

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestFollowDirectedUsesOutgoingEdgesOnly(t *testing.T) {
	edges := []store.TopologyEdge{
		{SrcConnectorID: "c", SrcKind: "dns_record", SrcName: "app.example", DstConnectorID: "c", DstKind: "vm", DstName: "app", Kind: store.TopologyEdgeResolvesTo},
		{SrcConnectorID: "c", SrcKind: "vm", SrcName: "app", DstConnectorID: "c", DstKind: "host", DstName: "node-1", Kind: store.TopologyEdgeRunsOn},
	}
	path := FollowDirected(edges, "app.example")
	if len(path) != 3 || path[0].Name != "app.example" || path[1].Name != "app" || path[2].Name != "node-1" {
		t.Fatalf("FollowDirected() = %+v", path)
	}
	if path := FollowDirected(edges, "node-1"); len(path) != 1 || path[0].Name != "node-1" {
		t.Fatalf("reverse traversal = %+v, want only start node", path)
	}
}

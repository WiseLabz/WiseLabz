package topology

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func edge(src, dst, kind string) store.TopologyEdge {
	return store.TopologyEdge{
		SrcConnectorID: "c", SrcKind: "x", SrcName: src,
		DstConnectorID: "c", DstKind: "x", DstName: dst, Kind: kind, Source: "test", Detail: "d-" + src,
	}
}

func names(steps []Step) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.Name
	}
	return out
}

func TestFollowDirectedUsesOutgoingEdgesOnly(t *testing.T) {
	edges := []store.TopologyEdge{
		{SrcConnectorID: "c", SrcKind: "dns_record", SrcName: "app.example", DstConnectorID: "c", DstKind: "vm", DstName: "app", Kind: store.TopologyEdgeResolvesTo},
		{SrcConnectorID: "c", SrcKind: "vm", SrcName: "app", DstConnectorID: "c", DstKind: "host", DstName: "node-1", Kind: store.TopologyEdgeRunsOn},
	}
	path, truncated := FollowDirected(edges, "app.example", 0)
	if truncated || len(path) != 3 || path[0].Name != "app.example" || path[1].Name != "app" || path[2].Name != "node-1" {
		t.Fatalf("FollowDirected() = %+v truncated=%v", path, truncated)
	}
	if path, _ := FollowDirected(edges, "node-1", 0); len(path) != 1 || path[0].Name != "node-1" {
		t.Fatalf("reverse traversal = %+v, want only start node", path)
	}
}

func TestFollowDirectedCapsSteps(t *testing.T) {
	edges := []store.TopologyEdge{edge("a", "b", "k"), edge("b", "c", "k"), edge("c", "d", "k")}
	path, truncated := FollowDirected(edges, "a", 2)
	if !truncated || len(path) != 2 {
		t.Fatalf("capped walk = %v truncated=%v, want 2 steps truncated", names(path), truncated)
	}
	if path, truncated := FollowDirected(edges, "a", 4); truncated || len(path) != 4 {
		t.Fatalf("walk that exactly fits = %v truncated=%v", names(path), truncated)
	}
}

func TestShortestPathUndirectedTraversesEdgesBothWays(t *testing.T) {
	edges := []store.TopologyEdge{edge("a", "b", "k"), edge("b", "c", "k")}
	p := ShortestPath(edges, "c", "a", false)
	if got := names(p); len(got) != 3 || got[0] != "c" || got[1] != "b" || got[2] != "a" {
		t.Fatalf("undirected path = %v", got)
	}
	if p[1].EdgeKind != "k" || p[1].EdgeSource != "test" || p[1].Detail != "d-b" {
		t.Fatalf("edge details on step = %+v", p[1])
	}
}

func TestShortestPathDirectedFollowsEdgeDirection(t *testing.T) {
	edges := []store.TopologyEdge{edge("a", "b", "k"), edge("b", "c", "k")}
	if got := names(ShortestPath(edges, "a", "c", true)); len(got) != 3 || got[2] != "c" {
		t.Fatalf("forward directed path = %v", got)
	}
	if p := ShortestPath(edges, "c", "a", true); p != nil {
		t.Fatalf("reverse directed path = %v, want none", names(p))
	}
}

func TestShortestPathMatchesRefAndIgnoresCase(t *testing.T) {
	e := edge("a", "b", "k")
	e.DstRef = "ID-9"
	if p := ShortestPath([]store.TopologyEdge{e}, "A", "id-9", true); len(p) != 2 {
		t.Fatalf("path = %v, want match by name/ref case-insensitively", names(p))
	}
}

func TestFollowDirectedStepsNameTheNodeTheyWereReachedFrom(t *testing.T) {
	// A→B, A→C, B→D: BFS order is A,B,C,D, so D's parent is B, not C.
	edges := []store.TopologyEdge{edge("a", "b", "k"), edge("a", "c", "k"), edge("b", "d", "k")}
	path, _ := FollowDirected(edges, "a", 0)
	from := map[string]string{}
	for _, s := range path {
		from[s.Name] = s.FromKey
	}
	keyOf := func(name string) string { return Node{ConnectorID: "c", Kind: "x", Name: name}.key() }
	want := map[string]string{"a": "", "b": keyOf("a"), "c": keyOf("a"), "d": keyOf("b")}
	for name, w := range want {
		if from[name] != w {
			t.Errorf("FromKey(%s) = %q, want %q", name, from[name], w)
		}
	}
}

func TestShortestPathMarksEdgesTraversedAgainstTheirDirection(t *testing.T) {
	edges := []store.TopologyEdge{edge("a", "b", "k"), edge("c", "b", "k")}
	path := ShortestPath(edges, "a", "c", false)
	if len(path) != 3 || path[1].EdgeReversed || !path[2].EdgeReversed {
		t.Fatalf("path = %+v, want a→b forward then b←c reversed", path)
	}
	if path[0].FromKey != "" || path[2].FromKey != path[1].GraphNodeKey {
		t.Fatalf("FromKey chain wrong: %+v", path)
	}
}

func TestFollowDirectedCrossesSameAsEitherWay(t *testing.T) {
	// same_as is stored in one canonical direction; walking from either end
	// must still reach the other.
	edges := []store.TopologyEdge{edge("a", "b", store.TopologyEdgeSameAs)}
	for _, from := range []string{"a", "b"} {
		path, _ := FollowDirected(edges, from, 0)
		if len(path) != 2 {
			t.Fatalf("FollowDirected from %s = %v, want both ends", from, names(path))
		}
	}
}

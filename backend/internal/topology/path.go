// Package topology contains traversal shared by MCP and HTTP topology APIs.
package topology

import (
	"strings"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// Node identifies one topology endpoint.
type Node struct {
	ConnectorID   string `json:"connectorId"`
	ConnectorName string `json:"connectorName,omitempty"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Ref           string `json:"ref,omitempty"`
}

func (n Node) key() string {
	ref := n.Ref
	if ref == "" {
		ref = n.Name
	}
	return n.ConnectorID + "\x00" + n.Kind + "\x00" + ref
}
func (n Node) matches(q string) bool {
	return strings.EqualFold(n.Name, q) || (n.Ref != "" && strings.EqualFold(n.Ref, q))
}

// Step is one node in a traversed path, including the edge leading into it.
type Step struct {
	ConnectorID   string `json:"connectorId"`
	ConnectorName string `json:"connectorName"`
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	EdgeKind      string `json:"edgeKind,omitempty"`
	EdgeSource    string `json:"edgeSource,omitempty"`
	Detail        string `json:"detail,omitempty"`
}

type hop struct{ prev, edgeKind, source, detail string }
type adjacent struct{ to, kind, source, detail string }

// ShortestPath runs breadth-first search from any matching node. When directed
// is false, every edge can be traversed both ways (the MCP contract).
func ShortestPath(edges []store.TopologyEdge, from, to string, directed bool) []Step {
	nodes := map[string]Node{}
	adj := map[string][]adjacent{}
	order := []string{}
	touch := func(n Node) string {
		k := n.key()
		if _, ok := nodes[k]; !ok {
			nodes[k] = n
			order = append(order, k)
		}
		return k
	}
	for _, e := range edges {
		a := touch(Node{ConnectorID: e.SrcConnectorID, Kind: e.SrcKind, Name: e.SrcName, Ref: e.SrcRef})
		b := touch(Node{ConnectorID: e.DstConnectorID, Kind: e.DstKind, Name: e.DstName, Ref: e.DstRef})
		adj[a] = append(adj[a], adjacent{b, e.Kind, e.Source, e.Detail})
		if !directed {
			adj[b] = append(adj[b], adjacent{a, e.Kind, e.Source, e.Detail})
		}
	}
	visited := map[string]hop{}
	var queue []string
	for _, k := range order {
		if nodes[k].matches(from) {
			visited[k] = hop{}
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if nodes[cur].matches(to) {
			var rev []string
			for k := cur; ; {
				rev = append(rev, k)
				h := visited[k]
				if h.prev == "" {
					break
				}
				k = h.prev
			}
			steps := make([]Step, 0, len(rev))
			for i := len(rev) - 1; i >= 0; i-- {
				n := nodes[rev[i]]
				h := visited[rev[i]]
				steps = append(steps, Step{ConnectorID: n.ConnectorID, Kind: n.Kind, Name: n.Name, EdgeKind: h.edgeKind, EdgeSource: h.source, Detail: h.detail})
			}
			return steps
		}
		for _, nb := range adj[cur] {
			if _, ok := visited[nb.to]; ok {
				continue
			}
			visited[nb.to] = hop{cur, nb.kind, nb.source, nb.detail}
			queue = append(queue, nb.to)
		}
	}
	return nil
}

// FollowDirected returns a breadth-first spanning walk of all nodes reachable
// from matching start nodes, following edge direction and visiting each node once.
func FollowDirected(edges []store.TopologyEdge, from string) []Step {
	return directedReachable(edges, from)
}

func directedReachable(edges []store.TopologyEdge, from string) []Step {
	// Use a synthetic destination that cannot match a real node to collect the
	// traversal; the normal search's path reconstruction is not suitable here.
	nodes := map[string]Node{}
	adj := map[string][]adjacent{}
	order := []string{}
	touch := func(n Node) string {
		k := n.key()
		if _, ok := nodes[k]; !ok {
			nodes[k] = n
			order = append(order, k)
		}
		return k
	}
	for _, e := range edges {
		a := touch(Node{ConnectorID: e.SrcConnectorID, Kind: e.SrcKind, Name: e.SrcName, Ref: e.SrcRef})
		b := touch(Node{ConnectorID: e.DstConnectorID, Kind: e.DstKind, Name: e.DstName, Ref: e.DstRef})
		adj[a] = append(adj[a], adjacent{b, e.Kind, e.Source, e.Detail})
	}
	seen := map[string]bool{}
	type item struct {
		k   string
		via adjacent
	}
	var queue []item
	for _, k := range order {
		if nodes[k].matches(from) {
			seen[k] = true
			queue = append(queue, item{k, adjacent{}})
		}
	}
	var out []Step
	for len(queue) > 0 {
		it := queue[0]
		queue = queue[1:]
		n := nodes[it.k]
		out = append(out, Step{ConnectorID: n.ConnectorID, Kind: n.Kind, Name: n.Name, EdgeKind: it.via.kind, EdgeSource: it.via.source, Detail: it.via.detail})
		for _, nb := range adj[it.k] {
			if !seen[nb.to] {
				seen[nb.to] = true
				queue = append(queue, item{nb.to, nb})
			}
		}
	}
	return out
}

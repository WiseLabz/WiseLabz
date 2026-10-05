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
	NodeID        string `json:"nodeId,omitempty"`
	GraphNodeKey  string `json:"-"`
	// FromKey is the graph key of the node this step was reached from (empty
	// for start nodes); EdgeReversed reports that the edge into this step was
	// traversed against its direction. Neither is part of the MCP output.
	FromKey      string `json:"-"`
	EdgeReversed bool   `json:"-"`
	EdgeKind     string `json:"edgeKind,omitempty"`
	EdgeSource   string `json:"edgeSource,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type hop struct {
	prev, edgeKind, source, detail string
	reversed                       bool
}
type adjacent struct {
	to, kind, source, detail string
	reversed                 bool
}

// graph is the adjacency structure both traversals share. order lists node
// keys in first-seen edge order, which keeps results deterministic.
type graph struct {
	nodes map[string]Node
	adj   map[string][]adjacent
	order []string
}

func buildGraph(edges []store.TopologyEdge, directed bool) *graph {
	g := &graph{nodes: map[string]Node{}, adj: map[string][]adjacent{}}
	touch := func(n Node) string {
		k := n.key()
		if _, ok := g.nodes[k]; !ok {
			g.nodes[k] = n
			g.order = append(g.order, k)
		}
		return k
	}
	for _, e := range edges {
		a := touch(Node{ConnectorID: e.SrcConnectorID, Kind: e.SrcKind, Name: e.SrcName, Ref: e.SrcRef})
		b := touch(Node{ConnectorID: e.DstConnectorID, Kind: e.DstKind, Name: e.DstName, Ref: e.DstRef})
		g.adj[a] = append(g.adj[a], adjacent{b, e.Kind, e.Source, e.Detail, false})
		// same_as is stored in one canonical direction but is symmetric, so a
		// directed walk must still cross it either way.
		if !directed || e.Kind == store.TopologyEdgeSameAs {
			g.adj[b] = append(g.adj[b], adjacent{a, e.Kind, e.Source, e.Detail, true})
		}
	}
	return g
}

func (g *graph) step(k string, h hop) Step {
	n := g.nodes[k]
	return Step{
		ConnectorID:  n.ConnectorID,
		Kind:         n.Kind,
		Name:         n.Name,
		GraphNodeKey: k,
		FromKey:      h.prev,
		EdgeReversed: h.reversed,
		EdgeKind:     h.edgeKind,
		EdgeSource:   h.source,
		Detail:       h.detail,
	}
}

// ShortestPath runs breadth-first search from any matching node. When directed
// is false, every edge can be traversed both ways (the MCP contract).
func ShortestPath(edges []store.TopologyEdge, from, to string, directed bool) []Step {
	g := buildGraph(edges, directed)
	visited := map[string]hop{}
	var queue []string
	for _, k := range g.order {
		if g.nodes[k].matches(from) {
			visited[k] = hop{}
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if g.nodes[cur].matches(to) {
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
				steps = append(steps, g.step(rev[i], visited[rev[i]]))
			}
			return steps
		}
		for _, nb := range g.adj[cur] {
			if _, ok := visited[nb.to]; ok {
				continue
			}
			visited[nb.to] = hop{cur, nb.kind, nb.source, nb.detail, nb.reversed}
			queue = append(queue, nb.to)
		}
	}
	return nil
}

// FollowDirected returns a breadth-first spanning walk of all nodes reachable
// from matching start nodes, following edge direction and visiting each node
// once. At most limit steps are returned (limit <= 0 means no cap); truncated
// reports whether reachable nodes were left out.
func FollowDirected(edges []store.TopologyEdge, from string, limit int) (steps []Step, truncated bool) {
	g := buildGraph(edges, true)
	visited := map[string]hop{}
	var queue []string
	for _, k := range g.order {
		if g.nodes[k].matches(from) {
			visited[k] = hop{}
			queue = append(queue, k)
		}
	}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if limit > 0 && len(steps) >= limit {
			return steps, true
		}
		steps = append(steps, g.step(cur, visited[cur]))
		for _, nb := range g.adj[cur] {
			if _, ok := visited[nb.to]; !ok {
				visited[nb.to] = hop{cur, nb.kind, nb.source, nb.detail, nb.reversed}
				queue = append(queue, nb.to)
			}
		}
	}
	return steps, false
}

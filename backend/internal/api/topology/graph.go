// Package topology provides authorized live topology HTTP endpoints.
package topology

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type graphNode struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Kind        string `json:"kind,omitempty"`
	ConnectorID string `json:"connectorId,omitempty"`
	Ref         string `json:"ref,omitempty"`
}
type graphEdge struct {
	ID          string `json:"id"`
	Source      string `json:"source"`
	Target      string `json:"target"`
	Kind        string `json:"kind"`
	SourceLabel string `json:"sourceLabel,omitempty"`
	Detail      string `json:"detail,omitempty"`
}

// graphResponse is the body of GET /api/topology/graph. Truncated is true when
// maxGraphNodes or maxGraphEdges cut the result short.
type graphResponse struct {
	Nodes     []graphNode `json:"nodes"`
	Edges     []graphEdge `json:"edges"`
	Truncated bool        `json:"truncated"`
}
type activeMember struct{ EntityID, ConnectorID, Kind, Ref, Name string }

// Graph returns a graph whose edges and active members are scoped to visible
// connectors. Members of merged identities (entities.merged_into set) are
// ignored; their live members belong to the surviving identity. The result is
// capped at maxGraphNodes nodes and maxGraphEdges edges.
func (h *Handler) Graph(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	connectorFilter := q.Get("connector")
	kindFilter := q.Get("kind")
	includeUnlinked := false
	if raw := q.Get("includeUnlinked"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			httputil.Error(w, 400, "invalid_request", "includeUnlinked must be a boolean")
			return
		}
		includeUnlinked = v
	}
	allowed, names, err := h.visibleConnectors(r)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if connectorFilter != "" {
		// Match by ID or name among the visible connectors only, so a hidden
		// connector is indistinguishable from a nonexistent one.
		matched := ""
		for _, id := range allowed {
			if id == connectorFilter || names[id] == connectorFilter {
				matched = id
				break
			}
		}
		if matched == "" {
			httputil.JSON(w, 200, graphResponse{Nodes: []graphNode{}, Edges: []graphEdge{}})
			return
		}
		allowed = []string{matched}
	}
	edges, err := h.Store.ListTopologyEdges(r.Context(), allowed)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	sortEdgesForCap(edges)
	members, err := h.activeMembers(r, allowed)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	// An identity node is named by its best active visible member, the same
	// rule as the entity page (store.EntityLabel.Better).
	identityNames := map[string]graphNode{}
	bestLabel := map[string]store.EntityLabel{}
	for _, m := range members {
		label := store.EntityLabel{ConnectorID: m.ConnectorID, Kind: m.Kind, Ref: m.Ref, Name: m.Name}
		if label.IsServicePlaceholder() {
			continue
		}
		if kindFilter != "" && m.Kind != kindFilter {
			continue
		}
		if current, ok := bestLabel[m.EntityID]; !ok || label.Better(current) {
			bestLabel[m.EntityID] = label
			identityNames[m.EntityID] = graphNode{ID: m.EntityID, Type: "identity", Name: label.Text(), Kind: m.Kind}
		}
	}
	nodes := map[string]graphNode{}
	linked := map[string]bool{}
	outEdges := make([]graphEdge, 0, len(edges))
	endpoint := func(cid, kind, name, ref string) graphNode {
		if kind != "service" || ref != cid {
			if member, ok := members[memberKey(cid, kind, ref, name)]; ok {
				if n, visible := identityNames[member.EntityID]; visible {
					return n
				}
			}
		}
		id := cid + "\x00" + kind + "\x00" + ref
		if ref == "" {
			id = cid + "\x00" + kind + "\x00" + name
		}
		return graphNode{ID: id, Type: "node", ConnectorID: cid, Kind: kind, Name: name, Ref: ref}
	}
	truncated := false
	seenEdges := map[string]bool{}
	for _, e := range edges {
		if kindFilter != "" && e.SrcKind != kindFilter && e.DstKind != kindFilter {
			continue
		}
		a := endpoint(e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef)
		b := endpoint(e.DstConnectorID, e.DstKind, e.DstName, e.DstRef)
		if a.ID == b.ID {
			// Both ends collapsed into one identity node: not a relation.
			continue
		}
		// Edges owned by different connectors can describe the same relation
		// (and several members of one identity collapse onto one node).
		edgeKey := strings.Join([]string{a.ID, b.ID, e.Kind, e.Source, e.Detail}, "\x00")
		if seenEdges[edgeKey] {
			continue
		}
		newNodes := 0
		if _, ok := nodes[a.ID]; !ok {
			newNodes++
		}
		if _, ok := nodes[b.ID]; !ok {
			newNodes++
		}
		if len(outEdges) >= maxGraphEdges || len(nodes)+newNodes > maxGraphNodes {
			truncated = true
			continue
		}
		seenEdges[edgeKey] = true
		nodes[a.ID] = a
		nodes[b.ID] = b
		linked[a.ID] = true
		linked[b.ID] = true
		outEdges = append(outEdges, graphEdge{ID: e.ID, Source: a.ID, Target: b.ID, Kind: e.Kind, SourceLabel: e.Source, Detail: e.Detail})
	}
	if includeUnlinked {
		unlinked := make([]graphNode, 0, len(identityNames))
		for id, n := range identityNames {
			if !linked[id] {
				unlinked = append(unlinked, n)
			}
		}
		slices.SortFunc(unlinked, func(a, b graphNode) int {
			if c := strings.Compare(a.Name, b.Name); c != 0 {
				return c
			}
			return strings.Compare(a.ID, b.ID)
		})
		for _, n := range unlinked {
			if len(nodes) >= maxGraphNodes {
				truncated = true
				break
			}
			nodes[n.ID] = n
		}
	}
	out := make([]graphNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b graphNode) int {
		if a.Type != b.Type {
			return strings.Compare(a.Type, b.Type)
		}
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		}
		return strings.Compare(a.ID, b.ID)
	})
	slices.SortFunc(outEdges, func(a, b graphEdge) int {
		return strings.Compare(edgeOutputKey(a), edgeOutputKey(b))
	})
	httputil.JSON(w, http.StatusOK, graphResponse{Nodes: out, Edges: outEdges, Truncated: truncated})
}

// edgeKindPriority orders edge kinds for the edge cap: the typed relations
// come first so a large lab cannot push them out behind contains and
// dependency edges.
var edgeKindPriority = map[string]int{
	store.TopologyEdgeProxiesTo: 0, store.TopologyEdgeResolvesTo: 1, store.TopologyEdgeRunsOn: 2,
	store.TopologyEdgeSameAs: 3, store.TopologyEdgeDependency: 4, store.TopologyEdgeContains: 5,
}

func edgeKindRank(kind string) int {
	if r, ok := edgeKindPriority[kind]; ok {
		return r
	}
	return len(edgeKindPriority)
}

// sortEdgesForCap sorts by kind priority and then by endpoint and detail
// values only, never the row ID, so which edges survive the cap does not
// change when the edges are rebuilt.
func sortEdgesForCap(edges []store.TopologyEdge) {
	key := func(e store.TopologyEdge) string {
		return strings.Join([]string{e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
			e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Source, e.Detail}, "\x00")
	}
	slices.SortStableFunc(edges, func(a, b store.TopologyEdge) int {
		if c := edgeKindRank(a.Kind) - edgeKindRank(b.Kind); c != 0 {
			return c
		}
		if c := strings.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return strings.Compare(key(a), key(b))
	})
}

func edgeOutputKey(e graphEdge) string {
	return strings.Join([]string{e.Source, e.Target, e.Kind, e.SourceLabel, e.Detail}, "\x00")
}

func memberKey(connectorID, kind, ref, name string) string {
	if ref == "" {
		ref = name
	}
	return connectorID + "\x00" + kind + "\x00" + ref
}
func (h *Handler) activeMembers(r *http.Request, ids []string) (map[string]activeMember, error) {
	out := map[string]activeMember{}
	if len(ids) == 0 {
		return out, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := h.Store.DB().QueryContext(r.Context(), fmt.Sprintf(`SELECT m.entity_id, m.connector_id, m.kind, m.ref, m.name
		FROM entity_members m JOIN entities e ON e.id = m.entity_id
		WHERE m.gone_at IS NULL AND e.merged_into IS NULL AND m.connector_id IN (%s)
		ORDER BY m.connector_id, m.kind, m.ref`, marks), args...)
	if err != nil {
		return nil, fmt.Errorf("list active topology members: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	for rows.Next() {
		var m activeMember
		if err := rows.Scan(&m.EntityID, &m.ConnectorID, &m.Kind, &m.Ref, &m.Name); err != nil {
			return nil, fmt.Errorf("scan active topology member: %w", err)
		}
		out[memberKey(m.ConnectorID, m.Kind, m.Ref, m.Name)] = m
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active topology members: %w", err)
	}
	return out, nil
}

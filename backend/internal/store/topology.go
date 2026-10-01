package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Topology edge kinds.
const (
	// TopologyEdgeDependency links a connector's service node to something
	// it depends on (host, network, storage, upstream service).
	TopologyEdgeDependency = "dependency"
	// TopologyEdgeSameAs links entities of two connectors that were matched
	// as the same object (external ID, IP address or hostname).
	TopologyEdgeSameAs = "same_as"
	// TopologyEdgeContains links a connector's service node to one of its
	// own entities that participates in a match or dependency.
	TopologyEdgeContains = "contains"
)

// TopologyEdge is one persisted edge of the entity-level topology graph.
// Both endpoints are identified by their owning connector plus kind and
// name (Ref is the connector-local external ID when known). The graph is
// treated as undirected by path queries; Source records how the edge was
// derived so heuristic matches stay explainable.
type TopologyEdge struct {
	ID             string `json:"id"`
	ConnectorID    string `json:"connectorId"` // connector whose sync produced the edge
	SrcConnectorID string `json:"srcConnectorId"`
	SrcKind        string `json:"srcKind"`
	SrcName        string `json:"srcName"`
	SrcRef         string `json:"srcRef"`
	DstConnectorID string `json:"dstConnectorId"`
	DstKind        string `json:"dstKind"`
	DstName        string `json:"dstName"`
	DstRef         string `json:"dstRef"`
	Kind           string `json:"kind"`
	Source         string `json:"source"`
	CreatedAt      string `json:"createdAt"`
}

const topologyEdgeColumns = `id, connector_id, src_connector_id, src_kind, src_name, src_ref,
	dst_connector_id, dst_kind, dst_name, dst_ref, kind, source, created_at`

// ReplaceTopologyEdgesForConnector atomically rebuilds the edges attributable
// to connectorID: everything it produced earlier plus any same_as edge that
// touches it (those are re-derived symmetrically from its latest snapshot, so
// entities that disappeared from either side don't leave stale edges behind).
func (s *Store) ReplaceTopologyEdgesForConnector(ctx context.Context, connectorID string, edges []TopologyEdge) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if _, err := tx.db.ExecContext(ctx, `
			DELETE FROM topology_edges
			WHERE connector_id = ?
			   OR (kind = ? AND (src_connector_id = ? OR dst_connector_id = ?))
		`, connectorID, TopologyEdgeSameAs, connectorID, connectorID); err != nil {
			return fmt.Errorf("delete topology edges: %w", err)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		for i := range edges {
			e := edges[i]
			if e.ID == "" {
				e.ID = uuid.New().String()
			}
			if e.CreatedAt == "" {
				e.CreatedAt = now
			}
			e.ConnectorID = connectorID
			if _, err := tx.db.ExecContext(ctx, `
				INSERT INTO topology_edges (`+topologyEdgeColumns+`)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, e.ID, e.ConnectorID, e.SrcConnectorID, e.SrcKind, e.SrcName, e.SrcRef,
				e.DstConnectorID, e.DstKind, e.DstName, e.DstRef, e.Kind, e.Source, e.CreatedAt); err != nil {
				return fmt.Errorf("insert topology edge: %w", err)
			}
		}
		return nil
	})
}

// ListTopologyEdges returns every edge whose two endpoints both belong to
// one of connectorIDs. Edges touching any other connector are never loaded,
// so callers can pass the caller's allowed set and traverse safely.
func (s *Store) ListTopologyEdges(ctx context.Context, connectorIDs []string) ([]TopologyEdge, error) {
	if len(connectorIDs) == 0 {
		return []TopologyEdge{}, nil
	}
	in := placeholders(len(connectorIDs))
	args := make([]any, 0, 2*len(connectorIDs))
	for range 2 {
		for _, id := range connectorIDs {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+topologyEdgeColumns+` FROM topology_edges
		WHERE src_connector_id IN (`+in+`) AND dst_connector_id IN (`+in+`)
		ORDER BY kind, src_name, dst_name, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list topology edges: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	edges := []TopologyEdge{}
	for rows.Next() {
		var e TopologyEdge
		if err := rows.Scan(&e.ID, &e.ConnectorID, &e.SrcConnectorID, &e.SrcKind, &e.SrcName, &e.SrcRef,
			&e.DstConnectorID, &e.DstKind, &e.DstName, &e.DstRef, &e.Kind, &e.Source, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan topology edge: %w", err)
		}
		edges = append(edges, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topology edges: %w", err)
	}
	return edges, nil
}

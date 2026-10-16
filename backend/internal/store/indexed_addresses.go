package store

import (
	"context"
	"fmt"
)

// IndexedAddress is an entity address and the connector that reported it.
type IndexedAddress struct {
	ConnectorType string
	Kind          string
	IP            string
}

// ListIndexedAddresses lists addresses across the instance without grant scoping.
// Callers must enforce instance-admin access before using this inventory.
func (s *Store) ListIndexedAddresses(ctx context.Context) ([]IndexedAddress, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT c.type, e.kind, e.ip
 FROM entity_index e JOIN connectors c ON c.id = e.connector_id
 WHERE e.ip <> ''`)
	if err != nil {
		return nil, fmt.Errorf("list indexed addresses: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	out := []IndexedAddress{}
	for rows.Next() {
		var address IndexedAddress
		if err := rows.Scan(&address.ConnectorType, &address.Kind, &address.IP); err != nil {
			return nil, fmt.Errorf("scan indexed address: %w", err)
		}
		out = append(out, address)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indexed addresses: %w", err)
	}
	return out, nil
}

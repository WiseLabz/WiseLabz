package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// CertificateEntityID returns the active identity for a connector's certificate,
// or an empty string before identity reconciliation has run or when the
// member's merge chain is dangling or cyclic (ErrNotFound means no link).
func (s *Store) CertificateEntityID(ctx context.Context, connectorID, ref string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT entity_id FROM entity_members
		WHERE connector_id = ? AND kind = 'certificate' AND ref = ? AND gone_at IS NULL`,
		connectorID, ref).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load certificate entity identity: %w", err)
	}
	identity, err := s.ResolveEntityIdentity(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("resolve certificate entity identity: %w", err)
	}
	return identity.ID, nil
}

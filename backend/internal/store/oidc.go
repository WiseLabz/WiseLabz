package store

import (
	"context"
	"fmt"
)

// GetOIDCProviderFlags returns enabled flags for OIDC providers, keyed by
// provider id. Providers with no row are absent from the map — callers
// should default to enabled in that case (matches column default).
func (s *Store) GetOIDCProviderFlags(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT provider_id, enabled FROM oidc_provider_flags`)
	if err != nil {
		return nil, fmt.Errorf("get oidc provider flags: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	out := make(map[string]bool)
	for rows.Next() {
		var id string
		var enabled int
		if err := rows.Scan(&id, &enabled); err != nil {
			return nil, fmt.Errorf("scan oidc provider flag: %w", err)
		}
		out[id] = enabled != 0
	}
	return out, nil
}

// SetOIDCProviderEnabled upserts the enabled flag for an OIDC provider.
func (s *Store) SetOIDCProviderEnabled(ctx context.Context, providerID string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oidc_provider_flags (provider_id, enabled) VALUES (?, ?)
		ON CONFLICT (provider_id) DO UPDATE SET enabled = excluded.enabled
	`, providerID, boolToInt(enabled))
	if err != nil {
		return fmt.Errorf("set oidc provider enabled: %w", err)
	}
	return nil
}

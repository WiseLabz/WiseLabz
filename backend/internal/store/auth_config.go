package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AuthRuntimeSettings is the enforced subset of the auth_config singleton.
type AuthRuntimeSettings struct {
	LocalEnabled         bool
	AccessTokenTTL       int // seconds
	RefreshTokenTTL      int // seconds
	StepUpForDestructive bool
}

// GetAuthRuntimeSettings reads the auth_config singleton. ok is false when the
// row doesn't exist, in which case callers fall back to static config.
func (s *Store) GetAuthRuntimeSettings(ctx context.Context) (settings AuthRuntimeSettings, ok bool, err error) {
	var local, stepUp int
	err = s.db.QueryRowContext(ctx,
		`SELECT local_enabled, access_token_ttl, refresh_token_ttl, step_up_for_destructive FROM auth_config WHERE id = 1`,
	).Scan(&local, &settings.AccessTokenTTL, &settings.RefreshTokenTTL, &stepUp)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, false, nil
	}
	if err != nil {
		return settings, false, fmt.Errorf("get auth settings: %w", err)
	}
	settings.LocalEnabled = local != 0
	settings.StepUpForDestructive = stepUp != 0
	return settings, true, nil
}

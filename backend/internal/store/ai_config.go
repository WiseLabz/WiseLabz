package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// AIConfigRecord is the ai_config singleton row. API keys stay encrypted;
// callers decrypt them.
type AIConfigRecord struct {
	Enabled              bool
	Provider             string
	Model                string
	BaseURL              string
	Mode                 string
	APIKeyEncrypted      string
	EmbedProvider        string
	EmbedModel           string
	EmbedBaseURL         string
	EmbedAPIKeyEncrypted string
}

// GetAIConfigRecord reads the ai_config singleton. It returns an error when
// the row can't be read, in which case callers fall back to static config.
func (s *Store) GetAIConfigRecord(ctx context.Context) (AIConfigRecord, error) {
	var (
		rec                                          AIConfigRecord
		enabled                                      int
		provider, model, baseURL, apiKey             sql.NullString
		embedProvider, embedModel, embedBaseURL, key sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT enabled, provider, model, base_url, mode, api_key_encrypted,
		       embed_provider, embed_model, embed_base_url, embed_api_key_encrypted
		FROM ai_config WHERE id = 1
	`).Scan(&enabled, &provider, &model, &baseURL, &rec.Mode, &apiKey,
		&embedProvider, &embedModel, &embedBaseURL, &key)
	if err != nil {
		return AIConfigRecord{}, fmt.Errorf("get ai config: %w", err)
	}
	rec.Enabled = enabled != 0
	rec.Provider, rec.Model, rec.BaseURL = provider.String, model.String, baseURL.String
	rec.APIKeyEncrypted = apiKey.String
	rec.EmbedProvider, rec.EmbedModel, rec.EmbedBaseURL = embedProvider.String, embedModel.String, embedBaseURL.String
	rec.EmbedAPIKeyEncrypted = key.String
	return rec, nil
}

// AIConfigUpdate is a partial update of the ai_config singleton; nil fields
// are left untouched. API keys must already be encrypted.
type AIConfigUpdate struct {
	Enabled              *bool
	Provider             *string
	Model                *string
	APIKeyEncrypted      *string
	BaseURL              *string
	Mode                 *string
	EmbedProvider        *string
	EmbedModel           *string
	EmbedAPIKeyEncrypted *string
	EmbedBaseURL         *string
}

// UpdateAIConfig applies the non-nil fields of u to the ai_config singleton.
// It reports false (and writes nothing) when u has no fields set.
func (s *Store) UpdateAIConfig(ctx context.Context, u AIConfigUpdate) (bool, error) {
	var parts []string
	var args []any
	set := func(col string, v any) {
		parts = append(parts, col+" = ?")
		args = append(args, v)
	}
	if u.Enabled != nil {
		enabled := 0
		if *u.Enabled {
			enabled = 1
		}
		set("enabled", enabled)
	}
	for _, f := range []struct {
		col string
		val *string
	}{
		{"provider", u.Provider}, {"model", u.Model}, {"api_key_encrypted", u.APIKeyEncrypted},
		{"base_url", u.BaseURL}, {"mode", u.Mode}, {"embed_provider", u.EmbedProvider},
		{"embed_model", u.EmbedModel}, {"embed_api_key_encrypted", u.EmbedAPIKeyEncrypted},
		{"embed_base_url", u.EmbedBaseURL},
	} {
		if f.val != nil {
			set(f.col, *f.val)
		}
	}
	if len(parts) == 0 {
		return false, nil
	}
	query := "UPDATE ai_config SET " + strings.Join(parts, ", ") + " WHERE id = 1"
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return false, fmt.Errorf("update ai config: %w", err)
	}
	return true, nil
}

// AIFallbackProviderRecord is one ai_config_providers row (priority 2+).
type AIFallbackProviderRecord struct {
	Provider        string
	Model           string
	APIKeyEncrypted string
	BaseURL         string
}

// ListAIFallbackProviders returns the fallback chain in priority order.
func (s *Store) ListAIFallbackProviders(ctx context.Context) ([]AIFallbackProviderRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT provider, model, api_key_encrypted, base_url
		FROM ai_config_providers WHERE config_id = 1 ORDER BY priority ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list ai fallback providers: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []AIFallbackProviderRecord
	for rows.Next() {
		var r AIFallbackProviderRecord
		if err := rows.Scan(&r.Provider, &r.Model, &r.APIKeyEncrypted, &r.BaseURL); err != nil {
			return nil, fmt.Errorf("scan ai fallback provider: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplaceAIFallbackProviders atomically replaces the whole fallback chain;
// entry i is stored at priority i+2 (priority 1 is the primary provider).
func (s *Store) ReplaceAIFallbackProviders(ctx context.Context, providers []AIFallbackProviderRecord) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if _, err := tx.db.ExecContext(ctx, `DELETE FROM ai_config_providers WHERE config_id = 1`); err != nil {
			return err
		}
		for i, p := range providers {
			_, err := tx.db.ExecContext(ctx, `
				INSERT INTO ai_config_providers (id, config_id, priority, provider, model, api_key_encrypted, base_url)
				VALUES (?, 1, ?, ?, ?, ?, ?)
			`, uuid.New().String(), i+2, p.Provider, p.Model, p.APIKeyEncrypted, p.BaseURL)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

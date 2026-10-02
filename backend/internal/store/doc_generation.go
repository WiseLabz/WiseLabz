package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Doc origins: sync merges generated blocks of "generated" docs and never
// touches "human" ones.
const (
	DocOriginGenerated = "generated"
	DocOriginHuman     = "human"
)

// ApplyGeneratedRender writes a sync/regeneration result: content, the
// rendered block keys and last_synced_at, plus the matching doc_versions row,
// in one transaction. With a non-nil expectedVersion a concurrent save makes
// it return ErrVersionConflict instead of overwriting.
func (s *Store) ApplyGeneratedRender(ctx context.Context, id, content, genKeys string, expectedVersion *int, author, trigger string) (int, error) {
	var rev int
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		r, err := tx.updateDocRev(ctx, id, content, expectedVersion)
		if err != nil {
			return err
		}
		rev = r
		if err := tx.CreateDocVersion(ctx, &DocVersionRecord{
			DocID: id, Rev: r, Content: content, Author: author, Trigger: trigger,
		}); err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.db.ExecContext(ctx, `UPDATE docs SET gen_keys = ?, last_synced_at = ? WHERE id = ?`, genKeys, now, id)
		if err != nil {
			return fmt.Errorf("set doc gen keys: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return rev, nil
}

// DocGeneration is the provenance state SetDocGeneration writes.
type DocGeneration struct {
	Origin     string
	TemplateID string  // "" stores NULL
	GenKeys    *string // nil stores NULL
	Synced     bool    // stamp last_synced_at with now
}

// SetDocGeneration overwrites a doc's origin, template and gen_keys without
// touching content or version.
func (s *Store) SetDocGeneration(ctx context.Context, id string, g DocGeneration) error {
	query := `UPDATE docs SET origin = ?, template_id = ?, gen_keys = ?`
	args := []any{g.Origin, nilToStr(g.TemplateID), g.GenKeys}
	if g.Synced {
		query += `, last_synced_at = ?`
		args = append(args, time.Now().UTC().Format(time.RFC3339))
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("set doc generation: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDocOrigin changes only a doc's origin.
func (s *Store) SetDocOrigin(ctx context.Context, id, origin string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE docs SET origin = ? WHERE id = ?`, origin, id)
	if err != nil {
		return fmt.Errorf("set doc origin: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchDocSynced stamps last_synced_at without changing content, so a sync
// with nothing new doesn't bump updated_at or write a version.
func (s *Store) TouchDocSynced(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE docs SET last_synced_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("touch doc synced: %w", err)
	}
	return nil
}

// GetOpenChangeByPattern returns the newest still-open ("new") change with
// patternID, or ErrNotFound.
func (s *Store) GetOpenChangeByPattern(ctx context.Context, patternID string) (*ChangeRecord, error) {
	c, err := scanChange(s.db.QueryRowContext(ctx, `
		SELECT `+changeColumns+` FROM changes
		WHERE pattern_id = ? AND status = 'new'
		ORDER BY detected_at DESC LIMIT 1
	`, patternID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get open change by pattern: %w", err)
	}
	return &c, nil
}

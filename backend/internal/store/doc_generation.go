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

// GeneratedRender is one ApplyGeneratedRender write.
type GeneratedRender struct {
	Content         string
	GenKeys         string // JSON array of the render's block keys
	ExpectedVersion *int   // non-nil: fail with ErrVersionConflict on a concurrent save
	Author          string // "" for system writes
	Trigger         string
	Origin          string  // "" leaves origin unchanged
	TemplateID      *string // nil leaves template_id unchanged; "" clears it
}

// ApplyGeneratedRender writes a sync/regeneration result: content, the
// rendered block keys and last_synced_at (plus origin/template when set),
// and the matching doc_versions row, in one transaction.
func (s *Store) ApplyGeneratedRender(ctx context.Context, id string, r GeneratedRender) (int, error) {
	var rev int
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		v, err := tx.updateDocRev(ctx, id, r.Content, r.ExpectedVersion)
		if err != nil {
			return err
		}
		rev = v
		if err := tx.CreateDocVersion(ctx, &DocVersionRecord{
			DocID: id, Rev: v, Content: r.Content, Author: r.Author, Trigger: r.Trigger,
		}); err != nil {
			return err
		}
		query := `UPDATE docs SET gen_keys = ?, last_synced_at = ?`
		args := []any{r.GenKeys, time.Now().UTC().Format(time.RFC3339)}
		if r.Origin != "" {
			query += `, origin = ?`
			args = append(args, r.Origin)
		}
		if r.TemplateID != nil {
			query += `, template_id = ?`
			args = append(args, nilToStr(*r.TemplateID))
		}
		if _, err := tx.db.ExecContext(ctx, query+` WHERE id = ?`, append(args, id)...); err != nil {
			return fmt.Errorf("set doc generation: %w", err)
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

// TouchDocSynced stamps last_synced_at and records the render's block keys
// without changing content, so a sync with nothing new doesn't bump
// updated_at or write a version.
func (s *Store) TouchDocSynced(ctx context.Context, id, genKeys string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE docs SET last_synced_at = ?, gen_keys = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), genKeys, id)
	if err != nil {
		return fmt.Errorf("touch doc synced: %w", err)
	}
	return nil
}

// GetLatestChangeByPattern returns the most recently detected change with
// patternID, whatever its status, or ErrNotFound.
func (s *Store) GetLatestChangeByPattern(ctx context.Context, patternID string) (*ChangeRecord, error) {
	c, err := scanChange(s.db.QueryRowContext(ctx, `
		SELECT `+changeColumns+` FROM changes
		WHERE pattern_id = ?
		ORDER BY detected_at DESC, id DESC LIMIT 1
	`, patternID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest change by pattern: %w", err)
	}
	return &c, nil
}

// SetDocTopologyFingerprint records the edge-set hash the doc was rendered
// from. With content, it also replaces the stored content in place, without
// a new version, only while the doc is still at expectedVersion (so a
// concurrent edit is never overwritten); a lost race returns ErrVersionConflict.
func (s *Store) SetDocTopologyFingerprint(ctx context.Context, id, fingerprint string, content *string, expectedVersion int) error {
	return s.docTransaction(ctx, func(tx *Store) error {
		return tx.setDocTopologyFingerprint(ctx, id, fingerprint, content, expectedVersion)
	})
}

func (s *Store) setDocTopologyFingerprint(ctx context.Context, id, fingerprint string, content *string, expectedVersion int) error {
	query := `UPDATE docs SET topology_fingerprint = ?`
	args := []any{fingerprint}
	if content != nil {
		query += `, content = ?`
		args = append(args, *content)
	}
	query += ` WHERE id = ? AND deleted_at IS NULL`
	args = append(args, id)
	if content != nil {
		query += ` AND current_version = ?`
		args = append(args, expectedVersion)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("set doc topology fingerprint: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		if _, getErr := s.GetDoc(ctx, id); getErr == nil {
			return ErrVersionConflict
		}
		return ErrNotFound
	}
	if content != nil {
		return s.syncDocLinks(ctx, id, *content)
	}
	return nil
}

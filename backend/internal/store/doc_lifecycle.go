package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrDocHierarchy indicates an invalid scope, cycle or depth in a doc tree.
var ErrDocHierarchy = errors.New("invalid doc hierarchy")

// lockDocHierarchy serializes validation and writes. The harmless UPDATE takes
// SQLite's writer lock before any reads; Postgres uses a table lock so root
// creation and moves cannot independently validate incompatible trees.
func (s *Store) lockDocHierarchy(ctx context.Context) error {
	switch s.db.(type) {
	case pgTransactionDB:
		_, err := s.db.ExecContext(ctx, `LOCK TABLE docs IN SHARE ROW EXCLUSIVE MODE`)
		return err
	default:
		_, err := s.db.ExecContext(ctx, `UPDATE docs SET parent_id = parent_id WHERE 1 = 0`)
		return err
	}
}

func (s *Store) validateDocParent(ctx context.Context, d *DocRecord, parentID string) error {
	depth := 1
	seen := map[string]bool{d.ID: true}
	for id := parentID; id != ""; {
		if seen[id] {
			return fmt.Errorf("%w: cycle", ErrDocHierarchy)
		}
		seen[id] = true
		parent, err := s.GetDoc(ctx, id)
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("%w: parent not found", ErrDocHierarchy)
		}
		if err != nil {
			return err
		}
		if parent.ServiceID != d.ServiceID {
			return fmt.Errorf("%w: parent must have the same scope", ErrDocHierarchy)
		}
		if d.ServiceID == "" && d.Origin == DocOriginHuman && parent.Origin != DocOriginHuman {
			return fmt.Errorf("%w: human lab docs require a human parent", ErrDocHierarchy)
		}
		depth++
		if depth > 5 {
			return fmt.Errorf("%w: maximum depth is five", ErrDocHierarchy)
		}
		id = parent.ParentID
	}
	var height int
	err := s.db.QueryRowContext(ctx, `WITH RECURSIVE subtree(id, depth) AS (
  SELECT id, 1 FROM docs WHERE id = ? AND deleted_at IS NULL
  UNION ALL SELECT d.id, t.depth + 1 FROM docs d JOIN subtree t ON d.parent_id = t.id
  WHERE d.deleted_at IS NULL AND t.depth < 6
 ) SELECT COALESCE(MAX(depth), 1) FROM subtree`, d.ID).Scan(&height)
	if err != nil {
		return fmt.Errorf("doc subtree height: %w", err)
	}
	if depth+height-1 > 5 {
		return fmt.Errorf("%w: subtree exceeds maximum depth five", ErrDocHierarchy)
	}
	return nil
}

// CreateHumanDoc creates the first revision atomically with its metadata.
func (s *Store) CreateHumanDoc(ctx context.Context, d *DocRecord) error {
	d.Origin = DocOriginHuman
	d.Kind = "lab"
	if d.ServiceID != "" {
		d.Kind = "service"
	}
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.lockDocHierarchy(ctx); err != nil {
			return fmt.Errorf("lock doc hierarchy: %w", err)
		}
		if err := tx.validateDocParent(ctx, d, d.ParentID); err != nil {
			return err
		}
		if err := tx.CreateDoc(ctx, d); err != nil {
			return err
		}
		return tx.CreateDocVersion(ctx, &DocVersionRecord{DocID: d.ID, Rev: 1, Content: d.Content, Author: d.CreatedBy, Trigger: "create"})
	})
}

// UpdateDocMetadata atomically renames or re-parents an active doc.
func (s *Store) UpdateDocMetadata(ctx context.Context, id string, title, parentID *string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.lockDocHierarchy(ctx); err != nil {
			return fmt.Errorf("lock doc hierarchy: %w", err)
		}
		d, err := tx.GetDoc(ctx, id)
		if err != nil {
			return err
		}
		if title != nil {
			d.Title = *title
		}
		if parentID != nil {
			if err := tx.validateDocParent(ctx, d, *parentID); err != nil {
				return err
			}
			d.ParentID = *parentID
		}
		_, err = tx.db.ExecContext(ctx, `UPDATE docs SET title = ?, parent_id = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, d.Title, nilToStr(d.ParentID), time.Now().UTC().Format(time.RFC3339), id)
		if err != nil {
			return fmt.Errorf("update doc metadata: %w", err)
		}
		return nil
	})
}

// SoftDeleteDoc marks the active subtree with a shared deletion timestamp.
func (s *Store) SoftDeleteDoc(ctx context.Context, id string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.lockDocHierarchy(ctx); err != nil {
			return fmt.Errorf("lock doc hierarchy: %w", err)
		}
		if _, err := tx.GetDoc(ctx, id); err != nil {
			return err
		}
		_, err := tx.db.ExecContext(ctx, `WITH RECURSIVE subtree(id) AS (
   SELECT id FROM docs WHERE id = ? AND deleted_at IS NULL
   UNION ALL SELECT d.id FROM docs d JOIN subtree t ON d.parent_id = t.id WHERE d.deleted_at IS NULL
  ) UPDATE docs SET deleted_at = ? WHERE id IN (SELECT id FROM subtree)`, id, time.Now().UTC().Format(time.RFC3339Nano))
		if err != nil {
			return fmt.Errorf("soft delete docs: %w", err)
		}
		return nil
	})
}

// GetDeletedDoc reads a trash record for administrator recovery.
func (s *Store) GetDeletedDoc(ctx context.Context, id string) (*DocRecord, error) {
	d, err := scanDoc(s.db.QueryRowContext(ctx, `SELECT `+docColumns+` FROM docs WHERE id = ? AND deleted_at IS NOT NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get deleted doc: %w", err)
	}
	return &d, nil
}

// ListDeletedDocs lists trash including content and deletion metadata.
func (s *Store) ListDeletedDocs(ctx context.Context) ([]DocRecord, error) {
	return scanAll(ctx, s.db, "docs", `SELECT `+docColumns+` FROM docs WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC, id`, nil, scanDoc)
}

// ListBackupDocs includes trash; ordinary list helpers deliberately exclude it.
func (s *Store) ListBackupDocs(ctx context.Context, offset, limit int) ([]DocRecord, int, error) {
	return paginatedQuery(ctx, s.db, "docs", docColumns, "", nil, "id", limit, offset, scanDoc)
}

// RestoreDeletedDoc recovers the selected subtree from its deletion batch.
func (s *Store) RestoreDeletedDoc(ctx context.Context, id string) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.lockDocHierarchy(ctx); err != nil {
			return fmt.Errorf("lock doc hierarchy: %w", err)
		}
		d, err := tx.GetDeletedDoc(ctx, id)
		if err != nil {
			return err
		}
		if d.ParentID != "" {
			_, err := tx.GetDoc(ctx, d.ParentID)
			if errors.Is(err, ErrNotFound) {
				if _, err := tx.db.ExecContext(ctx, `UPDATE docs SET parent_id = NULL WHERE id = ?`, id); err != nil {
					return fmt.Errorf("detach restored root: %w", err)
				}
			} else if err != nil {
				return err
			}
		}
		_, err = tx.db.ExecContext(ctx, `WITH RECURSIVE batch(id) AS (
   SELECT id FROM docs WHERE id = ? AND deleted_at = ?
   UNION ALL SELECT d.id FROM docs d JOIN batch b ON d.parent_id = b.id WHERE d.deleted_at = ?
  ) UPDATE docs SET deleted_at = NULL WHERE id IN (SELECT id FROM batch)`, id, d.DeletedAt, d.DeletedAt)
		if err != nil {
			return fmt.Errorf("restore deleted docs: %w", err)
		}
		restored, err := tx.GetDoc(ctx, id)
		if err != nil {
			return err
		}
		return tx.validateDocParent(ctx, restored, restored.ParentID)
	})
}

// PurgeDeletedDocs permanently removes trash older than cutoff.
func (s *Store) PurgeDeletedDocs(ctx context.Context, cutoff string) (int64, error) {
	n, err := s.batchDelete(ctx, "docs", `t.deleted_at IS NOT NULL AND t.deleted_at < ?`, cutoff)
	if err != nil {
		return n, fmt.Errorf("purge deleted docs: %w", err)
	}
	return n, nil
}

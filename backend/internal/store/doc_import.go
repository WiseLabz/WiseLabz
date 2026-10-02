package store

import (
	"context"
	"fmt"
)

// ImportedTitle returns title, or the first " (imported)" variant that taken
// reports free.
func ImportedTitle(title string, taken func(string) (bool, error)) (string, error) {
	candidate := title
	for i := 1; ; i++ {
		used, err := taken(candidate)
		if err != nil || !used {
			return candidate, err
		}
		candidate = title + " (imported)"
		if i > 1 {
			candidate = fmt.Sprintf("%s (imported %d)", title, i)
		}
	}
}

// SiblingTitleTaken reports whether an active doc in the same scope and parent has title.
func (s *Store) SiblingTitleTaken(ctx context.Context, serviceID, parentID, title string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs WHERE deleted_at IS NULL
  AND COALESCE(service_id, '') = ? AND COALESCE(parent_id, '') = ? AND title = ?`, serviceID, parentID, title).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("check sibling title: %w", err)
	}
	return n > 0, nil
}

// ImportDocs creates human docs (parents first) and their attachment rows in
// one transaction. A title already used by a sibling gets an " (imported)"
// suffix; docs are updated in place with their final title.
func (s *Store) ImportDocs(ctx context.Context, docs []DocRecord, attachments []DocAttachment) error {
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.lockDocHierarchy(ctx); err != nil {
			return fmt.Errorf("lock doc hierarchy: %w", err)
		}
		for i := range docs {
			d := &docs[i]
			d.Origin, d.Kind = DocOriginHuman, "lab"
			if d.ServiceID != "" {
				d.Kind = "service"
			}
			if err := tx.validateDocParent(ctx, d, d.ParentID); err != nil {
				return err
			}
			title, err := ImportedTitle(d.Title, func(t string) (bool, error) {
				return tx.SiblingTitleTaken(ctx, d.ServiceID, d.ParentID, t)
			})
			if err != nil {
				return err
			}
			d.Title = title
			if err := tx.CreateDoc(ctx, d); err != nil {
				return err
			}
			if err := tx.CreateDocVersion(ctx, &DocVersionRecord{DocID: d.ID, Rev: 1, Content: d.Content, Author: d.CreatedBy, Trigger: "import"}); err != nil {
				return err
			}
		}
		for i := range attachments {
			if err := tx.CreateDocAttachment(ctx, &attachments[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

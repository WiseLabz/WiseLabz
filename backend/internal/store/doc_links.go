package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/doclink"
)

// docTransaction reuses a writer's transaction when an import or version write
// already owns it. Standalone writes atomically update content and its index.
func (s *Store) docTransaction(ctx context.Context, fn func(*Store) error) error {
	switch s.db.(type) {
	case transactionDB, pgTransactionDB:
		return fn(s)
	default:
		return s.WithinTransaction(ctx, fn)
	}
}

func (s *Store) syncDocLinks(ctx context.Context, id, content string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM doc_links WHERE source_doc_id = ?`, id); err != nil {
		return fmt.Errorf("delete doc links: %w", err)
	}
	for _, target := range doclink.Extract(content) {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO doc_links (source_doc_id, target_type, target_id) VALUES (?, ?, ?)`,
			id, target.Type, target.ID); err != nil {
			return fmt.Errorf("insert doc link: %w", err)
		}
	}
	return nil
}

// BackfillDocLinks reindexes existing bodies without rewriting them. Running it
// again has the same result. It runs before startup begins serving requests.
func (s *Store) BackfillDocLinks(ctx context.Context) error {
	return s.docTransaction(ctx, func(tx *Store) error {
		query := `SELECT id, content FROM docs ORDER BY id`
		// Other HA instances may already serve saves during this startup.
		// Lock before reading so the backfill cannot replace a newer index.
		if tx.driver == "postgres" {
			query += ` FOR UPDATE`
		}
		rows, err := tx.db.QueryContext(ctx, query)
		if err != nil {
			return fmt.Errorf("read docs for link backfill: %w", err)
		}
		type body struct{ id, content string }
		docs := []body{}
		for rows.Next() {
			var d body
			if err := rows.Scan(&d.id, &d.content); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan doc for link backfill: %w", err)
			}
			docs = append(docs, d)
		}
		err = rows.Err()
		closeErr := rows.Close()
		if err != nil {
			return fmt.Errorf("iterate doc link backfill: %w", err)
		}
		if closeErr != nil {
			return fmt.Errorf("close doc link backfill: %w", closeErr)
		}
		for _, d := range docs {
			if err := tx.syncDocLinks(ctx, d.id, d.content); err != nil {
				return err
			}
		}
		return nil
	})
}

// LookupDocLink applies the save caller's visibility before resolving titles or
// connector-local references. A doc title takes precedence over an entity name.
// A qualified kind:ref target tries the entity first and falls back to a doc
// whose whole title contains the colon, such as "Runbook: Restore DB".
func (s *Store) LookupDocLink(ctx context.Context, userID, target string) ([]doclink.Target, error) {
	kind, ref, qualified := strings.Cut(target, ":")
	if !qualified {
		out, err := s.lookupDocLinkTitle(ctx, userID, target)
		if err != nil || len(out) > 0 {
			return out, err
		}
		return s.lookupDocLinkEntity(ctx, userID, target, "", "", false)
	}
	out, err := s.lookupDocLinkEntity(ctx, userID, target, kind, ref, true)
	if err != nil || len(out) > 0 {
		return out, err
	}
	return s.lookupDocLinkTitle(ctx, userID, target)
}

func (s *Store) lookupDocLinkTitle(ctx context.Context, userID, title string) ([]doclink.Target, error) {
	where, args := viewableDocWhere(ctx, userID, "")
	args = append(args, title)
	rows, err := s.db.QueryContext(ctx, `SELECT id, title FROM docs `+where+` AND LOWER(title) = LOWER(?) ORDER BY id`, args...)
	if err != nil {
		return nil, fmt.Errorf("lookup link doc: %w", err)
	}
	out := []doclink.Target{}
	for rows.Next() {
		t := doclink.Target{Type: "doc"}
		if err := rows.Scan(&t.ID, &t.Label); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan link doc: %w", err)
		}
		out = append(out, t)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("iterate link docs: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close link docs: %w", closeErr)
	}
	return out, nil
}

func (s *Store) lookupDocLinkEntity(ctx context.Context, userID, target, kind, ref string, qualified bool) ([]doclink.Target, error) {
	keyFilter, keyArgs := apiKeyConnectorFilter(ctx, "em.connector_id")
	where := `em.gone_at IS NULL AND em.connector_id IN (SELECT connector_id FROM user_connector_roles WHERE user_id = ?` + keyFilter + `)`
	args := []any{userID}
	args = append(args, keyArgs...)
	if qualified {
		where += ` AND LOWER(em.kind) = LOWER(?) AND em.ref = ?`
		args = append(args, strings.TrimSpace(kind), strings.TrimSpace(ref))
	} else {
		where += ` AND LOWER(e.display_name) = LOWER(?)`
		args = append(args, target)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT e.id FROM entities e JOIN entity_members em ON em.entity_id = e.id WHERE `+where+` ORDER BY e.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("lookup link entity: %w", err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan link entity: %w", err)
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return nil, fmt.Errorf("iterate link entities: %w", err)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close link entities: %w", closeErr)
	}
	out := []doclink.Target{}
	seen := map[string]bool{}
	for _, id := range ids {
		e, err := s.ResolveEntityIdentity(ctx, id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !seen[e.ID] {
			out = append(out, doclink.Target{Type: "entity", ID: e.ID, Label: e.Name})
			seen[e.ID] = true
		}
	}
	return out, nil
}

// DocBacklink is the public summary of a visible source doc.
type DocBacklink struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ListDocBacklinks includes merged descendants for an entity target and filters
// source docs with the same predicate as the ordinary docs list.
func (s *Store) ListDocBacklinks(ctx context.Context, userID, targetType, targetID string) ([]DocBacklink, error) {
	where, args := viewableDocWhere(ctx, userID, "")
	prefix := ""
	targets := "?"
	if targetType == "entity" {
		// UNION, rather than UNION ALL, also terminates for corrupt merge cycles.
		prefix = `WITH RECURSIVE targets(id) AS (SELECT ? UNION SELECT e.id FROM entities e JOIN targets t ON e.merged_into = t.id) `
		targets = "SELECT id FROM targets"
		args = append([]any{targetID}, args...)
	}
	query := prefix + `SELECT id, title FROM docs ` + where + ` AND id IN (SELECT source_doc_id FROM doc_links WHERE target_type = ? AND target_id IN (` + targets + `)) ORDER BY LOWER(title), id`
	args = append(args, targetType)
	if targetType != "entity" {
		args = append(args, targetID)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list backlinks: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	out := []DocBacklink{}
	for rows.Next() {
		var d DocBacklink
		if err := rows.Scan(&d.ID, &d.Title); err != nil {
			return nil, fmt.Errorf("scan backlink: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate backlinks: %w", err)
	}
	return out, nil
}

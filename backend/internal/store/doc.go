package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/google/uuid"
)

// DocRecord represents a row in the docs table.
type DocRecord struct {
	ParentID       string `json:"parentId"`
	DeletedAt      string `json:"deletedAt"`
	CreatedBy      string `json:"createdBy"`
	ID             string `json:"docId"`
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	ServiceID      string `json:"serviceId"`
	Content        string `json:"content"`
	CurrentVersion int    `json:"currentVersion"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	// Origin is "generated" (sync may merge generated blocks) or "human"
	// (sync never touches it). "" is treated as "generated" on insert.
	Origin string `json:"origin"`
	// TemplateID is the template the doc was generated from; "" means the
	// template-less snapshot render.
	TemplateID string `json:"templateId"`
	// LastSyncedAt is when sync last merged this doc ("" = never).
	LastSyncedAt string `json:"lastSyncedAt"`
	// GenKeys is the raw JSON array of block keys in the last applied
	// render; nil means the doc predates wl:gen markers.
	GenKeys *string `json:"-"`
}

// --- Doc CRUD ---

// CreateDoc inserts a new documentation record.
func (s *Store) CreateDoc(ctx context.Context, d *DocRecord) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if d.CreatedAt == "" {
		d.CreatedAt = now
	}
	if d.UpdatedAt == "" {
		d.UpdatedAt = now
	}
	if d.CurrentVersion == 0 {
		d.CurrentVersion = 1
	}
	if d.Kind == "" {
		d.Kind = "lab"
	}
	if d.Origin == "" {
		d.Origin = DocOriginGenerated
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO docs (id, title, kind, service_id, content, current_version, created_at, updated_at,
			origin, template_id, last_synced_at, gen_keys, parent_id, deleted_at, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.Title, d.Kind, nilToStr(d.ServiceID), d.Content, d.CurrentVersion, d.CreatedAt, d.UpdatedAt,
		d.Origin, nilToStr(d.TemplateID), nilToStr(d.LastSyncedAt), d.GenKeys, nilToStr(d.ParentID), nilToStr(d.DeletedAt), nilToStr(d.CreatedBy))
	if err != nil {
		return fmt.Errorf("create doc: %w", err)
	}
	return nil
}

// ExistingDocIDs returns the subset of ids that already exist as docs, in
// one query — used by backup import to check N records without N round-trips.
func (s *Store) ExistingDocIDs(ctx context.Context, ids []string) (map[string]bool, error) {
	return existingIDs(ctx, s.db, "docs", ids)
}

// GetDoc retrieves a single documentation record by ID.
func (s *Store) GetDoc(ctx context.Context, id string) (*DocRecord, error) {
	d, err := scanDoc(s.db.QueryRowContext(ctx, `SELECT `+docColumns+` FROM docs WHERE id = ? AND deleted_at IS NULL`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get doc: %w", err)
	}
	return &d, nil
}

// UpdateDoc updates the content of a documentation record. If expectedVersion
// is non-nil, the update only applies when current_version matches it
// (optimistic concurrency); a stale version returns ErrVersionConflict instead
// of silently overwriting a newer edit.
func (s *Store) UpdateDoc(ctx context.Context, id, content string, expectedVersion *int) error {
	_, err := s.updateDocRev(ctx, id, content, expectedVersion)
	return err
}

// updateDocRev performs the UPDATE and returns the revision it produced, read
// atomically via RETURNING so concurrent writers cannot change it in between.
func (s *Store) updateDocRev(ctx context.Context, id, content string, expectedVersion *int) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE docs SET content = ?, updated_at = ?, current_version = current_version + 1 WHERE id = ? AND deleted_at IS NULL`
	args := []any{content, now, id}
	if expectedVersion != nil {
		query += ` AND current_version = ?`
		args = append(args, *expectedVersion)
	}
	query += ` RETURNING current_version`

	var rev int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&rev)
	if errors.Is(err, sql.ErrNoRows) {
		if expectedVersion != nil {
			if _, getErr := s.GetDoc(ctx, id); getErr == nil {
				return 0, ErrVersionConflict
			}
		}
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("update doc: %w", err)
	}
	return rev, nil
}

// UpdateDocWithVersion updates a doc's content and records the matching
// doc_versions row in one transaction, so the history always contains the
// revision this write produced. It returns that revision.
func (s *Store) UpdateDocWithVersion(ctx context.Context, id, content string, expectedVersion *int, author, trigger string) (int, error) {
	var rev int
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		r, err := tx.updateDocRev(ctx, id, content, expectedVersion)
		if err != nil {
			return err
		}
		rev = r
		return tx.CreateDocVersion(ctx, &DocVersionRecord{
			DocID: id, Rev: r, Content: content, Author: author, Trigger: trigger,
		})
	})
	if err != nil {
		return 0, err
	}
	return rev, nil
}

// DeleteDoc removes a documentation record by ID.
func (s *Store) DeleteDoc(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM docs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete doc: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DocPreviewChars is how much of a doc's content ListDocPreviewsByService
// returns.
const DocPreviewChars = 512

// ListDocPreviewsByService is ListDocsByService for callers that only need
// metadata: Content holds at most the first DocPreviewChars characters
// instead of the full body.
func (s *Store) ListDocPreviewsByService(ctx context.Context, serviceID string) ([]DocRecord, error) {
	return s.listDocsByService(ctx, serviceID, fmt.Sprintf("substr(content, 1, %d)", DocPreviewChars))
}

// ListDocsByService returns all documentation records for a given service.
func (s *Store) ListDocsByService(ctx context.Context, serviceID string) ([]DocRecord, error) {
	return s.listDocsByService(ctx, serviceID, "content")
}

func (s *Store) listDocsByService(ctx context.Context, serviceID, contentExpr string) ([]DocRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, kind, service_id, `+contentExpr+`, current_version, created_at, updated_at,
			origin, template_id, last_synced_at, gen_keys, parent_id, deleted_at, created_by
		FROM docs WHERE service_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC
	`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list docs by service: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var docs []DocRecord
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		docs = append(docs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate docs: %w", err)
	}
	if docs == nil {
		docs = []DocRecord{}
	}
	return docs, nil
}

// ListDocsGroupedByService returns every doc grouped by connector ID; service-less (lab) docs
// are grouped under the empty key.
// Content is deliberately not loaded (Content is empty): callers render titles.
func (s *Store) ListDocsGroupedByService(ctx context.Context) (map[string][]DocRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+docSummaryColumns+`
		FROM docs WHERE deleted_at IS NULL ORDER BY service_id, updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list docs grouped by service: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	docsByService := map[string][]DocRecord{}
	for rows.Next() {
		d, err := scanDocSummary(rows)
		if err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		docsByService[d.ServiceID] = append(docsByService[d.ServiceID], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate docs: %w", err)
	}
	return docsByService, nil
}

// docColumns is the full column list, including the (potentially large) content.
const docColumns = `id, title, kind, service_id, content, current_version, created_at, updated_at, origin, template_id, last_synced_at, gen_keys, parent_id, deleted_at, created_by`

// docSummaryColumns omits content for list/tree views that never render it.
const docSummaryColumns = `id, title, kind, service_id, current_version, created_at, updated_at, origin, template_id, last_synced_at, parent_id, deleted_at, created_by`

func scanDocSummary(row rowScanner) (DocRecord, error) {
	var d DocRecord
	var svcID, tmplID, synced, parent, deleted, creator sql.NullString
	if err := row.Scan(&d.ID, &d.Title, &d.Kind, &svcID, &d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt,
		&d.Origin, &tmplID, &synced, &parent, &deleted, &creator); err != nil {
		return DocRecord{}, err
	}
	d.ServiceID, d.TemplateID, d.LastSyncedAt = svcID.String, tmplID.String, synced.String
	d.ParentID, d.DeletedAt, d.CreatedBy = parent.String, deleted.String, creator.String
	return d, nil
}

func scanDoc(row rowScanner) (DocRecord, error) {
	var d DocRecord
	var svcID, tmplID, synced, genKeys, parent, deleted, creator sql.NullString
	err := row.Scan(&d.ID, &d.Title, &d.Kind, &svcID, &d.Content, &d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt,
		&d.Origin, &tmplID, &synced, &genKeys, &parent, &deleted, &creator)
	if err != nil {
		return DocRecord{}, err
	}
	d.ServiceID, d.TemplateID, d.LastSyncedAt = svcID.String, tmplID.String, synced.String
	d.ParentID, d.DeletedAt, d.CreatedBy = parent.String, deleted.String, creator.String
	if genKeys.Valid {
		d.GenKeys = &genKeys.String
	}
	return d, nil
}

// ListAllDocs returns a paginated, optionally search-filtered list of all docs
// without their content (Content is empty). Use ListAllDocsWithContent when the
// body is needed.
func (s *Store) ListAllDocs(ctx context.Context, search string, offset, limit int) ([]DocRecord, int, error) {
	where, args := docSearchWhere(search)
	return paginatedQuery(ctx, s.db, "docs", docSummaryColumns, where, args, "updated_at DESC", limit, offset, scanDocSummary)
}

// ListViewableDocs is ListAllDocs limited to docs the caller may view, with the
// total computed over that same set (so it can't reveal hidden docs): docs on
// connectors userID holds a grant on, plus human lab notes for all users
// and generated lab inventory for instance admins.
func (s *Store) ListViewableDocs(ctx context.Context, userID, search string, offset, limit int) ([]DocRecord, int, error) {
	where, args := docSearchWhere(search)
	keyFilter, keyArgs := apiKeyConnectorFilter(ctx, "service_id")
	where += ` AND (service_id IN (SELECT connector_id FROM user_connector_roles WHERE user_id = ?` + keyFilter + `)`
	args = append(args, userID)
	args = append(args, keyArgs...)
	if auth.InstanceAdminFromContext(ctx) {
		where += ` OR service_id IS NULL OR service_id = ''`
	}
	if !auth.InstanceAdminFromContext(ctx) {
		where += ` OR ((service_id IS NULL OR service_id = '') AND origin = 'human')`
	}
	where += `)`
	return paginatedQuery(ctx, s.db, "docs", docSummaryColumns, where, args, "updated_at DESC", limit, offset, scanDocSummary)
}

// ListAllDocsWithContent is like ListAllDocs but also loads each doc's content
// (e.g. for backup export).
func (s *Store) ListAllDocsWithContent(ctx context.Context, search string, offset, limit int) ([]DocRecord, int, error) {
	where, args := docSearchWhere(search)
	return paginatedQuery(ctx, s.db, "docs", docColumns, where, args, "updated_at DESC", limit, offset, scanDoc)
}

// likeEscaper escapes LIKE wildcards and the escape character itself.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

func docSearchWhere(search string) (string, []any) {
	where := "WHERE deleted_at IS NULL"
	var args []any
	if search != "" {
		where += ` AND LOWER(title) LIKE LOWER(?) ESCAPE '\'`
		args = append(args, "%"+escapeLike(search)+"%")
	}
	return where, args
}

// CountDocs returns total number of docs.
func (s *Store) CountDocs(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs WHERE deleted_at IS NULL`).Scan(&count)
	return count, err
}

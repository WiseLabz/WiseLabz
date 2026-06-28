package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DocRecord represents a row in the docs table.
type DocRecord struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Kind           string `json:"kind"`
	ServiceID      string `json:"serviceId"`
	Content        string `json:"content"`
	CurrentVersion int    `json:"currentVersion"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// DocVersionRecord represents a row in the doc_versions table.
type DocVersionRecord struct {
	ID        string `json:"id"`
	DocID     string `json:"docId"`
	Rev       int    `json:"rev"`
	Content   string `json:"content"`
	Author    string `json:"author"`
	Trigger   string `json:"trigger"`
	CreatedAt string `json:"createdAt"`
}

// TemplateRecord represents a row in the templates table.
type TemplateRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppliesTo   string `json:"appliesTo"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// TemplateSectionRecord represents a row in the template_sections table.
type TemplateSectionRecord struct {
	ID         string `json:"id"`
	TemplateID string `json:"templateId"`
	Title      string `json:"title"`
	Ord        int    `json:"ord"`
	Body       string `json:"body"`
}

// --- Doc CRUD ---

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

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO docs (id, title, kind, service_id, content, current_version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, d.ID, d.Title, d.Kind, nilToStr(d.ServiceID), d.Content, d.CurrentVersion, d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create doc: %w", err)
	}
	return nil
}

func (s *Store) GetDoc(ctx context.Context, id string) (*DocRecord, error) {
	d := &DocRecord{}
	var serviceID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, kind, service_id, content, current_version, created_at, updated_at
		FROM docs WHERE id = ?
	`, id).Scan(&d.ID, &d.Title, &d.Kind, &serviceID, &d.Content,
		&d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get doc: %w", err)
	}
	d.ServiceID = serviceID.String
	return d, nil
}

func (s *Store) UpdateDoc(ctx context.Context, id, content string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(ctx, `
		UPDATE docs SET content = ?, updated_at = ?, current_version = current_version + 1 WHERE id = ?
	`, content, now, id)
	if err != nil {
		return fmt.Errorf("update doc: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteDoc(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM docs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete doc: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListDocs(ctx context.Context, offset, limit int) ([]DocRecord, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs`).Scan(&total)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, kind, service_id, content, current_version, created_at, updated_at
		FROM docs ORDER BY updated_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list docs: %w", err)
	}
	defer rows.Close()

	var docs []DocRecord
	for rows.Next() {
		var d DocRecord
		var serviceID sql.NullString
		rows.Scan(&d.ID, &d.Title, &d.Kind, &serviceID, &d.Content,
			&d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt)
		d.ServiceID = serviceID.String
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []DocRecord{}
	}
	return docs, total, nil
}

func (s *Store) ListDocsByService(ctx context.Context, serviceID string) ([]DocRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, kind, service_id, content, current_version, created_at, updated_at
		FROM docs WHERE service_id = ? ORDER BY updated_at DESC
	`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("list docs by service: %w", err)
	}
	defer rows.Close()

	var docs []DocRecord
	for rows.Next() {
		var d DocRecord
		var svcID sql.NullString
		rows.Scan(&d.ID, &d.Title, &d.Kind, &svcID, &d.Content,
			&d.CurrentVersion, &d.CreatedAt, &d.UpdatedAt)
		d.ServiceID = svcID.String
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []DocRecord{}
	}
	return docs, nil
}

// --- Doc versions ---

func (s *Store) CreateDocVersion(ctx context.Context, v *DocVersionRecord) error {
	if v.ID == "" {
		v.ID = uuid.New().String()
	}
	if v.CreatedAt == "" {
		v.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO doc_versions (id, doc_id, rev, content, author, trigger, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, v.ID, v.DocID, v.Rev, v.Content, nilToStr(v.Author), v.Trigger, v.CreatedAt)
	if err != nil {
		return fmt.Errorf("create doc version: %w", err)
	}
	return nil
}

func (s *Store) GetDocVersions(ctx context.Context, docID string) ([]DocVersionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, doc_id, rev, content, author, trigger, created_at
		FROM doc_versions WHERE doc_id = ? ORDER BY rev DESC
	`, docID)
	if err != nil {
		return nil, fmt.Errorf("get doc versions: %w", err)
	}
	defer rows.Close()

	var versions []DocVersionRecord
	for rows.Next() {
		var v DocVersionRecord
		var author sql.NullString
		rows.Scan(&v.ID, &v.DocID, &v.Rev, &v.Content, &author, &v.Trigger, &v.CreatedAt)
		v.Author = author.String
		versions = append(versions, v)
	}
	if versions == nil {
		versions = []DocVersionRecord{}
	}
	return versions, nil
}

// --- Template CRUD ---

func (s *Store) CreateTemplate(ctx context.Context, t *TemplateRecord) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	if t.UpdatedAt == "" {
		t.UpdatedAt = now
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO templates (id, name, description, applies_to, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.ID, t.Name, t.Description, nilToStr(t.AppliesTo), t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	return nil
}

func (s *Store) GetTemplate(ctx context.Context, id string) (*TemplateRecord, error) {
	t := &TemplateRecord{}
	var appliesTo sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, description, applies_to, created_at, updated_at
		FROM templates WHERE id = ?
	`, id).Scan(&t.ID, &t.Name, &t.Description, &appliesTo, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	t.AppliesTo = appliesTo.String
	return t, nil
}

func (s *Store) UpdateTemplate(ctx context.Context, id string, updates map[string]any) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `UPDATE templates SET updated_at = ?`
	args := []any{now}
	for k, v := range updates {
		switch k {
		case "name":
			query += ", name = ?"
			args = append(args, v)
		case "description":
			query += ", description = ?"
			args = append(args, v)
		case "applies_to":
			query += ", applies_to = ?"
			args = append(args, v)
		}
	}
	query += " WHERE id = ?"
	args = append(args, id)

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete template: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListTemplates(ctx context.Context, offset, limit int) ([]TemplateRecord, int, error) {
	var total int
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM templates`).Scan(&total)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, description, applies_to, created_at, updated_at
		FROM templates ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []TemplateRecord
	for rows.Next() {
		var t TemplateRecord
		var appliesTo sql.NullString
		rows.Scan(&t.ID, &t.Name, &t.Description, &appliesTo, &t.CreatedAt, &t.UpdatedAt)
		t.AppliesTo = appliesTo.String
		templates = append(templates, t)
	}
	if templates == nil {
		templates = []TemplateRecord{}
	}
	return templates, total, nil
}

// --- Template sections ---

func (s *Store) CreateTemplateSection(ctx context.Context, sec *TemplateSectionRecord) error {
	if sec.ID == "" {
		sec.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO template_sections (id, template_id, title, ord, body)
		VALUES (?, ?, ?, ?, ?)
	`, sec.ID, sec.TemplateID, sec.Title, sec.Ord, sec.Body)
	if err != nil {
		return fmt.Errorf("create template section: %w", err)
	}
	return nil
}

func (s *Store) GetTemplateSections(ctx context.Context, templateID string) ([]TemplateSectionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, template_id, title, ord, body
		FROM template_sections WHERE template_id = ? ORDER BY ord
	`, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template sections: %w", err)
	}
	defer rows.Close()

	var sections []TemplateSectionRecord
	for rows.Next() {
		var sec TemplateSectionRecord
		rows.Scan(&sec.ID, &sec.TemplateID, &sec.Title, &sec.Ord, &sec.Body)
		sections = append(sections, sec)
	}
	if sections == nil {
		sections = []TemplateSectionRecord{}
	}
	return sections, nil
}

func (s *Store) DeleteTemplateSections(ctx context.Context, templateID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM template_sections WHERE template_id = ?`, templateID)
	return err
}

// CountDocs returns total number of docs.
func (s *Store) CountDocs(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM docs`).Scan(&count)
	return count, err
}

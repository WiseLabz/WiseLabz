// Package doc provides documentation generation from templates and snapshots.
package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Engine generates documentation from templates and connector snapshots.
type Engine struct {
	store *store.Store
}

// NewEngine creates a new doc engine.
func NewEngine(s *store.Store) *Engine {
	return &Engine{store: s}
}

// GenerateResult holds the output of document generation.
type GenerateResult struct {
	DocID   string `json:"docId"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// GenerateFromTemplate generates a document using a template and a snapshot.
func (e *Engine) GenerateFromTemplate(ctx context.Context, templateID, connectorID string) (*GenerateResult, error) {
	// Get template and its sections
	tmpl, err := e.store.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	sections, err := e.store.GetTemplateSections(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template sections: %w", err)
	}

	// Get latest snapshot
	sn, err := e.store.GetLatestSnapshot(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}

	var snap connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(sn.Data), &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	// Render template
	var buf bytes.Buffer
	data := templateData{
		ServiceName: snap.ServiceName,
		Type:        snap.Type,
		Sections:    snap.Sections,
		Metadata:    snap.Metadata,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	buf.WriteString(fmt.Sprintf("# %s\n\n", snap.ServiceName))
	if tmpl.Description != "" {
		buf.WriteString(fmt.Sprintf("> %s\n\n", tmpl.Description))
	}

	for _, sec := range sections {
		tmpl, err := template.New("section").Parse(sec.Body)
		if err != nil {
			buf.WriteString(fmt.Sprintf("## %s\n\n_Template error: %v_\n\n", sec.Title, err))
			continue
		}
		buf.WriteString(fmt.Sprintf("## %s\n\n", sec.Title))
		if err := tmpl.Execute(&buf, data); err != nil {
			buf.WriteString(fmt.Sprintf("\n_Template error: %v_\n", err))
		}
		buf.WriteString("\n")
	}

	content := buf.String()

	// Create or update doc
	existingDocs, err := e.store.ListDocsByService(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("list existing docs: %w", err)
	}

	var docID string
	if len(existingDocs) > 0 {
		docID = existingDocs[0].ID
		if err := e.store.UpdateDoc(ctx, docID, content); err != nil {
			return nil, fmt.Errorf("update doc: %w", err)
		}
		// Get updated version
		doc, _ := e.store.GetDoc(ctx, docID)
		e.store.CreateDocVersion(ctx, &store.DocVersionRecord{
			DocID:   docID,
			Rev:     doc.CurrentVersion,
			Content: content,
			Trigger: "template",
		})
	} else {
		doc := &store.DocRecord{
			Title:     snap.ServiceName,
			Kind:      "service",
			ServiceID: connectorID,
			Content:   content,
		}
		if err := e.store.CreateDoc(ctx, doc); err != nil {
			return nil, fmt.Errorf("create doc: %w", err)
		}
		docID = doc.ID
		e.store.CreateDocVersion(ctx, &store.DocVersionRecord{
			DocID:   docID,
			Rev:     1,
			Content: content,
			Trigger: "template",
		})
	}

	return &GenerateResult{
		DocID:   docID,
		Title:   snap.ServiceName,
		Content: content,
	}, nil
}

// GenerateFromSnapshot generates a raw document from a snapshot without a template.
func (e *Engine) GenerateFromSnapshot(ctx context.Context, connectorID string) (*GenerateResult, error) {
	sn, err := e.store.GetLatestSnapshot(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}

	var snap connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(sn.Data), &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("# %s\n\n", snap.ServiceName))
	buf.WriteString(fmt.Sprintf("**Type:** %s\n", snap.Type))
	buf.WriteString(fmt.Sprintf("**Fetched:** %s\n\n", snap.FetchedAt.Format(time.RFC3339)))

	for _, sec := range snap.Sections {
		buf.WriteString(sec.Content)
		buf.WriteString("\n")
	}

	content := buf.String()
	doc := &store.DocRecord{
		Title:     snap.ServiceName,
		Kind:      "service",
		ServiceID: connectorID,
		Content:   content,
	}
	if err := e.store.CreateDoc(ctx, doc); err != nil {
		return nil, fmt.Errorf("create doc: %w", err)
	}

	docID := doc.ID
	e.store.CreateDocVersion(ctx, &store.DocVersionRecord{
		DocID:   docID,
		Rev:     1,
		Content: content,
		Trigger: "manual",
	})

	return &GenerateResult{
		DocID:   docID,
		Title:   snap.ServiceName,
		Content: content,
	}, nil
}

type templateData struct {
	ServiceName string
	Type        string
	Sections    []connector.SnapshotSection
	Metadata    map[string]string
	GeneratedAt string
}

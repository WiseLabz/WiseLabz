// Package doc provides documentation generation from templates and snapshots.
package doc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// Engine generates documentation from templates and connector snapshots.
type Engine struct {
	store     *store.Store
	snapshots *snapshotCache
	// onDocUpdated, when set, runs after sync or a Change resolution rewrites
	// a doc's content (wired to embedding refresh; a hook avoids doc→chat).
	onDocUpdated func(ctx context.Context, docID, content string)
}

// SetOnDocUpdated registers the hook run after the engine rewrites a doc.
func (e *Engine) SetOnDocUpdated(fn func(ctx context.Context, docID, content string)) {
	e.onDocUpdated = fn
}

func (e *Engine) docUpdated(ctx context.Context, docID, content string) {
	if e.onDocUpdated != nil {
		e.onDocUpdated(ctx, docID, content)
	}
}

// NewEngine creates a new doc engine.
func NewEngine(s *store.Store) *Engine {
	return &Engine{store: s, snapshots: newSnapshotCache(s)}
}

// GenerateResult holds the output of document generation.
type GenerateResult struct {
	DocID   string `json:"docId"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// renderResult is one render: the doc title and its generated blocks.
type renderResult struct {
	Title  string
	Blocks []Block
}

// content lays the blocks out as a fresh doc.
func (r *renderResult) content() string { return renderFresh(r.Blocks) }

// plain is the render without ownership markers, for previews that are
// never stored.
func (r *renderResult) plain() string {
	bodies := make([]string, len(r.Blocks))
	for i, b := range r.Blocks {
		bodies[i] = b.Body
	}
	return strings.Join(bodies, "\n\n") + "\n"
}

// genKeys is the JSON array of the render's block keys, stored on the doc so
// a later merge can tell new upstream sections from ones a user deleted.
func (r *renderResult) genKeys() string {
	b, _ := json.Marshal(blockKeys(r.Blocks))
	return string(b)
}

// render executes a template against a connector's latest snapshot without persisting it.
func (e *Engine) render(ctx context.Context, templateID, connectorID string) (*renderResult, error) {
	tmpl, err := e.store.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	sections, err := e.store.GetTemplateSections(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template sections: %w", err)
	}

	snapPtr, err := e.snapshots.latest(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	snap := *snapPtr

	links, err := matchEntities(ctx, e.store, e.snapshots, connectorID, snap.Entities)
	if err != nil {
		return nil, fmt.Errorf("match entities: %w", err)
	}

	data := templateData{
		ServiceName:  snap.ServiceName,
		Type:         snap.Type,
		Sections:     snap.Sections,
		Dependencies: snap.Dependencies,
		Metadata:     snap.Metadata,
		Links:        links,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	head := "# " + snap.ServiceName
	if tmpl.Description != "" {
		head += "\n\n> " + tmpl.Description
	}
	blocks := []Block{NewBlock("head", head)}

	titles := make([]string, len(sections))
	for i, sec := range sections {
		titles[i] = sec.Title
	}
	keys := slugKeys("tpl", titles)
	for i, sec := range sections {
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "## %s\n\n", sec.Title)
		sectionTemplate, err := template.New("section").Funcs(TemplateFuncs()).Parse(sec.Body)
		if err != nil {
			fmt.Fprintf(&buf, "_Template error: %v_", err)
		} else if err := sectionTemplate.Execute(&buf, data); err != nil {
			fmt.Fprintf(&buf, "\n_Template error: %v_", err)
		}
		blocks = append(blocks, NewBlock(keys[i], strings.TrimRight(buf.String(), "\n")))
	}

	return &renderResult{Title: snap.ServiceName, Blocks: blocks}, nil
}

// PreviewFromTemplate renders a document without persisting it.
func (e *Engine) PreviewFromTemplate(ctx context.Context, templateID, connectorID string) (*GenerateResult, error) {
	rendered, err := e.render(ctx, templateID, connectorID)
	if err != nil {
		return nil, err
	}

	return &GenerateResult{
		Title:   rendered.Title,
		Content: rendered.plain(),
	}, nil
}

// GenerateFromTemplate generates and persists a document using a template
// and a snapshot. An existing generated doc of the connector is re-rendered
// through the template, keeping human-owned text (see Merge); otherwise a new
// doc is created. The template is recorded so later syncs re-apply it.
func (e *Engine) GenerateFromTemplate(ctx context.Context, templateID, connectorID string) (*GenerateResult, error) {
	rendered, err := e.render(ctx, templateID, connectorID)
	if err != nil {
		return nil, err
	}

	existingDocs, err := e.store.ListDocsByService(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("list existing docs: %w", err)
	}
	for _, d := range existingDocs {
		if d.Origin == store.DocOriginHuman {
			continue
		}
		content := rendered.content()
		if d.GenKeys != nil {
			var prev []string
			_ = json.Unmarshal([]byte(*d.GenKeys), &prev)
			out, _ := Merge(ParseBlocks(d.Content), prev, rendered.Blocks)
			content = RenderSegments(out)
		}
		v := d.CurrentVersion
		if _, err := e.store.ApplyGeneratedRender(ctx, d.ID, store.GeneratedRender{
			Content: content, GenKeys: rendered.genKeys(), ExpectedVersion: &v,
			Trigger: "template", Origin: store.DocOriginGenerated, TemplateID: &templateID,
		}); err != nil {
			return nil, fmt.Errorf("update doc: %w", err)
		}
		return &GenerateResult{DocID: d.ID, Title: rendered.Title, Content: content}, nil
	}

	docID, err := e.createGeneratedDoc(ctx, connectorID, templateID, rendered, "template")
	if err != nil {
		return nil, err
	}
	return &GenerateResult{DocID: docID, Title: rendered.Title, Content: rendered.content()}, nil
}

// createGeneratedDoc inserts a new marked-up generated doc and its first
// version in one transaction.
func (e *Engine) createGeneratedDoc(ctx context.Context, connectorID, templateID string, rendered *renderResult, trigger string) (string, error) {
	keys := rendered.genKeys()
	doc := &store.DocRecord{
		Title: rendered.Title, Kind: "service", ServiceID: connectorID, Content: rendered.content(),
		Origin: store.DocOriginGenerated, TemplateID: templateID, GenKeys: &keys,
		LastSyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := e.store.WithinTransaction(ctx, func(tx *store.Store) error {
		if err := tx.CreateDoc(ctx, doc); err != nil {
			return fmt.Errorf("create doc: %w", err)
		}
		if err := tx.CreateDocVersion(ctx, &store.DocVersionRecord{
			DocID: doc.ID, Rev: 1, Content: doc.Content, Trigger: trigger,
		}); err != nil {
			return fmt.Errorf("create doc version: %w", err)
		}
		return nil
	}); err != nil {
		return "", err
	}
	return doc.ID, nil
}

// MatchingConnectors returns connectors covered by a template's applicability scope.
func (e *Engine) MatchingConnectors(ctx context.Context, templateID string) ([]store.ConnectorRecord, error) {
	tmpl, err := e.store.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}

	var appliesTo struct {
		Category string `json:"category"`
		Type     string `json:"type"`
	}
	if tmpl.AppliesTo != "" {
		if err := json.Unmarshal([]byte(tmpl.AppliesTo), &appliesTo); err != nil {
			return nil, fmt.Errorf("unmarshal template applies_to: %w", err)
		}
	}

	connectors, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	matches := make([]store.ConnectorRecord, 0, len(connectors))
	for _, candidate := range connectors {
		if appliesTo.Category != "" && candidate.Category != appliesTo.Category {
			continue
		}
		if appliesTo.Type != "" && candidate.Type != appliesTo.Type {
			continue
		}
		matches = append(matches, candidate)
	}

	return matches, nil
}

// renderSnapshot renders a document from a connector's latest snapshot
// without a template, without persisting it. Shared by GenerateFromSnapshot
// (first-time generation) and RegenerateForConnector (sync-triggered refresh
// of existing template-less docs).
func (e *Engine) renderSnapshot(ctx context.Context, connectorID string) (*renderResult, error) {
	snapPtr, err := e.snapshots.latest(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	snap := *snapPtr

	// The fetch time is deliberately not rendered: it changes every sync and
	// would turn every sync into a new doc version. It is tracked as the
	// doc's last_synced_at instead.
	blocks := []Block{NewBlock("head", fmt.Sprintf("# %s\n\n**Type:** %s", snap.ServiceName, snap.Type))}

	titles := make([]string, len(snap.Sections))
	for i, sec := range snap.Sections {
		titles[i] = sec.Title
	}
	for i, key := range slugKeys("snap", titles) {
		blocks = append(blocks, NewBlock(key, strings.TrimRight(snap.Sections[i].Content, "\n")))
	}

	if len(snap.Dependencies) > 0 {
		var buf strings.Builder
		buf.WriteString("## Dependencies\n")
		for _, dep := range snap.Dependencies {
			fmt.Fprintf(&buf, "\n- **%s**: %s", dep.Kind, dep.Name)
			if dep.Ref != "" {
				fmt.Fprintf(&buf, " (%s)", dep.Ref)
			}
		}
		blocks = append(blocks, NewBlock("deps", buf.String()))
	}

	if links, err := matchEntities(ctx, e.store, e.snapshots, connectorID, snap.Entities); err == nil && len(links) > 0 {
		blocks = append(blocks, NewBlock("related",
			"## Related Entities\n\n"+strings.TrimRight(relatedEntities(snap.ServiceName, links), "\n")))
	}

	return &renderResult{Title: snap.ServiceName, Blocks: blocks}, nil
}

// GenerateFromSnapshot generates a raw document from a snapshot without a template.
func (e *Engine) GenerateFromSnapshot(ctx context.Context, connectorID string) (*GenerateResult, error) {
	rendered, err := e.renderSnapshot(ctx, connectorID)
	if err != nil {
		return nil, err
	}
	docID, err := e.createGeneratedDoc(ctx, connectorID, "", rendered, "snapshot")
	if err != nil {
		return nil, err
	}
	return &GenerateResult{DocID: docID, Title: rendered.Title, Content: rendered.content()}, nil
}

// labTopologyTitle is the fixed title of the single lab-wide topology doc;
// GenerateLabTopology looks it up by this title to update it in place
// rather than creating a new doc on every call.
const labTopologyTitle = "Lab Topology"

// GenerateLabTopology aggregates every connector's latest snapshot entities
// into one lab-wide Mermaid diagram and persists it as the single Kind:
// "lab" doc titled "Lab Topology" (created on first call, updated after).
func (e *Engine) GenerateLabTopology(ctx context.Context) (*GenerateResult, error) {
	return e.generateLabTopology(ctx, false)
}

// generateLabTopology renders and persists the Lab Topology doc. With
// skipIfUnchanged, an existing doc whose content already equals the render
// (ignoring the fingerprint comment) is left alone, so a doc that only lacks
// the comment does not get a new version.
func (e *Engine) generateLabTopology(ctx context.Context, skipIfUnchanged bool) (*GenerateResult, error) {
	connectors, err := e.store.ListAllConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("list connectors: %w", err)
	}

	var entities []labEntity
	for _, c := range connectors {
		snap, err := e.snapshots.latest(ctx, c.ID)
		if err != nil {
			continue // no snapshot yet; soft-skip
		}
		for _, ent := range snap.Entities {
			entities = append(entities, labEntity{ConnectorID: c.ID, ConnectorName: c.Name, Entity: ent})
		}
	}

	var links []labLink
	seen := map[string]bool{}
	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			a, b := entities[i], entities[j]
			if a.ConnectorID == b.ConnectorID {
				continue
			}
			reason := matchReason(a.Entity, b.Entity)
			if reason == "" {
				continue
			}
			key := dedupKey(a.ConnectorID, a.Entity) + ">" + dedupKey(b.ConnectorID, b.Entity)
			if seen[key] {
				continue
			}
			seen[key] = true
			links = append(links, labLink{A: a, B: b, Reason: reason})
		}
	}

	content := fmt.Sprintf(
		"# %s\n\n_%d entities across %d connectors._\n\n```mermaid\n%s```\n",
		labTopologyTitle, len(entities), len(connectors), renderLabMermaid(entities, links),
	)
	// Record which stored edge set this render reflects, so a later topology
	// rebuild regenerates only when the edges moved on.
	fingerprint, err := e.store.TopologyEdgesFingerprint(ctx)
	if err != nil {
		return nil, fmt.Errorf("fingerprint topology edges: %w", err)
	}
	content += "\n" + topologyDocMarker + fingerprint + " -->\n"

	docs, _, err := e.store.ListAllDocs(ctx, labTopologyTitle, 0, 50)
	if err != nil {
		return nil, fmt.Errorf("list docs: %w", err)
	}
	var existing *store.DocRecord
	for i := range docs {
		// A human lab note may share the title; only the generated doc is ours.
		if docs[i].Kind == "lab" && docs[i].Title == labTopologyTitle && docs[i].Origin != store.DocOriginHuman {
			existing = &docs[i]
			break
		}
	}

	var docID string
	if existing != nil {
		docID = existing.ID
		if skipIfUnchanged {
			current, err := e.store.GetDoc(ctx, docID)
			if err != nil {
				return nil, fmt.Errorf("get topology doc: %w", err)
			}
			if StripMarkers(current.Content) == StripMarkers(content) {
				return &GenerateResult{DocID: docID, Title: labTopologyTitle, Content: current.Content}, nil
			}
		}
		if err := e.store.UpdateDoc(ctx, docID, content, nil); err != nil {
			return nil, fmt.Errorf("update doc: %w", err)
		}
		updated, err := e.store.GetDoc(ctx, docID)
		if err != nil {
			return nil, fmt.Errorf("get updated doc: %w", err)
		}
		_ = e.store.CreateDocVersion(ctx, &store.DocVersionRecord{
			DocID: docID, Rev: updated.CurrentVersion, Content: content, Trigger: "manual",
		})
	} else {
		doc := &store.DocRecord{Title: labTopologyTitle, Kind: "lab", Content: content}
		if err := e.store.CreateDoc(ctx, doc); err != nil {
			return nil, fmt.Errorf("create doc: %w", err)
		}
		docID = doc.ID
		_ = e.store.CreateDocVersion(ctx, &store.DocVersionRecord{
			DocID: docID, Rev: 1, Content: content, Trigger: "manual",
		})
	}

	return &GenerateResult{DocID: docID, Title: labTopologyTitle, Content: content}, nil
}

type templateData struct {
	ServiceName  string
	Type         string
	Sections     []connector.SnapshotSection
	Dependencies []connector.ServiceDependency
	Metadata     map[string]string
	Links        []EntityLink
	GeneratedAt  string
}

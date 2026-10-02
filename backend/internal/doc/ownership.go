package doc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// Change types sync raises for docs (#478).
const (
	// ChangeTypeDocConflict: a human-edited generated block whose upstream
	// also changed. Resolved by accepting the generated body or keeping the
	// edit (which detaches the block).
	ChangeTypeDocConflict = "doc_conflict"
	// ChangeTypeDocAdopt: a pre-ownership doc whose last save was human. It
	// became human-owned; accepting replaces it with the marked render.
	ChangeTypeDocAdopt = "doc_adopt"
)

// Resolution actions for doc Changes.
const (
	ResolveAccept = "accept"
	ResolveKeep   = "keep"
)

var (
	// ErrNotResolvable means the Change is not an open doc Change.
	ErrNotResolvable = errors.New("change is not an open doc change")
	// ErrBlockGone means the conflicting block was removed from the doc since.
	ErrBlockGone = errors.New("generated block no longer exists in the doc")
)

// ChangeDiff is the stored diff of a doc Change. It is a JSON object (infra
// changes store an array), which is how readers tell the two apart.
type ChangeDiff struct {
	Format    string `json:"format"` // always "doc"
	DocID     string `json:"docId"`
	Key       string `json:"key,omitempty"` // block key; "" for adopt
	Human     string `json:"human"`
	Generated string `json:"generated"`
	GenHash   string `json:"genHash"`
}

// ParseChangeDiff decodes a doc Change diff; ok is false for anything else.
func ParseChangeDiff(raw string) (ChangeDiff, bool) {
	var d ChangeDiff
	if err := json.Unmarshal([]byte(raw), &d); err != nil || d.Format != "doc" {
		return ChangeDiff{}, false
	}
	return d, true
}

// RegenerateForConnector refreshes the generated parts of every generated
// doc of a connector after a sync, without overwriting human edits:
// human-origin and currently locked docs are skipped, each doc is re-rendered
// (through its template when it has one) and merged block by block, and the
// write is version-checked so a concurrent human save always wins. Edits that
// clash with upstream changes become doc_conflict Changes.
func (e *Engine) RegenerateForConnector(ctx context.Context, connectorID string) error {
	docs, err := e.store.ListDocsByService(ctx, connectorID)
	if err != nil {
		return fmt.Errorf("list docs by service: %w", err)
	}

	renders := map[string]*renderResult{}
	var errs []error
	for _, d := range docs {
		if d.Origin == store.DocOriginHuman {
			continue
		}
		if _, err := e.store.GetDocLock(ctx, d.ID); err == nil {
			continue // someone is editing; the next sync picks it up
		} else if !errors.Is(err, store.ErrNotFound) {
			errs = append(errs, fmt.Errorf("doc %s lock: %w", d.ID, err))
			continue
		}
		rendered, ok := renders[d.TemplateID]
		if !ok {
			if rendered, err = e.renderFor(ctx, d); err != nil {
				errs = append(errs, fmt.Errorf("render doc %s: %w", d.ID, err))
				continue
			}
			renders[d.TemplateID] = rendered
		}
		if err := e.syncDoc(ctx, d, rendered); err != nil {
			errs = append(errs, fmt.Errorf("regenerate doc %s: %w", d.ID, err))
		}
	}
	return errors.Join(errs...)
}

// renderFor renders a doc through its template, falling back to the plain
// snapshot render when it has none (or the template is gone).
func (e *Engine) renderFor(ctx context.Context, d store.DocRecord) (*renderResult, error) {
	if d.TemplateID != "" {
		r, err := e.render(ctx, d.TemplateID, d.ServiceID)
		if err == nil || !errors.Is(err, store.ErrNotFound) {
			return r, err
		}
	}
	return e.renderSnapshot(ctx, d.ServiceID)
}

// syncDoc merges one render into one doc.
func (e *Engine) syncDoc(ctx context.Context, d store.DocRecord, rendered *renderResult) error {
	if d.GenKeys == nil {
		return e.upgradeLegacyDoc(ctx, d, rendered)
	}
	var prev []string
	if err := json.Unmarshal([]byte(*d.GenKeys), &prev); err != nil {
		slog.Warn("doc: invalid stored gen_keys, treating as empty", "docId", d.ID, "error", err)
	}
	out, conflicts := Merge(ParseBlocks(d.Content), prev, rendered.Blocks)
	content := RenderSegments(out)

	if content == d.Content {
		if err := e.store.TouchDocSynced(ctx, d.ID, rendered.genKeys()); err != nil {
			return err
		}
	} else {
		v := d.CurrentVersion
		_, err := e.store.ApplyGeneratedRender(ctx, d.ID, store.GeneratedRender{
			Content: content, GenKeys: rendered.genKeys(), ExpectedVersion: &v, Trigger: "sync",
		})
		if errors.Is(err, store.ErrVersionConflict) {
			return nil // a human saved meanwhile; their save wins, retry next sync
		}
		if err != nil {
			return err
		}
		e.docUpdated(ctx, d.ID, content)
	}

	for _, c := range conflicts {
		if err := e.raiseDocChange(ctx, d, ChangeTypeDocConflict, ChangeDiff{
			Key: c.Key, Human: c.Human, Generated: c.Generated.Body, GenHash: c.Generated.Hash,
		}); err != nil {
			return err
		}
	}
	return nil
}

// upgradeLegacyDoc handles a doc written before block markers existed. If
// its latest version came from the system it is simply re-rendered with
// markers; if a human saved it last, it becomes human-owned (sync leaves it
// alone from now on) and one doc_adopt Change offers the generated layout.
func (e *Engine) upgradeLegacyDoc(ctx context.Context, d store.DocRecord, rendered *renderResult) error {
	versions, err := e.store.GetDocVersions(ctx, d.ID)
	if err != nil {
		return err
	}
	fresh := rendered.content()
	if len(versions) > 0 && versions[0].Author != "" {
		if err := e.store.SetDocOrigin(ctx, d.ID, store.DocOriginHuman); err != nil {
			return err
		}
		return e.raiseDocChange(ctx, d, ChangeTypeDocAdopt, ChangeDiff{
			Human: d.Content, Generated: fresh, GenHash: HashBody(fresh),
		})
	}
	v := d.CurrentVersion
	_, err = e.store.ApplyGeneratedRender(ctx, d.ID, store.GeneratedRender{
		Content: fresh, GenKeys: rendered.genKeys(), ExpectedVersion: &v, Trigger: "sync",
	})
	if errors.Is(err, store.ErrVersionConflict) {
		return nil
	}
	if err != nil {
		return err
	}
	e.docUpdated(ctx, d.ID, fresh)
	return nil
}

// raiseDocChange records a doc Change, keeping at most one open Change per
// doc (and block): an identical open one is left alone, an outdated one is
// dismissed in favour of the new proposal.
func (e *Engine) raiseDocChange(ctx context.Context, d store.DocRecord, changeType string, diff ChangeDiff) error {
	diff.Format, diff.DocID = "doc", d.ID
	pattern := changeType + ":" + d.ID
	summary := fmt.Sprintf("Doc %q has human edits; review the generated layout", d.Title)
	if diff.Key != "" {
		pattern += ":" + diff.Key
		summary = fmt.Sprintf("Doc %q: edited section %q also changed upstream", d.Title, diff.Key)
	}

	open, err := e.store.GetOpenChangeByPattern(ctx, pattern)
	switch {
	case err == nil:
		if prev, ok := ParseChangeDiff(open.Diff); ok && prev.GenHash == diff.GenHash {
			return nil
		}
		if err := e.store.UpdateChangeStatus(ctx, open.ID, "dismissed"); err != nil {
			return err
		}
	case !errors.Is(err, store.ErrNotFound):
		return err
	}

	rawDiff, err := json.Marshal(diff)
	if err != nil {
		return err
	}
	affected, _ := json.Marshal([]string{d.ID})
	return e.store.CreateChange(ctx, &store.ChangeRecord{
		ServiceID: d.ServiceID, ChangeType: changeType, Severity: "info", Summary: summary,
		Diff: string(rawDiff), AffectedDocIDs: string(affected), PatternID: pattern,
	})
}

// ResolveChange applies an accept/keep decision to an open doc Change on
// behalf of userID and acknowledges the Change. It returns the doc ID.
func (e *Engine) ResolveChange(ctx context.Context, c *store.ChangeRecord, action, userID string) (string, error) {
	diff, ok := ParseChangeDiff(c.Diff)
	if !ok || c.Status != "new" || (c.ChangeType != ChangeTypeDocConflict && c.ChangeType != ChangeTypeDocAdopt) {
		return "", ErrNotResolvable
	}
	d, err := e.store.GetDoc(ctx, diff.DocID)
	if err != nil {
		return "", err
	}
	v := d.CurrentVersion

	var content string
	switch {
	case c.ChangeType == ChangeTypeDocAdopt && action == ResolveKeep:
		// Stays human-owned; nothing to write.
	case c.ChangeType == ChangeTypeDocAdopt:
		content = diff.Generated
		keys, _ := json.Marshal(blockKeys(blocksOf(ParseBlocks(content))))
		if _, err := e.store.ApplyGeneratedRender(ctx, d.ID, store.GeneratedRender{
			Content: content, GenKeys: string(keys), ExpectedVersion: &v,
			Author: userID, Trigger: "sync-resolve", Origin: store.DocOriginGenerated,
		}); err != nil {
			return "", err
		}
	default:
		segs := ParseBlocks(d.Content)
		i := blockIndex(segs, diff.Key)
		if i < 0 {
			return "", ErrBlockGone
		}
		if action == ResolveAccept {
			nb := NewBlock(diff.Key, diff.Generated)
			segs[i] = Segment{Block: &nb}
		} else {
			segs[i] = Segment{Text: segs[i].Block.Body}
		}
		content = RenderSegments(segs)
		if _, err := e.store.UpdateDocWithVersion(ctx, d.ID, content, &v, userID, "sync-resolve"); err != nil {
			return "", err
		}
	}

	if err := e.store.UpdateChangeStatus(ctx, c.ID, "acknowledged"); err != nil {
		return "", err
	}
	if content != "" {
		e.docUpdated(ctx, d.ID, content)
	}
	return d.ID, nil
}

func blocksOf(segs []Segment) []Block {
	var out []Block
	for _, s := range segs {
		if s.Block != nil {
			out = append(out, *s.Block)
		}
	}
	return out
}

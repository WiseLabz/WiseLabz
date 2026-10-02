package doc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

var snapshotSeq atomic.Int64

// pushSnapshot stores a newer snapshot for the connector so the next render
// picks it up. FetchedAt always moves forward, mimicking a real sync.
func pushSnapshot(t *testing.T, s *store.Store, connectorID, name string, sections ...connector.SnapshotSection) {
	t.Helper()
	at := time.Now().Add(time.Duration(snapshotSeq.Add(1)) * time.Minute).UTC()
	data, err := json.Marshal(connector.ServiceSnapshot{
		ServiceName: name, Type: "proxmox", Sections: sections, FetchedAt: at,
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: connectorID, Data: string(data), FetchedAt: at.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
}

// ownershipFixture generates a snapshot doc for a fresh connector.
func ownershipFixture(t *testing.T) (*store.Store, *Engine, string, string) {
	t.Helper()
	s := newEngineTestStore(t)
	connectorID := seedEngineConnector(t, s, "Node one", "virtualization", "proxmox", true)
	e := NewEngine(s)
	res, err := e.GenerateFromSnapshot(context.Background(), connectorID)
	if err != nil {
		t.Fatalf("GenerateFromSnapshot() error: %v", err)
	}
	return s, e, connectorID, res.DocID
}

func mustGetDoc(t *testing.T, s *store.Store, id string) *store.DocRecord {
	t.Helper()
	d, err := s.GetDoc(context.Background(), id)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	return d
}

// humanSave stores content as a user edit, like PUT /api/docs/{id}.
func humanSave(t *testing.T, s *store.Store, d *store.DocRecord, content string) {
	t.Helper()
	v := d.CurrentVersion
	if _, err := s.UpdateDocWithVersion(context.Background(), d.ID, content, &v, "user-1", "manual"); err != nil {
		t.Fatalf("UpdateDocWithVersion() error: %v", err)
	}
}

func docChanges(t *testing.T, s *store.Store, connectorID string) []store.ChangeRecord {
	t.Helper()
	changes, _, err := s.ListChanges(context.Background(), connectorID, "", 0, 100)
	if err != nil {
		t.Fatalf("ListChanges() error: %v", err)
	}
	var out []store.ChangeRecord
	for _, c := range changes {
		if strings.HasPrefix(c.ChangeType, "doc_") {
			out = append(out, c)
		}
	}
	return out
}

func runSync(t *testing.T, e *Engine, connectorID string) {
	t.Helper()
	if err := e.RegenerateForConnector(context.Background(), connectorID); err != nil {
		t.Fatalf("RegenerateForConnector() error: %v", err)
	}
}

func TestGeneratedDocHasMarkersAndNoFetchedLine(t *testing.T) {
	s, _, _, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	if strings.Contains(d.Content, "Fetched") {
		t.Fatalf("content still has the volatile Fetched line:\n%s", d.Content)
	}
	keys := blockKeys(blocksOf(ParseBlocks(d.Content)))
	if strings.Join(keys, ",") != "head,snap.status" {
		t.Fatalf("block keys = %v, want [head snap.status]", keys)
	}
	if d.Origin != store.DocOriginGenerated || d.GenKeys == nil || d.LastSyncedAt == "" {
		t.Fatalf("provenance = %+v", d)
	}
}

func TestSyncWithOnlyNewFetchTimeWritesNoVersion(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "healthy"})
	before := mustGetDoc(t, s, docID)
	runSync(t, e, connectorID)
	after := mustGetDoc(t, s, docID)
	if after.CurrentVersion != before.CurrentVersion || after.UpdatedAt != before.UpdatedAt {
		t.Fatalf("version/updated_at changed: before %+v after %+v", before, after)
	}
	if after.LastSyncedAt == "" {
		t.Fatal("last_synced_at not stamped")
	}
}

func TestSyncKeepsHumanTextAndRefreshesUneditedBlocks(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	humanSave(t, s, d, d.Content+"\nMy own notes about this node.\n")

	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)

	got := mustGetDoc(t, s, docID)
	if !strings.Contains(got.Content, "My own notes about this node.") {
		t.Fatalf("human text lost:\n%s", got.Content)
	}
	if !strings.Contains(got.Content, "degraded") || strings.Contains(got.Content, "healthy") {
		t.Fatalf("unedited block not refreshed:\n%s", got.Content)
	}
	versions, _ := s.GetDocVersions(context.Background(), docID)
	if versions[0].Trigger != "sync" || versions[0].Author != "" {
		t.Fatalf("latest version = %+v, want a system sync version", versions[0])
	}
	if len(docChanges(t, s, connectorID)) != 0 {
		t.Fatal("unexpected doc Change for an unedited block")
	}
}

func TestSyncConflictRaisesOneChangeAndResolves(t *testing.T) {
	for _, action := range []string{ResolveAccept, ResolveKeep} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			s, e, connectorID, docID := ownershipFixture(t)
			d := mustGetDoc(t, s, docID)
			humanSave(t, s, d, strings.Replace(d.Content, "healthy", "healthy (UPS on circuit B)", 1))

			pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
			runSync(t, e, connectorID)
			runSync(t, e, connectorID) // same upstream again: no duplicate

			got := mustGetDoc(t, s, docID)
			if !strings.Contains(got.Content, "UPS on circuit B") {
				t.Fatalf("human edit overwritten:\n%s", got.Content)
			}
			changes := docChanges(t, s, connectorID)
			if len(changes) != 1 || changes[0].ChangeType != ChangeTypeDocConflict {
				t.Fatalf("doc changes = %+v, want one doc_conflict", changes)
			}
			if changes[0].AffectedDocIDs != `["`+docID+`"]` {
				t.Fatalf("affected doc ids = %s", changes[0].AffectedDocIDs)
			}

			if _, err := e.ResolveChange(ctx, &changes[0], action, "user-1"); err != nil {
				t.Fatalf("ResolveChange() error: %v", err)
			}
			got = mustGetDoc(t, s, docID)
			segs := ParseBlocks(got.Content)
			switch action {
			case ResolveAccept:
				if !strings.Contains(got.Content, "degraded") || strings.Contains(got.Content, "UPS") {
					t.Fatalf("accept didn't apply generated body:\n%s", got.Content)
				}
				if i := blockIndex(segs, "snap.status"); i < 0 || segs[i].Block.Edited() {
					t.Fatal("accepted block should be a fresh, unedited block")
				}
			case ResolveKeep:
				if !strings.Contains(got.Content, "UPS on circuit B") || blockIndex(segs, "snap.status") >= 0 {
					t.Fatalf("keep should detach the block:\n%s", got.Content)
				}
				// A detached block is never touched or re-added again.
				pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "down"})
				runSync(t, e, connectorID)
				if c := mustGetDoc(t, s, docID).Content; strings.Contains(c, "down") {
					t.Fatalf("detached block re-added or changed:\n%s", c)
				}
			}
			resolved, _ := s.GetChange(ctx, changes[0].ID)
			if resolved.Status != "acknowledged" {
				t.Fatalf("change status = %q, want acknowledged", resolved.Status)
			}
			if _, err := e.ResolveChange(ctx, resolved, action, "user-1"); !errors.Is(err, ErrNotResolvable) {
				t.Fatalf("second ResolveChange() = %v, want ErrNotResolvable", err)
			}
		})
	}
}

func TestSyncConflictSupersededByNewerUpstream(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	humanSave(t, s, d, strings.Replace(d.Content, "healthy", "mine", 1))
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "down"})
	runSync(t, e, connectorID)

	var open int
	for _, c := range docChanges(t, s, connectorID) {
		if c.Status == "new" {
			open++
			if diff, _ := ParseChangeDiff(c.Diff); diff.Generated != "down" {
				t.Fatalf("open conflict proposes %q, want the newest upstream", diff.Generated)
			}
		}
	}
	if open != 1 {
		t.Fatalf("open conflicts = %d, want 1", open)
	}
}

func TestResolveConflictBlockGone(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	humanSave(t, s, d, strings.Replace(d.Content, "healthy", "mine", 1))
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	changes := docChanges(t, s, connectorID)

	d = mustGetDoc(t, s, docID)
	humanSave(t, s, d, "# Rewritten by hand\n")
	if _, err := e.ResolveChange(context.Background(), &changes[0], ResolveAccept, "user-1"); !errors.Is(err, ErrBlockGone) {
		t.Fatalf("ResolveChange() = %v, want ErrBlockGone", err)
	}
}

func TestSyncSkipsHumanAndLockedDocs(t *testing.T) {
	ctx := context.Background()
	s, e, connectorID, docID := ownershipFixture(t)

	human := &store.DocRecord{Title: "Notes", Kind: "service", ServiceID: connectorID, Content: "hand written", Origin: store.DocOriginHuman}
	if err := s.CreateDoc(ctx, human); err != nil {
		t.Fatalf("CreateDoc() error: %v", err)
	}
	if err := s.CreateUser(ctx, &store.User{ID: "user-1", Username: "user-1"}); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}
	if _, err := s.AcquireDocLock(ctx, docID, "user-1"); err != nil {
		t.Fatalf("AcquireDocLock() error: %v", err)
	}

	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	before := mustGetDoc(t, s, docID)
	runSync(t, e, connectorID)
	if got := mustGetDoc(t, s, human.ID); got.Content != "hand written" || got.CurrentVersion != 1 {
		t.Fatalf("human doc touched: %+v", got)
	}
	if got := mustGetDoc(t, s, docID); got.CurrentVersion != before.CurrentVersion {
		t.Fatal("locked doc was rewritten during sync")
	}

	if err := s.ReleaseDocLock(ctx, docID, "user-1"); err != nil {
		t.Fatalf("ReleaseDocLock() error: %v", err)
	}
	runSync(t, e, connectorID)
	if got := mustGetDoc(t, s, docID); !strings.Contains(got.Content, "degraded") {
		t.Fatal("doc not merged once the lock was released")
	}
}

func TestSyncDoesNotReaddDeletedBlock(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	segs := ParseBlocks(d.Content)
	i := blockIndex(segs, "snap.status")
	humanSave(t, s, d, RenderSegments(append(segs[:i:i], segs[i+1:]...)))

	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	if c := mustGetDoc(t, s, docID).Content; strings.Contains(c, "degraded") {
		t.Fatalf("deleted block re-added:\n%s", c)
	}

	// A brand-new upstream section still appears.
	pushSnapshot(t, s, connectorID, "Node one",
		connector.SnapshotSection{Title: "Status", Content: "degraded"},
		connector.SnapshotSection{Title: "Storage", Content: "zfs ok"})
	runSync(t, e, connectorID)
	if c := mustGetDoc(t, s, docID).Content; !strings.Contains(c, "zfs ok") {
		t.Fatalf("new upstream section not inserted:\n%s", c)
	}
}

func TestLegacyDocUpgrade(t *testing.T) {
	ctx := context.Background()
	newLegacy := func(t *testing.T, author string) (*store.Store, *Engine, string, string) {
		s := newEngineTestStore(t)
		connectorID := seedEngineConnector(t, s, "Node one", "virtualization", "proxmox", true)
		d := &store.DocRecord{Title: "Node one", Kind: "service", ServiceID: connectorID,
			Content: "# Node one\n\n**Type:** proxmox\n**Fetched:** 2026-09-05T12:00:00Z\n\nhealthy\n"}
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatalf("CreateDoc() error: %v", err)
		}
		if err := s.CreateDocVersion(ctx, &store.DocVersionRecord{DocID: d.ID, Rev: 1, Content: d.Content, Author: author, Trigger: "manual"}); err != nil {
			t.Fatalf("CreateDocVersion() error: %v", err)
		}
		return s, NewEngine(s), connectorID, d.ID
	}

	t.Run("system authored is re-rendered with markers", func(t *testing.T) {
		s, e, connectorID, docID := newLegacy(t, "")
		runSync(t, e, connectorID)
		got := mustGetDoc(t, s, docID)
		if got.GenKeys == nil || blockIndex(ParseBlocks(got.Content), "head") < 0 || strings.Contains(got.Content, "Fetched") {
			t.Fatalf("legacy doc not upgraded: %+v", got)
		}
	})

	t.Run("human authored becomes human with one adopt change", func(t *testing.T) {
		s, e, connectorID, docID := newLegacy(t, "user-1")
		before := mustGetDoc(t, s, docID)
		runSync(t, e, connectorID)
		runSync(t, e, connectorID)
		got := mustGetDoc(t, s, docID)
		if got.Content != before.Content || got.Origin != store.DocOriginHuman {
			t.Fatalf("human legacy doc changed: %+v", got)
		}
		changes := docChanges(t, s, connectorID)
		if len(changes) != 1 || changes[0].ChangeType != ChangeTypeDocAdopt {
			t.Fatalf("changes = %+v, want one doc_adopt", changes)
		}

		if _, err := e.ResolveChange(ctx, &changes[0], ResolveAccept, "user-1"); err != nil {
			t.Fatalf("ResolveChange(accept) error: %v", err)
		}
		got = mustGetDoc(t, s, docID)
		if got.Origin != store.DocOriginGenerated || got.GenKeys == nil || blockIndex(ParseBlocks(got.Content), "snap.status") < 0 {
			t.Fatalf("adopt accept didn't apply the generated layout: %+v", got)
		}
	})
}

func TestTemplateGenerationRecordsTemplateAndSyncReappliesIt(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	connectorID := seedEngineConnector(t, s, "Node one", "virtualization", "proxmox", true)
	templateID := seedEngineTemplate(t, s, `{}`, store.TemplateSectionRecord{
		Title: "Health", Ord: 1, Body: `{{range .Sections}}{{.Content}}{{end}}`,
	})
	e := NewEngine(s)
	res, err := e.GenerateFromTemplate(ctx, templateID, connectorID)
	if err != nil {
		t.Fatalf("GenerateFromTemplate() error: %v", err)
	}
	if d := mustGetDoc(t, s, res.DocID); d.TemplateID != templateID {
		t.Fatalf("template_id = %q, want %q", d.TemplateID, templateID)
	}

	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	got := mustGetDoc(t, s, res.DocID)
	if !strings.Contains(got.Content, "## Health\n\ndegraded") || blockIndex(ParseBlocks(got.Content), "tpl.health") < 0 {
		t.Fatalf("sync didn't re-render through the template:\n%s", got.Content)
	}
}

func TestOnDocUpdatedHookRuns(t *testing.T) {
	s, e, connectorID, docID := ownershipFixture(t)
	var calls []string
	e.SetOnDocUpdated(func(_ context.Context, id, _ string) { calls = append(calls, id) })
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	if len(calls) != 1 || calls[0] != docID {
		t.Fatalf("hook calls = %v, want [%s]", calls, docID)
	}
}

func TestDismissedConflictIsNotReraisedForSameUpstream(t *testing.T) {
	ctx := context.Background()
	s, e, connectorID, docID := ownershipFixture(t)
	d := mustGetDoc(t, s, docID)
	humanSave(t, s, d, strings.Replace(d.Content, "healthy", "mine", 1))
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "degraded"})
	runSync(t, e, connectorID)
	changes := docChanges(t, s, connectorID)
	if err := s.UpdateChangeStatus(ctx, changes[0].ID, "dismissed"); err != nil {
		t.Fatalf("UpdateChangeStatus() error: %v", err)
	}
	runSync(t, e, connectorID)
	if n := len(docChanges(t, s, connectorID)); n != 1 {
		t.Fatalf("doc changes after re-sync = %d, want 1 (dismissal sticks)", n)
	}
	pushSnapshot(t, s, connectorID, "Node one", connector.SnapshotSection{Title: "Status", Content: "down"})
	runSync(t, e, connectorID)
	if n := len(docChanges(t, s, connectorID)); n != 2 {
		t.Fatalf("doc changes after new upstream = %d, want 2", n)
	}
}

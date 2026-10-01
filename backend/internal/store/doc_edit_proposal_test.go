package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestDocEditProposalLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	u := &User{Username: "author", DisplayName: "A", InstanceAdminRole: "user", AuthSource: "local", PasswordHash: "x"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateDoc(ctx, &DocRecord{ID: "d1", Title: "Doc", Kind: "lab", Content: "v1"}); err != nil {
		t.Fatal(err)
	}

	propose := func(base int, content string) *DocEditProposal {
		t.Helper()
		p := &DocEditProposal{DocID: "d1", BaseVersion: base, Content: content, AuthorID: u.ID}
		if err := s.CreateDocEditProposal(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}

	t.Run("approve applies a new version", func(t *testing.T) {
		p := propose(1, "v2 from proposal")
		rev, err := s.ApproveDocEditProposal(ctx, p.ID, u.ID)
		if err != nil || rev != 2 {
			t.Fatalf("approve = %d, %v; want rev 2", rev, err)
		}
		d, _ := s.GetDoc(ctx, "d1")
		if d.Content != "v2 from proposal" || d.CurrentVersion != 2 {
			t.Fatalf("doc = %+v", d)
		}
		got, _ := s.GetDocEditProposal(ctx, p.ID)
		if got.Status != ProposalApproved || got.ReviewerID != u.ID || got.ReviewedAt == "" {
			t.Fatalf("proposal = %+v", got)
		}
		vs, _ := s.GetDocVersions(ctx, "d1")
		if len(vs) != 1 || vs[0].Trigger != DocEditProposalTrigger {
			t.Fatalf("versions = %+v", vs)
		}
		// Second approval is a conflict and does not re-apply.
		if _, err := s.ApproveDocEditProposal(ctx, p.ID, u.ID); !errors.Is(err, ErrConflict) {
			t.Fatalf("re-approve err = %v, want ErrConflict", err)
		}
	})

	t.Run("stale base version conflicts and stays pending", func(t *testing.T) {
		p := propose(1, "stale") // doc is at version 2 now
		if _, err := s.ApproveDocEditProposal(ctx, p.ID, u.ID); !errors.Is(err, ErrVersionConflict) {
			t.Fatalf("err = %v, want ErrVersionConflict", err)
		}
		got, _ := s.GetDocEditProposal(ctx, p.ID)
		if got.Status != ProposalPending || got.ReviewerID != "" {
			t.Fatalf("proposal = %+v, want pending", got)
		}
		d, _ := s.GetDoc(ctx, "d1")
		if d.Content != "v2 from proposal" {
			t.Fatalf("doc changed: %+v", d)
		}
	})

	t.Run("reject leaves doc unchanged", func(t *testing.T) {
		p := propose(2, "rejected content")
		if err := s.RejectDocEditProposal(ctx, p.ID, u.ID); err != nil {
			t.Fatal(err)
		}
		got, _ := s.GetDocEditProposal(ctx, p.ID)
		if got.Status != ProposalRejected {
			t.Fatalf("proposal = %+v", got)
		}
		if err := s.RejectDocEditProposal(ctx, p.ID, u.ID); !errors.Is(err, ErrConflict) {
			t.Fatalf("re-reject err = %v, want ErrConflict", err)
		}
		if _, err := s.ApproveDocEditProposal(ctx, "missing", u.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("missing err = %v, want ErrNotFound", err)
		}
		d, _ := s.GetDoc(ctx, "d1")
		if d.Content != "v2 from proposal" {
			t.Fatalf("doc changed: %+v", d)
		}
	})

	t.Run("list filters by status, omits content and pages in SQL", func(t *testing.T) {
		all := ProposalScope{All: true}
		pending, total, err := s.ListDocEditProposals(ctx, ProposalPending, all, 10, 0)
		// The stale proposal was superseded when the reject-case proposal was
		// created by the same author for the same doc, so nothing is pending.
		if err != nil || total != 0 || len(pending) != 0 {
			t.Fatalf("pending = %+v, total %d, %v", pending, total, err)
		}
		items, total, err := s.ListDocEditProposals(ctx, "", all, 1, 0)
		if err != nil || total != 2 || len(items) != 1 {
			t.Fatalf("page 1 = %+v, total %d, %v", items, total, err)
		}
		if items[0].Content != "" || items[0].DocTitle != "Doc" {
			t.Fatalf("list item = %+v, want empty content and joined title", items[0])
		}
		next, _, _ := s.ListDocEditProposals(ctx, "", all, 1, 1)
		if len(next) != 1 || next[0].ID == items[0].ID {
			t.Fatalf("page 2 = %+v", next)
		}
		if got, _ := s.GetDocEditProposal(ctx, items[0].ID); got.Content == "" {
			t.Fatal("single get must keep content")
		}
	})
}

func TestDocEditProposalSupersedeAndCap(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	mkUser := func(name string) string {
		u := &User{Username: name, DisplayName: name, InstanceAdminRole: "user", AuthSource: "local", PasswordHash: "x"}
		if err := s.CreateUser(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	author, other := mkUser("author"), mkUser("other")
	conn := ConnectorRecord{Name: "c", Category: "virtualization", Type: "proxmox", URL: "https://c.test"}
	if err := s.CreateConnector(ctx, &conn); err != nil {
		t.Fatal(err)
	}
	mkDoc := func(id, svc string) {
		if err := s.CreateDoc(ctx, &DocRecord{ID: id, Title: id, Kind: "service", ServiceID: svc, Content: "v1"}); err != nil {
			t.Fatal(err)
		}
	}
	propose := func(docID, authorID, content string) (*DocEditProposal, error) {
		p := &DocEditProposal{DocID: docID, BaseVersion: 1, Content: content, AuthorID: authorID}
		return p, s.CreateDocEditProposal(ctx, p)
	}
	all := ProposalScope{All: true}

	t.Run("same author and doc supersedes the older pending proposal", func(t *testing.T) {
		mkDoc("d-sup", "")
		first, _ := propose("d-sup", author, "first")
		if _, err := propose("d-sup", other, "other author keeps theirs"); err != nil {
			t.Fatal(err)
		}
		second, err := propose("d-sup", author, "second")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetDocEditProposal(ctx, first.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("first proposal err = %v, want superseded (ErrNotFound)", err)
		}
		if _, err := s.GetDocEditProposal(ctx, second.ID); err != nil {
			t.Fatal(err)
		}
		_, total, _ := s.ListDocEditProposals(ctx, ProposalPending, all, 10, 0)
		if total != 2 {
			t.Fatalf("pending = %d, want 2 (author's latest + other author's)", total)
		}
	})

	t.Run("per-author pending cap", func(t *testing.T) {
		capped := mkUser("capped")
		for i := 0; i < MaxPendingProposalsPerAuthor; i++ {
			id := fmt.Sprintf("d-cap-%d", i)
			mkDoc(id, "")
			if _, err := propose(id, capped, "x"); err != nil {
				t.Fatalf("proposal %d: %v", i, err)
			}
		}
		mkDoc("d-cap-over", "")
		if _, err := propose("d-cap-over", capped, "x"); !errors.Is(err, ErrProposalLimit) {
			t.Fatalf("over-cap err = %v, want ErrProposalLimit", err)
		}
		// Replacing an existing pending proposal at the cap is still allowed.
		if _, err := propose("d-cap-0", capped, "replaced"); err != nil {
			t.Fatalf("replace at cap: %v", err)
		}
		// Other authors are unaffected.
		if _, err := propose("d-cap-over", other, "x"); err != nil {
			t.Fatalf("other author: %v", err)
		}
	})

	t.Run("scope filters lab-wide and connector docs in SQL", func(t *testing.T) {
		mkDoc("d-conn", conn.ID)
		if _, err := propose("d-conn", author, "x"); err != nil {
			t.Fatal(err)
		}
		titles := func(sc ProposalScope) map[string]bool {
			items, _, err := s.ListDocEditProposals(ctx, ProposalPending, sc, 200, 0)
			if err != nil {
				t.Fatal(err)
			}
			m := map[string]bool{}
			for _, it := range items {
				m[it.DocID] = true
			}
			return m
		}
		if got := titles(ProposalScope{ConnectorIDs: []string{conn.ID}}); len(got) != 1 || !got["d-conn"] {
			t.Fatalf("connector scope = %v", got)
		}
		if got := titles(ProposalScope{LabWide: true}); got["d-conn"] || !got["d-sup"] {
			t.Fatalf("lab-wide scope = %v", got)
		}
		if got := titles(ProposalScope{}); len(got) != 0 {
			t.Fatalf("empty scope = %v, want none", got)
		}
	})
}

package store

import (
	"context"
	"errors"
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

	t.Run("list filters by status", func(t *testing.T) {
		pending, err := s.ListDocEditProposals(ctx, ProposalPending)
		if err != nil || len(pending) != 1 || pending[0].DocTitle != "Doc" {
			t.Fatalf("pending = %+v, %v", pending, err)
		}
		all, _ := s.ListDocEditProposals(ctx, "")
		if len(all) != 3 {
			t.Fatalf("all = %d, want 3", len(all))
		}
	})
}

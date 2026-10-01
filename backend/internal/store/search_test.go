package store

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

func TestSearchContent(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)

	connA := ConnectorRecord{Name: "a", Category: "virtualization", Type: "proxmox", URL: "https://a.test"}
	connB := ConnectorRecord{Name: "b", Category: "virtualization", Type: "proxmox", URL: "https://b.test"}
	for _, c := range []*ConnectorRecord{&connA, &connB} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	u := &User{Username: "viewer", DisplayName: "V", InstanceAdminRole: "user", AuthSource: "local", PasswordHash: "x"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertConnectorGrant(ctx, u.ID, connA.ID, "viewer"); err != nil {
		t.Fatal(err)
	}

	mk := func(id, title, content, svc string) {
		t.Helper()
		if err := s.CreateDoc(ctx, &DocRecord{ID: id, Title: title, Kind: "service", ServiceID: svc, Content: content}); err != nil {
			t.Fatal(err)
		}
	}
	mk("d-a", "Router notes", "How to reset the gateway router after a firmware upgrade", connA.ID)
	mk("d-b", "Hidden notes", "The secret router password rotation procedure", connB.ID)

	if _, err := s.CreateRunbook(ctx, &RunbookRecord{ID: "rb1", Title: "Reboot router", Body: "Restart the router cleanly", TargetType: "alert_severity", TargetValue: "critical"}); err != nil {
		t.Fatal(err)
	}

	ids := func(hits []SearchHit) map[string]bool {
		m := map[string]bool{}
		for _, h := range hits {
			m[h.Type+":"+h.ID] = true
		}
		return m
	}

	viewerCtx := auth.ContextWithUser(ctx, u.ID, false)
	hits, err := s.SearchContent(viewerCtx, u.ID, "router", 10)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(hits)
	if !got["doc:d-a"] || !got["runbook:rb1"] || got["doc:d-b"] {
		t.Fatalf("viewer hits = %+v", hits)
	}
	if hits[0].Snippet == "" {
		t.Fatal("expected snippet")
	}

	// Stemming and prefix matching.
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "upgrad", 10); !ids(hits)["doc:d-a"] {
		t.Fatalf("prefix hits = %+v", hits)
	}

	// Hostile input must not break the query.
	if _, err := s.SearchContent(viewerCtx, u.ID, `"router" OR * NEAR(`, 10); err != nil {
		t.Fatalf("hostile query: %v", err)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "   ", 10); len(hits) != 0 {
		t.Fatalf("empty query hits = %+v", hits)
	}

	// A connector-restricted key can't see connector A's doc.
	keyCtx := auth.ContextWithAPIKeyRestriction(viewerCtx, auth.APIKeyRestriction{ConnectorIDs: []string{connB.ID}})
	if hits, _ = s.SearchContent(keyCtx, u.ID, "router", 10); ids(hits)["doc:d-a"] {
		t.Fatalf("restricted key saw doc: %+v", hits)
	}

	// Freshness: update then delete.
	if err := s.UpdateDoc(ctx, "d-a", "Now about the printer", nil); err != nil {
		t.Fatal(err)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "gateway", 10); ids(hits)["doc:d-a"] {
		t.Fatalf("stale index after update: %+v", hits)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "printer", 10); !ids(hits)["doc:d-a"] {
		t.Fatalf("new content not indexed: %+v", hits)
	}
	if _, err := s.UpdateRunbook(ctx, "rb1", map[string]any{"body": "Power cycle the switch"}); err != nil {
		t.Fatal(err)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "restart", 10); ids(hits)["runbook:rb1"] {
		t.Fatalf("stale runbook index: %+v", hits)
	}
	if err := s.DeleteDoc(ctx, "d-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRunbook(ctx, "rb1"); err != nil {
		t.Fatal(err)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "printer switch", 10); len(hits) != 0 {
		t.Fatalf("deleted content still indexed: %+v", hits)
	}

	// Admin sees lab-wide docs.
	mk("d-lab", "Lab overview", "topology overview of everything", "")
	adminCtx := auth.ContextWithUser(ctx, u.ID, true)
	if hits, _ = s.SearchContent(adminCtx, u.ID, "topology", 10); !ids(hits)["doc:d-lab"] {
		t.Fatalf("admin lab doc hits = %+v", hits)
	}
	if hits, _ = s.SearchContent(viewerCtx, u.ID, "topology", 10); len(hits) != 0 {
		t.Fatalf("non-admin saw lab doc: %+v", hits)
	}
}

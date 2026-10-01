package store

import (
	"context"
	"strings"
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

func TestSearchContentQueryHandling(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	u := &User{Username: "admin", DisplayName: "A", InstanceAdminRole: "admin", AuthSource: "local", PasswordHash: "x"}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	adminCtx := auth.ContextWithUser(ctx, u.ID, true)
	for _, d := range []*DocRecord{
		{ID: "d1", Title: "The gateway", Kind: "lab", Content: "the quick brown router and the gateway"},
		{ID: "d2", Title: "Café Münchën", Kind: "lab", Content: "naïve résumé of the 東京 datacenter"},
	} {
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateRunbook(ctx, &RunbookRecord{ID: "rb1", Title: "Gateway restart", Body: "restart the gateway", TargetType: "alert_severity", TargetValue: "critical"}); err != nil {
		t.Fatal(err)
	}

	t.Run("stopword-only and punctuation-only queries return no hits and no error", func(t *testing.T) {
		for _, q := range []string{"the", "the and of", "a", "", "   ", "*", "-", `"`, "()", `" * NEAR OR - ( )`, "!!!"} {
			hits, err := s.SearchContent(adminCtx, u.ID, q, 10)
			if err != nil || len(hits) != 0 {
				t.Errorf("query %q: hits %+v, err %v; want none, nil", q, hits, err)
			}
		}
	})

	t.Run("stopwords are ignored next to real terms", func(t *testing.T) {
		hits, err := s.SearchContent(adminCtx, u.ID, "the gateway", 10)
		if err != nil || len(hits) == 0 {
			t.Fatalf("hits %+v, err %v", hits, err)
		}
	})

	t.Run("FTS operators in input are inert", func(t *testing.T) {
		for _, q := range []string{`gateway OR router`, `gateway NEAR router`, `-gateway`, `gate*`, `(gateway)`, `"gateway`, `gateway AND NOT router`, `col:gateway`, `^gateway`} {
			if _, err := s.SearchContent(adminCtx, u.ID, q, 10); err != nil {
				t.Errorf("query %q: %v", q, err)
			}
		}
		// OR is dropped as a stopword-like literal term, so both words are ANDed.
		hits, _ := s.SearchContent(adminCtx, u.ID, `gateway OR nonexistentword`, 10)
		if len(hits) != 0 {
			t.Errorf("operator treated as OR: %+v", hits)
		}
	})

	t.Run("non-ASCII text is searchable", func(t *testing.T) {
		for _, q := range []string{"café", "munchen", "naïve", "東京", "résumé"} {
			hits, err := s.SearchContent(adminCtx, u.ID, q, 10)
			if err != nil || len(hits) == 0 || hits[0].ID != "d2" {
				t.Errorf("query %q: hits %+v, err %v; want d2", q, hits, err)
			}
		}
	})

	t.Run("docs and runbooks interleave by rank with normalized scores", func(t *testing.T) {
		hits, err := s.SearchContent(adminCtx, u.ID, "gateway", 10)
		if err != nil || len(hits) != 2 {
			t.Fatalf("hits %+v, err %v", hits, err)
		}
		if hits[0].Type != SearchHitDoc || hits[1].Type != SearchHitRunbook {
			t.Fatalf("order = %s, %s; want doc then runbook", hits[0].Type, hits[1].Type)
		}
		for _, h := range hits {
			if h.Score <= 0 || h.Score > 1 {
				t.Errorf("score %v out of (0,1]", h.Score)
			}
		}
		if capped, _ := s.SearchContent(adminCtx, u.ID, "gateway", 1); len(capped) != 1 {
			t.Errorf("limit not applied: %+v", capped)
		}
	})
}

func TestInterleaveHits(t *testing.T) {
	d := []SearchHit{{ID: "d1"}, {ID: "d2"}, {ID: "d3"}}
	r := []SearchHit{{ID: "r1"}}
	var got []string
	for _, h := range interleaveHits(d, r, 10) {
		got = append(got, h.ID)
	}
	if want := "d1 r1 d2 d3"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
	if n := len(interleaveHits(d, r, 2)); n != 2 {
		t.Fatalf("limit: %d", n)
	}
}

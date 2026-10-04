package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/snapshotutil"
)

func TestEntityIndexSearch(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	if s.driver != "postgres" {
		if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			t.Fatal(err)
		}
	}
	mustCreateUser(t, s, "search-user")
	mustCreateUser(t, s, "outsider")
	a := ConnectorRecord{Name: "Router", Type: "unifi", Category: "networking", URL: "https://a.test"}
	b := ConnectorRecord{Name: "Secret", Type: "unifi", Category: "networking", URL: "https://b.test"}
	for _, c := range []*ConnectorRecord{&a, &b} {
		if err := s.CreateConnector(ctx, c); err != nil {
			t.Fatal(err)
		}
		if _, err := s.UpsertConnectorGrant(ctx, "search-user", c.ID, "viewer"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateDoc(ctx, &DocRecord{ID: "service-doc", Title: "Router", Kind: "service", ServiceID: a.ID, Content: "router"}); err != nil {
		t.Fatal(err)
	}
	entities := []connector.SnapshotEntity{
		{Kind: "device", Name: "a-router-extra", IP: "10.0.0.11"},
		{Kind: "device", Name: "ROUTER", IP: "10.0.0.1", Hostname: "Gateway.LAB", ExternalID: "100", MAC: snapshotutil.NormalizeMAC("AA-BB-CC-DD-EE-FF"), Aliases: []string{"dns.lab", `alias%_\name`}},
		{Kind: "device", Name: "ÜBER-NAS", Hostname: "HÖST.LAB", Aliases: []string{"ÄLIAS.LAB"}},
		{Kind: "device", Name: "a-mac-fragment", MAC: "00:aa:bb:cc:dd:ee:ff:11"},
		{Kind: "vm", Name: `literal%_\name`},
	}
	replace := func(id string, es []connector.SnapshotEntity) {
		t.Helper()
		if err := s.WithinTransaction(ctx, func(tx *Store) error { return tx.ReplaceEntityIndexForConnector(ctx, id, es) }); err != nil {
			t.Fatal(err)
		}
	}
	replace(a.ID, entities)
	replace(b.ID, []connector.SnapshotEntity{{Kind: "device", Name: "hidden-router"}})
	userCtx := auth.ContextWithUser(ctx, "search-user", false)
	search := func(ctx context.Context, q string, f SearchFilter, limit int) []EntityHit {
		t.Helper()
		hits, err := s.SearchEntities(ctx, auth.UserIDFromContext(ctx), q, f, limit)
		if err != nil {
			t.Fatal(err)
		}
		return hits
	}
	hits := search(userCtx, "router", SearchFilter{}, 10)
	if len(hits) != 3 || hits[0].Name != "ROUTER" || hits[0].DocID != "service-doc" || hits[0].ConnectorName != "Router" {
		t.Fatalf("hits: %+v", hits)
	}
	for _, q := range []string{"10.0.0.1", "gateway", "AA:BB:CC:DD:EE:FF", "100", "dns.lab"} {
		if hits := search(userCtx, q, SearchFilter{ConnectorID: a.ID, Kind: "device"}, 10); len(hits) == 0 {
			t.Fatalf("missing query %q", q)
		}
	}
	for _, q := range []string{"AA-BB-CC-DD-EE-FF", "aabb.ccdd.eeff", "aabbccddeeff", "AA:BB:CC:DD:EE:FF"} {
		hits := search(userCtx, q, SearchFilter{}, 10)
		if len(hits) != 2 || hits[0].Name != "ROUTER" {
			t.Fatalf("MAC exact ordering %q: %+v", q, hits)
		}
	}
	for _, q := range []string{"BB-CC-DD", "bbcc.dd", "bbccdd", "bb:cc:dd"} {
		if hits := search(userCtx, q, SearchFilter{}, 10); len(hits) != 2 {
			t.Fatalf("MAC substring %q: %+v", q, hits)
		}
	}
	for _, q := range []string{"ÜBER", "über", "HÖST", "höst", "ÄLIAS", "älias"} {
		hits := search(userCtx, q, SearchFilter{Kind: "DEVICE"}, 10)
		if len(hits) != 1 || hits[0].Name != "ÜBER-NAS" || hits[0].Aliases[0] != "ÄLIAS.LAB" {
			t.Fatalf("Unicode query %q: %+v", q, hits)
		}
	}
	for _, q := range []string{`%_\`, `alias%_\name`} {
		if hits := search(userCtx, q, SearchFilter{}, 10); len(hits) != map[string]int{`%_\`: 2, `alias%_\name`: 1}[q] {
			t.Fatalf("escaped %q: %+v", q, hits)
		}
	}
	if hits := search(userCtx, "[]", SearchFilter{}, 10); len(hits) != 0 {
		t.Fatalf("matched JSON syntax: %+v", hits)
	}
	for _, admin := range []bool{false, true} {
		if hits := search(auth.ContextWithUser(ctx, "outsider", admin), "router", SearchFilter{}, 10); len(hits) != 0 {
			t.Fatalf("default deny admin=%v: %+v", admin, hits)
		}
	}
	restricted := auth.ContextWithAPIKeyRestriction(userCtx, auth.APIKeyRestriction{ConnectorIDs: []string{b.ID}})
	if hits := search(restricted, "router", SearchFilter{}, 10); len(hits) != 1 || hits[0].ConnectorID != b.ID {
		t.Fatalf("key scope: %+v", hits)
	}
	if hits := search(restricted, "router", SearchFilter{ConnectorID: a.ID}, 10); len(hits) != 0 {
		t.Fatalf("filter bypassed key: %+v", hits)
	}
	if hits := search(userCtx, "router", SearchFilter{Kind: "vm"}, 10); len(hits) != 0 {
		t.Fatalf("kind: %+v", hits)
	}
	if hits := search(userCtx, "router", SearchFilter{}, 1); len(hits) != 1 || hits[0].Name != "ROUTER" {
		t.Fatalf("limit: %+v", hits)
	}
	// Exact aliases outrank partial name matches.
	replace(a.ID, append(entities, connector.SnapshotEntity{Kind: "device", Name: "a-dns.lab-extra"}))
	if hits := search(userCtx, "dns.lab", SearchFilter{}, 10); len(hits) != 2 || hits[0].Name != "ROUTER" {
		t.Fatalf("alias ranking: %+v", hits)
	}
	// A failed snapshot transaction must leave the previous index untouched.
	rollback := errors.New("rollback")
	if err := s.WithinTransaction(ctx, func(tx *Store) error {
		if err := tx.ReplaceEntityIndexForConnector(ctx, a.ID, nil); err != nil {
			return err
		}
		return rollback
	}); !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if hits := search(userCtx, "gateway", SearchFilter{}, 10); len(hits) != 1 {
		t.Fatalf("rollback: %+v", hits)
	}
	// A successful sync replaces all rows, removing upstream-deleted entities.
	if err := s.WithinTransaction(ctx, func(tx *Store) error {
		sn := connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "device", Name: "replacement"}}}
		data, err := json.Marshal(sn)
		if err != nil {
			return err
		}
		if err := tx.CreateSnapshot(ctx, &SnapshotRecord{ConnectorID: a.ID, Data: string(data), FetchedAt: time.Now().UTC().Format(SnapshotTimeFormat)}); err != nil {
			return err
		}
		return tx.ReplaceEntityIndexForConnector(ctx, a.ID, sn.Entities)
	}); err != nil {
		t.Fatal(err)
	}
	if hits := search(userCtx, "gateway", SearchFilter{}, 10); len(hits) != 0 {
		t.Fatalf("stale entity: %+v", hits)
	}
	if err := s.DeleteConnector(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_index WHERE connector_id = ?`, a.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("hard-delete cascade left %d rows", count)
	}
}

func TestEntityIndexBackfill(t *testing.T) {
	s := newDocTestStore(t)
	ctx := context.Background()
	c := ConnectorRecord{Name: "Legacy", Type: "unifi", Category: "networking", URL: "https://legacy.test"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"old", "latest"} {
		data, err := json.Marshal(connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{Kind: "device", Name: name}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.CreateSnapshot(ctx, &SnapshotRecord{ConnectorID: c.ID, Data: string(data), FetchedAt: time.Now().Add(time.Duration(i) * time.Hour).UTC().Format(SnapshotTimeFormat)}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if n, err := s.BackfillEntityIndex(ctx); err != nil || n != 1 {
			t.Fatalf("backfill %d, %v", n, err)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_index WHERE connector_id = ? AND name = 'latest'`, c.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("latest snapshot indexed %d times", count)
		}
	}
}

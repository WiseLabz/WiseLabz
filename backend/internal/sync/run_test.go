package sync

import (
	"context"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestRunSyncReplacesEntityIndex(t *testing.T) {
	snapshot := &connector.ServiceSnapshot{
		ServiceName: "router", FetchedAt: time.Now(),
		Entities: []connector.SnapshotEntity{
			{Kind: "device", Name: "retained-router"},
			{Kind: "device", Name: "removed-router"},
		},
	}
	connector.Register(
		connector.TypeSchema{Type: "sync_test_entity_index", Category: "test", Name: "Entity index fake"},
		func(_ map[string]any) (connector.Connector, error) {
			return &fakeConnector{snapshot: snapshot}, nil
		},
	)
	s := newTestStore(t)
	ctx := context.Background()
	user := &store.User{Username: "entity-search"}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatal(err)
	}
	conn := &store.ConnectorRecord{Name: "router", Type: "sync_test_entity_index", Category: "networking", Enabled: true}
	if err := s.CreateConnector(ctx, conn); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertConnectorGrant(ctx, user.ID, conn.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	ctx = auth.ContextWithUser(ctx, user.ID, false)
	engine := NewEngine(s, nil, nil, nil, "")
	if _, err := engine.RunSync(ctx, conn.ID, "first"); err != nil {
		t.Fatal(err)
	}
	hits, err := s.SearchEntities(ctx, user.ID, "router", store.SearchFilter{}, 20)
	if err != nil || len(hits) != 2 {
		t.Fatalf("initial index: %+v, %v", hits, err)
	}
	snapshot = &connector.ServiceSnapshot{
		ServiceName: "router", FetchedAt: time.Now().Add(time.Second),
		Entities: []connector.SnapshotEntity{{Kind: "device", Name: "retained-router"}},
	}
	if _, err := engine.RunSync(ctx, conn.ID, "second"); err != nil {
		t.Fatal(err)
	}
	hits, err = s.SearchEntities(ctx, user.ID, "router", store.SearchFilter{}, 20)
	if err != nil || len(hits) != 1 || hits[0].Name != "retained-router" {
		t.Fatalf("replacement index: %+v, %v", hits, err)
	}
	hits, err = s.SearchEntities(ctx, user.ID, "removed-router", store.SearchFilter{}, 20)
	if err != nil || len(hits) != 0 {
		t.Fatalf("stale index: %+v, %v", hits, err)
	}
}

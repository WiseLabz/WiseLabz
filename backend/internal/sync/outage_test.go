package sync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestSyncRetainsFailedSectionsAndInventory(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	rec := &store.ConnectorRecord{Name: "test", Type: "section_failure", Category: "networking", Enabled: true}
	if err := s.CreateConnector(ctx, rec); err != nil {
		t.Fatal(err)
	}
	old := &connector.ServiceSnapshot{
		Sections:  []connector.SnapshotSection{{Title: "Rules", Content: "rule-1"}, {Title: "System", Content: "before"}},
		Entities:  []connector.SnapshotEntity{{Kind: "rule", ExternalID: "1"}},
		FetchedAt: time.Now().Add(-time.Hour),
	}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: rec.ID, Data: string(data), FetchedAt: old.FetchedAt.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	fetched := &connector.ServiceSnapshot{Sections: []connector.SnapshotSection{connector.ErrorSection("Rules", errors.New("host unreachable")), {Title: "System", Content: "after"}}, FetchedAt: time.Now()}
	c := &fakeConnector{snapshot: fetched}
	connector.Register(connector.TypeSchema{Type: rec.Type, Category: "networking"}, func(map[string]any) (connector.Connector, error) { return c, nil })
	e := NewEngine(s, nil, nil, nil, "")
	docs := &fakeDocRegenerator{}
	e.SetDocRegenerator(docs)
	result, err := e.RunSync(ctx, rec.ID, "failed-section")
	if err == nil || result.ChangesCount != 1 {
		t.Fatalf("result = %+v, err = %v; want only the healthy section diff", result, err)
	}
	if docs.count() != 0 {
		t.Fatal("regenerated docs from a failed fetch")
	}
	latest, err := s.GetLatestSnapshot(ctx, rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saved connector.ServiceSnapshot
	if err := json.Unmarshal([]byte(latest.Data), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Sections[0].Content != "rule-1" || saved.Sections[0].Error == "" || len(saved.Entities) != 1 {
		t.Fatalf("lost prior data: %+v", saved)
	}
	// Recovery to unchanged rules must not create reverse drift.
	c.snapshot = &connector.ServiceSnapshot{Sections: []connector.SnapshotSection{{Title: "Rules", Content: "rule-1"}, {Title: "System", Content: "after"}}, Entities: old.Entities, FetchedAt: time.Now()}
	result, err = e.RunSync(ctx, rec.ID, "recovery")
	if err != nil || result.ChangesCount != 0 {
		t.Fatalf("recovery = %+v, err = %v", result, err)
	}
}

func TestDetachedSyncsShareLimitAndDrain(t *testing.T) {
	s := newTestStore(t)
	c := &blockingConnector{entered: make(chan context.Context, 3), release: make(chan struct{})}
	connector.Register(connector.TypeSchema{Type: "detached_limits", Category: "networking"}, func(map[string]any) (connector.Connector, error) { return c, nil })
	e := NewEngine(s, nil, nil, nil, "")
	e.SetLimits(1, 50, time.Minute)
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.SetBaseContext(base)
	for range 3 {
		rec := &store.ConnectorRecord{Name: "test", Type: "detached_limits", Category: "networking", Enabled: true}
		if err := s.CreateConnector(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
		e.Go(func(ctx context.Context) { _, _ = e.RunSync(ctx, rec.ID, "detached") })
	}
	select {
	case <-c.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("fetch did not start")
	}
	select {
	case <-c.entered:
		t.Fatal("exceeded concurrency limit")
	case <-time.After(100 * time.Millisecond):
	}
	expired, stop := context.WithCancel(context.Background())
	stop()
	if err := e.Wait(expired); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait = %v", err)
	}
	cancel()
	deadline, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if err := e.Wait(deadline); err != nil {
		t.Fatal(err)
	}
	if !e.Idle() {
		t.Fatal("engine did not drain")
	}
}

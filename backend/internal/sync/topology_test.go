package sync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type fakeTopologyBuilder struct {
	calls []string
	err   error
}

type fakeIdentityBuilder struct {
	calls []string
	err   error
}

func (f *fakeIdentityBuilder) RebuildEntityIdentitiesForConnector(_ context.Context, connectorID string) error {
	f.calls = append(f.calls, connectorID)
	return f.err
}

func (f *fakeTopologyBuilder) RebuildTopologyForConnector(_ context.Context, connectorID string) error {
	f.calls = append(f.calls, connectorID)
	return f.err
}

func TestRunSyncRebuildsTopologyAfterSuccess(t *testing.T) {
	connector.Register(
		connector.TypeSchema{Type: "sync_test_topology", Category: "test", Name: "Topology"},
		func(_ map[string]any) (connector.Connector, error) {
			return &fakeConnector{snapshot: &connector.ServiceSnapshot{ServiceName: "svc", FetchedAt: time.Now()}}, nil
		},
	)
	s := newTestStore(t)
	record := &store.ConnectorRecord{Name: "topology", Category: "networking", Type: "sync_test_topology", Enabled: true}
	if err := s.CreateConnector(context.Background(), record); err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}

	// A failing builder must not fail the sync.
	tb := &fakeTopologyBuilder{err: errors.New("boom")}
	ib := &fakeIdentityBuilder{err: errors.New("identity rebuild failed")}
	engine := NewEngine(s, nil, nil, nil, "")
	engine.SetTopologyBuilder(tb)
	engine.SetIdentityBuilder(ib)

	result, err := engine.RunSync(context.Background(), record.ID, "topology-job")
	if err != nil || result.Status != "success" {
		t.Fatalf("RunSync() = (%+v, %v), want successful sync", result, err)
	}
	if len(tb.calls) != 1 || tb.calls[0] != record.ID {
		t.Fatalf("topology builder calls = %v, want [%s]", tb.calls, record.ID)
	}
	if len(ib.calls) != 1 || ib.calls[0] != record.ID {
		t.Fatalf("identity builder calls = %v, want [%s]", ib.calls, record.ID)
	}
}

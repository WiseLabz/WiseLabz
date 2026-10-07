package compliance

import (
	"context"
	"errors"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestSnapshotFromConnectorCarriesTypedFields(t *testing.T) {
	got := SnapshotFromConnector(connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{
		Kind: "vm", Name: "web", ExternalID: "104", IP: "10.0.0.4", Hostname: "web.lan",
		MAC: "aa:bb:cc:dd:ee:ff", Attributes: map[string]any{"template": false},
	}}})
	if len(got.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(got.Entities))
	}
	e := got.Entities[0]
	if e.Kind != "vm" || e.Name != "web" || e.ExternalID != "104" || e.IP != "10.0.0.4" ||
		e.Hostname != "web.lan" || e.MAC != "aa:bb:cc:dd:ee:ff" || e.Attributes["template"] != false {
		t.Fatalf("entity = %#v, lost a field", e)
	}
}

type snapshotSource struct {
	record *store.SnapshotRecord
	err    error
}

func (s snapshotSource) GetLatestSnapshot(context.Context, string) (*store.SnapshotRecord, error) {
	return s.record, s.err
}

func TestLoadLatestSnapshot(t *testing.T) {
	ctx := context.Background()
	got, err := LoadLatestSnapshot(ctx, snapshotSource{record: &store.SnapshotRecord{
		Data: `{"entities":[{"kind":"vm","name":"web","externalId":"104","attributes":{"status":"running"}}]}`,
	}}, "c")
	if err != nil || got == nil || len(got.Entities) != 1 || got.Entities[0].ExternalID != "104" || got.Entities[0].Attributes["status"] != "running" {
		t.Fatalf("LoadLatestSnapshot() = %+v, %v; want the stored entity", got, err)
	}
	if got, err := LoadLatestSnapshot(ctx, snapshotSource{err: store.ErrNotFound}, "c"); got != nil || err != nil {
		t.Fatalf("no snapshot = %+v, %v; want nil, nil", got, err)
	}
	if got, err := LoadLatestSnapshot(ctx, snapshotSource{record: &store.SnapshotRecord{Data: "{"}}, "c"); got != nil || err != nil {
		t.Fatalf("malformed snapshot = %+v, %v; want nil, nil", got, err)
	}
	boom := errors.New("boom")
	if _, err := LoadLatestSnapshot(ctx, snapshotSource{err: boom}, "c"); !errors.Is(err, boom) {
		t.Fatalf("store error = %v, want it returned", err)
	}
}

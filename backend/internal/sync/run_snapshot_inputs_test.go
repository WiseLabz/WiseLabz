package sync

import (
	"context"
	"encoding/json"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type snapshotRecordingConnector struct {
	fieldsRecordingConnector
	relatedIDs   []string
	wantPrevious bool
	inputConfig  map[string]any
}

func (c *snapshotRecordingConnector) SnapshotInputs(config map[string]any) ([]string, bool) {
	c.inputConfig = maps.Clone(config)
	return c.relatedIDs, c.wantPrevious
}

func newSnapshotRecordingConnector() *snapshotRecordingConnector {
	return &snapshotRecordingConnector{
		fieldsRecordingConnector: fieldsRecordingConnector{
			snapshot: &connector.ServiceSnapshot{ServiceName: "probe", FetchedAt: time.Now().UTC()},
		},
	}
}

func setupSnapshotInputSync(t *testing.T, conn connector.Connector, config string) (*store.Store, *store.ConnectorRecord, *Engine) {
	t.Helper()
	typ := "sync_test_snapshot_" + strings.ReplaceAll(t.Name(), "/", "_")
	connector.Register(
		connector.TypeSchema{Type: typ, Category: "networking", Name: "Snapshot Inputs"},
		func(_ map[string]any) (connector.Connector, error) { return conn, nil },
	)
	s := newTestStore(t)
	rec := &store.ConnectorRecord{Name: "probe", Category: "networking", Type: typ, Enabled: true, ConfigData: config}
	if err := s.CreateConnector(context.Background(), rec); err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}
	return s, rec, NewEngine(s, nil, nil, nil, "")
}

func saveSnapshotInput(t *testing.T, s *store.Store, id string, snapshot *connector.ServiceSnapshot) {
	t.Helper()
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{
		ConnectorID: id, Data: string(data), FetchedAt: snapshot.FetchedAt.Format(store.SnapshotTimeFormat),
	}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
}

func TestRunSyncSuppliesSnapshotInputs(t *testing.T) {
	c := newSnapshotRecordingConnector()
	c.wantPrevious = true
	s, rec, engine := setupSnapshotInputSync(t, c, `{"targets":"manual.lab:443"}`)
	ctx := context.Background()
	// Invalid config proves that loading a related snapshot never parses its credentials.
	source := &store.ConnectorRecord{Name: "source", Type: "unregistered", Category: "networking", ConfigData: "invalid secret config"}
	if err := s.CreateConnector(ctx, source); err != nil {
		t.Fatalf("CreateConnector source: %v", err)
	}
	c.relatedIDs = []string{source.ID}
	now := time.Now().UTC().Truncate(time.Second)
	related := &connector.ServiceSnapshot{
		ServiceName: "source", Type: "traefik", FetchedAt: now.Add(-time.Minute),
		Sections:     []connector.SnapshotSection{{Title: "Routers", Content: "a.lab"}},
		Entities:     []connector.SnapshotEntity{{Kind: "router", ExternalID: "a", Attributes: map[string]any{"tls": true, "rule": "Host(a.lab)"}}},
		Dependencies: []connector.ServiceDependency{{Kind: "upstream_service", Name: "app"}},
		Metadata:     map[string]string{"version": "3"},
	}
	older := &connector.ServiceSnapshot{ServiceName: "older source", FetchedAt: now.Add(-2 * time.Hour)}
	saveSnapshotInput(t, s, source.ID, older)
	saveSnapshotInput(t, s, source.ID, related)
	previous := &connector.ServiceSnapshot{
		ServiceName: "probe", FetchedAt: now.Add(-time.Hour),
		Entities: []connector.SnapshotEntity{{Kind: "certificate", ExternalID: "a.lab:443", Attributes: map[string]any{"not_after": "2026-11-01T00:00:00Z"}}},
	}
	saveSnapshotInput(t, s, rec.ID, previous)

	result, err := engine.RunSync(ctx, rec.ID, "snapshot-inputs")
	if err != nil || result.Status != "success" {
		t.Fatalf("RunSync = %#v, %v, want success", result, err)
	}
	if got := connector.RelatedSnapshots(c.config); len(got) != 1 || !reflect.DeepEqual(got[source.ID], related) {
		t.Fatalf("related snapshots = %#v, want latest stored snapshot %#v", got, related)
	}
	if got := connector.PreviousSnapshot(c.config); !reflect.DeepEqual(got, previous) {
		t.Fatalf("previous snapshot = %#v, want %#v", got, previous)
	}
	if c.inputConfig["targets"] != "manual.lab:443" || c.config["targets"] != "manual.lab:443" {
		t.Fatalf("connector config lost its targets: %#v", c.config)
	}
	if _, ok := c.config["secret"]; ok {
		t.Fatal("related credentials were copied into Fetch config")
	}
}

func TestRunSyncMissingSnapshotInputs(t *testing.T) {
	for _, state := range []string{"never synced", "deleted", "unknown ID"} {
		t.Run(state, func(t *testing.T) {
			c := newSnapshotRecordingConnector()
			c.wantPrevious = true
			s, rec, engine := setupSnapshotInputSync(t, c, "{}")
			ctx := context.Background()
			source := &store.ConnectorRecord{Name: "source", Type: "unregistered", Category: "networking"}
			if err := s.CreateConnector(ctx, source); err != nil {
				t.Fatalf("CreateConnector source: %v", err)
			}
			c.relatedIDs = []string{source.ID}
			switch state {
			case "deleted":
				saveSnapshotInput(t, s, source.ID, &connector.ServiceSnapshot{ServiceName: "source", FetchedAt: time.Now().UTC()})
				if err := s.DeleteConnector(ctx, source.ID); err != nil {
					t.Fatalf("DeleteConnector: %v", err)
				}
			case "unknown ID":
				c.relatedIDs = []string{"nonexistent"}
			}
			result, err := engine.RunSync(ctx, rec.ID, "missing-inputs")
			if err != nil || result.Status != "success" {
				t.Fatalf("RunSync = %#v, %v, want success", result, err)
			}
			if got := connector.RelatedSnapshots(c.config); len(got) != 0 {
				t.Fatalf("related snapshots = %#v, want absent", got)
			}
			if got := connector.PreviousSnapshot(c.config); got != nil {
				t.Fatalf("previous snapshot = %#v, want absent on first sync", got)
			}
		})
	}
}

func TestRunSyncStripsConfigSuppliedSnapshotInputs(t *testing.T) {
	c := newSnapshotRecordingConnector()
	_, rec, engine := setupSnapshotInputSync(t, c, `{
		"_related_snapshots":{"fake":{"serviceName":"injected"}},
		"_previous_snapshot":{"serviceName":"injected"},
		"targets":"manual.lab:443"
	}`)
	if _, err := engine.RunSync(context.Background(), rec.ID, "strip-inputs"); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	for _, cfg := range []map[string]any{c.config, c.inputConfig} {
		for _, key := range []string{"_related_snapshots", "_previous_snapshot"} {
			if _, ok := cfg[key]; ok {
				t.Fatalf("config-supplied reserved key %q survived: %#v", key, cfg)
			}
		}
		if cfg["targets"] != "manual.lab:443" {
			t.Fatalf("regular config changed: %#v", cfg)
		}
	}
}

func TestFetchOverwritesTypedSnapshotInputs(t *testing.T) {
	c := newSnapshotRecordingConnector()
	c.wantPrevious = true
	s, rec, engine := setupSnapshotInputSync(t, c, "{}")
	previous := &connector.ServiceSnapshot{ServiceName: "stored", FetchedAt: time.Now().UTC().Truncate(time.Second)}
	saveSnapshotInput(t, s, rec.ID, previous)
	c.relatedIDs = []string{rec.ID}
	cfg := map[string]any{
		"_related_snapshots": map[string]*connector.ServiceSnapshot{"fake": {ServiceName: "injected"}},
		"_previous_snapshot": &connector.ServiceSnapshot{ServiceName: "injected"},
	}
	if _, err := engine.fetchSyncSnapshot(context.Background(), rec.ID, "typed-inputs", c, cfg, func(string, int) {}); err != nil {
		t.Fatalf("fetchSyncSnapshot: %v", err)
	}
	if got := connector.RelatedSnapshots(c.config); len(got) != 1 || !reflect.DeepEqual(got[rec.ID], previous) {
		t.Fatalf("related snapshots = %#v, want stored snapshot", got)
	}
	if got := connector.PreviousSnapshot(c.config); !reflect.DeepEqual(got, previous) {
		t.Fatalf("previous snapshot = %#v, want stored snapshot", got)
	}
}

func TestRunSyncDoesNotLoadUnrequestedPreviousSnapshot(t *testing.T) {
	c := newSnapshotRecordingConnector()
	s, rec, engine := setupSnapshotInputSync(t, c, "{}")
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: rec.ID, Data: "invalid snapshot"}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	// A selective sync avoids the later diff baseline read, isolating the input load.
	result, err := engine.RunSyncFields(context.Background(), rec.ID, "no-previous", []string{"certificates"})
	if err != nil || result.Status != "success" {
		t.Fatalf("RunSyncFields = %#v, %v, want success without reading the invalid previous snapshot", result, err)
	}
	if got := connector.PreviousSnapshot(c.config); got != nil {
		t.Fatalf("previous snapshot = %#v, want absent when not requested", got)
	}
	if got := connector.RequestedFields(c.config); !reflect.DeepEqual(got, []string{"certificates"}) {
		t.Fatalf("fields = %v, want [certificates]", got)
	}
}

func TestRunSyncInvalidRelatedSnapshotFailsBeforeFetch(t *testing.T) {
	c := newSnapshotRecordingConnector()
	s, rec, engine := setupSnapshotInputSync(t, c, "{}")
	c.relatedIDs = []string{rec.ID}
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: rec.ID, Data: "invalid snapshot"}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	result, err := engine.RunSync(context.Background(), rec.ID, "invalid-related")
	if err == nil || !strings.Contains(err.Error(), "decode snapshot") || result.Status != "error" {
		t.Fatalf("RunSync = %#v, %v, want decode error", result, err)
	}
	if c.config != nil {
		t.Fatal("Fetch called after related snapshot failed to decode")
	}
}

func TestRunSyncInvalidPreviousSnapshotTreatedAsAbsent(t *testing.T) {
	c := newSnapshotRecordingConnector()
	c.wantPrevious = true
	s, rec, engine := setupSnapshotInputSync(t, c, "{}")
	if err := s.CreateSnapshot(context.Background(), &store.SnapshotRecord{ConnectorID: rec.ID, Data: "invalid snapshot"}); err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	// persistSyncSnapshot also tolerates the corrupt baseline, so the sync can store a fresh snapshot.
	result, err := engine.RunSync(context.Background(), rec.ID, "invalid-previous")
	if err != nil || result.Status != "success" {
		t.Fatalf("RunSync = %#v, %v, want success", result, err)
	}
	if c.config == nil {
		t.Fatal("Fetch not called after previous snapshot failed to decode")
	}
	if got := connector.PreviousSnapshot(c.config); got != nil {
		t.Fatalf("previous snapshot = %#v, want absent", got)
	}
}

func TestRunSyncDuplicateRelatedIDsLoadedOnce(t *testing.T) {
	c := newSnapshotRecordingConnector()
	s, rec, engine := setupSnapshotInputSync(t, c, "{}")
	ctx := context.Background()
	source := &store.ConnectorRecord{Name: "source", Type: "unregistered", Category: "networking"}
	if err := s.CreateConnector(ctx, source); err != nil {
		t.Fatalf("CreateConnector source: %v", err)
	}
	related := &connector.ServiceSnapshot{ServiceName: "source", FetchedAt: time.Now().UTC().Truncate(time.Second)}
	saveSnapshotInput(t, s, source.ID, related)
	c.relatedIDs = []string{source.ID, source.ID, ""}
	result, err := engine.RunSync(ctx, rec.ID, "duplicate-related")
	if err != nil || result.Status != "success" {
		t.Fatalf("RunSync = %#v, %v, want success", result, err)
	}
	if got := connector.RelatedSnapshots(c.config); len(got) != 1 || !reflect.DeepEqual(got[source.ID], related) {
		t.Fatalf("related snapshots = %#v, want exactly one entry %#v", got, related)
	}
}

func TestRunSyncNonDependentConfigUnchanged(t *testing.T) {
	c := &fieldsRecordingConnector{snapshot: &connector.ServiceSnapshot{ServiceName: "plain", FetchedAt: time.Now().UTC()}}
	_, rec, engine := setupSnapshotInputSync(t, c, `{
		"targets":"manual.lab:443", "custom":true,
		"_related_snapshots":{"fake":{"serviceName":"unchanged"}},
		"_previous_snapshot":{"serviceName":"unchanged"}
	}`)
	want, err := engine.syncConfig(rec, nil)
	if err != nil {
		t.Fatalf("syncConfig: %v", err)
	}
	if _, err := engine.RunSync(context.Background(), rec.ID, "non-dependent"); err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	if !reflect.DeepEqual(c.config, want) {
		t.Fatalf("non-dependent config = %#v, want untouched %#v", c.config, want)
	}
}

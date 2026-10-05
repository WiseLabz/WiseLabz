package pbs

import (
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

func TestAttributeCatalogCoversEmittedKeys(t *testing.T) {
	now := time.Now()
	fixtures := []struct {
		name  string
		build func() (string, []connector.SnapshotEntity)
	}{
		{"datastores", func() (string, []connector.SnapshotEntity) {
			return buildDatastoresTable([]datastore{
				{Store: "store1", BackendType: "filesystem", Comment: "Test storage"},
			})
		}},
		{"verify jobs", func() (string, []connector.SnapshotEntity) {
			return buildVerifyJobsTable([]verifyJob{
				{ID: "verify1", Store: "store1", NS: "", Schedule: "daily", IgnoreVerified: true, OutdatedAfter: nil, MaxDepth: nil},
				{ID: "verify2", Store: "store1", NS: "ns1", Schedule: "", IgnoreVerified: false, OutdatedAfter: ptrFlexInt(30), MaxDepth: ptrFlexInt(2)},
			})
		}},
		{"prune jobs", func() (string, []connector.SnapshotEntity) {
			return buildPruneJobsTable([]pruneJob{
				{ID: "prune1", Store: "store1", NS: "", Schedule: "weekly", Disable: false, MaxDepth: nil, KeepLast: 0, KeepHourly: 0, KeepDaily: 0, KeepWeekly: 0, KeepMonthly: 0, KeepYearly: 0},
				{ID: "prune2", Store: "store1", NS: "ns1", Schedule: "", Disable: true, MaxDepth: ptrFlexInt(3), KeepLast: 3, KeepHourly: 24, KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 12, KeepYearly: 2},
			})
		}},
		{"backup groups", func() (string, []connector.SnapshotEntity) {
			groups := []backupGroup{
				{Store: "store1", Namespace: "", Type: "vm", ID: "100", BackupCount: 5, LastBackup: flexInt(now.Unix() - 3*24*3600)},
				{Store: "store1", Namespace: "ns1", Type: "ct", ID: "200", BackupCount: 3, LastBackup: flexInt(now.Unix() - 7*24*3600)},
				{Store: "store1", Namespace: "", Type: "host", ID: "nas1", BackupCount: 2, LastBackup: flexInt(now.Unix() - 1*3600)},
			}
			verify := map[groupRef]string{
				{Store: "store1", Namespace: "", Type: "vm", ID: "100"}:    "ok",
				{Store: "store1", Namespace: "ns1", Type: "ct", ID: "200"}: "failed",
			}
			return buildGroupsTable(groups, verify, now)
		}},
	}

	emitted := map[string]map[string]string{}
	for _, fixture := range fixtures {
		_, entities := fixture.build()
		for _, entity := range entities {
			if emitted[entity.Kind] == nil {
				emitted[entity.Kind] = map[string]string{}
			}
			for key, value := range entity.Attributes {
				emitted[entity.Kind][key] = connectortest.JSONType(value)
			}
		}
	}

	for kind, keys := range emitted {
		specs, ok := attributeCatalog[kind]
		if !ok {
			t.Fatalf("catalog missing entity kind %q", kind)
		}
		byName := make(map[string]connector.AttributeSpec, len(specs))
		for _, spec := range specs {
			byName[spec.Name] = spec
		}
		for key, typ := range keys {
			spec, ok := byName[key]
			if !ok {
				t.Errorf("catalog[%q] missing emitted attribute %q", kind, key)
				continue
			}
			if spec.Type != typ {
				t.Errorf("catalog[%q][%q].Type = %q, want %q", kind, key, spec.Type, typ)
			}
			if spec.Description == "" {
				t.Errorf("catalog[%q][%q] has no description", kind, key)
			}
		}
		for key := range byName {
			if _, ok := keys[key]; !ok {
				t.Errorf("catalog[%q] declares unused attribute %q", kind, key)
			}
		}
	}

	for kind := range attributeCatalog {
		if _, ok := emitted[kind]; !ok {
			t.Errorf("catalog declares unused entity kind %q", kind)
		}
	}
}

func ptrFlexInt(v int64) *flexInt {
	f := flexInt(v)
	return &f
}

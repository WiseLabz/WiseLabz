package sync

import (
	"context"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestRunSyncAppliesHealthReportedBySnapshot(t *testing.T) {
	c := newSnapshotRecordingConnector()
	_, rec, engine := setupSnapshotInputSync(t, c, "{}")
	ctx := context.Background()
	status := func() (string, string) {
		t.Helper()
		got, err := engine.store.GetConnector(ctx, rec.ID)
		if err != nil {
			t.Fatalf("GetConnector: %v", err)
		}
		return got.Status, got.StatusMessage
	}
	sync := func(metadata map[string]string) {
		t.Helper()
		c.snapshot = &connector.ServiceSnapshot{ServiceName: "probe", FetchedAt: time.Now().UTC(), Metadata: metadata}
		if result, err := engine.RunSync(ctx, rec.ID, "reported-health"); err != nil || result.Status != "success" {
			t.Fatalf("RunSync = %#v, %v, want success", result, err)
		}
	}

	allDown := map[string]string{}
	connector.ReportOffline(allDown, 3)
	sync(allDown)
	if s, m := status(); s != "offline" || m != "All 3 targets unreachable" {
		t.Fatalf("all down: status %q message %q", s, m)
	}

	// One target answers again: the next sync returns the connector to online.
	sync(map[string]string{"target_count": "3", "reachable_count": "1"})
	if s, m := status(); s != "online" || m != "Sync successful" {
		t.Fatalf("recovered: status %q message %q", s, m)
	}

	// No targets at all is online.
	sync(map[string]string{"target_count": "0"})
	if s, _ := status(); s != "online" {
		t.Fatalf("zero targets: status %q", s)
	}
}

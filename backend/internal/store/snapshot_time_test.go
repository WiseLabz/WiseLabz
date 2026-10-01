package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestSnapshotTimeOrderingAndRetention(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	c := ConnectorRecord{Name: "time", Category: "virtualization", Type: "proxmox", URL: "https://example.test"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	times := []string{
		"2026-10-25T02:50:00+02:00",
		"2026-10-25T02:10:00+01:00",
		"2026-10-25T01:10:00.000000001Z",
		"2026-10-25T01:10:00.000000001Z",
	}
	var last SnapshotRecord
	for _, at := range times {
		sn := SnapshotRecord{ConnectorID: c.ID, Data: "{}", FetchedAt: at}
		if err := s.CreateSnapshot(ctx, &sn); err != nil {
			t.Fatal(err)
		}
		last = sn
	}
	if last.FetchedAt != "2026-10-25T01:10:00.000000001Z" {
		t.Fatalf("stored time = %q", last.FetchedAt)
	}
	latest, err := s.GetLatestSnapshot(ctx, c.ID)
	if err != nil || latest.ID != last.ID {
		t.Fatalf("latest = %+v, %v; want %s", latest, err, last.ID)
	}
	snaps, err := s.GetSnapshotsByConnector(ctx, c.ID, 10)
	if err != nil || len(snaps) != 4 || snaps[0].ID != last.ID {
		t.Fatalf("snapshots = %+v, %v", snaps, err)
	}
	summaries, _, err := s.ListSnapshotsByConnectorKeyset(ctx, c.ID, Keyset{}, 10)
	if err != nil || len(summaries) != 4 || summaries[0].ID != last.ID {
		t.Fatalf("summaries = %+v, %v", summaries, err)
	}
	n, err := s.DeleteOldSnapshots(ctx, "2026-10-25T01:10:01Z")
	if err != nil || n != 3 {
		t.Fatalf("deleted = %d, %v; want 3", n, err)
	}
	latest, err = s.GetLatestSnapshot(ctx, c.ID)
	if err != nil || latest.ID != last.ID {
		t.Fatalf("retained = %+v, %v", latest, err)
	}
	if err := s.CreateSnapshot(ctx, &SnapshotRecord{ConnectorID: c.ID, Data: "{}", FetchedAt: "invalid"}); err == nil {
		t.Fatal("invalid timestamp accepted")
	}
}

func TestSnapshotUTCDataMigration(t *testing.T) {
	ctx := context.Background()
	s := newDocTestStore(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := RunMigrationsDown(s.rawDB, s.driver, logger); err != nil {
		t.Fatal(err)
	}
	c := ConnectorRecord{Name: "legacy", Category: "virtualization", Type: "proxmox", URL: "https://example.test"}
	if err := s.CreateConnector(ctx, &c); err != nil {
		t.Fatal(err)
	}
	cases := []struct{ input, want string }{
		{"2026-10-25T02:50:00+02:00", "2026-10-25T00:50:00.000000000Z"},
		{"2026-10-25T02:10:00+01:00", "2026-10-25T01:10:00.000000000Z"},
		{"2026-10-25T01:10:00.999999999Z", "2026-10-25T01:10:00.999999999Z"},
		{"2026-10-25T02:10:00.1+01:00", "2026-10-25T01:10:00.100000000Z"},
		{"2026-10-24T22:10:00.000000001-03:00", "2026-10-25T01:10:00.000000001Z"},
		{"2026-10-25T01:10:00Z", "2026-10-25T01:10:00.000000000Z"},
	}
	for _, tc := range cases {
		if _, err := s.db.ExecContext(ctx, "INSERT INTO service_snapshots (id, connector_id, data, fetched_at) VALUES (?, ?, ?, ?)", tc.input, c.ID, "{}", tc.input); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunMigrations(s.rawDB, s.driver, logger); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		sn, err := s.GetSnapshotForConnector(ctx, c.ID, tc.input)
		if err != nil || sn.FetchedAt != tc.want {
			t.Fatalf("migrate %q = %+v, %v; want %q", tc.input, sn, err, tc.want)
		}
	}
}

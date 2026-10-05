package doc

import (
	"context"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestIdentityClustersUseTransitiveStrongMatchesOnly(t *testing.T) {
	members := []store.EntityMemberRecord{
		{ConnectorID: "a", Kind: "vm", Ref: "id-1", Name: "first", ExternalID: "id-1", Hostname: "node.example"},
		{ConnectorID: "b", Kind: "vm", Ref: "id-1", Name: "second", ExternalID: "id-1"},
		{ConnectorID: "c", Kind: "host", Ref: "third", Name: "third", Hostname: "NODE.EXAMPLE"},
		{ConnectorID: "d", Kind: "vm", Ref: "fourth", Name: "fourth"},
	}
	clusters := identityClusters(members)
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2: %+v", len(clusters), clusters)
	}
	if len(clusters[0]) != 3 {
		t.Fatalf("strong-match cluster has %d members, want transitive group of 3: %+v", len(clusters[0]), clusters)
	}
	if len(clusters[1]) != 1 || clusters[1][0].ConnectorID != "d" {
		t.Fatalf("unmatched member should remain independent: %+v", clusters[1])
	}
}

func TestBackfillEntityIdentities(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	first := seedEngineConnectorWithEntities(t, s, "first", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "vm-a", ExternalID: "100", IP: "10.0.0.5", Hostname: "host.example"},
	})
	second := seedEngineConnectorWithEntities(t, s, "second", "networking", "unifi", []connector.SnapshotEntity{
		{Kind: "host", Name: "host-a", Hostname: "HOST.EXAMPLE", IP: "10.0.0.8"},
		{Kind: "device", Name: "ip-only", IP: "10.0.0.5"},
	})
	engine := NewEngine(s)
	count, err := engine.BackfillEntityIdentities(ctx)
	if err != nil || count != 2 {
		t.Fatalf("BackfillEntityIdentities() = %d, %v; want 2, nil", count, err)
	}
	var memberCount, total int
	var idAfterBackfill string
	var firstID, hostID, weakID string
	if err := s.DB().QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ? AND kind = 'vm' AND ref = '100'`, first).Scan(&firstID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ? AND kind = 'host' AND ref = 'host-a'`, second).Scan(&hostID); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ? AND kind = 'device' AND ref = 'ip-only'`, second).Scan(&weakID); err != nil {
		t.Fatal(err)
	}
	if firstID != hostID {
		t.Fatalf("hostname match identities differ: %q != %q", firstID, hostID)
	}
	if weakID == firstID {
		t.Fatal("IP-only match merged identities")
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entity_members`).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities WHERE merged_into IS NULL AND gone_at IS NULL`).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if memberCount != 3 || total != 2 {
		t.Fatalf("members=%d active identities=%d, want 3 and 2", memberCount, total)
	}
	if _, err := engine.BackfillEntityIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.DB().QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ? AND kind = 'vm' AND ref = '100'`, first).Scan(&idAfterBackfill); err != nil {
		t.Fatal(err)
	}
	if idAfterBackfill != firstID {
		t.Fatalf("identity changed on idempotent backfill: %q != %q", idAfterBackfill, firstID)
	}
}

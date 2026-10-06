package doc

import (
	"context"
	"fmt"
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

func TestIdentityClusterTreatsHostnameAsStrongWhenIPAlsoMatches(t *testing.T) {
	a := connector.SnapshotEntity{Kind: "device", Name: "a", IP: "192.0.2.1", Hostname: "node.example"}
	b := connector.SnapshotEntity{Kind: "device", Name: "b", IP: "192.0.2.1", Hostname: "NODE.EXAMPLE"}
	if got := strongMatchReason(a, b); got != "hostname" {
		t.Fatalf("strongMatchReason() = %q, want hostname", got)
	}
	if got := matchReason(a, b); got != "IP address" {
		t.Fatalf("matchReason() = %q, want existing IP-first topology reason", got)
	}
	clusters := identityClusters([]store.EntityMemberRecord{
		{ConnectorID: "a", Kind: a.Kind, Ref: "a", Name: a.Name, Hostname: a.Hostname},
		{ConnectorID: "b", Kind: b.Kind, Ref: "b", Name: b.Name, Hostname: b.Hostname},
	})
	if len(clusters) != 1 || len(clusters[0]) != 2 {
		t.Fatalf("hostname match with shared IP produced clusters: %+v", clusters)
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

func TestBackfillRetainsMembershipWhenSnapshotIsUnreadable(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	connectorID := seedEngineConnectorWithEntities(t, s, "unreadable", "virtualization", "proxmox", []connector.SnapshotEntity{{Kind: "vm", Name: "vm-a", ExternalID: "100"}})
	e := NewEngine(s)
	if _, err := e.BackfillEntityIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := s.DB().QueryRowContext(ctx, `SELECT entity_id FROM entity_members WHERE connector_id = ?`, connectorID).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE service_snapshots SET data = '{' WHERE connector_id = ?`, connectorID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.BackfillEntityIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	var got string
	var memberGone, entityGone bool
	if err := s.DB().QueryRowContext(ctx, `SELECT m.entity_id, m.gone_at IS NOT NULL, e.gone_at IS NOT NULL FROM entity_members m JOIN entities e ON e.id=m.entity_id WHERE m.connector_id=?`, connectorID).Scan(&got, &memberGone, &entityGone); err != nil {
		t.Fatal(err)
	}
	if got != before || memberGone || entityGone {
		t.Fatalf("unreadable snapshot changed membership: id=%q memberGone=%v entityGone=%v", got, memberGone, entityGone)
	}
}

func TestBackfillFirewallStableRefsRetireLegacyMembers(t *testing.T) {
	for _, firewallType := range []string{"opnsense", "pfsense"} {
		t.Run(firewallType, func(t *testing.T) {
			ctx := context.Background()
			s := newEngineTestStore(t)
			id := seedEngineConnectorWithEntities(
				t, s, "firewall", "networking", firewallType,
				[]connector.SnapshotEntity{
					{Kind: "rule", Name: "Allow SSH"},
					{Kind: "rule", Name: "Allow SSH"},
				},
			)
			engine := NewEngine(s)
			if _, err := engine.BackfillEntityIdentities(ctx); err != nil {
				t.Fatal(err)
			}
			addTopologySnapshot(t, s, id, connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{
				{Kind: "rule", Name: "Allow SSH", ExternalID: "rule-1"},
				{Kind: "rule", Name: "Allow SSH", ExternalID: "rule-2"},
			}})
			for range 2 {
				if _, err := engine.BackfillEntityIdentities(ctx); err != nil {
					t.Fatal(err)
				}
			}
			var active, distinct, gone int
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*), COUNT(DISTINCT entity_id) FROM entity_members WHERE connector_id = ? AND gone_at IS NULL`,
				id,
			).Scan(&active, &distinct); err != nil {
				t.Fatal(err)
			}
			if err := s.DB().QueryRowContext(ctx,
				`SELECT COUNT(*) FROM entity_members WHERE connector_id = ? AND ref = 'Allow SSH' AND gone_at IS NOT NULL`,
				id,
			).Scan(&gone); err != nil {
				t.Fatal(err)
			}
			if active != 2 || distinct != 2 || gone != 1 {
				t.Fatalf("active members=%d identities=%d gone legacy members=%d; want 2, 2, 1", active, distinct, gone)
			}
		})
	}
}

func TestBackfillRefChangePreservesMergedIdentityThroughPeer(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	id := seedEngineConnectorWithEntities(
		t, s, "firewall", "networking", "opnsense",
		[]connector.SnapshotEntity{{Kind: "interface", Name: "WAN", Hostname: "wan.example"}},
	)
	peer := seedEngineConnectorWithEntities(
		t, s, "peer", "networking", "test",
		[]connector.SnapshotEntity{{Kind: "interface", Name: "peer", ExternalID: "peer-1", Hostname: "wan.example"}},
	)
	engine := NewEngine(s)
	if _, err := engine.BackfillEntityIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	var original string
	if err := s.DB().QueryRowContext(ctx,
		`SELECT entity_id FROM entity_members WHERE connector_id = ? AND ref = 'WAN'`, id,
	).Scan(&original); err != nil {
		t.Fatal(err)
	}
	addTopologySnapshot(t, s, id, connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{
		{Kind: "interface", Name: "WAN", ExternalID: "igb0", Hostname: "wan.example"},
	}})
	for range 2 {
		if _, err := engine.BackfillEntityIdentities(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var members, identities int
	if err := s.DB().QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(DISTINCT entity_id) FROM entity_members WHERE connector_id IN (?, ?) AND gone_at IS NULL AND entity_id = ?`,
		id, peer, original,
	).Scan(&members, &identities); err != nil {
		t.Fatal(err)
	}
	if members != 2 || identities != 1 {
		t.Fatalf("merged identity %q has %d active members and %d identities; want 2 and 1", original, members, identities)
	}
	var gone bool
	if err := s.DB().QueryRowContext(ctx,
		`SELECT gone_at IS NOT NULL FROM entity_members WHERE connector_id = ? AND ref = 'WAN'`, id,
	).Scan(&gone); err != nil {
		t.Fatal(err)
	}
	if !gone {
		t.Fatal("legacy member remains active after reference change")
	}
}

func TestBackfillFirewallLocalIDsMatchOnlyWithinSource(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	for _, kind := range []string{"interface", "rule"} {
		for i, source := range []string{"https://one.test", "https://one.test/", "https://two.test"} {
			seedEngineConnectorWithEntities(
				t, s, fmt.Sprintf("%s-%d", kind, i), "networking", "pfsense",
				[]connector.SnapshotEntity{{
					Kind: kind, Name: "same name",
					ExternalID: connector.ScopedExternalID("pfsense", source, "same-local-key"),
				}},
			)
		}
	}
	if _, err := NewEngine(s).BackfillEntityIdentities(ctx); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"interface", "rule"} {
		var members, identities int
		if err := s.DB().QueryRowContext(ctx,
			`SELECT COUNT(*), COUNT(DISTINCT entity_id) FROM entity_members WHERE kind = ? AND gone_at IS NULL`, kind,
		).Scan(&members, &identities); err != nil {
			t.Fatal(err)
		}
		if members != 3 || identities != 2 {
			t.Fatalf("%s members=%d identities=%d, want 3 observations across 2 sources", kind, members, identities)
		}
	}
}

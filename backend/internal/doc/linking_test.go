package doc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func seedEngineConnectorWithEntities(t *testing.T, s *store.Store, name, category, connectorType string, entities []connector.SnapshotEntity) string {
	t.Helper()
	ctx := context.Background()
	record := &store.ConnectorRecord{Name: name, Category: category, Type: connectorType, URL: "https://example.test"}
	if err := s.CreateConnector(ctx, record); err != nil {
		t.Fatalf("create connector: %v", err)
	}
	data, err := json.Marshal(connector.ServiceSnapshot{
		ServiceName: name,
		Type:        connectorType,
		Entities:    entities,
		FetchedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: record.ID, Data: string(data)}); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	return record.ID
}

func TestMatchEntitiesExternalIDPrecedence(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.5"},
	})
	connectorID := seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", nil)

	links, err := matchEntities(ctx, s, newSnapshotCache(s), connectorID, []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.99"},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 1 || links[0].Reason != "external ID" {
		t.Fatalf("matchEntities() = %+v, want single external ID match", links)
	}
}

func TestMatchEntitiesIPPrecedence(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", IP: "10.0.0.5"},
	})
	connectorID := seedEngineConnectorWithEntities(t, s, "pfSense", "networking", "pfsense", nil)

	links, err := matchEntities(ctx, s, newSnapshotCache(s), connectorID, []connector.SnapshotEntity{
		{Kind: "rule", Name: "allow-web", IP: "10.0.0.5"},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 1 || links[0].Reason != "IP address" {
		t.Fatalf("matchEntities() = %+v, want single IP match", links)
	}
}

func TestMatchEntitiesHostnamePrecedenceCaseInsensitive(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Pi-hole", "dns", "pihole", []connector.SnapshotEntity{
		{Kind: "dns_record", Name: "web-01", Hostname: "Web-01.lab.local"},
	})
	connectorID := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", nil)

	links, err := matchEntities(ctx, s, newSnapshotCache(s), connectorID, []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", Hostname: "web-01.lab.local"},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 1 || links[0].Reason != "hostname" {
		t.Fatalf("matchEntities() = %+v, want single hostname match", links)
	}
}

func TestMatchReasonHostnameAndAliases(t *testing.T) {
	tests := []struct {
		name string
		a, b connector.SnapshotEntity
		want string
	}{
		{name: "hostname", a: connector.SnapshotEntity{Hostname: "app.example.test"}, b: connector.SnapshotEntity{Hostname: "app.example.test"}, want: "hostname"},
		{name: "alias on right", a: connector.SnapshotEntity{Hostname: "app.example.test"}, b: connector.SnapshotEntity{Aliases: []string{"app.example.test"}}, want: "hostname"},
		{name: "alias on left", a: connector.SnapshotEntity{Aliases: []string{"app.example.test"}}, b: connector.SnapshotEntity{Hostname: "app.example.test"}, want: "hostname"},
		{name: "alias to alias", a: connector.SnapshotEntity{Aliases: []string{"app.example.test"}}, b: connector.SnapshotEntity{Aliases: []string{"APP.EXAMPLE.TEST"}}, want: "hostname"},
		{name: "matching aliases with different hostnames", a: connector.SnapshotEntity{Hostname: "app.example.test", Aliases: []string{"shared.example.test"}}, b: connector.SnapshotEntity{Hostname: "other.example.test", Aliases: []string{"SHARED.EXAMPLE.TEST"}}, want: "hostname"},
		{name: "empty hostnames", a: connector.SnapshotEntity{}, b: connector.SnapshotEntity{Aliases: []string{""}}},
		{name: "no hostname match", a: connector.SnapshotEntity{Hostname: "app.example.test"}, b: connector.SnapshotEntity{Aliases: []string{"other.example.test"}}},
		{name: "external ID precedence", a: connector.SnapshotEntity{Kind: "vm", ExternalID: "7", IP: "10.0.0.1", Hostname: "app.example.test"}, b: connector.SnapshotEntity{Kind: "vm", ExternalID: "7", IP: "10.0.0.1", Aliases: []string{"app.example.test"}}, want: "external ID"},
		{name: "IP precedence", a: connector.SnapshotEntity{IP: "10.0.0.1", Hostname: "app.example.test"}, b: connector.SnapshotEntity{IP: "10.0.0.1", Aliases: []string{"app.example.test"}}, want: "IP address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchReason(tt.a, tt.b); got != tt.want {
				t.Errorf("matchReason() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostnameMatchesDoesNotAllocate(t *testing.T) {
	left := connector.SnapshotEntity{Aliases: []string{"api.example.test"}}
	right := connector.SnapshotEntity{Aliases: []string{"API.EXAMPLE.TEST"}}
	matched := false
	if allocations := testing.AllocsPerRun(100, func() {
		matched = hostnameMatches(left, right)
	}); allocations != 0 {
		t.Errorf("hostnameMatches() alias match allocated %v times per run, want 0", allocations)
	}
	if !matched {
		t.Error("hostnameMatches() = false, want alias match")
	}

	empty := connector.SnapshotEntity{}
	matched = true
	if allocations := testing.AllocsPerRun(100, func() {
		matched = hostnameMatches(empty, right)
	}); allocations != 0 {
		t.Errorf("hostnameMatches() empty entity allocated %v times per run, want 0", allocations)
	}
	if matched {
		t.Error("hostnameMatches() = true, want no match for an empty entity")
	}
}

func TestMatchEntitiesNPMProxyHostDomains(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "app-vm", IP: "10.0.0.25"},
	})
	seedEngineConnectorWithEntities(t, s, "Pi-hole", "dns", "pihole", []connector.SnapshotEntity{
		{Kind: "dns_record", Name: "app.example.test", Hostname: "app.example.test"},
		{Kind: "dns_record", Name: "www.example.test", Hostname: "www.example.test"},
	})
	npmID := seedEngineConnectorWithEntities(t, s, "NPM", "networking", "npm", nil)

	links, err := matchEntities(ctx, s, newSnapshotCache(s), npmID, []connector.SnapshotEntity{
		{Kind: "proxy_host", Name: "app.example.test", Hostname: "app.example.test", IP: "10.0.0.25", Aliases: []string{"www.example.test"}},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("matchEntities() returned %d links, want VM IP and both DNS domain links: %+v", len(links), links)
	}
	got := map[string]string{}
	for _, link := range links {
		got[link.Entity.Name] = link.Reason
	}
	for name, reason := range map[string]string{
		"app-vm":           "IP address",
		"app.example.test": "hostname",
		"www.example.test": "hostname",
	} {
		if got[name] != reason {
			t.Errorf("link for %q reason = %q, want %q (all links: %+v)", name, got[name], reason, links)
		}
	}
}

func TestMatchEntitiesFansOutAcrossConnectors(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", []connector.SnapshotEntity{
		{Kind: "container", Name: "web-01", IP: "10.0.0.5"},
	})
	seedEngineConnectorWithEntities(t, s, "Pi-hole", "dns", "pihole", []connector.SnapshotEntity{
		{Kind: "dns_record", Name: "web-01", Hostname: "web-01.lab.local"},
	})
	connectorID := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", nil)

	links, err := matchEntities(ctx, s, newSnapshotCache(s), connectorID, []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", IP: "10.0.0.5", Hostname: "web-01.lab.local"},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("matchEntities() returned %d links, want 2 (one per connector)", len(links))
	}
}

func TestGenerateLabTopologyCreatesThenUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", IP: "10.0.0.5"},
	})
	seedEngineConnectorWithEntities(t, s, "pfSense", "networking", "pfsense", []connector.SnapshotEntity{
		{Kind: "rule", Name: "allow-web", IP: "10.0.0.5"},
	})

	engine := NewEngine(s)

	result, err := engine.GenerateLabTopology(ctx)
	if err != nil {
		t.Fatalf("GenerateLabTopology() error: %v", err)
	}
	if result.Title != labTopologyTitle {
		t.Fatalf("GenerateLabTopology() title = %q, want %q", result.Title, labTopologyTitle)
	}
	if !strings.Contains(result.Content, "```mermaid") {
		t.Fatalf("GenerateLabTopology() content missing mermaid block: %q", result.Content)
	}
	if !strings.Contains(result.Content, "web-01") || !strings.Contains(result.Content, "allow-web") {
		t.Fatalf("GenerateLabTopology() content missing expected entities: %q", result.Content)
	}

	docs, total, err := s.ListAllDocs(ctx, labTopologyTitle, 0, 10)
	if err != nil {
		t.Fatalf("ListAllDocs() error: %v", err)
	}
	if total != 1 || len(docs) != 1 {
		t.Fatalf("ListAllDocs() = %d docs, want exactly 1", total)
	}
	firstDocID := docs[0].ID

	// A second call must update the same doc in place, not create another.
	result2, err := engine.GenerateLabTopology(ctx)
	if err != nil {
		t.Fatalf("GenerateLabTopology() second call error: %v", err)
	}
	if result2.DocID != firstDocID {
		t.Fatalf("GenerateLabTopology() second call DocID = %q, want %q (same doc)", result2.DocID, firstDocID)
	}

	docs2, total2, err := s.ListAllDocs(ctx, labTopologyTitle, 0, 10)
	if err != nil {
		t.Fatalf("ListAllDocs() error: %v", err)
	}
	if total2 != 1 || len(docs2) != 1 {
		t.Fatalf("ListAllDocs() after second call = %d docs, want still exactly 1", total2)
	}
}

func TestMatchEntitiesDedupesExactExternalIDDuplicates(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100", IP: "10.0.0.5"},
	})
	connectorID := seedEngineConnectorWithEntities(t, s, "Docker", "containers_paas", "docker", nil)

	// Two "mine" entities both matching the same external entity by
	// ExternalID+Kind should collapse to a single link.
	links, err := matchEntities(ctx, s, newSnapshotCache(s), connectorID, []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", ExternalID: "100"},
		{Kind: "vm", Name: "web-01-alias", ExternalID: "100"},
	})
	if err != nil {
		t.Fatalf("matchEntities() error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("matchEntities() returned %d links, want 1 deduped link", len(links))
	}
}

func TestSnapshotCacheRefreshesOnNewSnapshot(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	id := seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", nil)
	cache := newSnapshotCache(s)

	first, err := cache.latest(ctx, id)
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	again, _ := cache.latest(ctx, id)
	if first != again {
		t.Fatal("expected cached snapshot to be reused while unchanged")
	}

	data, _ := json.Marshal(connector.ServiceSnapshot{ServiceName: "Proxmox", Entities: []connector.SnapshotEntity{{Kind: "vm", Name: "new"}}})
	if err := s.CreateSnapshot(ctx, &store.SnapshotRecord{ConnectorID: id, Data: string(data), FetchedAt: time.Now().Add(time.Minute).UTC().Format(store.SnapshotTimeFormat)}); err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	next, err := cache.latest(ctx, id)
	if err != nil || len(next.Entities) != 1 {
		t.Fatalf("latest after new snapshot = %+v, %v; want refreshed entities", next, err)
	}
}

func TestGenerateLabTopologyLeavesHumanNoteWithSameTitle(t *testing.T) {
	ctx := context.Background()
	s := newEngineTestStore(t)
	seedEngineConnectorWithEntities(t, s, "Proxmox", "virtualization", "proxmox", []connector.SnapshotEntity{
		{Kind: "vm", Name: "web-01", IP: "10.0.0.5"},
	})
	note := &store.DocRecord{Title: labTopologyTitle, Content: "my notes"}
	if err := s.CreateHumanDoc(ctx, note); err != nil {
		t.Fatalf("CreateHumanDoc() error: %v", err)
	}

	result, err := NewEngine(s).GenerateLabTopology(ctx)
	if err != nil {
		t.Fatalf("GenerateLabTopology() error: %v", err)
	}
	if result.DocID == note.ID {
		t.Fatal("GenerateLabTopology() reused the human note")
	}
	got, err := s.GetDoc(ctx, note.ID)
	if err != nil {
		t.Fatalf("GetDoc() error: %v", err)
	}
	if got.Content != "my notes" || got.Origin != store.DocOriginHuman {
		t.Fatalf("human note changed: origin=%q content=%q", got.Origin, got.Content)
	}
}

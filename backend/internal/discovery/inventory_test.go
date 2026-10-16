package discovery

import (
	"net/netip"
	"reflect"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestInventorySource(t *testing.T) {
	rows := []store.IndexedAddress{
		{ConnectorType: "proxmox", Kind: "container", IP: "10.0.50.12"},
		{ConnectorType: "docker", Kind: "container", IP: "172.17.0.4"},
		{ConnectorType: "portainer", Kind: "container", IP: "172.18.0.4"},
		{ConnectorType: "docker", Kind: "host", IP: "10.0.0.2"},
		{ConnectorType: "unifi", Kind: "device", IP: "192.168.1.5"},
		{IP: "10.0.50.12"}, {IP: "::ffff:10.0.0.2"},
		{IP: "fd00::2"}, {IP: "fd00::1"}, {IP: "100.100.0.5"},
		{IP: "203.0.113.7"}, {IP: "169.254.1.2"}, {IP: "fe80::1%eth0"},
		{IP: "fc00::1"}, {IP: "2001:db8::1"}, {IP: "localhost"}, {IP: "10.0.0.1/24"}, {IP: ""},
	}
	ranges, err := ParseRanges([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	got := InventorySource(rows, ranges)
	want := []netip.Addr{
		netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("10.0.50.12"),
		netip.MustParseAddr("100.100.0.5"), netip.MustParseAddr("fd00::1"), netip.MustParseAddr("fd00::2"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inventory = %v, want %v", got, want)
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	if reversed := InventorySource(rows, ranges); !reflect.DeepEqual(got, reversed) {
		t.Fatalf("unstable order: %v vs %v", got, reversed)
	}
	if got := InventorySource(rows, nil); len(got) != len(want)+1 {
		t.Fatalf("known hosts count = %d, want %d", len(got), len(want)+1)
	}
	if got := InventorySource(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("empty inventory = %v", got)
	}
}

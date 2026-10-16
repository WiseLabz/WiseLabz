package discovery

import (
	"net/netip"
	"sort"

	"github.com/WiseLabz/wiselabz/internal/store"
)

// InventorySource returns allowed, distinct known hosts outside the typed ranges.
// Ordering is stable so truncating the inventory at the budget is deterministic.
func InventorySource(rows []store.IndexedAddress, ranges Ranges) []netip.Addr {
	seen := map[netip.Addr]bool{}
	out := []netip.Addr{}
	for _, row := range rows {
		container := row.Kind == "container" && (row.ConnectorType == "docker" || row.ConnectorType == "portainer")
		if container {
			continue
		}
		addr, err := netip.ParseAddr(row.IP)
		if err != nil {
			continue
		}
		addr = addr.Unmap()
		if !Allowed(addr) || seen[addr] {
			continue
		}
		inside := false
		for _, r := range ranges {
			if r.prefix.Contains(addr) {
				inside = true
				break
			}
		}
		if inside {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out
}

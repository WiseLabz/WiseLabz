package discovery

import (
	"net/netip"
	"sync"
)

// MaxAddresses is the per-scan limit across ranges and admitted single hosts.
const MaxAddresses = 1024

// Source identifies how an address first entered a scan's target set.
type Source string

const (
	// SourceRange is an address contributed by a submitted CIDR.
	SourceRange Source = "range"
	// SourceInventory is an address contributed by the entity index.
	SourceInventory Source = "inventory"
	// SourcePassive is an address learned from a network answer.
	SourcePassive Source = "passive"
)

// TargetSet stores only addresses admitted for a scan. Its methods are safe
// to call while the scanner is probing and passive answers are being added.
type TargetSet struct {
	mu        sync.RWMutex
	addresses map[netip.Addr]Source
}

// NewTargetSet admits the usable hosts of validated typed ranges in order.
func NewTargetSet(ranges Ranges) *TargetSet {
	targets := &TargetSet{addresses: make(map[netip.Addr]Source)}
	for _, addr := range ranges.Hosts() {
		if len(targets.addresses) == MaxAddresses {
			break
		}
		targets.addresses[addr.Unmap()] = SourceRange
	}
	return targets
}

// Contains reports whether addr itself has been admitted. Ranges are expanded
// to usable hosts at construction, so network and broadcast IPs never match.
func (s *TargetSet) Contains(addr netip.Addr) bool {
	if s == nil || !addr.IsValid() || addr.Zone() != "" {
		return false
	}
	s.mu.RLock()
	_, ok := s.addresses[addr.Unmap()]
	s.mu.RUnlock()
	return ok
}

// Admit adds a safe address if it is new and the scan budget has room.
// Duplicate addresses keep their first source and return false.
func (s *TargetSet) Admit(addr netip.Addr, source Source) bool {
	if s == nil || !addr.IsValid() || !validSource(source) {
		return false
	}
	addr = addr.Unmap()

	s.mu.Lock()
	defer s.mu.Unlock()
	if !Allowed(addr) {
		return false
	}
	if _, exists := s.addresses[addr]; exists || len(s.addresses) >= MaxAddresses {
		return false
	}
	if s.addresses == nil {
		s.addresses = make(map[netip.Addr]Source)
	}
	s.addresses[addr] = source
	return true
}

// Source reports the first source that admitted addr, or the empty source.
func (s *TargetSet) Source(addr netip.Addr) Source {
	if s == nil || !addr.IsValid() || addr.Zone() != "" {
		return ""
	}
	s.mu.RLock()
	source := s.addresses[addr.Unmap()]
	s.mu.RUnlock()
	return source
}

// Len returns the number of admitted addresses.
func (s *TargetSet) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	length := len(s.addresses)
	s.mu.RUnlock()
	return length
}

func validSource(source Source) bool {
	switch source {
	case SourceRange, SourceInventory, SourcePassive:
		return true
	default:
		return false
	}
}

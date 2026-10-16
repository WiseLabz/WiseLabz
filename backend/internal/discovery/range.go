// Package discovery finds products and infrastructure hosts known to the
// instance inside a set of explicitly admitted network targets.
package discovery

import (
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const (
	// MinPrefixBits is the broadest accepted range: a scan covers at most a /22.
	MinPrefixBits = 22
	// MaxRanges caps the number of submitted CIDRs.
	MaxRanges = 16
)

// Range rejection reasons are recorded in audit entries.
const (
	ReasonInvalid          = "invalid_cidr"
	ReasonNotIPv4          = "not_ipv4"
	ReasonNotPrivate       = "not_private"
	ReasonTooWide          = "too_wide"
	ReasonTooManyRanges    = "too_many_ranges"
	ReasonTooManyAddresses = "too_many_addresses"
	ReasonNoTargets        = "no_targets"
)

var (
	privateRanges = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	}
	meshRange = netip.MustParsePrefix("100.64.0.0/10")
	ulaRange  = netip.MustParsePrefix("fd00::/8")
)

// RangeError is a rejected scan range. Message is safe to show to the admin.
// Field identifies the request field for API validation responses.
type RangeError struct {
	Reason  string
	Message string
	Field   string
}

func (e *RangeError) Error() string { return e.Message }

// Range is a validated IPv4 range with host bits cleared.
type Range struct {
	prefix netip.Prefix
}

// Ranges preserves the submitted order of validated ranges.
type Ranges []Range

// ParseRange validates one CIDR and remains available to existing callers.
func ParseRange(cidr string) (Range, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return Range{}, &RangeError{Reason: ReasonInvalid, Message: "enter a range in CIDR notation, for example 192.168.1.0/24"}
	}
	if !p.Addr().Is4() {
		return Range{}, &RangeError{Reason: ReasonNotIPv4, Message: "only IPv4 ranges are accepted"}
	}
	p = p.Masked()
	if !inIPv4Range(p) {
		return Range{}, &RangeError{
			Reason:  ReasonNotPrivate,
			Message: "only private ranges are accepted (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 or 100.64.0.0/10)",
		}
	}
	if p.Bits() < MinPrefixBits {
		return Range{}, &RangeError{Reason: ReasonTooWide, Message: fmt.Sprintf("the range may be at most a /%d", MinPrefixBits)}
	}
	return Range{prefix: p}, nil
}

// ParseRanges validates and normalizes a list of CIDRs. An empty list is
// allowed here because the API also accepts scans of known hosts alone.
func ParseRanges(cidrs []string) (Ranges, error) {
	if len(cidrs) > MaxRanges {
		return nil, &RangeError{
			Reason:  ReasonTooManyRanges,
			Message: fmt.Sprintf("at most %d ranges are accepted", MaxRanges),
			Field:   "ranges",
		}
	}

	ranges := make(Ranges, 0, len(cidrs))
	for i, cidr := range cidrs {
		r, err := ParseRange(cidr)
		if err != nil {
			if rangeErr, ok := err.(*RangeError); ok {
				fieldErr := *rangeErr
				fieldErr.Field = fmt.Sprintf("ranges[%d]", i)
				return nil, &fieldErr
			}
			return nil, err
		}
		ranges = append(ranges, r)
	}
	if len(ranges.Hosts()) > MaxAddresses {
		return nil, &RangeError{
			Reason:  ReasonTooManyAddresses,
			Message: fmt.Sprintf("a scan may cover at most %d addresses", MaxAddresses),
			Field:   "ranges",
		}
	}
	return ranges, nil
}

// String returns the range in CIDR notation with host bits cleared.
func (r Range) String() string { return r.prefix.String() }

// Hosts returns the usable addresses in this range. Prefixes of /30 or wider
// skip their network and broadcast addresses; /31 and /32 include every IP.
func (r Range) Hosts() []netip.Addr {
	if !r.prefix.IsValid() || !r.prefix.Addr().Is4() {
		return nil
	}
	var hosts []netip.Addr
	for addr := r.prefix.Addr(); r.prefix.Contains(addr); addr = addr.Next() {
		hosts = append(hosts, addr)
		if !addr.Next().IsValid() {
			break
		}
	}
	if r.prefix.Bits() <= 30 {
		return hosts[1 : len(hosts)-1]
	}
	return hosts
}

// Hosts lists each usable address once, preserving range and host order.
func (rs Ranges) Hosts() []netip.Addr {
	seen := make(map[netip.Addr]struct{})
	var hosts []netip.Addr
	for _, r := range rs {
		for _, addr := range r.Hosts() {
			if _, ok := seen[addr]; ok {
				continue
			}
			seen[addr] = struct{}{}
			hosts = append(hosts, addr)
		}
	}
	return hosts
}

// Strings returns each normalized CIDR in submitted order.
func (rs Ranges) Strings() []string {
	strings := make([]string, 0, len(rs))
	for _, r := range rs {
		strings = append(strings, r.String())
	}
	return strings
}

// Allowed reports whether an individual address may be admitted as a target.
// IPv4-mapped IPv6 addresses are treated as their IPv4 address.
func Allowed(addr netip.Addr) bool {
	if !addr.IsValid() || addr.Zone() != "" {
		return false
	}
	addr = addr.Unmap()
	if connector.IsDangerousIP(net.IP(addr.AsSlice())) {
		return false
	}
	if addr.Is4() {
		for _, block := range privateRanges {
			if block.Contains(addr) {
				return true
			}
		}
		return meshRange.Contains(addr)
	}
	return addr.Is6() && ulaRange.Contains(addr)
}

func inIPv4Range(p netip.Prefix) bool {
	for _, block := range privateRanges {
		if p.Bits() >= block.Bits() && block.Contains(p.Addr()) {
			return true
		}
	}
	return p.Bits() >= meshRange.Bits() && meshRange.Contains(p.Addr())
}

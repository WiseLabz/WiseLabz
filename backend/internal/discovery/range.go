// Package discovery finds the products WiseLabz has connectors for inside a
// private IPv4 range an instance admin submits: range validation, the scanner
// that probes it, and the manager that runs one scan at a time.
package discovery

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
)

// MinPrefixBits is the narrowest accepted prefix length: a scan covers at
// most a /24.
const MinPrefixBits = 24

// Range rejection reasons, carried by RangeError for the API field error and
// the audit entry.
const (
	ReasonInvalid    = "invalid_cidr"
	ReasonNotIPv4    = "not_ipv4"
	ReasonNotPrivate = "not_private"
	ReasonTooWide    = "too_wide"
)

var private = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
}

// RangeError is a rejected scan range. Message is safe to show to the admin.
type RangeError struct {
	Reason  string
	Message string
}

func (e *RangeError) Error() string { return e.Message }

// Range is a validated scan range: IPv4, inside RFC 1918 and no wider than a
// /24, with host bits cleared.
type Range struct {
	prefix netip.Prefix
}

// ParseRange validates a CIDR the admin typed. Host bits are masked, so
// 192.168.1.57/24 covers 192.168.1.0/24.
func ParseRange(cidr string) (Range, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return Range{}, &RangeError{Reason: ReasonInvalid, Message: "enter a range in CIDR notation, for example 192.168.1.0/24"}
	}
	if !p.Addr().Is4() {
		return Range{}, &RangeError{Reason: ReasonNotIPv4, Message: "only IPv4 ranges are accepted"}
	}
	p = p.Masked()
	inPrivate := false
	for _, block := range private {
		if p.Bits() >= block.Bits() && block.Contains(p.Addr()) {
			inPrivate = true
			break
		}
	}
	if !inPrivate {
		return Range{}, &RangeError{Reason: ReasonNotPrivate, Message: "only private ranges are accepted (10.0.0.0/8, 172.16.0.0/12 or 192.168.0.0/16)"}
	}
	if p.Bits() < MinPrefixBits {
		return Range{}, &RangeError{Reason: ReasonTooWide, Message: fmt.Sprintf("the range may be at most a /%d", MinPrefixBits)}
	}
	return Range{prefix: p}, nil
}

// String returns the range in CIDR notation with host bits cleared.
func (r Range) String() string { return r.prefix.String() }

// IPNet returns the range as a *net.IPNet for the range-scoped dialer.
func (r Range) IPNet() *net.IPNet {
	return &net.IPNet{
		IP:   net.IP(r.prefix.Addr().AsSlice()),
		Mask: net.CIDRMask(r.prefix.Bits(), 32),
	}
}

// Hosts lists the addresses to probe. For prefixes of /30 or wider the
// network and broadcast addresses are left out; a /31 and a /32 list every
// address.
func (r Range) Hosts() []netip.Addr {
	if !r.prefix.IsValid() {
		return nil
	}
	size := 1 << (32 - r.prefix.Bits())
	hosts := make([]netip.Addr, 0, size)
	for a := r.prefix.Addr(); r.prefix.Contains(a); a = a.Next() {
		hosts = append(hosts, a)
		if !a.Next().IsValid() {
			break
		}
	}
	if r.prefix.Bits() <= 30 {
		hosts = hosts[1 : len(hosts)-1]
	}
	return hosts
}

package discovery

import (
	"errors"
	"fmt"
	"net/netip"
	"testing"
)

func TestParseRange(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		hosts  int
		reason string
	}{
		{name: "private /22", in: "10.0.0.0/22", want: "10.0.0.0/22", hosts: 1022},
		{name: "private /24", in: "192.168.1.0/24", want: "192.168.1.0/24", hosts: 254},
		{name: "ten /24", in: "10.20.30.0/24", want: "10.20.30.0/24", hosts: 254},
		{name: "172.16/12 /24", in: "172.31.255.0/24", want: "172.31.255.0/24", hosts: 254},
		{name: "mesh /24", in: "100.100.4.0/24", want: "100.100.4.0/24", hosts: 254},
		{name: "host bits set", in: "192.168.1.57/24", want: "192.168.1.0/24", hosts: 254},
		{name: "padded", in: " 192.168.1.0/24 ", want: "192.168.1.0/24", hosts: 254},
		{name: "/25", in: "10.0.0.128/25", want: "10.0.0.128/25", hosts: 126},
		{name: "/30", in: "10.0.0.4/30", want: "10.0.0.4/30", hosts: 2},
		{name: "/31", in: "10.0.0.4/31", want: "10.0.0.4/31", hosts: 2},
		{name: "/32", in: "10.0.0.4/32", want: "10.0.0.4/32", hosts: 1},
		{name: "too wide /21", in: "10.0.0.0/21", reason: ReasonTooWide},
		{name: "too wide /8", in: "10.0.0.0/8", reason: ReasonTooWide},
		{name: "wider than its block", in: "10.0.0.0/7", reason: ReasonNotPrivate},
		{name: "wider than 172.16/12", in: "172.16.0.0/11", reason: ReasonNotPrivate},
		{name: "wider than 192.168/16", in: "192.168.0.0/15", reason: ReasonNotPrivate},
		{name: "wider than mesh block", in: "100.64.0.0/9", reason: ReasonNotPrivate},
		{name: "public", in: "8.8.8.0/24", reason: ReasonNotPrivate},
		{name: "just outside 172.16/12", in: "172.32.0.0/24", reason: ReasonNotPrivate},
		{name: "just outside 172.16/12 below", in: "172.15.255.0/24", reason: ReasonNotPrivate},
		{name: "loopback", in: "127.0.0.0/24", reason: ReasonNotPrivate},
		{name: "link-local", in: "169.254.169.0/24", reason: ReasonNotPrivate},
		{name: "ipv6", in: "fd00::/120", reason: ReasonNotIPv4},
		{name: "ipv4-mapped ipv6", in: "::ffff:192.168.1.0/120", reason: ReasonNotIPv4},
		{name: "zone", in: "192.168.1.0%eth0/24", reason: ReasonInvalid},
		{name: "leading zero octet", in: "192.168.01.0/24", reason: ReasonInvalid},
		{name: "prefix over 32", in: "192.168.1.0/33", reason: ReasonInvalid},
		{name: "negative prefix", in: "192.168.1.0/-1", reason: ReasonInvalid},
		{name: "bare address", in: "192.168.1.1", reason: ReasonInvalid},
		{name: "garbage", in: "not a range", reason: ReasonInvalid},
		{name: "empty", in: "", reason: ReasonInvalid},
		{name: "host name", in: "router.lan/24", reason: ReasonInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRange(tc.in)
			if tc.reason != "" {
				var rangeErr *RangeError
				if !errors.As(err, &rangeErr) || rangeErr.Reason != tc.reason {
					t.Fatalf("ParseRange(%q) err = %v, want reason %q", tc.in, err, tc.reason)
				}
				if rangeErr.Message == "" {
					t.Error("empty message")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRange(%q): %v", tc.in, err)
			}
			if got := r.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
			hosts := r.Hosts()
			if len(hosts) != tc.hosts {
				t.Fatalf("len(Hosts()) = %d, want %d", len(hosts), tc.hosts)
			}
			for _, host := range hosts {
				if !r.prefix.Contains(host) {
					t.Errorf("host %s outside %s", host, r)
				}
			}
		})
	}
}

func TestParseRangesEmptyAndFields(t *testing.T) {
	ranges, err := ParseRanges(nil)
	if err != nil || len(ranges) != 0 {
		t.Fatalf("ParseRanges(nil) = %v, %v; want empty ranges and no error", ranges, err)
	}

	_, err = ParseRanges([]string{"192.168.1.0/24", "8.8.8.0/24"})
	var rangeErr *RangeError
	if !errors.As(err, &rangeErr) {
		t.Fatalf("ParseRanges invalid entry error = %v, want RangeError", err)
	}
	if rangeErr.Field != "ranges[1]" || rangeErr.Reason != ReasonNotPrivate {
		t.Fatalf("RangeError = {field:%q reason:%q}, want ranges[1] / %s", rangeErr.Field, rangeErr.Reason, ReasonNotPrivate)
	}
}

func TestParseRangesLimitAndAddressBudget(t *testing.T) {
	sixteen := make([]string, MaxRanges)
	for i := range sixteen {
		sixteen[i] = fmt.Sprintf("10.1.0.%d/32", i+1)
	}
	if ranges, err := ParseRanges(sixteen); err != nil || len(ranges) != MaxRanges {
		t.Fatalf("ParseRanges(16) = %d ranges, %v", len(ranges), err)
	}
	seventeen := append(append([]string(nil), sixteen...), "10.1.1.1/32")
	_, err := ParseRanges(seventeen)
	var rangeErr *RangeError
	if !errors.As(err, &rangeErr) || rangeErr.Reason != ReasonTooManyRanges || rangeErr.Field != "ranges" {
		t.Fatalf("ParseRanges(17) error = %#v, want too_many_ranges on ranges", err)
	}

	addressBudget := []string{
		"192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24", "192.168.3.0/24",
	}
	for i := 1; i <= 8; i++ {
		addressBudget = append(addressBudget, fmt.Sprintf("10.10.0.%d/32", i))
	}
	ranges, err := ParseRanges(addressBudget)
	if err != nil {
		t.Fatalf("ParseRanges(1024 targets): %v", err)
	}
	if got := len(ranges.Hosts()); got != MaxAddresses {
		t.Fatalf("hosts at limit = %d, want %d", got, MaxAddresses)
	}
	addressBudget = append(addressBudget, "10.10.0.9/32")
	_, err = ParseRanges(addressBudget)
	if !errors.As(err, &rangeErr) || rangeErr.Reason != ReasonTooManyAddresses || rangeErr.Field != "ranges" {
		t.Fatalf("ParseRanges(1025 targets) error = %#v, want too_many_addresses on ranges", err)
	}
}

func TestRangesHostsDeduplicateInAdmissionOrder(t *testing.T) {
	ranges, err := ParseRanges([]string{"192.168.1.0/24", "192.168.1.128/25"})
	if err != nil {
		t.Fatal(err)
	}
	hosts := ranges.Hosts()
	if len(hosts) != 254 || hosts[0].String() != "192.168.1.1" || hosts[253].String() != "192.168.1.254" {
		t.Fatalf("Hosts() length/order = %d, %v .. %v", len(hosts), hosts[0], hosts[len(hosts)-1])
	}
	wantStrings := []string{"192.168.1.0/24", "192.168.1.128/25"}
	gotStrings := ranges.Strings()
	if len(gotStrings) != len(wantStrings) {
		t.Fatalf("Strings() = %v, want %v", gotStrings, wantStrings)
	}
	for i := range wantStrings {
		if gotStrings[i] != wantStrings[i] {
			t.Fatalf("Strings() = %v, want %v", gotStrings, wantStrings)
		}
	}
}

func TestAllowedAddressClasses(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"10.20.30.40", true},
		{"172.31.255.1", true},
		{"192.168.1.1", true},
		{"100.64.0.1", true},
		{"100.63.255.255", false},
		{"100.128.0.1", false},
		{"100.127.255.254", true},
		{"fd00::1", true},
		{"fc00::1", false},
		{"fd00::1%eth0", false},
		{"fdff::1", true},
		{"::ffff:192.168.1.1", true},
		{"::ffff:100.64.0.1", true},
		{"8.8.8.8", false},
		{"2001:4860:4860::8888", false},
		{"127.0.0.1", false},
		{"::ffff:127.0.0.1", false},
		{"169.254.169.254", false},
		{"224.0.0.1", false},
		{"0.0.0.0", false},
		{"fe80::1", false},
		{"ff02::1", false},
		{"::", false},
		{"::ffff:8.8.8.8", false},
	}
	for _, tc := range tests {
		t.Run(tc.address, func(t *testing.T) {
			addr, err := netip.ParseAddr(tc.address)
			if err != nil {
				t.Fatal(err)
			}
			if got := Allowed(addr); got != tc.want {
				t.Errorf("Allowed(%s) = %t, want %t", addr, got, tc.want)
			}
		})
	}
	if Allowed(netip.Addr{}) {
		t.Error("Allowed(invalid address) = true")
	}
}

func TestRangeHostsSkipNetworkAndBroadcast(t *testing.T) {
	r, err := ParseRange("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	hosts := r.Hosts()
	if got := hosts[0].String(); got != "192.168.1.1" {
		t.Errorf("first = %s, want 192.168.1.1", got)
	}
	if got := hosts[len(hosts)-1].String(); got != "192.168.1.254" {
		t.Errorf("last = %s, want 192.168.1.254", got)
	}
}

func TestZeroRangeHasNoHosts(t *testing.T) {
	if hosts := (Range{}).Hosts(); len(hosts) != 0 {
		t.Errorf("zero Range hosts = %v", hosts)
	}
}

func TestRangeHostCountsFrom22Through32(t *testing.T) {
	for bits := 22; bits <= 32; bits++ {
		r, err := ParseRange(fmt.Sprintf("10.0.0.0/%d", bits))
		if err != nil {
			t.Fatal(err)
		}
		want := 1 << (32 - bits)
		if bits <= 30 {
			want -= 2
		}
		if got := len(r.Hosts()); got != want {
			t.Errorf("/%d hosts = %d, want %d", bits, got, want)
		}
	}
}

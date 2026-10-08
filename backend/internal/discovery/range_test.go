package discovery

import (
	"errors"
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
		{name: "private /24", in: "192.168.1.0/24", want: "192.168.1.0/24", hosts: 254},
		{name: "ten /24", in: "10.20.30.0/24", want: "10.20.30.0/24", hosts: 254},
		{name: "172.16/12 /24", in: "172.31.255.0/24", want: "172.31.255.0/24", hosts: 254},
		{name: "host bits set", in: "192.168.1.57/24", want: "192.168.1.0/24", hosts: 254},
		{name: "padded", in: " 192.168.1.0/24 ", want: "192.168.1.0/24", hosts: 254},
		{name: "/25", in: "10.0.0.128/25", want: "10.0.0.128/25", hosts: 126},
		{name: "/30", in: "10.0.0.4/30", want: "10.0.0.4/30", hosts: 2},
		{name: "/31", in: "10.0.0.4/31", want: "10.0.0.4/31", hosts: 2},
		{name: "/32", in: "10.0.0.4/32", want: "10.0.0.4/32", hosts: 1},
		{name: "too wide /23", in: "10.0.0.0/23", reason: ReasonTooWide},
		{name: "too wide /8", in: "10.0.0.0/8", reason: ReasonTooWide},
		{name: "wider than its block", in: "10.0.0.0/7", reason: ReasonNotPrivate},
		{name: "public", in: "8.8.8.0/24", reason: ReasonNotPrivate},
		{name: "just outside 172.16/12", in: "172.32.0.0/24", reason: ReasonNotPrivate},
		{name: "just outside 172.16/12 below", in: "172.15.255.0/24", reason: ReasonNotPrivate},
		{name: "loopback", in: "127.0.0.0/24", reason: ReasonNotPrivate},
		{name: "link-local", in: "169.254.169.0/24", reason: ReasonNotPrivate},
		{name: "cgnat", in: "100.64.0.0/24", reason: ReasonNotPrivate},
		{name: "ipv6", in: "fd00::/120", reason: ReasonNotIPv4},
		{name: "ipv4-mapped ipv6", in: "::ffff:192.168.1.0/120", reason: ReasonNotIPv4},
		{name: "bare address", in: "192.168.1.1", reason: ReasonInvalid},
		{name: "garbage", in: "not a range", reason: ReasonInvalid},
		{name: "empty", in: "", reason: ReasonInvalid},
		{name: "host name", in: "router.lan/24", reason: ReasonInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRange(tc.in)
			if tc.reason != "" {
				var re *RangeError
				if !errors.As(err, &re) || re.Reason != tc.reason {
					t.Fatalf("ParseRange(%q) err = %v, want reason %q", tc.in, err, tc.reason)
				}
				if re.Message == "" {
					t.Error("empty message")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRange(%q): %v", tc.in, err)
			}
			if r.String() != tc.want {
				t.Errorf("String() = %q, want %q", r.String(), tc.want)
			}
			hosts := r.Hosts()
			if len(hosts) != tc.hosts {
				t.Fatalf("len(Hosts()) = %d, want %d", len(hosts), tc.hosts)
			}
			for _, h := range hosts {
				if !r.IPNet().Contains(h.AsSlice()) {
					t.Errorf("host %s outside %s", h, r)
				}
			}
		})
	}
}

func TestHostsSkipNetworkAndBroadcast(t *testing.T) {
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

package connector

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"
)

type dialerTargetSet struct {
	prefixes  []netip.Prefix
	addresses map[netip.Addr]struct{}
}

func (s *dialerTargetSet) Contains(addr netip.Addr) bool {
	addr = addr.Unmap()
	if _, ok := s.addresses[addr]; ok {
		return true
	}
	for _, prefix := range s.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func dialerSet(prefixes []string, addresses ...string) *dialerTargetSet {
	set := &dialerTargetSet{addresses: make(map[netip.Addr]struct{})}
	for _, prefix := range prefixes {
		set.prefixes = append(set.prefixes, netip.MustParsePrefix(prefix))
	}
	for _, address := range addresses {
		set.addresses[netip.MustParseAddr(address)] = struct{}{}
	}
	return set
}

func TestGuardedDialerForSetControl(t *testing.T) {
	set := dialerSet([]string{"192.168.1.0/24"}, "10.0.0.5")
	tests := []struct {
		name    string
		addr    string
		blocked bool
		other   bool // a non-BlockedAddressError failure
	}{
		{name: "prefix member", addr: "192.168.1.5:8006"},
		{name: "single address member", addr: "10.0.0.5:8006"},
		{name: "prefix edge", addr: "192.168.1.254:80"},
		{name: "private but outside set", addr: "192.168.2.5:8006", blocked: true},
		{name: "other private block", addr: "172.16.0.5:8006", blocked: true},
		{name: "public", addr: "8.8.8.8:443", blocked: true},
		{name: "loopback in set", addr: "127.0.0.1:80", blocked: true},
		{name: "link-local in set", addr: "169.254.169.254:80", blocked: true},
		{name: "unspecified in set", addr: "0.0.0.0:80", blocked: true},
		{name: "multicast in set", addr: "224.0.0.1:80", blocked: true},
		{name: "hostname", addr: "example.com:80", other: true},
		{name: "no port", addr: "192.168.1.5", other: true},
	}
	set.addresses[netip.MustParseAddr("127.0.0.1")] = struct{}{}
	set.addresses[netip.MustParseAddr("169.254.169.254")] = struct{}{}
	set.addresses[netip.MustParseAddr("0.0.0.0")] = struct{}{}
	set.addresses[netip.MustParseAddr("224.0.0.1")] = struct{}{}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := GuardedDialerForSet(time.Second, set).Control("tcp", tc.addr, nil)
			var blocked *BlockedAddressError
			switch {
			case tc.blocked:
				if !errors.As(err, &blocked) {
					t.Fatalf("Control(%s) = %v, want BlockedAddressError", tc.addr, err)
				}
			case tc.other:
				if err == nil || errors.As(err, &blocked) {
					t.Fatalf("Control(%s) = %v, want a non-blocked error", tc.addr, err)
				}
			default:
				if err != nil {
					t.Fatalf("Control(%s) = %v, want nil", tc.addr, err)
				}
			}
		})
	}
}

func TestGuardedDialerForSetSeesLateAdmission(t *testing.T) {
	set := dialerSet(nil)
	d := GuardedDialerForSet(time.Second, set)
	if err := d.Control("tcp", "10.0.0.5:80", nil); !isBlockedAddress(err) {
		t.Fatalf("before admission Control = %v, want BlockedAddressError", err)
	}
	set.addresses[netip.MustParseAddr("10.0.0.5")] = struct{}{}
	if err := d.Control("tcp", "10.0.0.5:80", nil); err != nil {
		t.Fatalf("after admission Control = %v, want nil", err)
	}
}

func TestGuardedDialerForSetNilBlocksEverything(t *testing.T) {
	d := GuardedDialerForSet(time.Second, nil)
	if err := d.Control("tcp", "192.168.1.5:80", nil); !isBlockedAddress(err) {
		t.Fatalf("Control with nil set = %v, want BlockedAddressError", err)
	}
}

func TestGuardedDialerForSetLoopbackTestAllowanceDoesNotSkipSetCheck(t *testing.T) {
	AllowLoopbackForTest(t)
	d := GuardedDialerForSet(time.Second, dialerSet(nil))
	if err := d.Control("tcp", "127.0.0.1:80", nil); !isBlockedAddress(err) {
		t.Fatalf("loopback outside set with test allowance = %v, want BlockedAddressError", err)
	}

	set := dialerSet(nil, "127.0.0.1")
	if err := GuardedDialerForSet(time.Second, set).Control("tcp", "127.0.0.1:80", nil); err != nil {
		t.Fatalf("loopback in set with explicit test allowance = %v, want nil", err)
	}
}

func TestGuardedDialerForSetDialsInSet(t *testing.T) {
	AllowLoopbackForTest(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		if conn, err := ln.Accept(); err == nil {
			_ = conn.Close()
		}
	}()

	addr := ln.Addr().(*net.TCPAddr).IP.String()
	conn, err := GuardedDialerForSet(time.Second, dialerSet(nil, addr)).Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial in set: %v", err)
	}
	_ = conn.Close()

	_, err = GuardedDialerForSet(time.Second, dialerSet(nil, "127.0.0.2")).Dial("tcp", ln.Addr().String())
	if !isBlockedAddress(err) {
		t.Fatalf("dial out of set = %v, want BlockedAddressError", err)
	}
}

func isBlockedAddress(err error) bool {
	var blocked *BlockedAddressError
	return errors.As(err, &blocked)
}

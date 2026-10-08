package connector

import (
	"errors"
	"net"
	"testing"
	"time"
)

func mustCIDR(t *testing.T, s string) *net.IPNet {
	t.Helper()
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		t.Fatalf("ParseCIDR(%q): %v", s, err)
	}
	return n
}

func TestGuardedDialerForRangeControl(t *testing.T) {
	tests := []struct {
		name    string
		allowed string
		addr    string
		blocked bool
		other   bool // a non-BlockedAddressError failure
	}{
		{name: "in range", allowed: "192.168.1.0/24", addr: "192.168.1.5:8006"},
		{name: "range edge", allowed: "192.168.1.0/24", addr: "192.168.1.254:80"},
		{name: "private but out of range", allowed: "192.168.1.0/24", addr: "192.168.2.5:8006", blocked: true},
		{name: "other private block", allowed: "192.168.1.0/24", addr: "10.0.0.5:8006", blocked: true},
		{name: "public", allowed: "192.168.1.0/24", addr: "8.8.8.8:443", blocked: true},
		{name: "loopback inside hypothetical range", allowed: "127.0.0.0/8", addr: "127.0.0.1:80", blocked: true},
		{name: "link-local inside hypothetical range", allowed: "169.254.0.0/16", addr: "169.254.169.254:80", blocked: true},
		{name: "unspecified inside hypothetical range", allowed: "0.0.0.0/8", addr: "0.0.0.0:80", blocked: true},
		{name: "multicast inside hypothetical range", allowed: "224.0.0.0/4", addr: "224.0.0.1:80", blocked: true},
		{name: "host name", allowed: "192.168.1.0/24", addr: "example.com:80", other: true},
		{name: "no port", allowed: "192.168.1.0/24", addr: "192.168.1.5", other: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := GuardedDialerForRange(time.Second, mustCIDR(t, tc.allowed))
			err := d.Control("tcp", tc.addr, nil)
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

func TestGuardedDialerForRangeNilRangeBlocksEverything(t *testing.T) {
	d := GuardedDialerForRange(time.Second, nil)
	var blocked *BlockedAddressError
	if err := d.Control("tcp", "192.168.1.5:80", nil); !errors.As(err, &blocked) {
		t.Fatalf("Control with nil range = %v, want BlockedAddressError", err)
	}
}

func TestGuardedDialerForRangeLoopbackTestAllowance(t *testing.T) {
	AllowLoopbackForTest(t)
	d := GuardedDialerForRange(time.Second, mustCIDR(t, "127.0.0.0/8"))
	if err := d.Control("tcp", "127.0.0.1:80", nil); err != nil {
		t.Fatalf("loopback in range with test allowance = %v, want nil", err)
	}
	// The allowance relaxes the loopback block only, never the range check.
	var blocked *BlockedAddressError
	if err := d.Control("tcp", "192.168.1.5:80", nil); !errors.As(err, &blocked) {
		t.Fatalf("out-of-range with test allowance = %v, want BlockedAddressError", err)
	}
}

func TestGuardedDialerForRangeDialsInRange(t *testing.T) {
	AllowLoopbackForTest(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()

	conn, err := GuardedDialerForRange(time.Second, mustCIDR(t, "127.0.0.0/24")).Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial in range: %v", err)
	}
	_ = conn.Close()

	_, err = GuardedDialerForRange(time.Second, mustCIDR(t, "10.0.0.0/24")).Dial("tcp", ln.Addr().String())
	var blocked *BlockedAddressError
	if !errors.As(err, &blocked) {
		t.Fatalf("dial out of range = %v, want BlockedAddressError", err)
	}
}

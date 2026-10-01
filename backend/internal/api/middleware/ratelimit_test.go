package middleware

import (
	"strconv"
	"testing"
	"time"
)

func TestIPKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"203.0.113.7":                      "203.0.113.7",
		"2001:db8:1:2:aaaa:bbbb:cccc:dddd": "2001:db8:1:2::/64",
		"2001:db8:1:2:1111:2222:3333:4444": "2001:db8:1:2::/64",
		"::ffff:203.0.113.7":               "203.0.113.7",
		"not-an-ip":                        "not-an-ip",
	}
	for in, want := range cases {
		if got := IPKey(in); got != want {
			t.Errorf("IPKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLimiterStoreIsBounded(t *testing.T) {
	t.Parallel()
	l := &limiterStore{limiters: make(map[string]*visitor), rate: 1, burst: 1}
	for i := 0; i < maxLimiters; i++ {
		l.limiters[strconv.Itoa(i)] = &visitor{lastSeen: time.Now()}
	}
	n := len(l.limiters)
	if l.allow("brand-new-key") {
		t.Error("new key should be rejected once the map is full")
	}
	if len(l.limiters) != n {
		t.Errorf("map grew past cap: %d -> %d", n, len(l.limiters))
	}
}

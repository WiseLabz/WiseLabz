package caddy

import "testing"

func TestRouteHostsUsesFirstHostMatcher(t *testing.T) {
	got := routeHosts([]routeMatch{{}, {Host: []string{"primary", "alias"}}, {Host: []string{"ignored"}}})
	if len(got) != 2 || got[0] != "primary" || got[1] != "alias" {
		t.Fatalf("routeHosts = %v", got)
	}
}

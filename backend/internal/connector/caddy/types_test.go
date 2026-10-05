package caddy

import "testing"

func TestRouteHostsUsesFirstHostMatcher(t *testing.T) {
	got := routeHosts([]routeMatch{{}, {Host: []string{"primary", "alias"}}, {Host: []string{"ignored"}}})
	if len(got) != 2 || got[0] != "primary" || got[1] != "alias" {
		t.Fatalf("routeHosts = %v", got)
	}
}

func TestRoutePathsIncludesNestedSubrouteMatchers(t *testing.T) {
	paths := routePaths([]routeMatch{{Path: []string{"/root"}}}, []handler{{Handler: "subroute", Routes: []httpRoute{{Match: []routeMatch{{Path: []string{"/nested"}}}}}}}, 0)
	if len(paths) != 2 || paths[0] != "/nested" || paths[1] != "/root" {
		t.Fatalf("routePaths = %v", paths)
	}
}

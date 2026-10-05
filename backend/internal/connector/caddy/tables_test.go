package caddy

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildTablesAndHostIdentity(t *testing.T) {
	parsed, err := parseConfig([]byte(configFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.servers) != 1 || len(parsed.routes) != 3 || len(parsed.subjects) != 2 {
		t.Fatalf("parsed = %+v", parsed)
	}
	var routeEntity = parsed.entities[1]
	if routeEntity.Hostname != "api.example.com" || routeEntity.IP != "" {
		t.Fatalf("route entity = %+v", routeEntity)
	}
	for _, entity := range parsed.entities {
		if entity.Hostname == "www.example.com" {
			if len(entity.Aliases) != 1 || entity.Aliases[0] != "example.com" || entity.IP != "10.0.0.8" {
				t.Fatalf("route entity = %+v", entity)
			}
			if entity.Attributes["upstream"] != "10.0.0.8:8080, api.internal:8080, db.internal:5432" {
				t.Errorf("upstream attribute = %v", entity.Attributes["upstream"])
			}
		}
	}
	serverTable, routeTable, tlsTable := tables(parsed)
	if !strings.Contains(serverTable, "srv0") || !strings.Contains(routeTable, "10.0.0.8:8080, api.internal:8080") || !strings.Contains(tlsTable, "www.example.com") {
		t.Fatalf("tables missing parsed values: %s %s %s", serverTable, routeTable, tlsTable)
	}
}

func TestMalformedCaddyJSON(t *testing.T) {
	for _, raw := range []string{"", "[]", `"string"`} {
		if _, err := parseConfig([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

func TestAnalyzeUpstreamDials(t *testing.T) {
	tests := []struct {
		dial, host, port, dependency string
		isIP                         bool
	}{
		{"unix//run/app.sock", "", "", "", false},
		{"tcp/api.internal:80", "api.internal", "80", "api.internal", false},
		{"h2c://api.internal:8080", "api.internal", "8080", "api.internal", false},
		{"{env.BACKEND}:80", "", "", "", false},
		{"[2001:db8::1]:80", "2001:db8::1", "80", "", true},
		{"[::1]:80", "::1", "80", "", true},
		{"localhost:8080", "localhost", "8080", "localhost", false},
		{"range.internal:8000-8010", "range.internal", "8000-8010", "range.internal", false},
	}
	for _, test := range tests {
		t.Run(test.dial, func(t *testing.T) {
			host, port, dependency, isIP := analyzeDial(test.dial)
			if host != test.host || port != test.port || dependency != test.dependency || isIP != test.isIP {
				t.Fatalf("analyzeDial(%q) = %q, %q, %q, %v", test.dial, host, port, dependency, isIP)
			}
		})
	}
}

func TestRouteUpstreamsCollectsNestedHandlersAndCapsDepth(t *testing.T) {
	handlers := []handler{{Handler: "subroute", Routes: []httpRoute{
		{Handle: []handler{{Handler: "reverse_proxy", Upstreams: []upstream{{Dial: "one.internal:80"}, {Dial: "[2001:db8::1]:443"}}}}},
		{Handle: []handler{{Handler: "reverse_proxy", Upstreams: []upstream{{Dial: "two.internal:90"}}}}},
	}}}
	got := routeUpstreams(handlers, 0)
	want := []string{"one.internal:80", "[2001:db8::1]:443", "two.internal:90"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routeUpstreams = %v, want %v", got, want)
	}
	deep := []handler{{Handler: "reverse_proxy", Upstreams: []upstream{{Dial: "too.deep:80"}}}}
	for i := 0; i <= maxRouteDepth; i++ {
		deep = []handler{{Handler: "subroute", Routes: []httpRoute{{Handle: deep}}}}
	}
	if got := routeUpstreams(deep, 0); len(got) != 0 {
		t.Fatalf("depth limit returned %v", got)
	}
}

func TestHostMatcherDeduplicatesAliasesAndDerivesStableIDs(t *testing.T) {
	firstJSON := `{"apps":{"http":{"servers":{"s":{"routes":[{"match":[{"host":["app.test","app.test","www.test"],"path":["/a/*"]}],"handle":[]},{"match":[{"host":["app.test","www.test"],"path":["/b/*"]}],"handle":[]}]}}}}}`
	secondJSON := `{"apps":{"http":{"servers":{"s":{"routes":[{"match":[{"host":["app.test","www.test"],"path":["/b/*"]}],"handle":[]},{"match":[{"host":["app.test","app.test","www.test"],"path":["/a/*"]}],"handle":[]}]}}}}}`
	first, err := parseConfig([]byte(firstJSON))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseConfig([]byte(secondJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.entities, second.entities) || !reflect.DeepEqual(first.routes, second.routes) {
		t.Fatalf("reordered routes changed entities or routes:\n%+v\n%+v", first, second)
	}
	if first.entities[1].Hostname != "app.test" || !reflect.DeepEqual(first.entities[1].Aliases, []string{"www.test"}) {
		t.Fatalf("deduplicated host entity = %+v", first.entities[1])
	}
	if first.entities[1].ExternalID == first.entities[2].ExternalID {
		t.Fatal("distinct path matches share an external ID")
	}
}

func TestCollidingHostPathRoutesGetStableDisambiguators(t *testing.T) {
	firstJSON := `{"apps":{"http":{"servers":{"s":{"routes":[{"match":[{"host":["app.test"],"path":["/same"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"one.internal:80"}]}]},{"match":[{"host":["app.test"],"path":["/same"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"two.internal:80"}]}]}]}}}}}`
	secondJSON := `{"apps":{"http":{"servers":{"s":{"routes":[{"match":[{"host":["app.test"],"path":["/same"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"two.internal:80"}]}]},{"match":[{"host":["app.test"],"path":["/same"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"one.internal:80"}]}]}]}}}}}`
	first, err := parseConfig([]byte(firstJSON))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseConfig([]byte(secondJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.entities, second.entities) || !reflect.DeepEqual(first.deps, second.deps) {
		t.Fatalf("reordered colliding routes changed identities or dependencies:\n%+v\n%+v", first, second)
	}
	if first.routes[0].externalID == first.routes[1].externalID || !strings.HasSuffix(first.routes[0].externalID, ":1") || !strings.HasSuffix(first.routes[1].externalID, ":2") {
		t.Fatalf("collision IDs = %q, %q", first.routes[0].externalID, first.routes[1].externalID)
	}
}

func TestRoutesWithoutHostUseIndexIdentity(t *testing.T) {
	parsed, err := parseConfig([]byte(`{"apps":{"http":{"servers":{"srv":{"routes":[{"handle":[]}]}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	entity := parsed.entities[1]
	if entity.Name != "srv route 1" || entity.Hostname != "" || entity.ExternalID != "srv:0" {
		t.Fatalf("hostless route = %+v", entity)
	}
}

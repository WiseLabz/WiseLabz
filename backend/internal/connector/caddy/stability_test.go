package caddy

import (
	"bytes"
	"reflect"
	"testing"
)

func TestConfigParsingIsStable(t *testing.T) {
	first, err := parseConfig([]byte(configFixture))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseConfig([]byte(configFixture))
	if err != nil {
		t.Fatal(err)
	}
	firstServers, firstRoutes, firstTLS := tables(first)
	secondServers, secondRoutes, secondTLS := tables(second)
	if !bytes.Equal([]byte(firstServers+firstRoutes+firstTLS), []byte(secondServers+secondRoutes+secondTLS)) {
		t.Fatal("same Caddy config produced unstable tables")
	}
}

func TestFetchDataStableAcrossMapAndRouteOrdering(t *testing.T) {
	firstJSON := `{"apps":{"http":{"servers":{"z":{"listen":[":8443"],"routes":[{"match":[{"host":["z.test"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"z.internal:80"}]}]},{"match":[{"host":["a.test"]}],"handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.2:8080"}]}]}]},"a":{"listen":[":443"],"routes":[{"match":[{"host":["b.test"]}],"handle":[]}]}}},"tls":{"automation":{"policies":[{"subjects":["z.test","a.test"]}]}}}}`
	secondJSON := `{"apps":{"tls":{"automation":{"policies":[{"subjects":["a.test","z.test"]}]}},"http":{"servers":{"a":{"routes":[{"handle":[],"match":[{"host":["b.test"]}]}],"listen":[":443"]},"z":{"routes":[{"handle":[{"upstreams":[{"dial":"10.0.0.2:8080"}],"handler":"reverse_proxy"}],"match":[{"host":["a.test"]}]},{"handle":[{"upstreams":[{"dial":"z.internal:80"}],"handler":"reverse_proxy"}],"match":[{"host":["z.test"]}]}],"listen":[":8443"]}}}}}`
	first, err := parseConfig([]byte(firstJSON))
	if err != nil {
		t.Fatal(err)
	}
	second, err := parseConfig([]byte(secondJSON))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.entities, second.entities) || !reflect.DeepEqual(first.deps, second.deps) || !reflect.DeepEqual(first.routes, second.routes) {
		t.Fatalf("reordering changed parsed snapshot data:\nfirst=%+v\nsecond=%+v", first, second)
	}
	firstTables := tablesAsString(first)
	secondTables := tablesAsString(second)
	if !bytes.Equal([]byte(firstTables), []byte(secondTables)) {
		t.Fatalf("reordering changed tables:\n%s\n%s", firstTables, secondTables)
	}
}

func tablesAsString(parsed *parsedConfig) string {
	servers, routes, subjects := tables(parsed)
	return servers + routes + subjects
}

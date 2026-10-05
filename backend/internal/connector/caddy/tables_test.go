package caddy

import (
	"strings"
	"testing"
)

func TestBuildTablesAndHostIdentity(t *testing.T) {
	parsed, err := parseConfig([]byte(configFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.servers) != 1 || len(parsed.routes) != 2 || len(parsed.subjects) != 2 {
		t.Fatalf("parsed = %+v", parsed)
	}
	var routeEntity = parsed.entities[1]
	if routeEntity.Hostname != "www.example.com" || len(routeEntity.Aliases) != 1 || routeEntity.IP != "10.0.0.8" {
		t.Fatalf("route entity = %+v", routeEntity)
	}
	serverTable, routeTable, tlsTable := tables(parsed)
	if !strings.Contains(serverTable, "srv0") || !strings.Contains(routeTable, "api.internal") || !strings.Contains(tlsTable, "www.example.com") {
		t.Fatalf("tables missing parsed values: %s %s %s", serverTable, routeTable, tlsTable)
	}
}

func TestMalformedCaddyJSON(t *testing.T) {
	for _, raw := range []string{"", "[]", "{}", `{"apps":{}}`, `{"apps":{"http":{"servers":[1]}}}`} {
		if _, err := parseConfig([]byte(raw)); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}

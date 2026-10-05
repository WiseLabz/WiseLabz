package caddy

import (
	"bytes"
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

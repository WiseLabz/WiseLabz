package caddy

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

func TestAttributeCatalogCoversEmittedKeys(t *testing.T) {
	parsed, err := parseConfig([]byte(configFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, entity := range parsed.entities {
		specs, ok := attributeCatalog[entity.Kind]
		if !ok {
			t.Fatalf("catalog missing %q", entity.Kind)
		}
		byName := map[string]connector.AttributeSpec{}
		for _, spec := range specs {
			byName[spec.Name] = spec
		}
		for key, value := range entity.Attributes {
			spec, ok := byName[key]
			if !ok {
				t.Errorf("catalog[%s] missing %q", entity.Kind, key)
				continue
			}
			if spec.Type != connectortest.JSONType(value) {
				t.Errorf("catalog[%s][%s] type %q != %q", entity.Kind, key, spec.Type, connectortest.JSONType(value))
			}
		}
	}
}

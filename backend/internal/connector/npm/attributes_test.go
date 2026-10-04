package npm

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/connectortest"
)

func TestAttributeCatalogCoversEmittedKeys(t *testing.T) {
	fixtures := []struct {
		build tableBuilder
		raw   string
	}{
		{buildProxyHostTable, proxyHostsFixture},
		{buildRedirectionHostTable, redirectFixture},
		{buildStreamTable, streamFixture},
		{buildDeadHostTable, deadHostsFixture},
		{buildCertificateTable, certificatesFixture},
		{buildAccessListTable, accessListsFixture},
	}
	emitted := map[string]map[string]string{}
	for _, fixture := range fixtures {
		_, entities, _, err := fixture.build([]byte(fixture.raw))
		if err != nil {
			t.Fatalf("fixture build failed: %v", err)
		}
		for _, entity := range entities {
			if emitted[entity.Kind] == nil {
				emitted[entity.Kind] = map[string]string{}
			}
			for key, value := range entity.Attributes {
				emitted[entity.Kind][key] = connectortest.JSONType(value)
			}
		}
	}
	for kind, keys := range emitted {
		specs, ok := attributeCatalog[kind]
		if !ok {
			t.Fatalf("catalog missing entity kind %q", kind)
		}
		byName := make(map[string]connector.AttributeSpec, len(specs))
		for _, spec := range specs {
			byName[spec.Name] = spec
		}
		for key, typ := range keys {
			spec, ok := byName[key]
			if !ok {
				t.Errorf("catalog[%q] missing emitted attribute %q", kind, key)
				continue
			}
			if spec.Type != typ {
				t.Errorf("catalog[%q][%q].Type = %q, want %q", kind, key, spec.Type, typ)
			}
			if spec.Description == "" {
				t.Errorf("catalog[%q][%q] has no description", kind, key)
			}
		}
		for key := range byName {
			if _, ok := keys[key]; !ok {
				t.Errorf("catalog[%q] declares unused attribute %q", kind, key)
			}
		}
	}
	for kind := range attributeCatalog {
		if _, ok := emitted[kind]; !ok {
			t.Errorf("catalog declares unused entity kind %q", kind)
		}
	}
}

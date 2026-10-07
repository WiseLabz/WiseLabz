package store

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestTextareaIsPlaintextAndRoundTrips(t *testing.T) {
	if IsSecretFieldType("textarea") {
		t.Fatal("textarea must not be encrypted")
	}
	typ := "store_textarea_test"
	connector.Register(connector.TypeSchema{Type: typ, Fields: []connector.SchemaField{{Key: "recipe", Type: "textarea"}}}, func(map[string]any) (connector.Connector, error) { return nil, nil })
	recipe := "version: 1\ncategory: media\n"
	data, err := MarshalConnectorConfig(typ, map[string]any{"recipe": recipe}, testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseConnectorConfig(typ, data, testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	if got["recipe"] != recipe {
		t.Fatalf("recipe changed: %v", got["recipe"])
	}
	// Plaintext fields can be read even without the encryption key.
	got, err = ParseConnectorConfig(typ, data, "")
	if err != nil || got["recipe"] != recipe {
		t.Fatalf("read plaintext: %v, %v", got, err)
	}
}

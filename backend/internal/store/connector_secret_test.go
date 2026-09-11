package store

import (
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// testEncKey is a fixed valid base64-encoded 32-byte AES-256 key, used only
// to exercise Marshal/ParseConnectorConfig's encryption in tests.
const testEncKey = "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="

const secretTestConnType = "store_test_secret_conn"

func init() {
	// A minimal schema with one secret-bearing ("password") field and one
	// plain ("text") field, so Marshal/ParseConnectorConfig has something
	// real to look up via connector.GetTypeSchema.
	connector.Register(connector.TypeSchema{
		Type:     secretTestConnType,
		Category: "test",
		Name:     "Secret test connector",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "URL", Type: "text"},
			{Key: "api_key", Label: "API Key", Type: "password"},
		},
	}, func(_ map[string]any) (connector.Connector, error) { return nil, nil })
}

func TestConnectorConfigEncryptsPasswordFieldsAtRest(t *testing.T) {
	plaintext := "hunter2-super-secret"
	data, err := MarshalConnectorConfig(secretTestConnType, map[string]any{
		"url":     "https://example.com",
		"api_key": plaintext,
	}, testEncKey)
	if err != nil {
		t.Fatalf("MarshalConnectorConfig() error: %v", err)
	}
	if strings.Contains(data, plaintext) {
		t.Fatalf("stored config_data contains the plaintext secret: %s", data)
	}

	cfg, err := ParseConnectorConfig(secretTestConnType, data, testEncKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig() error: %v", err)
	}
	if cfg["api_key"] != plaintext {
		t.Fatalf("api_key = %v, want %q", cfg["api_key"], plaintext)
	}
	if cfg["url"] != "https://example.com" {
		t.Fatalf("url = %v, want unchanged", cfg["url"])
	}
}

func TestConnectorConfigLegacyPlaintextFallback(t *testing.T) {
	plaintext := "legacy-plaintext-secret"
	// Simulates a row saved before encryption was added: api_key is stored
	// as plain JSON text rather than ciphertext.
	legacyJSON := `{"url":"https://example.com","api_key":"` + plaintext + `"}`

	cfg, err := ParseConnectorConfig(secretTestConnType, legacyJSON, testEncKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig() error: %v", err)
	}
	if cfg["api_key"] != plaintext {
		t.Fatalf("api_key = %v, want legacy plaintext %q", cfg["api_key"], plaintext)
	}

	// The connector's next save must re-encrypt it.
	remarshaled, err := MarshalConnectorConfig(secretTestConnType, cfg, testEncKey)
	if err != nil {
		t.Fatalf("MarshalConnectorConfig() error: %v", err)
	}
	if strings.Contains(remarshaled, plaintext) {
		t.Fatalf("re-marshaled config_data still contains the plaintext secret: %s", remarshaled)
	}

	// And it should decrypt back to the same value.
	cfg2, err := ParseConnectorConfig(secretTestConnType, remarshaled, testEncKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig() (2nd) error: %v", err)
	}
	if cfg2["api_key"] != plaintext {
		t.Fatalf("api_key after re-encrypt round trip = %v, want %q", cfg2["api_key"], plaintext)
	}
}

package connector

import (
	"strings"
	"testing"
)

func TestValidateDeclared(t *testing.T) {
	Register(TypeSchema{Type: "declared_test", Category: "test", Name: "Declared", Fields: []SchemaField{
		{Key: "url", Type: "text", Required: true},
		{Key: "token_id", Type: "text", Required: true},
		{Key: "token_secret", Type: "password", Required: true, MinLength: 4},
		{Key: "mode", Type: "select", Options: []string{"a", "b"}},
		{Key: "verify_tls", Type: "toggle"},
	}}, func(map[string]any) (Connector, error) { return nil, nil })

	valid := map[string]any{"token_id": "id", "token_secret": "secret"}
	tests := []struct {
		name   string
		typ    string
		config map[string]any
		want   []string // substrings of the error; empty means valid
	}{
		{name: "valid", typ: "declared_test", config: valid},
		{name: "optional field", typ: "declared_test", config: map[string]any{"token_id": "id", "token_secret": "secret", "mode": "b"}},
		{name: "unknown type", typ: "declared_test_missing", config: valid, want: []string{"unknown connector type"}},
		{name: "missing required", typ: "declared_test", config: map[string]any{"token_id": ""}, want: []string{`"token_id": is required`, `"token_secret": is required`}},
		{name: "nil config", typ: "declared_test", want: []string{`"token_id": is required`}},
		{name: "unknown key", typ: "declared_test", config: map[string]any{"token_id": "id", "token_secret": "secret", "tokn": "x"}, want: []string{`"tokn": is not a declared_test setting`}},
		{name: "column field inside config", typ: "declared_test", config: map[string]any{"token_id": "id", "token_secret": "secret", "url": "https://x"}, want: []string{`"url": must be set on the connector entry`}},
		{name: "field rule", typ: "declared_test", config: map[string]any{"token_id": "id", "token_secret": "abc"}, want: []string{"at least 4 characters"}},
		{name: "select option", typ: "declared_test", config: map[string]any{"token_id": "id", "token_secret": "secret", "mode": "c"}, want: []string{"must be one of"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDeclared(tt.typ, tt.config)
			if len(tt.want) == 0 {
				if err != nil {
					t.Fatalf("ValidateDeclared() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateDeclared() = nil, want error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not mention %q", err, w)
				}
			}
		})
	}
}

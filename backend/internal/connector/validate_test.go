package connector

import "testing"

func testSchema() TypeSchema {
	return TypeSchema{
		Type: "validate_test",
		Fields: []SchemaField{
			{Key: "url", Type: "text", Pattern: `^https?://`},
			{Key: "token", Type: "password", MinLength: 8, MaxLength: 64},
			{Key: "mode", Type: "select", Options: []string{"read", "write"}},
		},
	}
}

func TestValidateConfigAcceptsMissingAndEmptyFields(t *testing.T) {
	// Absence (or "") is not enforced here — a connector can be created
	// before its credentials are filled in; presence is a connector-level
	// (Validate/Fetch) concern, not a schema-shape concern.
	if err := ValidateConfig(testSchema(), map[string]any{}); err != nil {
		t.Errorf("ValidateConfig(empty) = %v, want nil", err)
	}
}

func TestValidateConfigPattern(t *testing.T) {
	err := ValidateConfig(testSchema(), map[string]any{"url": "ftp://example.com"})
	if err == nil {
		t.Fatal("ValidateConfig() = nil, want pattern error")
	}
	var cve *ConfigValidationError
	if !isConfigValidationError(err, &cve) {
		t.Fatalf("error = %v, want *ConfigValidationError", err)
	}
	if cve.Field != "url" {
		t.Errorf("Field = %q, want %q", cve.Field, "url")
	}

	if err := ValidateConfig(testSchema(), map[string]any{"url": "https://example.com"}); err != nil {
		t.Errorf("ValidateConfig(valid url) = %v, want nil", err)
	}
}

func TestValidateConfigLength(t *testing.T) {
	if err := ValidateConfig(testSchema(), map[string]any{"token": "short"}); err == nil {
		t.Error("ValidateConfig(too-short token) = nil, want error")
	}
	if err := ValidateConfig(testSchema(), map[string]any{"token": "longenoughtoken"}); err != nil {
		t.Errorf("ValidateConfig(valid token) = %v, want nil", err)
	}
}

func TestValidateConfigEnum(t *testing.T) {
	if err := ValidateConfig(testSchema(), map[string]any{"mode": "delete"}); err == nil {
		t.Error("ValidateConfig(invalid enum) = nil, want error")
	}
	if err := ValidateConfig(testSchema(), map[string]any{"mode": "write"}); err != nil {
		t.Errorf("ValidateConfig(valid enum) = %v, want nil", err)
	}
}

func TestTypeSchemaDegradedLatencyThreshold(t *testing.T) {
	if got := (TypeSchema{}).DegradedLatencyThreshold(); got != DegradedLatencyThreshold {
		t.Errorf("zero threshold = %v, want package default %v", got, DegradedLatencyThreshold)
	}
	custom := TypeSchema{DegradedLatencyThresholdMs: 5000}
	if got, want := custom.DegradedLatencyThreshold(), 5000000000; int64(got) != int64(want) {
		t.Errorf("custom threshold = %v, want 5s", got)
	}
}

func isConfigValidationError(err error, target **ConfigValidationError) bool {
	cve, ok := err.(*ConfigValidationError)
	if ok {
		*target = cve
	}
	return ok
}

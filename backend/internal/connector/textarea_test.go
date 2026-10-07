package connector

import (
	"errors"
	"testing"
)

func TestTextareaSchemaValidation(t *testing.T) {
	schema := TypeSchema{Type: "textarea_test", Category: "virtualization", Fields: []SchemaField{{Key: "recipe", Type: "textarea", Required: true, MaxLength: 20}}}
	Register(schema, func(map[string]any) (Connector, error) { return nil, nil })
	got, err := GetTypeSchema(schema.Type)
	if err != nil {
		t.Fatal(err)
	}
	if got.Fields[0].Type != "textarea" {
		t.Fatalf("field %+v", got.Fields[0])
	}
	if err := ValidateDeclared(schema.Type, map[string]any{"recipe": "one\ntwo"}); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{123, "more than twenty characters here"} {
		err := ValidateConfig(schema, map[string]any{"recipe": value})
		var fieldErr *ConfigValidationError
		if !errors.As(err, &fieldErr) || fieldErr.Field != "recipe" {
			t.Fatalf("value %v: %v", value, err)
		}
	}
}

func TestConfigCategoryHook(t *testing.T) {
	schema := TypeSchema{Category: "virtualization"}
	if category, err := schema.ConfigCategory(nil); err != nil || category != "virtualization" {
		t.Fatalf("category %q: %v", category, err)
	}
	schema.CategoryForConfig = func(cfg map[string]any) (string, error) { return cfg["category"].(string), nil }
	if category, err := schema.ConfigCategory(map[string]any{"category": "media"}); err != nil || category != "media" {
		t.Fatalf("category %q: %v", category, err)
	}
}

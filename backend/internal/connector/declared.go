package connector

import (
	"errors"
	"fmt"
	"sort"
)

// columnFields are schema fields stored as connector columns rather than in
// the type-specific config; a declared connector sets them at the entry's top
// level.
var columnFields = map[string]bool{"url": true, "verify_tls": true}

// declaredFields are extra config keys permitted only for declared entries of
// specific connector types.
var declaredFields = map[string]map[string]bool{
	"tlsprobe": {"import_connector": true},
}

// ValidateDeclared strictly checks the type-specific config of a connector
// declared in config.yaml (#500): the type must be registered, every required
// field must be present, and no key may be unknown to the type. It then applies
// the same per-field rules as ValidateConfig. Unlike ValidateConfig, which lets
// the UI save a half-filled form, a declared connector has nobody to finish it
// later, so a typo must be reported instead of stored. All problems are joined
// into one error.
func ValidateDeclared(typ string, config map[string]any) error {
	schema, err := GetTypeSchema(typ)
	if err != nil {
		return err
	}
	var errs []error
	known := make(map[string]bool, len(schema.Fields))
	for _, f := range schema.Fields {
		known[f.Key] = true
		if columnFields[f.Key] || !f.Required {
			continue
		}
		if v, ok := config[f.Key]; !ok || v == nil || v == "" {
			errs = append(errs, &ConfigValidationError{Field: f.Key, Message: "is required"})
		}
	}
	for k := range declaredFields[typ] {
		known[k] = true
	}
	keys := make([]string, 0, len(config))
	for k := range config {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch {
		case columnFields[k]:
			errs = append(errs, &ConfigValidationError{Field: k, Message: "must be set on the connector entry, not inside config"})
		case !known[k]:
			errs = append(errs, &ConfigValidationError{Field: k, Message: fmt.Sprintf("is not a %s setting", typ)})
		}
	}
	if err := ValidateConfig(*schema, config); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

package connector

import (
	"fmt"
	"sync"
)

// Factory creates a Connector from its config.
type Factory func(config map[string]any) (Connector, error)

// TypeSchema describes the configuration schema for a connector type.
type TypeSchema struct {
	Type     string        `json:"type"`
	Category string        `json:"category"`
	Name     string        `json:"displayName"`
	Fields   []SchemaField `json:"fields"`
}

// SchemaField describes a single configuration field.
type SchemaField struct {
	Key         string `json:"name"`
	Label       string `json:"label"`
	Type        string `json:"kind"` // "text", "password", "number", "select", "toggle"
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

var (
	registry   = make(map[string]Factory)
	typeSchema = make(map[string]TypeSchema)
	mu         sync.RWMutex
)

// Register registers a connector factory and its type schema.
func Register(schema TypeSchema, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[schema.Type] = factory
	typeSchema[schema.Type] = schema
}

// Get returns a new connector instance for the given type and config.
func Get(typ string, config map[string]any) (Connector, error) {
	mu.RLock()
	factory, ok := registry[typ]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown connector type: %q", typ)
	}
	return factory(config)
}

// GetTypeSchema returns the configuration schema for a connector type.
func GetTypeSchema(typ string) (*TypeSchema, error) {
	mu.RLock()
	schema, ok := typeSchema[typ]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown connector type: %q", typ)
	}
	return &schema, nil
}

// ListSchemas returns all registered connector type schemas.
func ListSchemas() []TypeSchema {
	mu.RLock()
	defer mu.RUnlock()
	var out []TypeSchema
	for _, s := range typeSchema {
		out = append(out, s)
	}
	return out
}

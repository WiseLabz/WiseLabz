// Package pfsense implements a mock pfSense connector.
package pfsense

import (
	"context"
	"fmt"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "pfsense"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "networking",
		Name:     "pfSense",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "pfSense URL", Type: "text", Required: true, Placeholder: "https://pfsense.example.com"},
			{Key: "api_key", Label: "API Key", Type: "password", Required: true},
		},
	}, func(_ map[string]any) (connector.Connector, error) {
		return &Connector{}, nil
	})
}

// Connector is a mock pfSense connector stub.
type Connector struct{}

// Name returns the connector display name.
func (p *Connector) Name() string { return "pfSense" }

// Type returns the connector type identifier.
func (p *Connector) Type() string { return typeName }

// Category returns the connector category.
func (p *Connector) Category() string { return "networking" }

// Validate is a stub that always returns a not-implemented error.
func (p *Connector) Validate(_ context.Context, _ map[string]any) error {
	return fmt.Errorf("pfSense connector is a stub — not yet implemented")
}

// Fetch returns a stub snapshot for the pfSense connector.
func (p *Connector) Fetch(_ context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{
		ServiceName: "pfSense (stub)",
		Type:        typeName,
		Sections: []connector.SnapshotSection{
			{Title: "Stub", Content: "pfSense connector is a stub. Real implementation pending."},
		},
		Metadata:  map[string]string{"stub": "true"},
		FetchedAt: time.Now(),
	}, nil
}

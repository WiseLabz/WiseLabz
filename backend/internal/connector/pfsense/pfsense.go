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
	}, func(config map[string]any) (connector.Connector, error) {
		return &PfSenseConnector{}, nil
	})
}

// PfSenseConnector is a mock pfSense connector stub.
type PfSenseConnector struct{}

func (p *PfSenseConnector) Name() string     { return "pfSense" }
func (p *PfSenseConnector) Type() string     { return typeName }
func (p *PfSenseConnector) Category() string { return "networking" }

func (p *PfSenseConnector) Validate(ctx context.Context, config map[string]any) error {
	return fmt.Errorf("pfSense connector is a stub — not yet implemented")
}

func (p *PfSenseConnector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
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

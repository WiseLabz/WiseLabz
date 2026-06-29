// Package docker implements a mock Docker connector.
package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "docker"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "containers_paas",
		Name:     "Docker",
		Fields: []connector.SchemaField{
			{Key: "host", Label: "Docker Host", Type: "text", Required: true, Placeholder: "unix:///var/run/docker.sock or tcp://host:2375"},
		},
	}, func(_ map[string]any) (connector.Connector, error) {
		return &Connector{}, nil
	})
}

// Connector is a mock Docker connector stub.
type Connector struct{}

// Name returns the connector display name.
func (d *Connector) Name() string { return "Docker" }

// Type returns the connector type identifier.
func (d *Connector) Type() string { return typeName }

// Category returns the connector category.
func (d *Connector) Category() string { return "containers_paas" }

// Validate is a stub that always returns a not-implemented error.
func (d *Connector) Validate(_ context.Context, _ map[string]any) error {
	return fmt.Errorf("docker connector is a stub — not yet implemented")
}

// Fetch returns a stub snapshot for the Docker connector.
func (d *Connector) Fetch(_ context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	return &connector.ServiceSnapshot{
		ServiceName: "Docker (stub)",
		Type:        typeName,
		Sections: []connector.SnapshotSection{
			{Title: "Stub", Content: "Docker connector is a stub. Real implementation pending."},
		},
		Metadata:  map[string]string{"stub": "true"},
		FetchedAt: time.Now(),
	}, nil
}

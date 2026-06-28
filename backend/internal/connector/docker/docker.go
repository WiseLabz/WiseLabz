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
	}, func(config map[string]any) (connector.Connector, error) {
		return &DockerConnector{}, nil
	})
}

// DockerConnector is a mock Docker connector stub.
type DockerConnector struct{}

func (d *DockerConnector) Name() string     { return "Docker" }
func (d *DockerConnector) Type() string     { return typeName }
func (d *DockerConnector) Category() string { return "containers_paas" }

func (d *DockerConnector) Validate(ctx context.Context, config map[string]any) error {
	return fmt.Errorf("Docker connector is a stub — not yet implemented")
}

func (d *DockerConnector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
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

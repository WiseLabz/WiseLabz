package docker

import (
	"context"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// ConfigRead returns a container's current restart policy from a fresh inspect
// included in Fetch.
func (d *Connector) ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (any, error) {
	if fieldKey != "restartPolicy" {
		return nil, fmt.Errorf("unsupported field %q", fieldKey)
	}
	if entityRef == "" {
		return nil, fmt.Errorf("docker config-read requires a target container ID")
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return nil, fmt.Errorf("invalid entityRef: %w", err)
	}

	fetchConfig := make(map[string]any, len(config)+1)
	for key, value := range config {
		fetchConfig[key] = value
	}
	fetchConfig["fields"] = []string{"containers"}

	snapshot, err := d.Fetch(ctx, fetchConfig)
	if err != nil {
		return nil, fmt.Errorf("fetch docker config value: %w", err)
	}
	for _, entity := range snapshot.Entities {
		if entity.Kind != "container" || entity.ExternalID != entityRef {
			continue
		}
		value, ok := entity.Attributes["restart_policy"].(string)
		if !ok || value == "" {
			return nil, fmt.Errorf("docker restart policy is unavailable for container %q", entityRef)
		}
		return value, nil
	}
	return nil, fmt.Errorf("docker container %q not found", entityRef)
}

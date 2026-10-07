package opnsense

import (
	"context"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// ConfigRead returns a firewall rule's current enabled state from a fresh
// firewall-rule fetch.
func (c *Connector) ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (any, error) {
	if fieldKey != "enabled" {
		return nil, fmt.Errorf("unsupported field %q", fieldKey)
	}
	if entityRef == "" {
		return nil, fmt.Errorf("opnsense config-read requires a target rule UUID")
	}
	if isFallbackExternalID(entityRef) {
		return nil, fmt.Errorf("opnsense config-read requires an upstream rule UUID")
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return nil, fmt.Errorf("invalid entityRef: %w", err)
	}

	snapshot, err := c.Fetch(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("fetch opnsense config value: %w", err)
	}
	for _, entity := range snapshot.Entities {
		if entity.Kind != "rule" || entity.ExternalID != entityRef {
			continue
		}
		value, ok := entity.Attributes["enabled"].(bool)
		if !ok {
			return nil, fmt.Errorf("opnsense rule enabled state is unavailable for %q", entityRef)
		}
		return value, nil
	}
	return nil, fmt.Errorf("opnsense firewall rule %q not found", entityRef)
}

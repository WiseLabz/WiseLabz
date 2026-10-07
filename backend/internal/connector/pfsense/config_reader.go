package pfsense

import (
	"context"
	"fmt"
)

// ConfigRead returns a firewall rule's current enabled state from a fresh
// firewall-rule fetch.
func (c *Connector) ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (any, error) {
	if fieldKey != "enabled" {
		return nil, fmt.Errorf("unsupported field %q", fieldKey)
	}
	if entityRef == "" {
		return nil, fmt.Errorf("pfsense config-read requires a target rule ID")
	}
	if _, ok := c.localRuleReference(entityRef); !ok {
		return nil, fmt.Errorf("pfsense config-read requires a scoped firewall rule reference")
	}

	snapshot, err := c.Fetch(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("fetch pfsense config value: %w", err)
	}
	var found any
	for _, entity := range snapshot.Entities {
		if entity.Kind != "rule" || entity.ExternalID != entityRef {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("pfsense firewall rule reference %q is ambiguous", entityRef)
		}
		value, ok := entity.Attributes["enabled"].(bool)
		if !ok {
			return nil, fmt.Errorf("pfsense rule enabled state is unavailable for %q", entityRef)
		}
		found = value
	}
	if found == nil {
		return nil, fmt.Errorf("pfsense firewall rule %q not found", entityRef)
	}
	return found, nil
}

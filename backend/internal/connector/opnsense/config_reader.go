package opnsense

import (
	"context"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// ConfigRead returns a firewall rule's current saved enabled state from getRule.
// For a rule with an uncertain previous push it returns nil with no error, so
// the caller pushes again instead of concluding "already at target".
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

	enabled, err := c.ruleEnabled(ctx, entityRef)
	if err != nil {
		return nil, fmt.Errorf("read opnsense rule config value: %w", err)
	}
	if filterStateFor(c.url).marked(entityRef) {
		return nil, nil
	}
	return enabled == "1", nil
}

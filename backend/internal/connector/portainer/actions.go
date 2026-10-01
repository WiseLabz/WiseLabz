package portainer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// Restart restarts the stack identified by entityRef (a Portainer stack ID).
// Portainer has no restart endpoint for stacks, so this is a stop followed by
// a start.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, entityRef string) error {
	endpointID, err := c.stackEndpoint(ctx, "restart", entityRef)
	if err != nil {
		return err
	}
	if err := c.stackAction(ctx, entityRef, endpointID, "stop"); err != nil {
		return err
	}
	return c.stackAction(ctx, entityRef, endpointID, "start")
}

// Start starts the stack identified by entityRef (a Portainer stack ID).
func (c *Connector) Start(ctx context.Context, _ map[string]any, entityRef string) error {
	endpointID, err := c.stackEndpoint(ctx, "start", entityRef)
	if err != nil {
		return err
	}
	return c.stackAction(ctx, entityRef, endpointID, "start")
}

// Stop stops the stack identified by entityRef (a Portainer stack ID).
func (c *Connector) Stop(ctx context.Context, _ map[string]any, entityRef string) error {
	endpointID, err := c.stackEndpoint(ctx, "stop", entityRef)
	if err != nil {
		return err
	}
	return c.stackAction(ctx, entityRef, endpointID, "stop")
}

// stackEndpoint validates entityRef and resolves the environment the stack
// is deployed to, which Portainer requires as the endpointId query parameter.
func (c *Connector) stackEndpoint(ctx context.Context, verb, entityRef string) (int, error) {
	if entityRef == "" {
		return 0, fmt.Errorf("portainer %s requires a target stack ID", verb)
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return 0, fmt.Errorf("invalid entityRef: %w", err)
	}
	raw, err := c.doRequest(ctx, pathStacks+"/"+entityRef)
	if err != nil {
		return 0, fmt.Errorf("resolve stack environment: %w", err)
	}
	var stack struct {
		EndpointID int `json:"EndpointId"`
	}
	if err := json.Unmarshal(raw, &stack); err != nil {
		return 0, connector.NewMalformedResponseError(fmt.Errorf("decode stack: %w", err))
	}
	return stack.EndpointID, nil
}

func (c *Connector) stackAction(ctx context.Context, stackID string, endpointID int, action string) error {
	_, err := c.doMethod(ctx, http.MethodPost, fmt.Sprintf("%s/%s/%s?endpointId=%d", pathStacks, stackID, action, endpointID))
	return err
}

package truenas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// Restart redeploys the app identified by entityRef (an app name), which
// recreates its containers. TrueNAS has no dedicated app restart call.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.appAction(ctx, "restart", "redeploy", entityRef)
}

// Start starts the app identified by entityRef (an app name).
func (c *Connector) Start(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.appAction(ctx, "start", "start", entityRef)
}

// Stop stops the app identified by entityRef (an app name).
func (c *Connector) Stop(ctx context.Context, _ map[string]any, entityRef string) error {
	return c.appAction(ctx, "stop", "stop", entityRef)
}

// appAction queues the app.<method> job for the named app. The API call
// returns a job ID as soon as the job is queued; completion is not awaited.
func (c *Connector) appAction(ctx context.Context, verb, method, entityRef string) error {
	if entityRef == "" {
		return fmt.Errorf("truenas %s requires a target app name", verb)
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return fmt.Errorf("invalid entityRef: %w", err)
	}
	body, err := json.Marshal(entityRef)
	if err != nil {
		return err
	}
	_, err = c.doMethod(ctx, http.MethodPost, pathApps+"/"+method, bytes.NewReader(body))
	return err
}

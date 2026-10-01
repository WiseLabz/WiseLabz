package homeassistant

import (
	"context"
	"net/http"
)

// pathRestartService is the core service that restarts Home Assistant.
const pathRestartService = "/api/services/homeassistant/restart"

// Restart restarts the Home Assistant instance. The connector manages one
// implicit service, so entityRef is ignored.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, _ string) error {
	_, err := c.doMethod(ctx, http.MethodPost, pathRestartService)
	return err
}

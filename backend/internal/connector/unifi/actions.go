package unifi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// Restart restarts a device or power-cycles a PoE port. entityRef is a
// device MAC address for a device restart, or "<switch MAC>:<port number>"
// (the ExternalID of a "port" entity) for a PoE power-cycle.
func (c *Connector) Restart(ctx context.Context, _ map[string]any, entityRef string) error {
	if entityRef == "" {
		return fmt.Errorf("unifi restart requires a target device MAC or switch port")
	}
	mac, portIdx, isPort, err := parseRef(entityRef)
	if err != nil {
		return err
	}
	cmd := map[string]any{"cmd": "restart", "mac": mac}
	if isPort {
		cmd = map[string]any{"cmd": "power-cycle", "mac": mac, "port_idx": portIdx}
	}
	return c.devmgr(ctx, cmd)
}

// parseRef splits an entityRef into the device MAC and, for a port ref, its
// port number.
func parseRef(entityRef string) (mac string, portIdx int, isPort bool, err error) {
	mac = entityRef
	// A MAC has five colons; a sixth introduces the port number.
	if strings.Count(entityRef, ":") == 6 {
		i := strings.LastIndex(entityRef, ":")
		mac, isPort = entityRef[:i], true
		portIdx, err = strconv.Atoi(entityRef[i+1:])
		if err != nil || portIdx < 1 {
			return "", 0, false, fmt.Errorf("invalid entityRef: bad port number in %q", entityRef)
		}
	}
	if err := connector.ValidateRefSegment(strings.ReplaceAll(mac, ":", "-")); err != nil {
		return "", 0, false, fmt.Errorf("invalid entityRef: %w", err)
	}
	return mac, portIdx, isPort, nil
}

// devmgr posts a device-manager command to the configured site.
func (c *Connector) devmgr(ctx context.Context, cmd map[string]any) error {
	s, err := c.session(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+s.prefix+c.sitePath("/cmd/devmgr"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.authMode == authAPIKey {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	if c.csrf != "" {
		req.Header.Set("X-Csrf-Token", c.csrf)
	}
	_, err = c.send(req)
	return err
}

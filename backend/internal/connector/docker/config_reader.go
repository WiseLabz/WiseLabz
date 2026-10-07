package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// ConfigRead returns a container's current restart policy from one inspect
// call for exactly the addressed container. An empty policy name is reported
// as "no", which is how Docker treats it.
func (d *Connector) ConfigRead(ctx context.Context, _ map[string]any, entityRef, fieldKey string) (any, error) {
	if fieldKey != "restartPolicy" {
		return nil, fmt.Errorf("unsupported field %q", fieldKey)
	}
	if entityRef == "" {
		return nil, fmt.Errorf("docker config-read requires a target container ID")
	}
	if err := connector.ValidateRefSegment(entityRef); err != nil {
		return nil, fmt.Errorf("invalid entityRef: %w", err)
	}

	raw, err := d.doRequest(ctx, "/containers/"+entityRef+"/json")
	if err != nil {
		return nil, fmt.Errorf("inspect docker container %q: %w", entityRef, err)
	}
	var inspect struct {
		HostConfig *struct {
			RestartPolicy *struct {
				Name string `json:"Name"`
			} `json:"RestartPolicy"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal(raw, &inspect); err != nil {
		return nil, fmt.Errorf("decode docker container %q: %w", entityRef, connector.NewMalformedResponseError(err))
	}
	if inspect.HostConfig == nil || inspect.HostConfig.RestartPolicy == nil {
		return nil, fmt.Errorf("docker restart policy is unavailable for container %q", entityRef)
	}
	if name := inspect.HostConfig.RestartPolicy.Name; name != "" {
		return name, nil
	}
	return "no", nil
}

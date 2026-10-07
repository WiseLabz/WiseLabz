package proxmox

import (
	"context"
	"fmt"
	"strconv"
)

// ConfigRead returns current memory or configured cores from a fresh snapshot.
// Memory comes from the guest list's maxmem, so it is the effective value of a
// running guest and a pending config change is not visible.
func (p *Connector) ConfigRead(ctx context.Context, config map[string]any, entityRef, fieldKey string) (any, error) {
	if fieldKey != "memory" && fieldKey != "cores" {
		return nil, fmt.Errorf("unsupported field %q", fieldKey)
	}
	if entityRef == "" {
		return nil, fmt.Errorf("proxmox config-read requires a target VMID")
	}
	vmid, err := strconv.Atoi(entityRef)
	if err != nil || vmid <= 0 {
		return nil, fmt.Errorf("invalid Proxmox VMID %q", entityRef)
	}

	fetchConfig := make(map[string]any, len(config)+1)
	for key, value := range config {
		fetchConfig[key] = value
	}
	fetchConfig["fields"] = []string{"vms", "containers", "entities"}

	snapshot, err := p.Fetch(ctx, fetchConfig)
	if err != nil {
		return nil, fmt.Errorf("fetch Proxmox config value: %w", err)
	}
	for _, entity := range snapshot.Entities {
		if entity.ExternalID != entityRef || (entity.Kind != "vm" && entity.Kind != "container") {
			continue
		}
		value, ok := entity.Attributes[fieldKey]
		if !ok && fieldKey == "cores" {
			// onboot is set whenever the guest's /config call succeeded, so
			// onboot without cores means the config holds no explicit core
			// count (QEMU default, or an LXC with no limit): there is no
			// numeric value a revert could restore.
			if _, configRead := entity.Attributes["onboot"]; configRead {
				return nil, nil
			}
		}
		if !ok {
			return nil, fmt.Errorf("proxmox %s is unavailable for VMID %q", fieldKey, entityRef)
		}
		switch number := value.(type) {
		case int:
			return float64(number), nil
		case int64:
			return float64(number), nil
		case float64:
			return number, nil
		default:
			return nil, fmt.Errorf("proxmox %s has an invalid value for VMID %q", fieldKey, entityRef)
		}
	}
	return nil, fmt.Errorf("proxmox VMID %q not found", entityRef)
}

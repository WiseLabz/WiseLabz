package proxmox

import (
	"context"
	"fmt"
	"strconv"
)

// ConfigRead returns configured memory or configured cores from a fresh snapshot.
// Both are read from the guest's /config endpoint, which shows pending
// configuration changes before the restart. If /config cannot be read, memory
// is an error rather than the running guest's maxmem, which would hide a
// pending change. If /config was read but holds no usable value (no key, or one
// that cannot be decoded), memory and cores are both nil with no error: the
// value is unknown.
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
		// onboot is set whenever the guest's /config call succeeded. Without
		// it the memory attribute is only the running maxmem, so it must not
		// be reported.
		_, configRead := entity.Attributes["onboot"]
		if fieldKey == "memory" && !configRead {
			ok = false
		}
		if !ok && configRead {
			// The config holds no usable value (for cores: QEMU default, or an
			// LXC with no limit): there is no number a revert could restore.
			return nil, nil
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

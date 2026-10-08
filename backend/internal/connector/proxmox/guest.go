package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// decodeProxmoxMemory decodes the "memory" field from a Proxmox QEMU or LXC
// /config response. Proxmox formats memory in several ways:
// - a JSON number (e.g. 2048 or 2048.0)
// - a numeric string (e.g. "2048")
// - a property string containing "current=<n>" (e.g. "current=2048" or "current=2048,max=4096")
// Returns (nil, nil) if raw is empty or null.
func decodeProxmoxMemory(raw json.RawMessage) (*int, error) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return nil, nil
	}

	// Try JSON number first
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		val := int(num)
		return &val, nil
	}

	// Try string formats
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		str = strings.TrimSpace(str)
		// Check comma-separated options for current=<n>
		for _, part := range strings.Split(str, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "current=") {
				valStr := strings.TrimSpace(strings.TrimPrefix(part, "current="))
				if f, err := strconv.ParseFloat(valStr, 64); err == nil {
					val := int(f)
					return &val, nil
				}
			}
		}
		// Bare numeric string
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			val := int(f)
			return &val, nil
		}
		return nil, fmt.Errorf("invalid memory format %q", str)
	}

	return nil, fmt.Errorf("invalid memory JSON: %s", string(raw))
}

// fetchQemuIP returns the first non-loopback IPv4 address reported by the
// QEMU guest agent, or "" if the agent isn't installed/running (most labs
// won't have it on every VM) or reports nothing usable. Soft-fails: any
// error here is not a Fetch failure.
func (p *Connector) fetchQemuIP(ctx context.Context, node string, vmid int) string {
	raw, err := p.doRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", node, vmid), nil)
	if err != nil {
		return ""
	}
	var resp struct {
		Data struct {
			Result []struct {
				Name        string `json:"name"`
				IPAddresses []struct {
					IPAddress     string `json:"ip-address"`
					IPAddressType string `json:"ip-address-type"`
				} `json:"ip-addresses"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ""
	}
	for _, iface := range resp.Data.Result {
		if iface.Name == "lo" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" && addr.IPAddress != "" {
				return addr.IPAddress
			}
		}
	}
	return ""
}

// fetchLxcIP returns the first non-loopback IPv4 address reported for the
// container. Unlike the QEMU path this needs no guest agent, but still
// soft-fails since older Proxmox versions lack this endpoint.
func (p *Connector) fetchLxcIP(ctx context.Context, node string, vmid int) string {
	raw, err := p.doRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/lxc/%d/interfaces", node, vmid), nil)
	if err != nil {
		return ""
	}
	var resp struct {
		Data []struct {
			Name  string `json:"name"`
			Inet  string `json:"inet"`
			Inet6 string `json:"inet6"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ""
	}
	for _, iface := range resp.Data {
		if iface.Name == "lo" || iface.Inet == "" {
			continue
		}
		ip, _, err := net.ParseCIDR(iface.Inet)
		if err == nil {
			return ip.String()
		}
		return iface.Inet
	}
	return ""
}

// qemuConfig holds the subset of VM /config fields we surface as
// Attributes. Missing flag values decode to their zero value; Cores stays nil
// when Proxmox does not report a configured value so ConfigRead can fail soft.
// Memory is decoded via decodeProxmoxMemory from RawMemory.
type qemuConfig struct {
	Onboot     int             `json:"onboot"`
	Protection int             `json:"protection"`
	Agent      string          `json:"agent"`
	Template   int             `json:"template"`
	OSType     string          `json:"ostype"`
	Cores      *int            `json:"cores"`
	RawMemory  json.RawMessage `json:"memory"`
	Memory     *int            `json:"-"`
}

// fetchQemuConfig fetches a VM's /config and returns the fields relevant to
// Attributes, or ok=false if the request/decode failed (soft-fail: any
// error here is not a Fetch failure, the caller just omits the attribute).
func (p *Connector) fetchQemuConfig(ctx context.Context, node string, vmid int) (qemuConfig, bool) {
	raw, err := p.doRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/qemu/%d/config", node, vmid), nil)
	if err != nil {
		return qemuConfig{}, false
	}
	var resp struct {
		Data qemuConfig `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return qemuConfig{}, false
	}
	if len(resp.Data.RawMemory) > 0 {
		mem, err := decodeProxmoxMemory(resp.Data.RawMemory)
		if err == nil {
			resp.Data.Memory = mem
		}
	}
	return resp.Data, true
}

// lxcConfig holds the subset of container /config fields we surface as
// Attributes.
type lxcConfig struct {
	Onboot       int             `json:"onboot"`
	Protection   int             `json:"protection"`
	Template     int             `json:"template"`
	Agent        string          `json:"agent"`
	Unprivileged int             `json:"unprivileged"`
	OSType       string          `json:"ostype"`
	Cores        *int            `json:"cores"`
	RawMemory    json.RawMessage `json:"memory"`
	Memory       *int            `json:"-"`
}

// fetchLxcConfig fetches a container's /config and returns the fields
// relevant to Attributes, or ok=false on request/decode failure.
func (p *Connector) fetchLxcConfig(ctx context.Context, node string, vmid int) (lxcConfig, bool) {
	raw, err := p.doRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/lxc/%d/config", node, vmid), nil)
	if err != nil {
		return lxcConfig{}, false
	}
	var resp struct {
		Data lxcConfig `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return lxcConfig{}, false
	}
	if len(resp.Data.RawMemory) > 0 {
		mem, err := decodeProxmoxMemory(resp.Data.RawMemory)
		if err == nil {
			resp.Data.Memory = mem
		}
	}
	return resp.Data, true
}

// fetchFirewallEnabled fetches a guest's firewall/options and reports
// whether the per-guest firewall is enabled. guestType is "qemu" or "lxc".
// Soft-fails to ok=false on any request/decode error.
func (p *Connector) fetchFirewallEnabled(ctx context.Context, guestType, node string, vmid int) (bool, bool) {
	raw, err := p.doRequest(ctx, "GET", fmt.Sprintf("/nodes/%s/%s/%d/firewall/options", node, guestType, vmid), nil)
	if err != nil {
		return false, false
	}
	var resp struct {
		Data struct {
			Enable int `json:"enable"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, false
	}
	return resp.Data.Enable != 0, true
}

// agentEnabled reports whether a QEMU config's "agent" field indicates the
// guest agent is enabled. Proxmox stores it either as a bare "1"/"0" or a
// comma-separated option string like "enabled=1,fstrim_cloned_disks=1", so
// presence of a leading "1" is treated as enabled.
func agentEnabled(agent string) bool {
	if agent == "" {
		return false
	}
	first := strings.SplitN(agent, ",", 2)[0]
	first = strings.TrimPrefix(first, "enabled=")
	return strings.TrimSpace(first) == "1"
}

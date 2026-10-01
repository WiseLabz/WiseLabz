package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const bytesPerMB = 1024 * 1024

// Fetch retrieves the current state of nodes, VMs, containers, and storage.
// config may carry a "fields" selective-fetch hint (see
// connector.RequestedFields) naming a subset of {"vms","containers","storage"}
// to skip the other upstream calls — e.g. a dashboard quick-check that only
// needs VM state doesn't need to also hit /storage on every node.
func (p *Connector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	start := time.Now()
	fields := connector.RequestedFields(config)
	wantVMs := connector.WantsField(fields, "vms")
	wantContainers := connector.WantsField(fields, "containers")
	wantStorage := connector.WantsField(fields, "storage")
	wantEntities := connector.WantsField(fields, "entities")

	// Fetch nodes
	nodesRaw, err := p.doRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch nodes: %w", err)
	}

	var nodesResponse struct {
		Data []struct {
			Node   string `json:"node"`
			Status string `json:"status"`
			// mem/maxmem are flat byte counts in GET /nodes. Only maxmem is
			// rendered: live usage, uptime and CPU change every fetch and
			// would register as drift on each sync.
			MaxMem int64 `json:"maxmem"`
		} `json:"data"`
	}
	if err := json.Unmarshal(nodesRaw, &nodesResponse); err != nil {
		return nil, connector.NewMalformedResponseError(fmt.Errorf("decode nodes: %w", err))
	}

	var sections []connector.SnapshotSection
	var dependencies []connector.ServiceDependency
	var entities []connector.SnapshotEntity
	metadata := map[string]string{
		"node_count": fmt.Sprintf("%d", len(nodesResponse.Data)),
	}

	// For each node, fetch VMs, containers, and storage
	totalVMs := 0
	totalCTs := 0
	totalStorage := 0

	for _, node := range nodesResponse.Data {
		var nodeErr error
		nodeSection := fmt.Sprintf("## Node: %s\n\n", node.Node)
		nodeSection += fmt.Sprintf("- **Status**: %s\n", node.Status)
		nodeSection += fmt.Sprintf("- **Memory**: %d MB\n\n", node.MaxMem/bytesPerMB)

		dependencies = append(dependencies, connector.ServiceDependency{Kind: "host", Name: node.Node})

		// Fetch VMs
		if wantVMs {
			func() {
				vmsRaw, err := p.doRequest(ctx, "GET", "/nodes/"+node.Node+"/qemu", nil)
				if err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}
				var vmsResponse struct {
					Data []struct {
						VMID   int    `json:"vmid"`
						Name   string `json:"name"`
						Status string `json:"status"`
						CPU    int    `json:"cpus"`
						MaxMem int64  `json:"maxmem"`
					} `json:"data"`
				}
				if err := json.Unmarshal(vmsRaw, &vmsResponse); err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}

				if len(vmsResponse.Data) > 0 {
					nodeSection += "### Virtual Machines\n\n"
					nodeSection += "| VMID | Name | Status | CPUs | Memory (MB) |\n"
					nodeSection += "|------|------|--------|------|-------------|\n"
					for _, vm := range vmsResponse.Data {
						nodeSection += fmt.Sprintf("| %d | %s | %s | %d | %d |\n",
							vm.VMID, vm.Name, vm.Status, vm.CPU, vm.MaxMem/bytesPerMB)
						ent := connector.SnapshotEntity{
							Kind:       "vm",
							Name:       vm.Name,
							ExternalID: fmt.Sprintf("%d", vm.VMID),
						}
						attrs := map[string]any{"status": vm.Status}
						if wantEntities {
							if vm.Status == "running" {
								ent.IP = p.fetchQemuIP(ctx, node.Node, vm.VMID)
							}
							if cfg, ok := p.fetchQemuConfig(ctx, node.Node, vm.VMID); ok {
								attrs["onboot"] = cfg.Onboot != 0
								attrs["protection"] = cfg.Protection != 0
								attrs["template"] = cfg.Template != 0
								attrs["agent_enabled"] = agentEnabled(cfg.Agent)
								if cfg.OSType != "" {
									attrs["os_type"] = cfg.OSType
								}
							}
							if enabled, ok := p.fetchFirewallEnabled(ctx, "qemu", node.Node, vm.VMID); ok {
								attrs["firewall_enabled"] = enabled
							}
						}
						ent.Attributes = attrs
						entities = append(entities, ent)
					}
					nodeSection += "\n"
					totalVMs += len(vmsResponse.Data)
				}
			}()
		}

		// Fetch containers
		if wantContainers {
			func() {
				ctsRaw, err := p.doRequest(ctx, "GET", "/nodes/"+node.Node+"/lxc", nil)
				if err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}
				var ctsResponse struct {
					Data []struct {
						VMID   int    `json:"vmid"`
						Name   string `json:"name"`
						Status string `json:"status"`
						CPU    int    `json:"cpus"`
						MaxMem int64  `json:"maxmem"`
					} `json:"data"`
				}
				if err := json.Unmarshal(ctsRaw, &ctsResponse); err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}

				if len(ctsResponse.Data) > 0 {
					nodeSection += "### Containers\n\n"
					nodeSection += "| VMID | Name | Status | CPUs | Memory (MB) |\n"
					nodeSection += "|------|------|--------|------|-------------|\n"
					for _, ct := range ctsResponse.Data {
						nodeSection += fmt.Sprintf("| %d | %s | %s | %d | %d |\n",
							ct.VMID, ct.Name, ct.Status, ct.CPU, ct.MaxMem/bytesPerMB)
						ent := connector.SnapshotEntity{
							Kind:       "container",
							Name:       ct.Name,
							ExternalID: fmt.Sprintf("%d", ct.VMID),
						}
						attrs := map[string]any{"status": ct.Status}
						if wantEntities {
							if ct.Status == "running" {
								ent.IP = p.fetchLxcIP(ctx, node.Node, ct.VMID)
							}
							if cfg, ok := p.fetchLxcConfig(ctx, node.Node, ct.VMID); ok {
								attrs["onboot"] = cfg.Onboot != 0
								attrs["protection"] = cfg.Protection != 0
								attrs["template"] = cfg.Template != 0
								attrs["agent_enabled"] = agentEnabled(cfg.Agent)
								attrs["unprivileged"] = cfg.Unprivileged != 0
								if cfg.OSType != "" {
									attrs["os_type"] = cfg.OSType
								}
							}
							if enabled, ok := p.fetchFirewallEnabled(ctx, "lxc", node.Node, ct.VMID); ok {
								attrs["firewall_enabled"] = enabled
							}
						}
						ent.Attributes = attrs
						entities = append(entities, ent)
					}
					nodeSection += "\n"
					totalCTs += len(ctsResponse.Data)
				}
			}()
		}

		// Fetch storage
		if wantStorage {
			func() {
				storageRaw, err := p.doRequest(ctx, "GET", "/nodes/"+node.Node+"/storage", nil)
				if err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}
				var storageResponse struct {
					Data []struct {
						Storage string `json:"storage"`
						Type    string `json:"type"`
						Used    int64  `json:"used"`
						Total   int64  `json:"total"`
						Avail   int64  `json:"avail"`
					} `json:"data"`
				}
				if err := json.Unmarshal(storageRaw, &storageResponse); err != nil {
					if nodeErr == nil {
						nodeErr = err
					}
					return
				}

				if len(storageResponse.Data) > 0 {
					nodeSection += "### Storage\n\n"
					nodeSection += "| Storage | Type | Used (bytes) | Total (bytes) | Avail (bytes) |\n"
					nodeSection += "|---------|------|---------------|----------------|----------------|\n"
					for _, st := range storageResponse.Data {
						nodeSection += fmt.Sprintf("| %s | %s | %d | %d | %d |\n",
							st.Storage, st.Type, st.Used, st.Total, st.Avail)
						dependencies = append(dependencies, connector.ServiceDependency{Kind: "storage", Name: st.Storage})
					}
					nodeSection += "\n"
					totalStorage += len(storageResponse.Data)
				}
			}()
		}

		section := connector.SnapshotSection{Title: node.Node, Content: nodeSection}
		if nodeErr != nil {
			section = connector.ErrorSection(node.Node, nodeErr)
		}
		sections = append(sections, section)
	}

	metadata["total_vms"] = fmt.Sprintf("%d", totalVMs)
	metadata["total_cts"] = fmt.Sprintf("%d", totalCTs)
	metadata["total_storage"] = fmt.Sprintf("%d", totalStorage)
	metadata["proxmox_url"] = p.url

	return &connector.ServiceSnapshot{
		ServiceName:  "Proxmox VE",
		Type:         typeName,
		Sections:     sections,
		Dependencies: dependencies,
		Entities:     entities,
		Metadata:     metadata,
		FetchedAt:    start,
	}, nil
}

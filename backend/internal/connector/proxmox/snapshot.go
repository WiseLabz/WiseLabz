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
		if wantEntities {
			entities = append(entities, connector.SnapshotEntity{Kind: "node", Name: node.Node, ExternalID: node.Node})
		}
		nr := &nodeResult{section: fmt.Sprintf("## Node: %s\n\n- **Status**: %s\n- **Memory**: %d MB\n\n",
			node.Node, node.Status, node.MaxMem/bytesPerMB)}
		dependencies = append(dependencies, connector.ServiceDependency{Kind: "host", Name: node.Node})

		if wantVMs {
			p.fetchVMs(ctx, node.Node, wantEntities, nr)
		}
		if wantContainers {
			p.fetchContainers(ctx, node.Node, wantEntities, nr)
		}
		if wantStorage {
			p.fetchStorage(ctx, node.Node, nr)
		}

		section := connector.SnapshotSection{Title: node.Node, Content: nr.section}
		if nr.err != nil {
			section = connector.ErrorSection(node.Node, nr.err)
		}
		sections = append(sections, section)
		entities = append(entities, nr.entities...)
		dependencies = append(dependencies, nr.dependencies...)
		totalVMs += nr.vms
		totalCTs += nr.cts
		totalStorage += nr.storage
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

// nodeResult accumulates what one node contributes to the snapshot. A failed
// resource call records only the first error and leaves the others to run.
type nodeResult struct {
	section      string
	entities     []connector.SnapshotEntity
	dependencies []connector.ServiceDependency
	vms, cts     int
	storage      int
	err          error
}

func (r *nodeResult) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

// guestList is the shared shape of the /qemu and /lxc listings.
type guestList struct {
	Data []struct {
		VMID   int    `json:"vmid"`
		Name   string `json:"name"`
		Status string `json:"status"`
		CPU    int    `json:"cpus"`
		MaxMem int64  `json:"maxmem"`
		Tags   string `json:"tags"`
	} `json:"data"`
}

// fetchGuests lists the guests of one kind ("qemu" or "lxc") on a node.
func (p *Connector) fetchGuests(ctx context.Context, node, kind string) (guestList, error) {
	var list guestList
	raw, err := p.doRequest(ctx, "GET", "/nodes/"+node+"/"+kind, nil)
	if err != nil {
		return list, err
	}
	err = json.Unmarshal(raw, &list)
	return list, err
}

func (p *Connector) fetchVMs(ctx context.Context, node string, wantEntities bool, nr *nodeResult) {
	list, err := p.fetchGuests(ctx, node, "qemu")
	if err != nil {
		nr.fail(err)
		return
	}
	if len(list.Data) == 0 {
		return
	}
	nr.section += "### Virtual Machines\n\n"
	nr.section += "| VMID | Name | Status | CPUs | Memory (MB) |\n"
	nr.section += "|------|------|--------|------|-------------|\n"
	for _, vm := range list.Data {
		nr.section += fmt.Sprintf("| %d | %s | %s | %d | %d |\n",
			vm.VMID, vm.Name, vm.Status, vm.CPU, vm.MaxMem/bytesPerMB)
		ent := connector.SnapshotEntity{
			Kind:       "vm",
			Name:       vm.Name,
			ExternalID: fmt.Sprintf("%d", vm.VMID),
		}
		attrs := map[string]any{"status": vm.Status}
		if wantEntities {
			if vm.MaxMem > 0 {
				attrs["memory"] = vm.MaxMem / bytesPerMB
			}
			attrs["node"] = node
			attrs["tags"] = parseTags(vm.Tags)
			if vm.Status == "running" {
				ent.IP = p.fetchQemuIP(ctx, node, vm.VMID)
			}
			if cfg, ok := p.fetchQemuConfig(ctx, node, vm.VMID); ok {
				applyConfigMemory(attrs, cfg.Memory, cfg.MemoryErr)
				if cfg.Cores != nil {
					attrs["cores"] = *cfg.Cores
				}
				attrs["onboot"] = cfg.Onboot != 0
				attrs["protection"] = cfg.Protection != 0
				attrs["template"] = cfg.Template != 0
				attrs["agent_enabled"] = agentEnabled(cfg.Agent)
				if cfg.OSType != "" {
					attrs["os_type"] = cfg.OSType
				}
			}
			if enabled, ok := p.fetchFirewallEnabled(ctx, "qemu", node, vm.VMID); ok {
				attrs["firewall_enabled"] = enabled
			}
		}
		ent.Attributes = attrs
		nr.entities = append(nr.entities, ent)
	}
	nr.section += "\n"
	nr.vms += len(list.Data)
}

func (p *Connector) fetchContainers(ctx context.Context, node string, wantEntities bool, nr *nodeResult) {
	list, err := p.fetchGuests(ctx, node, "lxc")
	if err != nil {
		nr.fail(err)
		return
	}
	if len(list.Data) == 0 {
		return
	}
	nr.section += "### Containers\n\n"
	nr.section += "| VMID | Name | Status | CPUs | Memory (MB) |\n"
	nr.section += "|------|------|--------|------|-------------|\n"
	for _, ct := range list.Data {
		nr.section += fmt.Sprintf("| %d | %s | %s | %d | %d |\n",
			ct.VMID, ct.Name, ct.Status, ct.CPU, ct.MaxMem/bytesPerMB)
		ent := connector.SnapshotEntity{
			Kind:       "container",
			Name:       ct.Name,
			ExternalID: fmt.Sprintf("%d", ct.VMID),
		}
		attrs := map[string]any{"status": ct.Status}
		if wantEntities {
			if ct.MaxMem > 0 {
				attrs["memory"] = ct.MaxMem / bytesPerMB
			}
			attrs["node"] = node
			attrs["tags"] = parseTags(ct.Tags)
			if ct.Status == "running" {
				ent.IP = p.fetchLxcIP(ctx, node, ct.VMID)
			}
			if cfg, ok := p.fetchLxcConfig(ctx, node, ct.VMID); ok {
				applyConfigMemory(attrs, cfg.Memory, cfg.MemoryErr)
				if cfg.Cores != nil {
					attrs["cores"] = *cfg.Cores
				}
				attrs["onboot"] = cfg.Onboot != 0
				attrs["protection"] = cfg.Protection != 0
				attrs["template"] = cfg.Template != 0
				attrs["agent_enabled"] = agentEnabled(cfg.Agent)
				attrs["unprivileged"] = cfg.Unprivileged != 0
				if cfg.OSType != "" {
					attrs["os_type"] = cfg.OSType
				}
			}
			if enabled, ok := p.fetchFirewallEnabled(ctx, "lxc", node, ct.VMID); ok {
				attrs["firewall_enabled"] = enabled
			}
		}
		ent.Attributes = attrs
		nr.entities = append(nr.entities, ent)
	}
	nr.section += "\n"
	nr.cts += len(list.Data)
}

// applyConfigMemory sets the memory attribute from a guest's /config, which
// shows a pending change before the restart. Without a memory key the guest
// runs with the default and nothing is pending, so the guest-list maxmem value
// stays. A key that could not be decoded is dropped: an unknown form must not
// be passed off as the configured value. When /config cannot be read at all the
// caller never gets here and the maxmem value stays as the fallback.
func applyConfigMemory(attrs map[string]any, memory *int, memoryErr error) {
	switch {
	case memoryErr != nil:
		delete(attrs, "memory")
	case memory != nil:
		attrs["memory"] = int64(*memory)
	}
}

func (p *Connector) fetchStorage(ctx context.Context, node string, nr *nodeResult) {
	raw, err := p.doRequest(ctx, "GET", "/nodes/"+node+"/storage", nil)
	if err != nil {
		nr.fail(err)
		return
	}
	var resp struct {
		Data []struct {
			Storage string `json:"storage"`
			Type    string `json:"type"`
			Used    int64  `json:"used"`
			Total   int64  `json:"total"`
			Avail   int64  `json:"avail"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		nr.fail(err)
		return
	}
	if len(resp.Data) == 0 {
		return
	}
	nr.section += "### Storage\n\n"
	nr.section += "| Storage | Type | Used (bytes) | Total (bytes) | Avail (bytes) |\n"
	nr.section += "|---------|------|---------------|----------------|----------------|\n"
	for _, st := range resp.Data {
		nr.section += fmt.Sprintf("| %s | %s | %d | %d | %d |\n",
			st.Storage, st.Type, st.Used, st.Total, st.Avail)
		nr.dependencies = append(nr.dependencies, connector.ServiceDependency{Kind: "storage", Name: st.Storage})
	}
	nr.section += "\n"
	nr.storage += len(resp.Data)
}

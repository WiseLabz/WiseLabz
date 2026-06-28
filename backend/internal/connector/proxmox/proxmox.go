// Package proxmox implements a real Proxmox VE API connector.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "proxmox"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "virtualization",
		Name:     "Proxmox VE",
		Fields: []connector.SchemaField{
			{Key: "url", Label: "API URL", Type: "text", Required: true, Placeholder: "https://pve.example.com:8006/api2/json"},
			{Key: "token_id", Label: "API Token ID", Type: "text", Required: true, Placeholder: "root@pam!monitoring"},
			{Key: "token_secret", Label: "API Token Secret", Type: "password", Required: true},
			{Key: "verify_tls", Label: "Verify TLS", Type: "toggle", Required: false, Default: "true"},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		url, _ := config["url"].(string)
		tokenID, _ := config["token_id"].(string)
		tokenSecret, _ := config["token_secret"].(string)
		verifyTLS := true
		if v, ok := config["verify_tls"]; ok {
			if b, ok := v.(bool); ok {
				verifyTLS = b
			}
		}
		client := &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: !verifyTLS},
			},
		}
		return &Connector{
			url:         strings.TrimSuffix(url, "/"),
			tokenID:     tokenID,
			tokenSecret: tokenSecret,
			client:      client,
		}, nil
	})
}

// Connector fetches data from a Proxmox VE API.
type Connector struct {
	url         string
	tokenID     string
	tokenSecret string
	client      *http.Client
}

// Name returns the connector display name.
func (p *Connector) Name() string { return "Proxmox VE" }

// Type returns the connector type.
func (p *Connector) Type() string { return typeName }

// Category returns the connector category.
func (p *Connector) Category() string { return "virtualization" }

// Validate tests the connection to the Proxmox API.
func (p *Connector) Validate(ctx context.Context, _ map[string]any) error {
	_, err := p.doRequest(ctx, "GET", "/nodes", nil)
	return err
}

// Fetch retrieves the current state of nodes, VMs, containers, and storage.
func (p *Connector) Fetch(ctx context.Context, _ map[string]any) (*connector.ServiceSnapshot, error) {
	start := time.Now()

	// Fetch nodes
	nodesRaw, err := p.doRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return nil, fmt.Errorf("fetch nodes: %w", err)
	}

	var nodesResponse struct {
		Data []struct {
			Node   string  `json:"node"`
			Status string  `json:"status"`
			Uptime int64   `json:"uptime"`
			CPU    float64 `json:"cpu"`
			Memory struct {
				Used  int64 `json:"used"`
				Total int64 `json:"total"`
			} `json:"mem"`
		} `json:"data"`
	}
	if err := json.Unmarshal(nodesRaw, &nodesResponse); err != nil {
		return nil, fmt.Errorf("decode nodes: %w", err)
	}

	var sections []connector.SnapshotSection
	metadata := map[string]string{
		"node_count": fmt.Sprintf("%d", len(nodesResponse.Data)),
	}

	// For each node, fetch VMs and containers
	totalVMs := 0
	totalCTs := 0

	for _, node := range nodesResponse.Data {
		nodeSection := fmt.Sprintf("## Node: %s\n\n", node.Node)
		nodeSection += fmt.Sprintf("- **Status**: %s\n", node.Status)
		nodeSection += fmt.Sprintf("- **Uptime**: %d seconds\n", node.Uptime)
		nodeSection += fmt.Sprintf("- **CPU**: %.2f%%\n", node.CPU*100)
		nodeSection += fmt.Sprintf("- **Memory**: %d / %d bytes\n\n", node.Memory.Used, node.Memory.Total)

		// Fetch VMs
		vmsRaw, err := p.doRequest(ctx, "GET", "/nodes/"+node.Node+"/qemu", nil)
		if err != nil {
			sections = append(sections, connector.SnapshotSection{
				Title:   node.Node,
				Content: nodeSection + "_VM fetch error: " + err.Error() + "_",
			})
			continue
		}
		var vmsResponse struct {
			Data []struct {
				VMID   int    `json:"vmid"`
				Name   string `json:"name"`
				Status string `json:"status"`
				CPU    int    `json:"cpus"`
				Memory int64  `json:"mem"`
				Uptime int64  `json:"uptime"`
			} `json:"data"`
		}
		if err := json.Unmarshal(vmsRaw, &vmsResponse); err != nil {
			sections = append(sections, connector.SnapshotSection{
				Title:   node.Node,
				Content: nodeSection + "_VM decode error: " + err.Error() + "_",
			})
			continue
		}

		if len(vmsResponse.Data) > 0 {
			nodeSection += "### Virtual Machines\n\n"
			nodeSection += "| VMID | Name | Status | CPUs | Memory (MB) | Uptime |\n"
			nodeSection += "|------|------|--------|------|-------------|--------|\n"
			for _, vm := range vmsResponse.Data {
				nodeSection += fmt.Sprintf("| %d | %s | %s | %d | %d | %d |\n",
					vm.VMID, vm.Name, vm.Status, vm.CPU, vm.Memory, vm.Uptime)
			}
			nodeSection += "\n"
			totalVMs += len(vmsResponse.Data)
		}

		// Fetch containers
		ctsRaw, err := p.doRequest(ctx, "GET", "/nodes/"+node.Node+"/lxc", nil)
		if err != nil {
			sections = append(sections, connector.SnapshotSection{
				Title:   node.Node,
				Content: nodeSection + "_CT fetch error: " + err.Error() + "_",
			})
			continue
		}
		var ctsResponse struct {
			Data []struct {
				VMID   int    `json:"vmid"`
				Name   string `json:"name"`
				Status string `json:"status"`
				CPU    int    `json:"cpus"`
				Memory int64  `json:"mem"`
				Uptime int64  `json:"uptime"`
			} `json:"data"`
		}
		if err := json.Unmarshal(ctsRaw, &ctsResponse); err != nil {
			sections = append(sections, connector.SnapshotSection{
				Title:   node.Node,
				Content: nodeSection + "_CT decode error: " + err.Error() + "_",
			})
			continue
		}

		if len(ctsResponse.Data) > 0 {
			nodeSection += "### Containers\n\n"
			nodeSection += "| VMID | Name | Status | CPUs | Memory (MB) | Uptime |\n"
			nodeSection += "|------|------|--------|------|-------------|--------|\n"
			for _, ct := range ctsResponse.Data {
				nodeSection += fmt.Sprintf("| %d | %s | %s | %d | %d | %d |\n",
					ct.VMID, ct.Name, ct.Status, ct.CPU, ct.Memory, ct.Uptime)
			}
			nodeSection += "\n"
			totalCTs += len(ctsResponse.Data)
		}

		sections = append(sections, connector.SnapshotSection{
			Title:   node.Node,
			Content: nodeSection,
		})
	}

	metadata["total_vms"] = fmt.Sprintf("%d", totalVMs)
	metadata["total_cts"] = fmt.Sprintf("%d", totalCTs)
	metadata["proxmox_url"] = p.url

	return &connector.ServiceSnapshot{
		ServiceName: "Proxmox VE",
		Type:        typeName,
		Sections:    sections,
		Metadata:    metadata,
		FetchedAt:   start,
	}, nil
}

func (p *Connector) doRequest(ctx context.Context, method, path string, _ io.Reader) ([]byte, error) {
	url := p.url + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "PVEAPIToken="+p.tokenID+"="+p.tokenSecret)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// Package docker implements a real Docker Engine API connector.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

const typeName = "docker"

func init() {
	connector.Register(connector.TypeSchema{
		Type:     typeName,
		Category: "containers_paas",
		Name:     "Docker",
		Fields: []connector.SchemaField{
			{Key: "host", Label: "Docker Host", Type: "text", Required: true, Placeholder: "unix:///var/run/docker.sock or tcp://host:2375"},
		},
	}, func(config map[string]any) (connector.Connector, error) {
		host, _ := config["host"].(string)
		client, baseURL, err := newDockerClient(host)
		// Construction never fails here: an invalid/empty host (e.g. an
		// unconfigured connector instance) surfaces as an error from
		// Validate/Fetch instead, matching how other connectors treat
		// missing config as a runtime rather than a registration error.
		return &Connector{host: host, baseURL: baseURL, client: client, configErr: err}, nil
	})
}

// Connector fetches data from the Docker Engine API.
type Connector struct {
	host      string
	baseURL   string
	client    *http.Client
	configErr error
}

// Name returns the connector display name.
func (d *Connector) Name() string { return "Docker" }

// Type returns the connector type identifier.
func (d *Connector) Type() string { return typeName }

// Category returns the connector category.
func (d *Connector) Category() string { return "containers_paas" }

// Validate tests the connection to the Docker Engine API.
func (d *Connector) Validate(ctx context.Context, _ map[string]any) error {
	_, err := d.doRequest(ctx, "/version")
	return err
}

// Fetch retrieves engine info, containers, images, volumes, and networks.
// config may carry a "fields" selective-fetch hint naming a subset of
// {"containers","images","volumes","networks"} to skip the other calls.
// The System section always runs. Each section fetch failure is tolerated
// as a placeholder rather than failing the whole Fetch.
func (d *Connector) Fetch(ctx context.Context, config map[string]any) (*connector.ServiceSnapshot, error) {
	start := time.Now()
	fields := connector.RequestedFields(config)

	var sections []connector.SnapshotSection
	metadata := map[string]string{"docker_host": d.host}

	if raw, err := d.doRequest(ctx, "/info"); err != nil {
		sections = append(sections, connector.SnapshotSection{Title: "System", Content: "_System info unavailable: " + err.Error() + "_"})
	} else {
		var info struct {
			Name              string `json:"Name"`
			ServerVersion     string `json:"ServerVersion"`
			Containers        int    `json:"Containers"`
			ContainersRunning int    `json:"ContainersRunning"`
			Images            int    `json:"Images"`
			NCPU              int    `json:"NCPU"`
			MemTotal          int64  `json:"MemTotal"`
		}
		if err := json.Unmarshal(raw, &info); err != nil {
			sections = append(sections, connector.SnapshotSection{
				Title:   "System",
				Content: "_System info unavailable: " + connector.NewMalformedResponseError(err).Error() + "_",
			})
		} else {
			content := fmt.Sprintf("**Host**: %s\n**Engine Version**: %s\n**Containers**: %d (%d running)\n**Images**: %d\n**CPUs**: %d\n**Memory**: %d bytes\n",
				info.Name, info.ServerVersion, info.Containers, info.ContainersRunning, info.Images, info.NCPU, info.MemTotal)
			sections = append(sections, connector.SnapshotSection{Title: "System", Content: content})
			metadata["engine_version"] = info.ServerVersion
		}
	}

	if connector.WantsField(fields, "containers") {
		sections = append(sections, d.fetchSection(ctx, "Containers", "/containers/json?all=true", buildContainerTable))
	}
	if connector.WantsField(fields, "images") {
		sections = append(sections, d.fetchSection(ctx, "Images", "/images/json", buildImageTable))
	}
	if connector.WantsField(fields, "volumes") {
		sections = append(sections, d.fetchSection(ctx, "Volumes", "/volumes", buildVolumeTable))
	}
	if connector.WantsField(fields, "networks") {
		sections = append(sections, d.fetchSection(ctx, "Networks", "/networks", buildNetworkTable))
	}

	return &connector.ServiceSnapshot{
		ServiceName: "Docker",
		Type:        typeName,
		Sections:    sections,
		Dependencies: []connector.ServiceDependency{
			{Kind: "host", Name: d.host},
		},
		Metadata:  metadata,
		FetchedAt: start,
	}, nil
}

// fetchSection runs a single endpoint fetch and renders it with build,
// tolerating failure as a placeholder section rather than failing Fetch.
func (d *Connector) fetchSection(ctx context.Context, title, path string, build func([]byte) string) connector.SnapshotSection {
	raw, err := d.doRequest(ctx, path)
	if err != nil {
		return connector.SnapshotSection{Title: title, Content: "_" + title + " unavailable: " + err.Error() + "_"}
	}
	return connector.SnapshotSection{Title: title, Content: build(raw)}
}

func (d *Connector) doRequest(ctx context.Context, path string) ([]byte, error) {
	if d.configErr != nil {
		return nil, d.configErr
	}
	req, err := http.NewRequestWithContext(ctx, "GET", d.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		if isTimeout(err) {
			return nil, connector.NewTimeoutError(fmt.Errorf("request failed: %w", err))
		}
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout:
		return nil, connector.NewServiceUnavailableError(fmt.Errorf("API returned %d: %s", resp.StatusCode, string(data)))
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// newDockerClient builds an HTTP client and base URL for the given Docker
// host address. unix:// sockets are dialed directly; tcp:// hosts are dialed
// through a guarded dialer that rejects loopback/link-local targets (mirrors
// custom.newGuardedClient — to be shared via connector.go in the pfSense PR).
// Any other scheme (including ssh://, deferred to a fast-follow) is rejected.
func newDockerClient(host string) (*http.Client, string, error) {
	switch {
	case strings.HasPrefix(host, "unix://"):
		socketPath := strings.TrimPrefix(host, "unix://")
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socketPath)
			},
		}
		return &http.Client{Timeout: 30 * time.Second, Transport: transport}, "http://unix", nil

	case strings.HasPrefix(host, "tcp://"):
		addr := strings.TrimPrefix(host, "tcp://")
		dialer := &net.Dialer{
			Timeout: 30 * time.Second,
			Control: func(_, address string, _ syscall.RawConn) error {
				h, _, err := net.SplitHostPort(address)
				if err != nil {
					return fmt.Errorf("split address %q: %w", address, err)
				}
				ip := net.ParseIP(h)
				if ip == nil {
					return fmt.Errorf("unresolvable address %q", h)
				}
				if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
					return fmt.Errorf("connection to blocked address %s denied", ip)
				}
				return nil
			},
		}
		transport := &http.Transport{DialContext: dialer.DialContext}
		return &http.Client{Timeout: 30 * time.Second, Transport: transport}, "http://" + addr, nil

	default:
		return nil, "", fmt.Errorf("unsupported docker host scheme in %q (only unix:// and tcp:// are supported)", host)
	}
}

func buildContainerTable(raw []byte) string {
	var containers []struct {
		Names  []string `json:"Names"`
		Image  string   `json:"Image"`
		State  string   `json:"State"`
		Status string   `json:"Status"`
	}
	if err := json.Unmarshal(raw, &containers); err != nil || len(containers) == 0 {
		return "_No containers returned_"
	}
	var b strings.Builder
	b.WriteString("| Name | Image | State | Status |\n")
	b.WriteString("|------|-------|-------|--------|\n")
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", name, c.Image, c.State, c.Status); err != nil {
			return ""
		}
	}
	return b.String()
}

func buildImageTable(raw []byte) string {
	var images []struct {
		RepoTags []string `json:"RepoTags"`
		Size     int64    `json:"Size"`
	}
	if err := json.Unmarshal(raw, &images); err != nil || len(images) == 0 {
		return "_No images returned_"
	}
	var b strings.Builder
	b.WriteString("| Tags | Size (bytes) |\n")
	b.WriteString("|------|---------------|\n")
	for _, img := range images {
		tags := strings.Join(img.RepoTags, ", ")
		if tags == "" {
			tags = "<none>"
		}
		if _, err := fmt.Fprintf(&b, "| %s | %d |\n", tags, img.Size); err != nil {
			return ""
		}
	}
	return b.String()
}

func buildVolumeTable(raw []byte) string {
	var resp struct {
		Volumes []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
		} `json:"Volumes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil || len(resp.Volumes) == 0 {
		return "_No volumes returned_"
	}
	var b strings.Builder
	b.WriteString("| Name | Driver | Mountpoint |\n")
	b.WriteString("|------|--------|------------|\n")
	for _, v := range resp.Volumes {
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s |\n", v.Name, v.Driver, v.Mountpoint); err != nil {
			return ""
		}
	}
	return b.String()
}

func buildNetworkTable(raw []byte) string {
	var networks []struct {
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := json.Unmarshal(raw, &networks); err != nil || len(networks) == 0 {
		return "_No networks returned_"
	}
	var b strings.Builder
	b.WriteString("| Name | Driver | Scope |\n")
	b.WriteString("|------|--------|-------|\n")
	for _, n := range networks {
		if _, err := fmt.Fprintf(&b, "| %s | %s | %s |\n", n.Name, n.Driver, n.Scope); err != nil {
			return ""
		}
	}
	return b.String()
}

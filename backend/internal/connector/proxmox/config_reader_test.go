package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRead(t *testing.T) {
	failQemuConfig := false
	lxcCoresNull := false
	qemuMemory := `2048`
	lxcMemory := `2048`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online"}]}`))
		case "/nodes/pve1/qemu":
			// Guest list has maxmem: 4294967296 (4096 MB)
			_, _ = w.Write([]byte(`{"data":[{"vmid":100,"name":"vm1","status":"stopped","cpus":8,"maxmem":4294967296},{"vmid":101,"name":"vm2","status":"stopped","cpus":2}]}`))
		case "/nodes/pve1/lxc":
			// Guest list has maxmem: 4294967296 (4096 MB)
			_, _ = w.Write([]byte(`{"data":[{"vmid":200,"name":"ct1","status":"stopped","cpus":16,"maxmem":4294967296}]}`))
		case "/nodes/pve1/qemu/100/config":
			if failQemuConfig {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("unavailable"))
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":{"cores":4,"sockets":2,"memory":%s}}`, qemuMemory)
		case "/nodes/pve1/qemu/101/config":
			_, _ = w.Write([]byte(`{"data":{"cores":null}}`))
		case "/nodes/pve1/lxc/200/config":
			if lxcCoresNull {
				_, _ = w.Write([]byte(`{"data":{"cores":null,"cpulimit":0}}`))
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":{"cores":3,"cpulimit":0,"memory":%s}}`, lxcMemory)
		case "/nodes/pve1/qemu/100/firewall/options", "/nodes/pve1/qemu/101/firewall/options", "/nodes/pve1/lxc/200/firewall/options":
			_, _ = w.Write([]byte(`{"data":{"enable":0}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, tokenID: "user@pam!token", tokenSecret: "secret", client: server.Client()}

	// Test VM and LXC with guest list maxmem 4096 and /config memory 2048 in different formats:
	// JSON number, numeric string, and current=<n> property string.
	memoryFormats := []struct {
		name       string
		qemuFormat string
		lxcFormat  string
	}{
		{name: "JSON number", qemuFormat: `2048`, lxcFormat: `2048`},
		{name: "numeric string", qemuFormat: `"2048"`, lxcFormat: `"2048"`},
		{name: "current=<n> property string", qemuFormat: `"current=2048"`, lxcFormat: `"current=2048"`},
		{name: "current=<n> with additional options", qemuFormat: `"current=2048,max=4096"`, lxcFormat: `"shares=1000,current=2048"`},
	}

	for _, fmtCase := range memoryFormats {
		t.Run(fmtCase.name, func(t *testing.T) {
			qemuMemory = fmtCase.qemuFormat
			lxcMemory = fmtCase.lxcFormat

			for _, tc := range []struct {
				entityRef string
				fieldKey  string
				want      float64
			}{
				{entityRef: "100", fieldKey: "memory", want: 2048}, // guest list has 4096, /config has 2048
				{entityRef: "100", fieldKey: "cores", want: 4},     // configured cores, not cpus=8
				{entityRef: "200", fieldKey: "memory", want: 2048}, // guest list has 4096, /config has 2048
				{entityRef: "200", fieldKey: "cores", want: 3},
			} {
				value, err := c.ConfigRead(context.Background(), nil, tc.entityRef, tc.fieldKey)
				if err != nil || value != tc.want {
					t.Errorf("[%s] ConfigRead(%q, %q) = (%#v, %v), want (%v, nil)", fmtCase.name, tc.entityRef, tc.fieldKey, value, err, tc.want)
				}
			}
		})
	}

	if _, err := c.ConfigRead(context.Background(), nil, "not-a-vmid", "memory"); err == nil {
		t.Error("ConfigRead() error = nil for invalid VMID")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "100", "status"); err == nil {
		t.Error("ConfigRead() error = nil for unsupported field")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "999", "memory"); err == nil {
		t.Error("ConfigRead() error = nil for missing VMID")
	}
	if _, err := c.ConfigRead(context.Background(), nil, "101", "memory"); err == nil {
		t.Error("ConfigRead() memory error = nil when memory is absent")
	}
	if value, err := c.ConfigRead(context.Background(), nil, "101", "cores"); err != nil || value != nil {
		t.Errorf("ConfigRead() cores for null config cores = (%#v, %v), want (nil, nil)", value, err)
	}

	failQemuConfig = true
	if _, err := c.ConfigRead(context.Background(), nil, "100", "cores"); err == nil {
		t.Error("ConfigRead() cores error = nil when config fetch fails")
	}
	if value, err := c.ConfigRead(context.Background(), nil, "100", "memory"); err != nil || value != float64(4096) {
		t.Errorf("ConfigRead() memory after config fetch failure = (%#v, %v), want (4096, nil)", value, err)
	}

	failQemuConfig = false
	lxcCoresNull = true
	if value, err := c.ConfigRead(context.Background(), nil, "200", "cores"); err != nil || value != nil {
		t.Errorf("ConfigRead() cores for null LXC cores = (%#v, %v), want (nil, nil), not zero", value, err)
	}
}

func TestDecodeProxmoxMemory(t *testing.T) {
	intPtr := func(i int) *int { return &i }

	for _, tc := range []struct {
		name    string
		raw     string
		want    *int
		wantErr bool
	}{
		{name: "empty", raw: "", want: nil},
		{name: "null", raw: "null", want: nil},
		{name: "number", raw: "2048", want: intPtr(2048)},
		{name: "float number", raw: "2048.0", want: intPtr(2048)},
		{name: "numeric string", raw: `"2048"`, want: intPtr(2048)},
		{name: "numeric string with spaces", raw: `" 2048 "`, want: intPtr(2048)},
		{name: "current format", raw: `"current=2048"`, want: intPtr(2048)},
		{name: "current with max", raw: `"current=2048,max=4096"`, want: intPtr(2048)},
		{name: "max before current", raw: `"max=4096,current=2048"`, want: intPtr(2048)},
		{name: "invalid string", raw: `"invalid"`, wantErr: true},
		{name: "invalid current", raw: `"current=abc"`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeProxmoxMemory(json.RawMessage(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Errorf("decodeProxmoxMemory(%q) error = nil, want error", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeProxmoxMemory(%q) unexpected error: %v", tc.raw, err)
			}
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Errorf("decodeProxmoxMemory(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

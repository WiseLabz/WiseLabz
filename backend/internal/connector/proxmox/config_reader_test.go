package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

func TestConfigRead(t *testing.T) {
	failQemuConfig := false
	failLxcConfig := false
	lxcCoresNull := false
	qemuMemory := `2048`
	lxcMemory := `2048`

	var hitsMu sync.Mutex
	hits := map[string]int{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsMu.Lock()
		hits[r.URL.Path]++
		hitsMu.Unlock()
		switch r.URL.Path {
		case "/nodes":
			_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online"}]}`))
		case "/nodes/pve1/qemu":
			// Guest list has maxmem: 4294967296 (4096 MB)
			_, _ = w.Write([]byte(`{"data":[{"vmid":100,"name":"vm1","status":"stopped","cpus":8,"maxmem":4294967296},{"vmid":101,"name":"vm2","status":"stopped","cpus":2},{"vmid":102,"name":"vm3","status":"stopped","cpus":2,"maxmem":4294967296}]}`))
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
		case "/nodes/pve1/qemu/102/config":
			_, _ = w.Write([]byte(`{"data":{"cores":2}}`))
		case "/nodes/pve1/lxc/200/config":
			if failLxcConfig {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("unavailable"))
				return
			}
			if lxcCoresNull {
				_, _ = w.Write([]byte(`{"data":{"cores":null,"cpulimit":0}}`))
				return
			}
			_, _ = fmt.Fprintf(w, `{"data":{"cores":3,"cpulimit":0,"memory":%s}}`, lxcMemory)
		case "/nodes/pve1/qemu/100/firewall/options", "/nodes/pve1/qemu/101/firewall/options", "/nodes/pve1/qemu/102/firewall/options", "/nodes/pve1/lxc/200/firewall/options":
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

	// /config succeeds without a memory key: the guest-list value is the
	// configured one (default memory, nothing pending).
	if value, err := c.ConfigRead(context.Background(), nil, "102", "memory"); err != nil || value != float64(4096) {
		t.Errorf("ConfigRead() memory without a config memory key = (%#v, %v), want (4096, nil)", value, err)
	}

	// /config succeeds but its memory cannot be decoded: never fall back to
	// the running maxmem or pass the unknown form off as a value.
	qemuMemory, lxcMemory = `"max=4096"`, `"max=4096"`
	for _, ref := range []string{"100", "200"} {
		if value, err := c.ConfigRead(context.Background(), nil, ref, "memory"); err == nil {
			t.Errorf("ConfigRead(%s, memory) with undecodable config memory = (%#v, nil), want error", ref, value)
		}
	}
	qemuMemory, lxcMemory = `2048`, `2048`

	// /config cannot be fetched: memory is an error, not the running maxmem.
	failQemuConfig, failLxcConfig = true, true
	if _, err := c.ConfigRead(context.Background(), nil, "100", "cores"); err == nil {
		t.Error("ConfigRead() cores error = nil when config fetch fails")
	}
	for _, ref := range []string{"100", "200"} {
		if value, err := c.ConfigRead(context.Background(), nil, ref, "memory"); err == nil {
			t.Errorf("ConfigRead(%s, memory) after config fetch failure = (%#v, nil), want error", ref, value)
		}
	}

	failQemuConfig, failLxcConfig = false, false
	lxcCoresNull = true
	if value, err := c.ConfigRead(context.Background(), nil, "200", "cores"); err != nil || value != nil {
		t.Errorf("ConfigRead() cores for null LXC cores = (%#v, %v), want (nil, nil), not zero", value, err)
	}

	// One ConfigRead makes no extra per-guest call: every guest's /config is
	// fetched exactly once and nothing outside the snapshot's own requests.
	hitsMu.Lock()
	hits = map[string]int{}
	hitsMu.Unlock()
	if _, err := c.ConfigRead(context.Background(), nil, "100", "memory"); err != nil {
		t.Fatalf("ConfigRead() error = %v", err)
	}
	hitsMu.Lock()
	defer hitsMu.Unlock()
	wantHits := map[string]int{
		"/nodes":                                1,
		"/nodes/pve1/qemu":                      1,
		"/nodes/pve1/lxc":                       1,
		"/nodes/pve1/qemu/100/config":           1,
		"/nodes/pve1/qemu/101/config":           1,
		"/nodes/pve1/qemu/102/config":           1,
		"/nodes/pve1/lxc/200/config":            1,
		"/nodes/pve1/qemu/100/firewall/options": 1,
		"/nodes/pve1/qemu/101/firewall/options": 1,
		"/nodes/pve1/qemu/102/firewall/options": 1,
		"/nodes/pve1/lxc/200/firewall/options":  1,
	}
	if !reflect.DeepEqual(hits, wantHits) {
		t.Errorf("requests of one ConfigRead = %v, want %v", hits, wantHits)
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
		{name: "bare first element", raw: `"2048,foo=bar"`, want: intPtr(2048)},
		{name: "current after other options", raw: `"foo=bar,current=2048"`, want: intPtr(2048)},
		{name: "invalid string", raw: `"invalid"`, wantErr: true},
		{name: "invalid current", raw: `"current=abc"`, wantErr: true},
		{name: "empty string", raw: `""`, wantErr: true},
		{name: "empty current", raw: `"current="`, wantErr: true},
		{name: "NaN string", raw: `"NaN"`, wantErr: true},
		{name: "Inf string", raw: `"Inf"`, wantErr: true},
		{name: "exponent string", raw: `"1e3"`, wantErr: true},
		{name: "hex float string", raw: `"0x1p10"`, wantErr: true},
		{name: "fractional string", raw: `"2048.5"`, wantErr: true},
		{name: "zero string", raw: `"0"`, wantErr: true},
		{name: "negative string", raw: `"-1"`, wantErr: true},
		{name: "max without current", raw: `"max=4096"`, wantErr: true},
		{name: "keyed first element without current", raw: `"max=4096,foo=bar"`, wantErr: true},
		{name: "fractional number", raw: `2048.5`, wantErr: true},
		{name: "zero number", raw: `0`, wantErr: true},
		{name: "negative number", raw: `-1`, wantErr: true},
		{name: "huge number", raw: `1e30`, wantErr: true},
		{name: "out of range number", raw: `1e999`, wantErr: true},
		{name: "bool", raw: `true`, wantErr: true},
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

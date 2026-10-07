package proxmox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigRead(t *testing.T) {
	failQemuConfig := false
	lxcCoresNull := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			_, _ = w.Write([]byte(`{"data":[{"node":"pve1","status":"online"}]}`))
		case "/nodes/pve1/qemu":
			_, _ = w.Write([]byte(`{"data":[{"vmid":100,"name":"vm1","status":"stopped","cpus":8,"maxmem":4294967296},{"vmid":101,"name":"vm2","status":"stopped","cpus":2}]}`))
		case "/nodes/pve1/lxc":
			_, _ = w.Write([]byte(`{"data":[{"vmid":200,"name":"ct1","status":"stopped","cpus":16,"maxmem":2147483648}]}`))
		case "/nodes/pve1/qemu/100/config":
			if failQemuConfig {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("unavailable"))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"cores":4,"sockets":2}}`))
		case "/nodes/pve1/qemu/101/config":
			_, _ = w.Write([]byte(`{"data":{"cores":null}}`))
		case "/nodes/pve1/lxc/200/config":
			if lxcCoresNull {
				_, _ = w.Write([]byte(`{"data":{"cores":null,"cpulimit":0}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"cores":3,"cpulimit":0}}`))
		case "/nodes/pve1/qemu/100/firewall/options", "/nodes/pve1/qemu/101/firewall/options", "/nodes/pve1/lxc/200/firewall/options":
			_, _ = w.Write([]byte(`{"data":{"enable":0}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	c := &Connector{url: server.URL, tokenID: "user@pam!token", tokenSecret: "secret", client: server.Client()}
	for _, tc := range []struct {
		entityRef string
		fieldKey  string
		want      float64
	}{
		{entityRef: "100", fieldKey: "memory", want: 4096},
		{entityRef: "100", fieldKey: "cores", want: 4}, // configured cores, not cpus=8 (cores * sockets)
		{entityRef: "200", fieldKey: "memory", want: 2048},
		{entityRef: "200", fieldKey: "cores", want: 3},
	} {
		value, err := c.ConfigRead(context.Background(), nil, tc.entityRef, tc.fieldKey)
		if err != nil || value != tc.want {
			t.Errorf("ConfigRead(%q, %q) = (%#v, %v), want (%v, nil)", tc.entityRef, tc.fieldKey, value, err, tc.want)
		}
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
		t.Error("ConfigRead() memory error = nil when maxmem is absent")
	}

	failQemuConfig = true
	if _, err := c.ConfigRead(context.Background(), nil, "100", "cores"); err == nil {
		t.Error("ConfigRead() cores error = nil when config fetch fails")
	}
	if value, err := c.ConfigRead(context.Background(), nil, "100", "memory"); err != nil || value != float64(4096) {
		t.Errorf("ConfigRead() memory after config fetch failure = (%#v, %v), want (4096, nil)", value, err)
	}

	lxcCoresNull = true
	if _, err := c.ConfigRead(context.Background(), nil, "200", "cores"); err == nil {
		t.Error("ConfigRead() cores error = nil for null LXC cores, which must not be treated as zero")
	}
}

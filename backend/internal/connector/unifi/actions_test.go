package unifi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	poeDevicesJSON = `{"meta":{"rc":"ok"},"data":[
		{"mac":"aa:bb:cc:00:11:33","name":"Core Switch","port_table":[
			{"port_idx":1,"name":"Uplink","port_poe":false},
			{"port_idx":2,"name":"Living Room AP","port_poe":true,"poe_enable":true},
			{"port_idx":3,"port_poe":true,"poe_enable":true}]},
		{"mac":"aa:bb:cc:00:11:22","name":"Living Room AP","uplink":{"uplink_mac":"aa:bb:cc:00:11:33","uplink_remote_port":2}}
	]}`
	poeClientsJSON = `{"meta":{"rc":"ok"},"data":[
		{"mac":"44:44:44:44:44:44","name":"Porch Camera","is_wired":true,"sw_mac":"aa:bb:cc:00:11:33","sw_port":3},
		{"mac":"55:55:55:55:55:55","is_wired":false,"sw_mac":"aa:bb:cc:00:11:33","sw_port":3}
	]}`
)

func TestBuildPoEPorts(t *testing.T) {
	ports := buildPoEPorts([]byte(poeDevicesJSON), []byte(poeClientsJSON))
	if len(ports) != 2 {
		t.Fatalf("ports = %d, want 2 (non-PoE uplink skipped)", len(ports))
	}
	if ports[0].ExternalID != "aa:bb:cc:00:11:33:2" || ports[0].Name != "Core Switch / Living Room AP" {
		t.Errorf("port[0] = %q / %q", ports[0].ExternalID, ports[0].Name)
	}
	if got := ports[0].Attributes["connectedDevices"]; !reflect.DeepEqual(got, []string{"Living Room AP"}) {
		t.Errorf("port 2 connectedDevices = %v", got)
	}
	if got := ports[1].Attributes["connectedDevices"]; !reflect.DeepEqual(got, []string{"Porch Camera"}) {
		t.Errorf("port 3 connectedDevices = %v (wireless client must be ignored)", got)
	}
	if ports[1].Name != "Core Switch / Port 3" {
		t.Errorf("port[1].Name = %q", ports[1].Name)
	}
}

func TestBuildPoEPortsWithoutClients(t *testing.T) {
	ports := buildPoEPorts([]byte(poeDevicesJSON), nil)
	if got := ports[1].Attributes["connectedDevices"]; !reflect.DeepEqual(got, []string{}) {
		t.Errorf("connectedDevices = %v, want empty", got)
	}
}

func TestRestart(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want map[string]any
	}{
		{"device", "aa:bb:cc:00:11:22", map[string]any{"cmd": "restart", "mac": "aa:bb:cc:00:11:22"}},
		{"poe port", "aa:bb:cc:00:11:33:2", map[string]any{"cmd": "power-cycle", "mac": "aa:bb:cc:00:11:33", "port_idx": float64(2)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotBody map[string]any
			api := unifiAPI(t, controllerOpts{unifiOS: true, apiKey: "k"})
			mux := http.NewServeMux()
			mux.HandleFunc("/proxy/network/api/s/default/cmd/devmgr", func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				if r.Method != http.MethodPost || r.Header.Get("X-API-KEY") != "k" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				b, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(b, &gotBody)
				_, _ = w.Write([]byte(`{"meta":{"rc":"ok"},"data":[]}`))
			})
			mux.Handle("/", api.Config.Handler)
			server := httptest.NewServer(mux)
			defer server.Close()
			c := newTestConnector(t, map[string]any{"url": server.URL, "auth_mode": authAPIKey, "api_key": "k", "controller_type": controllerUniFiOS})

			if err := c.Restart(context.Background(), nil, tt.ref); err != nil {
				t.Fatalf("Restart(%q): %v", tt.ref, err)
			}
			if gotPath == "" {
				t.Fatal("devmgr endpoint was not called")
			}
			if !reflect.DeepEqual(gotBody, tt.want) {
				t.Errorf("body = %v, want %v", gotBody, tt.want)
			}
		})
	}
}

func TestRestartRejectsBadRef(t *testing.T) {
	c := newTestConnector(t, map[string]any{"url": "http://127.0.0.1:1", "auth_mode": authAPIKey, "api_key": "k"})
	for _, ref := range []string{"", "../x", "aa:bb:cc:00:11:33:zero", "aa:bb:cc:00:11:33:0"} {
		if err := c.Restart(context.Background(), nil, ref); err == nil {
			t.Errorf("Restart(%q) succeeded, want error", ref)
		}
	}
}

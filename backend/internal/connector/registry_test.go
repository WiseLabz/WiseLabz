package connector

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type registryTestRefresher struct{}

func (registryTestRefresher) Name() string     { return "refresher" }
func (registryTestRefresher) Type() string     { return "registry_test_refresher" }
func (registryTestRefresher) Category() string { return "test" }
func (registryTestRefresher) Fetch(_ context.Context, _ map[string]any) (*ServiceSnapshot, error) {
	return nil, nil
}
func (registryTestRefresher) Validate(_ context.Context, _ map[string]any) error { return nil }
func (registryTestRefresher) RefreshCredentials(_ context.Context, config map[string]any) (map[string]any, time.Time, error) {
	return config, time.Time{}, nil
}

func TestRegisterStubRoundTrips(t *testing.T) {
	Register(TypeSchema{Type: "registry_test_stub", Category: "test", Name: "Stub", Stub: true},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	got, err := GetTypeSchema("registry_test_stub")
	if err != nil {
		t.Fatalf("GetTypeSchema: %v", err)
	}
	if !got.Stub {
		t.Errorf("Stub = false, want true")
	}

	found := false
	for _, s := range ListSchemas() {
		if s.Type == "registry_test_stub" {
			found = true
			if !s.Stub {
				t.Errorf("ListSchemas: Stub = false, want true")
			}
		}
	}
	if !found {
		t.Fatal("registry_test_stub not found in ListSchemas()")
	}
}

func TestRegisterDefaultsToNonStub(t *testing.T) {
	Register(TypeSchema{Type: "registry_test_real", Category: "test", Name: "Real"},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	got, err := GetTypeSchema("registry_test_real")
	if err != nil {
		t.Fatalf("GetTypeSchema: %v", err)
	}
	if got.Stub {
		t.Errorf("Stub = true, want false")
	}
}

func TestIsCredentialRefresherType(t *testing.T) {
	Register(TypeSchema{Type: "registry_test_refresher", Category: "test", Name: "Refresher"},
		func(_ map[string]any) (Connector, error) { return registryTestRefresher{}, nil })
	Register(TypeSchema{Type: "registry_test_nonrefresher", Category: "test", Name: "Non-refresher"},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	if !IsCredentialRefresherType("registry_test_refresher") {
		t.Error("IsCredentialRefresherType(refresher) = false, want true")
	}
	if IsCredentialRefresherType("registry_test_nonrefresher") {
		t.Error("IsCredentialRefresherType(non-refresher) = true, want false")
	}
	if IsCredentialRefresherType("registry_test_unknown_type") {
		t.Error("IsCredentialRefresherType(unknown) = true, want false")
	}

	for _, s := range ListSchemas() {
		switch s.Type {
		case "registry_test_refresher":
			if !s.IsCredentialRefresher {
				t.Error("ListSchemas: refresher's IsCredentialRefresher = false, want true")
			}
		case "registry_test_nonrefresher":
			if s.IsCredentialRefresher {
				t.Error("ListSchemas: non-refresher's IsCredentialRefresher = true, want false")
			}
		}
	}
}

func TestApplyRecordConfig(t *testing.T) {
	t.Run("empty url leaves the key absent", func(t *testing.T) {
		cfg := map[string]any{"url": "stale", "token": "t"}
		ApplyRecordConfig(cfg, "", false)
		if _, ok := cfg["url"]; ok {
			t.Fatalf("url key present: %v", cfg)
		}
		if cfg["verify_tls"] != false || cfg["token"] != "t" {
			t.Fatalf("cfg = %v", cfg)
		}
	})
	t.Run("non-empty url is kept", func(t *testing.T) {
		cfg := map[string]any{}
		ApplyRecordConfig(cfg, "https://a.example", true)
		if cfg["url"] != "https://a.example" || cfg["verify_tls"] != true {
			t.Fatalf("cfg = %v", cfg)
		}
	})
}

func TestDiscoveryHintsListsOnlyTypesWithAHint(t *testing.T) {
	match := func(DiscoveryResponse) bool { return true }
	Register(TypeSchema{Type: "registry_test_disc_a", Category: "test", Name: "Disc A", Discovery: &DiscoveryHint{
		Probes:      []DiscoveryProbe{{Port: 9101, Scheme: "http", Path: "/", Match: match}, {Port: 9100, Scheme: "https", Path: "/", Match: match}},
		URLTemplate: "{scheme}://{host}:{port}/api",
	}}, func(_ map[string]any) (Connector, error) { return nil, nil })
	Register(TypeSchema{Type: "registry_test_disc_none", Category: "test", Name: "No hint"},
		func(_ map[string]any) (Connector, error) { return nil, nil })

	var found *TypeDiscovery
	for _, d := range DiscoveryHints() {
		d := d
		if d.Type == "registry_test_disc_none" {
			t.Fatalf("type without a hint listed: %+v", d)
		}
		if d.Type == "registry_test_disc_a" {
			found = &d
		}
	}
	if found == nil {
		t.Fatal("type with a hint missing from DiscoveryHints")
	}
	if found.Name != "Disc A" || len(found.Probes) != 2 {
		t.Errorf("hint = %+v", found)
	}

	ports := DiscoveryPorts()
	var have []int
	for _, p := range ports {
		if p == 9100 || p == 9101 {
			have = append(have, p)
		}
	}
	if len(have) != 2 || have[0] != 9100 || have[1] != 9101 {
		t.Errorf("DiscoveryPorts() = %v, want 9100 and 9101 present and ascending", ports)
	}
	for i := 1; i < len(ports); i++ {
		if ports[i] <= ports[i-1] {
			t.Fatalf("DiscoveryPorts() not strictly ascending: %v", ports)
		}
	}
}

func TestDiscoveryHintURLTemplate(t *testing.T) {
	h := DiscoveryHint{URLTemplate: "{scheme}://{host}:{port}/api2/json"}
	if got, want := h.URL("https", "10.0.0.5", 8006), "https://10.0.0.5:8006/api2/json"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
	if got, want := (DiscoveryHint{URLTemplate: "http://{host}:{port}"}).URL("https", "10.0.0.5", 80), "http://10.0.0.5:80"; got != want {
		t.Errorf("fixed-scheme template = %q, want %q", got, want)
	}
}

func TestListSchemasStillOmitsDiscovery(t *testing.T) {
	// Discovery hints are server-side only: the schema JSON must not carry them.
	b, err := json.Marshal(TypeSchema{Type: "x", Discovery: &DiscoveryHint{URLTemplate: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "iscovery") || strings.Contains(string(b), "URLTemplate") {
		t.Errorf("schema JSON leaks discovery: %s", b)
	}
}

func TestDiscoveryResponseHelpers(t *testing.T) {
	obj := DiscoveryResponse{Body: []byte(`{"a":1}`)}.JSONObject()
	if obj["a"] != float64(1) {
		t.Errorf("JSONObject = %v", obj)
	}
	for _, body := range []string{`[1]`, `nope`, ``, `null`} {
		if got := (DiscoveryResponse{Body: []byte(body)}).JSONObject(); got != nil {
			t.Errorf("JSONObject(%q) = %v, want nil", body, got)
		}
	}
	html := DiscoveryResponse{Body: []byte("<html><head><TITLE>\n pve1 - Proxmox </TITLE></head>")}
	if got := html.Title(); got != "pve1 - Proxmox" {
		t.Errorf("Title = %q", got)
	}
	if got := (DiscoveryResponse{Body: []byte("<html></html>")}).Title(); got != "" {
		t.Errorf("Title without element = %q", got)
	}
}

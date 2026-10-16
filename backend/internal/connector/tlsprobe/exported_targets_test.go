package tlsprobe_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/tlsprobe"
)

func TestParseTargetsExportsManualTargetsAndErrors(t *testing.T) {
	targets, errs := tlsprobe.ParseTargets(strings.Join([]string{
		"NAS.lab:443",
		"10.0.0.5:8443",
		"nas.LAB:443",
		"[::ffff:10.0.0.5]:8443",
		"[FD00:0::10]:443",
		"missing-port",
	}, "\n"))
	if want := []tlsprobe.Target{
		{Host: "nas.lab", Port: 443},
		{Host: "10.0.0.5", Port: 8443},
		{Host: "fd00::10", Port: 443},
	}; !reflect.DeepEqual(targets, want) {
		t.Fatalf("targets = %+v, want %+v", targets, want)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), `line 6 ("missing-port")`) {
		t.Fatalf("errors = %v, want one error locating line 6", errs)
	}
}

func TestConfiguredTargetsExpandsRelatedTraefikSnapshot(t *testing.T) {
	snapshot := &connector.ServiceSnapshot{
		Type: "traefik",
		Entities: []connector.SnapshotEntity{{
			Kind: "router",
			Attributes: map[string]any{
				"tls":  true,
				"rule": "Host(`10.20.30.40`)",
			},
		}},
	}
	config := map[string]any{
		"targets":             "manual.lab:443",
		"import_connector_id": "traefik-1",
		"import_port":         8443,
		"_related_snapshots":  map[string]*connector.ServiceSnapshot{"traefik-1": snapshot},
	}

	targets, errs := tlsprobe.ConfiguredTargets(config)
	if len(errs) != 0 {
		t.Fatalf("ConfiguredTargets errors = %v", errs)
	}
	want := []tlsprobe.Target{
		{Host: "manual.lab", Port: 443},
		{Host: "10.20.30.40", Port: 8443, Imported: true},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("targets = %+v, want %+v", targets, want)
	}
}

func TestConfiguredTargetsCapsManualAndImportedTargetsTogether(t *testing.T) {
	entities := make([]connector.SnapshotEntity, 110)
	for i := range entities {
		entities[i] = connector.SnapshotEntity{
			Kind: "router",
			Attributes: map[string]any{
				"tls":  true,
				"rule": fmt.Sprintf("Host(`h%03d.lab`)", i),
			},
		}
	}
	config := map[string]any{
		"targets":             "manual.lab:443",
		"import_connector_id": "traefik-1",
		"import_port":         9443,
		"_related_snapshots": map[string]*connector.ServiceSnapshot{
			"traefik-1": {Type: "traefik", Entities: entities},
		},
	}

	targets, errs := tlsprobe.ConfiguredTargets(config)
	if len(errs) != 0 {
		t.Fatalf("ConfiguredTargets errors = %v", errs)
	}
	if len(targets) != 100 {
		t.Fatalf("target count = %d, want 100", len(targets))
	}
	if got := targets[0]; got != (tlsprobe.Target{Host: "manual.lab", Port: 443}) {
		t.Fatalf("first target = %+v, want the listed target", got)
	}
	if got := targets[1]; got != (tlsprobe.Target{Host: "h000.lab", Port: 9443, Imported: true}) {
		t.Fatalf("first imported target = %+v", got)
	}
	if got := targets[len(targets)-1]; got != (tlsprobe.Target{Host: "h098.lab", Port: 9443, Imported: true}) {
		t.Fatalf("last target = %+v, want last imported target that fits", got)
	}
	for i, target := range targets[1:] {
		if target.Port != 9443 || !target.Imported || target.Host != fmt.Sprintf("h%03d.lab", i) {
			t.Fatalf("imported target %d = %+v", i, target)
		}
	}
}

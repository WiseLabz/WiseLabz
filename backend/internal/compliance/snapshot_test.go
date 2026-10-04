package compliance

import (
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestSnapshotFromConnectorCarriesTypedFields(t *testing.T) {
	got := SnapshotFromConnector(connector.ServiceSnapshot{Entities: []connector.SnapshotEntity{{
		Kind: "vm", Name: "web", ExternalID: "104", IP: "10.0.0.4", Hostname: "web.lan",
		MAC: "aa:bb:cc:dd:ee:ff", Attributes: map[string]any{"template": false},
	}}})
	if len(got.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(got.Entities))
	}
	e := got.Entities[0]
	if e.Kind != "vm" || e.Name != "web" || e.ExternalID != "104" || e.IP != "10.0.0.4" ||
		e.Hostname != "web.lan" || e.MAC != "aa:bb:cc:dd:ee:ff" || e.Attributes["template"] != false {
		t.Fatalf("entity = %#v, lost a field", e)
	}
}

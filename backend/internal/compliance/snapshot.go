package compliance

import "github.com/WiseLabz/wiselabz/internal/connector"

// SnapshotFromConnector converts a stored connector snapshot into the engine's
// view, carrying the typed fields a related clause can join on.
func SnapshotFromConnector(sn connector.ServiceSnapshot) Snapshot {
	entities := make([]Entity, len(sn.Entities))
	for i, e := range sn.Entities {
		entities[i] = Entity{
			Kind: e.Kind, Name: e.Name, ExternalID: e.ExternalID, IP: e.IP,
			Hostname: e.Hostname, MAC: e.MAC, Attributes: e.Attributes,
		}
	}
	return Snapshot{Entities: entities}
}

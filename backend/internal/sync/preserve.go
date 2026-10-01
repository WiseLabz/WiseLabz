package sync

import "github.com/WiseLabz/wiselabz/internal/connector"

func preserveFailedSections(old, sn *connector.ServiceSnapshot) {
	previous := make(map[string]connector.SnapshotSection, len(old.Sections))
	for _, section := range old.Sections {
		previous[section.Title] = section
	}
	for i := range sn.Sections {
		section := &sn.Sections[i]
		if section.Error != "" {
			section.Content = previous[section.Title].Content
		}
	}
	// Entities and dependencies have no section ownership. Retain the complete
	// inventory until a healthy full fetch can authoritatively report deletions.
	sn.Entities = old.Entities
	sn.Dependencies = old.Dependencies
	sn.Metadata = old.Metadata
}

package compliance

import (
	"embed"
	"fmt"
	"sort"

	"go.yaml.in/yaml/v3"
)

//go:embed packs/*.yaml
var packFS embed.FS

// Pack is a named, built-in set of compliance rules a user can install.
type Pack struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Rules       []Rule `yaml:"rules"`
}

// LoadPacks parses every embedded pack, ordered by ID. The embedded files are
// fixed at build time, so a parse error is a programming error.
func LoadPacks() ([]Pack, error) {
	entries, err := packFS.ReadDir("packs")
	if err != nil {
		return nil, err
	}
	packs := make([]Pack, 0, len(entries))
	for _, entry := range entries {
		data, err := packFS.ReadFile("packs/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var pack Pack
		if err := yaml.Unmarshal(data, &pack); err != nil {
			return nil, fmt.Errorf("parse pack %s: %w", entry.Name(), err)
		}
		packs = append(packs, pack)
	}
	sort.Slice(packs, func(i, j int) bool { return packs[i].ID < packs[j].ID })
	return packs, nil
}

// FindPack returns the embedded pack with the given ID.
func FindPack(id string) (Pack, bool, error) {
	packs, err := LoadPacks()
	if err != nil {
		return Pack{}, false, err
	}
	for _, p := range packs {
		if p.ID == id {
			return p, true, nil
		}
	}
	return Pack{}, false, nil
}

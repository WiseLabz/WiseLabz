package sync

import (
	"fmt"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// DiffResult describes a detected change between two snapshots.
type DiffResult struct {
	Type     string      `json:"type"`     // "added", "removed", "modified"
	Severity string      `json:"severity"` // "info", "warning", "critical"
	Summary  string      `json:"summary"`
	Detail   string      `json:"detail"`
	Patches  []DiffPatch `json:"patches"`
}

// DiffPatch is a single text diff hunk.
type DiffPatch struct {
	Section string `json:"section"`
	Old     string `json:"old,omitempty"`
	New     string `json:"new,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// Compare compares two snapshots and returns detected differences.
func Compare(prev, curr *connector.ServiceSnapshot) []DiffResult {
	var results []DiffResult

	// Build section maps
	prevSections := make(map[string]connector.SnapshotSection)
	for _, s := range prev.Sections {
		prevSections[s.Title] = s
	}
	currSections := make(map[string]connector.SnapshotSection)
	for _, s := range curr.Sections {
		currSections[s.Title] = s
	}

	// Check for added sections
	for title, cs := range currSections {
		if _, ok := prevSections[title]; !ok {
			results = append(results, DiffResult{
				Type:     "added",
				Severity: "info",
				Summary:  fmt.Sprintf("New section added: %s", title),
				Detail:   fmt.Sprintf("Section %q was added with %d lines of content.", title, lineCount(cs.Content)),
				Patches: []DiffPatch{{
					Section: title,
					New:     cs.Content,
				}},
			})
		}
	}

	// Check for removed sections
	for title, ps := range prevSections {
		if _, ok := currSections[title]; !ok {
			results = append(results, DiffResult{
				Type:     "removed",
				Severity: "warning",
				Summary:  fmt.Sprintf("Section removed: %s", title),
				Detail:   fmt.Sprintf("Section %q was removed.", title),
				Patches: []DiffPatch{{
					Section: title,
					Old:     ps.Content,
				}},
			})
		}
	}

	// Check for modified sections
	for title, cs := range currSections {
		ps, ok := prevSections[title]
		if !ok {
			continue // already handled as "added"
		}
		if cs.Content != ps.Content {
			results = append(results, DiffResult{
				Type:     "modified",
				Severity: severityForChange(ps.Content, cs.Content),
				Summary:  fmt.Sprintf("Section modified: %s", title),
				Detail:   fmt.Sprintf("Section %q changed (%d → %d lines).", title, lineCount(ps.Content), lineCount(cs.Content)),
				Patches: []DiffPatch{{
					Section: title,
					Old:     ps.Content,
					New:     cs.Content,
				}},
			})
		}
	}

	return results
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func severityForChange(prev, curr string) string {
	// Simple heuristic: large diffs are more severe
	oldLines := lineCount(prev)
	newLines := lineCount(curr)
	diff := oldLines - newLines
	if diff < 0 {
		diff = -diff
	}
	if diff > 10 {
		return "critical"
	}
	if diff > 3 {
		return "warning"
	}
	return "info"
}

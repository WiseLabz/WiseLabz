// Package snapshotutil holds the markdown-table and attribute helpers shared
// by the connectors that render a ServiceSnapshot.
package snapshotutil

import (
	"net"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

// MDCell makes s safe for a markdown table cell: an empty value becomes an
// em dash, newlines become spaces and pipes are escaped.
func MDCell(s string) string {
	if s == "" {
		return "—"
	}
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.ReplaceAll(s, "|", `\|`)
}

// YesNo renders b as "yes" or "no".
func YesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// Empty is the placeholder for a table whose upstream list came back empty.
func Empty(noun string) string { return "_No " + noun + " returned_" }

// UnavailableSection is the section recorded when a fetch for title failed.
func UnavailableSection(title string, err error) connector.SnapshotSection {
	return connector.ErrorSection(title, err)
}

// MalformedSection is the placeholder content for a table whose upstream
// payload could not be decoded.
func MalformedSection(title string, err error) string {
	return "_" + title + " unavailable: " + connector.NewMalformedResponseError(err).Error() + "_"
}

// PutString sets attrs[key] = value unless value is empty.
func PutString(attrs map[string]any, key, value string) {
	if value != "" {
		attrs[key] = value
	}
}

// PutStrings sets attrs[key] = values unless values is empty.
func PutStrings(attrs map[string]any, key string, values []string) {
	if len(values) > 0 {
		attrs[key] = values
	}
}

// NormalizeMAC returns a lowercase, colon-separated MAC, or empty for invalid input.
func NormalizeMAC(value string) string {
	mac, err := net.ParseMAC(strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return mac.String()
}

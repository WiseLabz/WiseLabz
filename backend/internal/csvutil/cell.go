// Package csvutil provides safe spreadsheet export cells.
package csvutil

import (
	"strings"
	"unicode"
)

// SafeCell keeps spreadsheet applications from evaluating data as a formula.
func SafeCell(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

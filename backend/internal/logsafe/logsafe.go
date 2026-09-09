// Package logsafe sanitizes user-controlled strings before they reach a logger.
package logsafe

import "strings"

// Sanitize strips CR/LF so a value can't forge or split log lines.
func Sanitize(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

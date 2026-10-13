// Package logsafe sanitizes user-controlled strings before they reach a logger.
package logsafe

import (
	"strings"
	"unicode"
)

// Sanitize strips CR and LF so a value can't forge or split log lines, then
// drops every other control character (ESC, NUL, tab, ...) and the Unicode line
// and paragraph separators, which some log viewers also render as line breaks.
//
// The two ReplaceAll calls are deliberate: they are the form CodeQL's
// go/log-injection query recognizes as a sanitizer.
func Sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return -1
		}
		return r
	}, s)
}

// Err returns err's message sanitized for logging. Error messages often wrap
// user input (IDs, names, upstream responses), so log them through this. A
// nil error yields "".
func Err(err error) string {
	if err == nil {
		return ""
	}
	return Sanitize(err.Error())
}

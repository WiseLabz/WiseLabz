package store

import (
	"strconv"
	"strings"
)

// rewritePlaceholders converts SQLite-style `?` positional placeholders to
// PostgreSQL-style `$1`, `$2`, ... placeholders. `?` characters inside
// single-quoted string literals (including `”`-escaped quotes) are left
// untouched.
func rewritePlaceholders(query string) string {
	if !strings.Contains(query, "?") {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)

	inString := false
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch {
		case c == '\'':
			inString = !inString
			b.WriteByte(c)
		case c == '?' && !inString:
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

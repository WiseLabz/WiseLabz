package ai

import (
	"strings"
	"unicode/utf8"
)

// TruncateUTF8 caps s at limit bytes without splitting a rune.
func TruncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "\n[truncated]"
}

// StripPromptTags removes the open and close delimiter tags named in tags so
// untrusted content can't close its own block.
func StripPromptTags(s string, tags ...string) string {
	pairs := make([]string, 0, len(tags)*4)
	for _, tag := range tags {
		pairs = append(pairs, "<"+tag+">", "", "</"+tag+">", "")
	}
	return strings.NewReplacer(pairs...).Replace(s)
}

package ai

import (
	"regexp"
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
// untrusted content can't close its own block. Matching ignores case and
// whitespace inside the angle brackets, and repeats until stable because
// removing a tag can splice its neighbours into a new one.
func StripPromptTags(s string, tags ...string) string {
	if len(tags) == 0 {
		return s
	}
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = regexp.QuoteMeta(tag)
	}
	re := regexp.MustCompile(`(?i)<\s*/?\s*(?:` + strings.Join(names, "|") + `)(?:\s[^<>]*)?>`)
	for {
		out := re.ReplaceAllString(s, "")
		if out == s {
			return s
		}
		s = out
	}
}

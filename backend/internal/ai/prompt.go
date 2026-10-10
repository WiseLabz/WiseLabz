package ai

import (
	"regexp"
	"strings"
	"sync"
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

// maxStripPasses bounds the strip loop: each pass removes at most one level of
// tags spliced together by the previous one.
const maxStripPasses = 8

// tagSpace matches any whitespace-like rune a model may read as a gap inside a
// tag: RE2's \s is ASCII only, so add Unicode separators, NEL, VT and format
// characters such as zero-width spaces.
const tagSpace = `[\s\p{Z}\p{Cf}\x{85}\x{0B}]`

var tagPatterns sync.Map // joined tag names -> *regexp.Regexp

func tagPattern(tags []string) *regexp.Regexp {
	key := strings.Join(tags, "\x00")
	if re, ok := tagPatterns.Load(key); ok {
		return re.(*regexp.Regexp)
	}
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = regexp.QuoteMeta(tag)
	}
	re := regexp.MustCompile(`(?i)<` + tagSpace + `*/?` + tagSpace + `*(?:` + strings.Join(names, "|") + `)` +
		`(?:` + tagSpace + `[^<>]*)?/?` + tagSpace + `*>`)
	tagPatterns.Store(key, re)
	return re
}

// StripPromptTags removes the open, close and self-closing delimiter tags named
// in tags so untrusted content can't close its own block. Matching ignores case
// and whitespace inside the angle brackets, and repeats because removing a tag
// can splice its neighbours into a new one. The passes are capped: text that is
// still changing after the cap loses every angle bracket instead, so the cost
// stays linear in the input.
func StripPromptTags(s string, tags ...string) string {
	if len(tags) == 0 {
		return s
	}
	re := tagPattern(tags)
	for range maxStripPasses {
		out := re.ReplaceAllString(s, "")
		if out == s {
			return s
		}
		s = out
	}
	if re.MatchString(s) {
		s = strings.NewReplacer("<", "", ">", "").Replace(s)
	}
	return s
}

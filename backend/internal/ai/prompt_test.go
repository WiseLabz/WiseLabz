package ai

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateUTF8(t *testing.T) {
	if got := TruncateUTF8("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	got := TruncateUTF8("ééééé", 5) // 2-byte runes: cut lands mid-rune
	if !utf8.ValidString(got) || got != "éé\n[truncated]" {
		t.Fatalf("got %q", got)
	}
}

func TestStripPromptTags(t *testing.T) {
	in := "a <x>b</x> <y>c</y> <z>d"
	if got := StripPromptTags(in, "x", "y"); got != "a b c <z>d" {
		t.Fatalf("got %q", got)
	}
	if got := StripPromptTags(in); !strings.Contains(got, "<x>") {
		t.Fatalf("no tags must leave input untouched: %q", got)
	}
}

package alerts

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestTruncateDiffKeepsRunesIntact(t *testing.T) {
	// 3-byte runes: a byte cut at 4 would land mid-rune.
	got := truncateDiff(strings.Repeat("世", 10), 4)
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8 after truncation: %q", got)
	}
	if !strings.HasPrefix(got, "世\n... (truncated)") {
		t.Fatalf("got %q", got)
	}
	if got := truncateDiff("short", 100); got != "short" {
		t.Fatalf("short diff changed: %q", got)
	}
}

func TestCodeFence(t *testing.T) {
	cases := map[string]string{
		"plain":          "```",
		"a ``` b":        "````",
		"`` and ````` x": "``````",
		"one ` tick":     "```",
		"``\n```\n`":     "````",
		"":               "```",
	}
	for in, want := range cases {
		if got := codeFence(in); got != want {
			t.Errorf("codeFence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildRunbookDraftFenceAndTruncation(t *testing.T) {
	a := &store.AlertRecord{Title: "T", Severity: "warning"}
	diff := "before ``` injected\n## Fake heading\n```\n" + strings.Repeat("é", maxDraftDiffChars)
	c := &store.ChangeRecord{Summary: "s", ChangeType: "config_change", Severity: "warning", Diff: diff}
	d := buildRunbookDraft(a, "svc", c, nil, "change_type", "config_change")

	if !utf8.ValidString(d.Body) {
		t.Fatal("draft body is not valid UTF-8")
	}
	if !strings.Contains(d.Body, "\n````\nbefore ``` injected") || !strings.Contains(d.Body, "(truncated)\n````\n") {
		t.Fatalf("diff not wrapped in a 4-backtick fence:\n%s", d.Body)
	}
}

package logsafe

import (
	"errors"
	"testing"
)

func TestSanitize(t *testing.T) {
	got := Sanitize("evil\r\nINJECTED: fake line")
	want := "evilINJECTED: fake line"
	if got != want {
		t.Fatalf("Sanitize() = %q, want %q", got, want)
	}
}

func TestSanitizeStripsControlCharacters(t *testing.T) {
	cases := []struct{ in, want string }{
		{"plain text, unicode é世", "plain text, unicode é世"},
		{"a\nb\rc\r\nd", "abcd"},
		{"esc\x1b[31mred\x1b[0m", "esc[31mred[0m"},
		{"nul\x00byte\ttab\x7fdel\u0085nel", "nulbytetabdelnel"},
		{"sep\u2028line\u2029para", "seplinepara"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Sanitize(c.in); got != c.want {
			t.Errorf("Sanitize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestErr(t *testing.T) {
	if got := Err(nil); got != "" {
		t.Fatalf("Err(nil) = %q, want empty", got)
	}
	got := Err(errors.New("get job health \"x\r\nINJECTED\": boom"))
	want := "get job health \"xINJECTED\": boom"
	if got != want {
		t.Fatalf("Err() = %q, want %q", got, want)
	}
}

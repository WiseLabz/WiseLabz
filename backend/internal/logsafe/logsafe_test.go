package logsafe

import "testing"

func TestSanitize(t *testing.T) {
	got := Sanitize("evil\r\nINJECTED: fake line")
	want := "evilINJECTED: fake line"
	if got != want {
		t.Fatalf("Sanitize() = %q, want %q", got, want)
	}
}

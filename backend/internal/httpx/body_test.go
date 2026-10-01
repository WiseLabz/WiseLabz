package httpx

import (
	"strings"
	"testing"
)

func TestReadBodyRejectsOversize(t *testing.T) {
	if _, err := ReadBody(strings.NewReader(strings.Repeat("a", MaxResponseBytes+1))); err == nil {
		t.Fatal("expected error for body over MaxResponseBytes")
	}
	got, err := ReadBody(strings.NewReader("ok"))
	if err != nil || string(got) != "ok" {
		t.Fatalf("ReadBody = %q, %v", got, err)
	}
}

func TestErrorBodyTrimsAndCaps(t *testing.T) {
	if got := ErrorBody(strings.NewReader("  boom \n")); got != "boom" {
		t.Fatalf("ErrorBody = %q, want boom", got)
	}
	if got := ErrorBody(strings.NewReader(strings.Repeat("a", MaxResponseBytes+10))); len(got) != MaxResponseBytes {
		t.Fatalf("ErrorBody len = %d, want %d", len(got), MaxResponseBytes)
	}
}

package snapshotutil

import (
	"errors"
	"testing"
)

func TestMDCell(t *testing.T) {
	for in, want := range map[string]string{"": "—", "a|b": `a\|b`, "a\nb": "a b", "x": "x"} {
		if got := MDCell(in); got != want {
			t.Errorf("MDCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestYesNoEmpty(t *testing.T) {
	if YesNo(true) != "yes" || YesNo(false) != "no" {
		t.Error("YesNo")
	}
	if Empty("hosts") != "_No hosts returned_" {
		t.Error("Empty")
	}
}

func TestPutStringAndStrings(t *testing.T) {
	attrs := map[string]any{}
	PutString(attrs, "a", "")
	PutStrings(attrs, "b", nil)
	if len(attrs) != 0 {
		t.Fatalf("empty values stored: %v", attrs)
	}
	PutString(attrs, "a", "x")
	PutStrings(attrs, "b", []string{"y"})
	if len(attrs) != 2 {
		t.Fatalf("attrs = %v", attrs)
	}
}

func TestSections(t *testing.T) {
	err := errors.New("boom")
	if got := MalformedSection("Hosts", err); got == "" || got[0] != '_' {
		t.Errorf("MalformedSection = %q", got)
	}
	if got := UnavailableSection("Hosts", err); got.Title != "Hosts" {
		t.Errorf("UnavailableSection = %+v", got)
	}
}

func TestNormalizeMAC(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"aa:bb:cc:dd:ee:ff", "aa:bb:cc:dd:ee:ff"},
		{"AA:BB:CC:DD:EE:FF", "aa:bb:cc:dd:ee:ff"},
		{"AA-BB-CC-DD-EE-FF", "aa:bb:cc:dd:ee:ff"},
		{"aabb.ccdd.eeff", "aa:bb:cc:dd:ee:ff"},
		{"  AA-BB-CC-DD-EE-FF  ", "aa:bb:cc:dd:ee:ff"},
		{"invalid", ""}, {"", ""},
	} {
		t.Run(tc.input, func(t *testing.T) {
			if got := NormalizeMAC(tc.input); got != tc.want {
				t.Fatalf("NormalizeMAC(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

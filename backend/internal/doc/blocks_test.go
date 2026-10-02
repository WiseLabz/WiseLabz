package doc

import (
	"strings"
	"testing"
)

func mark(key, body string) string {
	return RenderSegments([]Segment{{Block: &Block{Key: key, Hash: HashBody(body), Body: body}}})
}

func TestParseBlocksRoundTripAndShape(t *testing.T) {
	ok := mark("head", "# Svc\n\n**Type:** docker")
	tests := []struct {
		name   string
		in     string
		blocks []string // keys of parsed blocks, in order
	}{
		{"empty", "", nil},
		{"plain text", "# Notes\n\nhello\n", nil},
		{"one block", ok + "\n", []string{"head"}},
		{"text around blocks", "intro\n\n" + ok + "\n\nmid\n\n" + mark("snap.a", "A") + "\n\nouter", []string{"head", "snap.a"}},
		{"empty body", "<!-- wl:gen key=\"k\" h=\"" + HashBody("") + "\" -->\n\n<!-- /wl:gen -->\n", []string{"k"}},
		{"no body line is text", "<!-- wl:gen key=\"k\" h=\"" + HashBody("") + "\" -->\n<!-- /wl:gen -->\n", nil},
		{"unclosed is text", "<!-- wl:gen key=\"k\" h=\"abcdefabcdef\" -->\nbody\n", nil},
		{"nested open is text", "<!-- wl:gen key=\"a\" h=\"abcdefabcdef\" -->\nx\n" + ok, []string{"head"}},
		{"crlf markers are text", "<!-- wl:gen key=\"k\" h=\"abcdefabcdef\" -->\r\nx\r\n<!-- /wl:gen -->\r\n", nil},
		{"bad hash is text", "<!-- wl:gen key=\"k\" h=\"XYZ\" -->\nx\n<!-- /wl:gen -->", nil},
		{"marker at EOF without newline", "<!-- wl:gen key=\"k\" h=\"abcdefabcdef\" -->", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segs := ParseBlocks(tt.in)
			if got := RenderSegments(segs); got != tt.in {
				t.Fatalf("round trip mismatch:\n got %q\nwant %q", got, tt.in)
			}
			var keys []string
			for _, s := range segs {
				if s.Block != nil {
					keys = append(keys, s.Block.Key)
				}
			}
			if strings.Join(keys, ",") != strings.Join(tt.blocks, ",") {
				t.Fatalf("block keys = %v, want %v", keys, tt.blocks)
			}
		})
	}
}

func TestBlockEditedDetection(t *testing.T) {
	b := NewBlock("k", "generated")
	if b.Edited() {
		t.Fatal("fresh block reported as edited")
	}
	segs := ParseBlocks(strings.Replace(RenderSegments([]Segment{{Block: &b}}), "generated", "human tweak", 1))
	if len(segs) != 1 || !segs[0].Block.Edited() {
		t.Fatalf("edited block not detected: %+v", segs)
	}
}

func TestNewBlockNeutralisesMarkers(t *testing.T) {
	b := NewBlock("k", "a\n<!-- /wl:gen -->\n<!-- wl:gen key=\"x\" h=\"abcdefabcdef\" -->\nb")
	segs := ParseBlocks(RenderSegments([]Segment{{Block: &b}}) + "\n")
	if len(segs) != 2 || segs[0].Block == nil || segs[0].Block.Body != b.Body {
		t.Fatalf("generated body broke block structure: %+v", segs)
	}
}

func TestSlugKeys(t *testing.T) {
	got := slugKeys("snap", []string{"Containers", "Node: pve-1", "containers", "", "!!"})
	want := []string{"snap.containers", "snap.node-pve-1", "snap.containers-2", "snap.section", "snap.section-2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("slugKeys = %v, want %v", got, want)
	}
}

func FuzzParseBlocksRoundTrip(f *testing.F) {
	f.Add("")
	f.Add(mark("head", "x") + "\ntext\n" + mark("a", ""))
	f.Add("<!-- wl:gen key=\"k\" h=\"abcdefabcdef\" -->\n<!-- /wl:gen -->")
	f.Fuzz(func(t *testing.T, s string) {
		if got := RenderSegments(ParseBlocks(s)); got != s {
			t.Fatalf("round trip mismatch for %q: got %q", s, got)
		}
	})
}

func TestStripMarkersMatchesPlainRender(t *testing.T) {
	r := &renderResult{Blocks: []Block{NewBlock("head", "# X"), NewBlock("snap.a", "body")}}
	if got := StripMarkers(r.content()); got != r.plain() {
		t.Fatalf("StripMarkers(fresh) = %q, want %q", got, r.plain())
	}
}

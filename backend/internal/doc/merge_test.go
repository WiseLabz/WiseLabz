package doc

import (
	"strings"
	"testing"
)

// edit returns a block with key whose body was changed after generation.
func edit(key, generated, human string) Segment {
	return Segment{Block: &Block{Key: key, Hash: HashBody(generated), Body: human}}
}

func gen(key, body string) Segment {
	b := NewBlock(key, body)
	return Segment{Block: &b}
}

func txt(s string) Segment { return Segment{Text: s} }

func TestMerge(t *testing.T) {
	tests := []struct {
		name      string
		existing  []Segment
		prevKeys  []string
		fresh     []Block
		want      string
		conflicts []string
	}{
		{
			name:     "unedited block is refreshed",
			existing: []Segment{gen("a", "old"), txt("\n")},
			prevKeys: []string{"a"},
			fresh:    []Block{NewBlock("a", "new")},
			want:     RenderSegments([]Segment{gen("a", "new"), txt("\n")}),
		},
		{
			name:     "edited block with unchanged upstream is kept, no conflict",
			existing: []Segment{edit("a", "gen", "mine")},
			prevKeys: []string{"a"},
			fresh:    []Block{NewBlock("a", "gen")},
			want:     RenderSegments([]Segment{edit("a", "gen", "mine")}),
		},
		{
			name:      "edited block with changed upstream is kept and conflicts",
			existing:  []Segment{edit("a", "gen", "mine")},
			prevKeys:  []string{"a"},
			fresh:     []Block{NewBlock("a", "gen2")},
			want:      RenderSegments([]Segment{edit("a", "gen", "mine")}),
			conflicts: []string{"a"},
		},
		{
			name:     "human text between blocks survives",
			existing: []Segment{gen("a", "1"), txt("\n\nmy notes\n\n"), gen("b", "2")},
			prevKeys: []string{"a", "b"},
			fresh:    []Block{NewBlock("a", "1x"), NewBlock("b", "2x")},
			want:     RenderSegments([]Segment{gen("a", "1x"), txt("\n\nmy notes\n\n"), gen("b", "2x")}),
		},
		{
			name:     "removed upstream, unedited block is dropped",
			existing: []Segment{gen("a", "1"), txt("\n\n"), gen("b", "2")},
			prevKeys: []string{"a", "b"},
			fresh:    []Block{NewBlock("a", "1")},
			want:     RenderSegments([]Segment{gen("a", "1"), txt("\n\n")}),
		},
		{
			name:     "removed upstream, edited block is detached",
			existing: []Segment{gen("a", "1"), txt("\n\n"), edit("b", "2", "mine")},
			prevKeys: []string{"a", "b"},
			fresh:    []Block{NewBlock("a", "1")},
			want:     RenderSegments([]Segment{gen("a", "1"), txt("\n\nmine")}),
		},
		{
			name:     "block the user deleted is not re-added",
			existing: []Segment{gen("a", "1"), txt("\n")},
			prevKeys: []string{"a", "b"},
			fresh:    []Block{NewBlock("a", "1"), NewBlock("b", "2")},
			want:     RenderSegments([]Segment{gen("a", "1"), txt("\n")}),
		},
		{
			name:     "new upstream key goes after its predecessor",
			existing: []Segment{gen("a", "1"), txt("\n\nnotes\n\n"), gen("c", "3"), txt("\n")},
			prevKeys: []string{"a", "c"},
			fresh:    []Block{NewBlock("a", "1"), NewBlock("b", "2"), NewBlock("c", "3")},
			want:     RenderSegments([]Segment{gen("a", "1"), txt("\n\n"), gen("b", "2"), txt("\n\nnotes\n\n"), gen("c", "3"), txt("\n")}),
		},
		{
			name:     "new key with no predecessor is appended",
			existing: []Segment{txt("only human text\n")},
			prevKeys: []string{"x"},
			fresh:    []Block{NewBlock("a", "1")},
			want:     RenderSegments([]Segment{txt("only human text\n"), txt("\n"), gen("a", "1"), txt("\n")}),
		},
		{
			name:     "empty doc gets the fresh render",
			existing: nil,
			prevKeys: nil,
			fresh:    []Block{NewBlock("a", "1"), NewBlock("b", "2")},
			want:     renderFresh([]Block{NewBlock("a", "1"), NewBlock("b", "2")}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, conflicts := Merge(tt.existing, tt.prevKeys, tt.fresh)
			if got := RenderSegments(out); got != tt.want {
				t.Fatalf("merge result:\n got %q\nwant %q", got, tt.want)
			}
			var keys []string
			for _, c := range conflicts {
				keys = append(keys, c.Key)
			}
			if strings.Join(keys, ",") != strings.Join(tt.conflicts, ",") {
				t.Fatalf("conflicts = %v, want %v", keys, tt.conflicts)
			}
		})
	}
}

func TestMergeConflictCarriesBothSides(t *testing.T) {
	_, conflicts := Merge([]Segment{edit("a", "gen", "mine")}, []string{"a"}, []Block{NewBlock("a", "gen2")})
	if len(conflicts) != 1 || conflicts[0].Human != "mine" || conflicts[0].Generated.Body != "gen2" {
		t.Fatalf("conflict = %+v", conflicts)
	}
}

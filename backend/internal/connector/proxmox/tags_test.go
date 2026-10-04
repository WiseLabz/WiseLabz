package proxmox

import (
	"slices"
	"testing"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "empty string returns non-nil empty slice",
			raw:  "",
			want: []string{},
		},
		{
			name: "single tag",
			raw:  "prod",
			want: []string{"prod"},
		},
		{
			name: "semicolon-delimited tags",
			raw:  "prod;web",
			want: []string{"prod", "web"},
		},
		{
			name: "semicolon-delimited tags unsorted input",
			raw:  "web;prod",
			want: []string{"prod", "web"},
		},
		{
			name: "comma-delimited tags",
			raw:  "a,b",
			want: []string{"a", "b"},
		},
		{
			name: "space-delimited tags",
			raw:  "a b c",
			want: []string{"a", "b", "c"},
		},
		{
			name: "mixed separators",
			raw:  "a,b c;d",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "duplicates removed",
			raw:  "x;x",
			want: []string{"x"},
		},
		{
			name: "duplicates with mixed case",
			raw:  "prod;prod",
			want: []string{"prod"},
		},
		{
			name: "leading/trailing whitespace",
			raw:  "  prod;web  ",
			want: []string{"prod", "web"},
		},
		{
			name: "mixed whitespace",
			raw:  "a\t,\tb",
			want: []string{"a", "b"},
		},
		{
			name: "case preservation",
			raw:  "ProdServer;WebApp",
			want: []string{"ProdServer", "WebApp"},
		},
		{
			name: "only whitespace returns empty slice",
			raw:  "   ",
			want: []string{},
		},
		{
			name: "empty entries skipped",
			raw:  "a;;b",
			want: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTags(tt.raw)

			// Ensure it's non-nil (even if empty).
			if got == nil {
				t.Errorf("parseTags(%q) returned nil, want non-nil", tt.raw)
				return
			}

			// Check the values match.
			if !slices.Equal(got, tt.want) {
				t.Errorf("parseTags(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

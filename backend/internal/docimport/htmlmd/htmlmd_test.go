package htmlmd

import (
	"strings"
	"testing"
)

func TestConvert(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
		absent   []string
	}{
		{"headings and paragraphs", "<h1>Title</h1><p>Hello <strong>bold</strong> and <em>it</em>.</p>", []string{"# Title", "Hello **bold** and *it*."}, nil},
		{"links and images", `<p><a href="/en/other">other</a> <img src="/uploads/a.png" alt="pic"></p>`, []string{"[other](/en/other)", "![pic](/uploads/a.png)"}, nil},
		{"lists", "<ul><li>one</li><li>two</li></ul>", []string{"- one", "- two"}, nil},
		{"code", `<pre><code class="language-go">fmt.Println()</code></pre>`, []string{"```go", "fmt.Println()"}, nil},
		{"table", "<table><thead><tr><th>A</th><th>B</th></tr></thead><tbody><tr><td>1</td><td>2</td></tr></tbody></table>", []string{"| A | B |", "| 1 | 2 |"}, nil},
		{"strikethrough", "<p><del>gone</del></p>", []string{"~~gone~~"}, nil},
		{"scripts dropped", `<p>ok</p><script>alert(1)</script><style>p{}</style><iframe src="x"></iframe>`, []string{"ok"}, []string{"alert", "<script", "<iframe", "p{}"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Convert(c.in)
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("missing %q in %q", w, got)
				}
			}
			for _, a := range c.absent {
				if strings.Contains(got, a) {
					t.Errorf("unexpected %q in %q", a, got)
				}
			}
		})
	}
}

func TestConvertEmpty(t *testing.T) {
	got, err := Convert("")
	if err != nil || got != "" {
		t.Fatalf("got %q, %v", got, err)
	}
}

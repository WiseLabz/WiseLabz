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
		{"javascript href dropped", `<a href="javascript:alert(1)">x</a>`, []string{"x"}, []string{"javascript", "alert"}},
		{"obfuscated javascript href dropped", "<a href=\"JaVa\tScRiPt:alert(1)\">x</a>", []string{"x"}, []string{"javascript", "alert", "ScRiPt"}},
		{"newline and space obfuscation dropped", "<a href=\" java\nscript:alert(1)\">x</a>", []string{"x"}, []string{"script:", "alert"}},
		{"data image dropped", `<img src="data:image/svg+xml;base64,PHN2Zz4=" alt="s">`, nil, []string{"data:", "PHN2"}},
		{"svg script dropped", `<svg><script>alert(1)</script><text>hi</text></svg>`, nil, []string{"<svg", "alert", "<script"}},
		{"safe links survive", `<a href="https://example.com">a</a> <a href="mailto:a@b.c">b</a> <a href="/en/page">c</a> <a href="#top">d</a> <a href="tel:+1555">e</a>`, []string{"[a](https://example.com)", "[b](mailto:a@b.c)", "[c](/en/page)", "[d](#top)", "[e](tel:+1555)"}, nil},
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

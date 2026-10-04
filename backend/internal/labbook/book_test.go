package labbook

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func testBook() *Book {
	return &Book{
		Docs: []store.DocRecord{
			{ID: "parent", Title: "Árvore", ServiceID: "c1", Content: "# Parent\n\n[Child](/docs/child)\n\n<!-- wl:gen x -->\nkept\n<!-- /wl:gen -->"},
			{ID: "child", ParentID: "parent", ServiceID: "c1", Title: "Child", Content: "![image](attachment:img)\n\n[manual](attachment:pdf)\n\n```mermaid\ngraph LR\n A --> B\n```\n\n|A|B|\n|-|-|\n|1|2|"},
			{ID: "lab", Title: "Lab Note", Content: "[Parent](/docs/parent)"},
		},
		Connectors: map[string]string{"c1": "Server"},
		Attachments: map[string][]store.DocAttachment{"child": {
			{ID: "img", SHA256: "image", Filename: "photo.png", ContentType: "image/png"},
			{ID: "pdf", SHA256: "pdf", Filename: "manual.pdf", ContentType: "application/pdf"},
		}},
		OpenBlob: func(hash string) (io.ReadCloser, error) {
			if hash == "pdf" {
				return io.NopCloser(strings.NewReader("%PDF-1.7\nmanual")), nil
			}
			return io.NopCloser(bytes.NewReader([]byte{137, 80, 78, 71, 13, 10, 26, 10})), nil
		},
	}
}
func TestHTMLPortableHierarchy(t *testing.T) {
	b := testBook()
	var out bytes.Buffer
	if err := b.Write(&out, "html"); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{"Árvore", "Server", `href="#` + anchor("child") + `"`, "data:image/png;base64,iVBORw0KGgo=", "manual.pdf", "language-mermaid", "mermaid.initialize", "@media print", "<table>"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(html, `globalThis["mermaid"]`) || !strings.Contains(html, "default-src 'none'") {
		t.Error("Mermaid bundle not inlined or CSP missing")
	}
	if strings.Contains(html, "wl:gen") || strings.Contains(html, "attachment:pdf") || strings.Contains(html, "data:application/pdf") {
		t.Fatal("ownership markers or nonimage bytes leaked")
	}
	roots := b.tree()
	if roots[0].Title != "Lab" || roots[1].Children[0].Title != "Árvore" || roots[1].Children[0].Children[0].Title != "Child" {
		t.Fatalf("hierarchy %+v", roots)
	}
}
func TestHTMLUntrustedContent(t *testing.T) {
	b := &Book{Docs: []store.DocRecord{{ID: "1", Title: "<script>evil</script>", Content: "<script>alert('doc')</script>\n\n![remote](https://example.com/photo.png)\n\n![bad](attachment:missing)\n\n[hidden](/docs/hidden)\n\n[unsafe](javascript:alert(1))"}}}
	var out bytes.Buffer
	if err := b.Write(&out, "html"); err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, bad := range []string{"<script>alert('doc')", "src=\"https://example.com", "href=\"javascript:", "href=\"/docs/hidden", "src=\"attachment:"} {
		if strings.Contains(html, bad) {
			t.Errorf("unsafe HTML %q", bad)
		}
	}
}
func TestMarkdownRoundTrip(t *testing.T) {
	b := testBook()
	var out bytes.Buffer
	if err := b.Write(&out, "md.zip"); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	a, err := docimport.OpenArchive(zr, docimport.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := docimport.Analyze(a, []docimport.Connector{{ID: "c1", Name: "Server"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Docs) != 3 || plan.AttachmentCount != 2 || len(plan.Warnings) != 0 {
		t.Fatalf("plan %+v", plan)
	}
	byTitle := map[string]docimport.Doc{}
	for _, d := range plan.Docs {
		byTitle[d.Title] = d
	}
	parent, child := byTitle["Árvore"], byTitle["Child"]
	if child.ParentID != parent.ID || child.ServiceID != "c1" {
		t.Fatalf("hierarchy lost %+v", plan.Docs)
	}
	if !strings.Contains(parent.Content, "/docs/"+child.ID) || !strings.Contains(child.Content, "attachment:") {
		t.Fatalf("links lost %+v", plan.Docs)
	}
}

func TestHierarchySiblingOrderAndUnicodeAnchors(t *testing.T) {
	b := &Book{Docs: []store.DocRecord{
		{ID: "second", Title: "文档", Content: "second"},
		{ID: "parent", Title: "Alpha", Content: "parent"},
		{ID: "z-child", Title: "Zulu", ParentID: "parent"},
		{ID: "a-child", Title: "Alpha child", ParentID: "parent"},
		{ID: "first", Title: "文档", Content: "first"},
	}}
	roots := b.tree()[0].Children
	if roots[0].Title != "Alpha" || roots[0].Children[0].Title != "Alpha child" || roots[0].Children[1].Title != "Zulu" {
		t.Fatal("siblings not title-sorted")
	}
	if anchor("first") == anchor("second") {
		t.Fatal("unicode titles must retain distinct anchors")
	}
	var out bytes.Buffer
	if err := b.Write(&out, "html"); err != nil {
		t.Fatal(err)
	}
	root, err := html.Parse(strings.NewReader(out.String()))
	if err != nil {
		t.Fatal(err)
	}
	var inspect func(*html.Node)
	inspect = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "iframe" || n.Data == "link" {
				t.Fatalf("external resource element %s", n.Data)
			}
			if n.Data == "script" || n.Data == "img" {
				for _, a := range n.Attr {
					if a.Key == "src" && !strings.HasPrefix(a.Val, "data:") {
						t.Fatalf("external resource URL %s", a.Val)
					}
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			inspect(child)
		}
	}
	inspect(root)

	if !strings.Contains(out.String(), "connect-src 'none'") {
		t.Fatal("network-blocking CSP missing")
	}
}

package docimport

import (
	"archive/zip"
	"errors"
	"strings"
	"testing"
)

// Page files follow Wiki.js 2.x server/helpers/page.js injectPageMetadata:
// a metadata block of raw "key: value" lines, then a blank line, then the
// content. Markdown pages wrap it in --- lines, HTML pages in <!-- -->.
func wikiMD(title, extra, body string) []byte {
	return md("---\ntitle: " + title + "\ndescription: \npublished: true\ndate: 2024-05-01T10:00:00.000Z\ntags: homelab, net\neditor: markdown\ndateCreated: 2024-04-01T10:00:00.000Z\n" + extra + "---\n\n" + body)
}

func wikiHTML(title, body string) []byte {
	return md("<!--\ntitle: " + title + "\ndescription: \npublished: true\ndate: 2024-05-01T10:00:00.000Z\ntags: \neditor: ckeditor\ndateCreated: 2024-04-01T10:00:00.000Z\n-->\n\n" + body)
}

func wikiExport(t *testing.T) *zip.Reader {
	return buildZip(t,
		entry{"home.md", wikiMD("Home", "", "# Welcome\nSee [Servers](/servers) and [Proxmox](/servers/proxmox#storage), [FR](/fr/accueil).\n![diagram](/uploads/net/diagram.png =300x) and [manual](/uploads/manual.pdf).\n`[code](/servers)` [ext](https://example.com) [gone](/nowhere) [dl](/uploads/missing.png)\n")},
		entry{"servers.md", wikiMD("Servers", "", "Rack notes: [Home](/en/home)\n")},
		entry{"servers/proxmox.html", wikiHTML("Proxmox: host", "<h2>Setup</h2><p>Back to <a href=\"/servers\">servers</a>.</p><script>alert(1)</script><table><tr><th>A</th></tr><tr><td>1</td></tr></table>")},
		entry{"servers/deep/a/b/c/d.md", wikiMD("Deep", "", "deep\n")},
		entry{"fr/accueil.md", wikiMD("Accueil", "published: false\n", "Bonjour\n")},
		entry{"guide/Intro.MD", md("No metadata, a title heading\n")},
		entry{"guide/Intro.html", wikiHTML("Duplicate", "<p>dup</p>")},
		entry{"ops/runbook.adoc", md("= AsciiDoc\n")},
		entry{"uploads/net/diagram.png", pngBytes},
		entry{"uploads/manual.pdf", md("%PDF-1.7\n%test\n")},
		entry{"uploads/evil.png", md(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		entry{"uploads/unused.png", pngBytes},
		entry{".git/config", md("x")},
	)
}

func analyzeWiki(t *testing.T, zr *zip.Reader) *Plan {
	t.Helper()
	a, err := OpenArchive(zr, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := AnalyzeWikiJS(a)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestAnalyzeWikiJSExport(t *testing.T) {
	plan := analyzeWiki(t, wikiExport(t))
	docs := byPath(plan)
	home, servers, proxmox := docs["home.md"], docs["servers.md"], docs["servers/proxmox.html"]
	if home.Title != "Home" || home.ParentID != "" || home.Folder {
		t.Fatalf("home: %+v", home)
	}
	if !servers.Folder || servers.Title != "Servers" || servers.ParentID != "" {
		t.Fatalf("a page beside a folder of its name must be the folder doc: %+v", servers)
	}
	if proxmox.ParentID != servers.ID || proxmox.Title != "Proxmox: host" {
		t.Fatalf("proxmox: %+v", proxmox)
	}
	if !strings.Contains(proxmox.Content, "## Setup") || !strings.Contains(proxmox.Content, "[servers]("+"/docs/"+servers.ID+")") {
		t.Fatalf("HTML not converted or link not rewritten: %q", proxmox.Content)
	}
	for _, bad := range []string{"alert", "<script"} {
		if strings.Contains(proxmox.Content, bad) {
			t.Fatalf("%s survived conversion: %q", bad, proxmox.Content)
		}
	}
	if !strings.Contains(proxmox.Content, "| A |") {
		t.Fatalf("table lost: %q", proxmox.Content)
	}
	if strings.Contains(home.Content, "title:") || !strings.HasPrefix(home.Content, "# Welcome") {
		t.Fatalf("metadata block not stripped: %q", home.Content)
	}
	if !strings.Contains(home.Content, "[Servers](/docs/"+servers.ID+")") ||
		!strings.Contains(home.Content, "[Proxmox](/docs/"+proxmox.ID+")") {
		t.Fatalf("site links not rewritten: %q", home.Content)
	}
	fr := docs["fr/accueil.md"]
	if !strings.Contains(home.Content, "[FR](/docs/"+fr.ID+")") {
		t.Fatalf("locale link not rewritten: %q", home.Content)
	}
	if !strings.Contains(servers.Content, "[Home](/docs/"+home.ID+")") {
		t.Fatalf("default-locale prefixed link not rewritten: %q", servers.Content)
	}
	if !strings.Contains(home.Content, "`[code](/servers)`") || !strings.Contains(home.Content, "[ext](https://example.com)") ||
		!strings.Contains(home.Content, "[gone](/nowhere)") {
		t.Fatalf("code, external or unresolved links changed: %q", home.Content)
	}
	if len(home.Attachments) != 2 || !strings.Contains(home.Content, "![diagram](attachment:"+home.Attachments[0].ID+")") ||
		!strings.Contains(home.Content, "[manual](attachment:") {
		t.Fatalf("assets: %+v %q", home.Attachments, home.Content)
	}
	// "fr" is a folder doc for its locale; the unpublished page is under it.
	if fr.ParentID == "" || docs["fr"].Title != "fr" || !docs["fr"].Folder {
		t.Fatalf("locale folder: %+v", fr)
	}
	if !hasIssue(plan.Warnings, "fr/accueil.md", "not published") {
		t.Fatalf("unpublished warning missing: %+v", plan.Warnings)
	}
	if !hasIssue(plan.Warnings, "home.md", "unresolved link [gone](/nowhere)") || !hasIssue(plan.Warnings, "home.md", "missing.png") {
		t.Fatalf("unresolved warnings missing: %+v", plan.Warnings)
	}
	intro := docs["guide/Intro.MD"]
	if intro.Title != "Intro" || intro.Content != "No metadata, a title heading\n" {
		t.Fatalf("page without metadata: %+v", intro)
	}
	for path, msg := range map[string]string{
		"guide/Intro.html":   "already provides",
		"ops/runbook.adoc":   "AsciiDoc",
		"uploads/evil.png":   "not an allowed type",
		"uploads/unused.png": "not embedded",
		".git":               "hidden",
	} {
		if !hasIssue(plan.Skipped, path, msg) {
			t.Errorf("skipped %s (%s) missing: %+v", path, msg, plan.Skipped)
		}
	}
	if plan.AttachmentCount != 2 {
		t.Fatalf("attachment count %d", plan.AttachmentCount)
	}
}

func TestWikiJSAssetLinksStayInsideTheArchive(t *testing.T) {
	plan := analyzeWiki(t, buildZip(t,
		entry{"home.md", wikiMD("Home", "", "[a](/uploads/../../etc/passwd.png)\n[b](/%2e%2e/%2e%2e/uploads/diagram.png)\n![c](/uploads/net/../net/diagram.png)\n")},
		entry{"uploads/diagram.png", pngBytes},
		entry{"uploads/net/diagram.png", pngBytes},
	))
	home := byPath(plan)["home.md"]
	if !strings.Contains(home.Content, "[a](/uploads/../../etc/passwd.png)") {
		t.Fatalf("escaping link must stay text: %q", home.Content)
	}
	if !hasIssue(plan.Warnings, "home.md", "unresolved link [a](/uploads/../../etc/passwd.png)") {
		t.Fatalf("unresolved warning missing: %+v", plan.Warnings)
	}
	if len(home.Attachments) != 2 {
		t.Fatalf("only the two existing entries may resolve: %+v", home.Attachments)
	}
	for _, a := range home.Attachments {
		if a.Path != "uploads/diagram.png" && a.Path != "uploads/net/diagram.png" {
			t.Fatalf("attachment outside the archive entries: %+v", a)
		}
	}
	if strings.Contains(home.Content, "[b](/%2e") || strings.Contains(home.Content, "![c](/uploads") ||
		!strings.Contains(home.Content, "[b](attachment:") || !strings.Contains(home.Content, "![c](attachment:") {
		t.Fatalf("cleaned asset links not resolved: %q", home.Content)
	}
}

func TestWikiJSDepthIsClamped(t *testing.T) {
	plan := analyzeWiki(t, buildZip(t, entry{"a/b/c/d/e/f/g.md", wikiMD("G", "", "g\n")}))
	if !hasIssue(plan.Warnings, "a/b/c/d/e/f/g", "nested deeper") {
		t.Fatalf("depth warning missing: %+v", plan.Warnings)
	}
	for _, d := range plan.Docs {
		if d.Title == "G" && d.ParentID == "" {
			t.Fatal("G must be nested")
		}
	}
}

func TestSplitWikiMeta(t *testing.T) {
	cases := []struct {
		name, in string
		html     bool
		title    string
		body     string
	}{
		{"colon in title", "---\ntitle: A: B - C\npublished: true\n---\n\nbody", false, "A: B - C", "body"},
		{"crlf and bom", "\ufeff---\r\ntitle: X\r\n---\r\n\r\nbody", false, "X", "body"},
		{"html", "<!--\ntitle: H\n-->\n\n<p>x</p>", true, "H", "<p>x</p>"},
		{"horizontal rule only", "---\nnot meta\n---\ntext", false, "", "---\nnot meta\n---\ntext"},
		{"unclosed", "---\ntitle: X\nbody", false, "", "---\ntitle: X\nbody"},
		{"html comment is not meta", "<!-- note -->\n<p>x</p>", true, "", "<!-- note -->\n<p>x</p>"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta, body := splitWikiMeta(c.in, c.html)
			if meta["title"] != c.title || body != c.body {
				t.Fatalf("title %q body %q", meta["title"], body)
			}
		})
	}
}

func TestParseSource(t *testing.T) {
	for in, want := range map[string]Source{"": SourceMarkdown, "markdown": SourceMarkdown, "wikijs": SourceWikiJS} {
		if got, err := ParseSource(in); err != nil || got != want {
			t.Errorf("%q: %v %v", in, got, err)
		}
	}
	if _, err := ParseSource("bookstack"); !errors.Is(err, ErrUnknownSource) {
		t.Fatalf("got %v", err)
	}
}

func TestAnalyzeSourceDispatch(t *testing.T) {
	a, err := OpenArchive(wikiExport(t), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	plan, err := AnalyzeSource(SourceWikiJS, a, nil)
	if err != nil || byPath(plan)["home.md"].Title != "Home" {
		t.Fatalf("%v", err)
	}
}

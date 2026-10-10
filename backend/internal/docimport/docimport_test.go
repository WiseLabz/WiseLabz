package docimport

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00")

type entry struct {
	name string
	data []byte
}

func buildZip(t *testing.T, entries ...entry) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, e := range entries {
		f, err := w.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

func md(s string) []byte { return []byte(s) }

// vault is a small Obsidian vault: nested folders, folder notes, front-matter,
// embeds, aliases, headings and two notes sharing a basename.
func vault(t *testing.T) *zip.Reader {
	return buildZip(t,
		entry{"Homelab/index.md", md("---\ntitle: My Homelab\ntags: [lab]\n---\n# Welcome\nSee [[Proxmox]] and [[Network|the network]].\n")},
		entry{"Homelab/Servers/Servers.md", md("# Server room\nRack: ![[rack.png|300]]\n")},
		entry{"Homelab/Servers/Proxmox.md", md("---\nconnector: PVE\n---\n# Proxmox host\n![](../attachments/diagram.png)\n[[Network#VLANs]]\n![[manual.pdf]]\n[up](../Network.md)\n")},
		entry{"Homelab/Network.md", md("Network notes `[[NotALink]]`\n```\n[[AlsoNot]]\n```\nSee [[Setup]] and [[Nowhere]] and ![[Proxmox]].\n[site](https://example.com) [x](attachment:abc)\n")},
		entry{"Homelab/A/Setup.md", md("A setup\n")},
		entry{"Homelab/B/Setup.md", md("B setup\n")},
		entry{"Homelab/attachments/rack.png", pngBytes},
		entry{"Homelab/attachments/diagram.png", pngBytes},
		entry{"Homelab/attachments/manual.pdf", md("%PDF-1.7\n%test\n")},
		entry{"Homelab/attachments/evil.png", md("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")},
		entry{"Homelab/attachments/unused.png", pngBytes},
		entry{"Homelab/.obsidian/app.json", md("{}")},
		entry{"Homelab/.obsidian/workspace.json", md("{}")},
		entry{"Homelab/report.docx", md("PK")},
	)
}

func analyze(t *testing.T, zr *zip.Reader, limits Limits) *Plan {
	t.Helper()
	a, err := OpenArchive(zr, limits)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Analyze(a, []Connector{{ID: "conn-pve", Name: "pve"}})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func byPath(plan *Plan) map[string]Doc {
	m := map[string]Doc{}
	for _, d := range plan.Docs {
		m[d.Path] = d
	}
	return m
}

func hasIssue(issues []Issue, path, substr string) bool {
	for _, i := range issues {
		if i.Path == path && strings.Contains(i.Message, substr) {
			return true
		}
	}
	return false
}

func TestAnalyzeObsidianVault(t *testing.T) {
	plan := analyze(t, vault(t), DefaultLimits())
	docs := byPath(plan)
	for _, p := range []string{"Homelab", "Homelab/Servers", "Homelab/Servers/Proxmox.md", "Homelab/Network.md", "Homelab/A", "Homelab/B", "Homelab/A/Setup.md", "Homelab/B/Setup.md"} {
		if _, ok := docs[p]; !ok {
			t.Fatalf("missing doc %s in %+v", p, plan.Docs)
		}
	}
	if len(plan.Docs) != 8 {
		t.Fatalf("want 8 docs (folder notes consumed), got %d", len(plan.Docs))
	}
	root, servers, pve, network := docs["Homelab"], docs["Homelab/Servers"], docs["Homelab/Servers/Proxmox.md"], docs["Homelab/Network.md"]
	// Parents come before children.
	seen := map[string]bool{"": true}
	for _, d := range plan.Docs {
		if !seen[d.ParentID] {
			t.Fatalf("%s listed before its parent", d.Path)
		}
		seen[d.ID] = true
	}
	// Title order: front-matter, then H1, then filename.
	if root.Title != "My Homelab" || servers.Title != "Server room" || pve.Title != "Proxmox host" || network.Title != "Network" {
		t.Fatalf("titles %q %q %q %q", root.Title, servers.Title, pve.Title, network.Title)
	}
	if strings.Contains(root.Content, "tags:") || !strings.HasPrefix(root.Content, "# Welcome") {
		t.Fatalf("front-matter not stripped: %q", root.Content)
	}
	if servers.ParentID != root.ID || network.ParentID != root.ID || root.ParentID != "" || root.ServiceID != "" {
		t.Fatal("folders not nested")
	}
	// A connector doc under a lab folder moves to its connector's root.
	if pve.ServiceID != "conn-pve" || pve.ParentID != "" || !hasIssue(plan.Warnings, "Homelab/Servers/Proxmox.md", "placed at the root of pve") {
		t.Fatalf("connector placement %+v %+v", pve, plan.Warnings)
	}
	want := "See [Proxmox](/docs/" + pve.ID + ") and [the network](/docs/" + network.ID + ")."
	if !strings.Contains(root.Content, want) {
		t.Fatalf("wikilinks not rewritten: %q", root.Content)
	}
	if len(servers.Attachments) != 1 || !strings.Contains(servers.Content, "![rack.png](attachment:"+servers.Attachments[0].ID+")") {
		t.Fatalf("sized image embed: %q %+v", servers.Content, servers.Attachments)
	}
	if len(pve.Attachments) != 2 || pve.Attachments[0].Path != "Homelab/attachments/diagram.png" || pve.Attachments[1].Filename != "manual.pdf" {
		t.Fatalf("pve attachments %+v", pve.Attachments)
	}
	for _, s := range []string{
		"![diagram.png](attachment:" + pve.Attachments[0].ID + ")",
		"[Network#VLANs](/docs/" + network.ID + ")",
		"[manual.pdf](attachment:" + pve.Attachments[1].ID + ")",
		"[up](/docs/" + network.ID + ")",
	} {
		if !strings.Contains(pve.Content, s) {
			t.Fatalf("pve content lacks %q: %q", s, pve.Content)
		}
	}
	if plan.AttachmentCount != 3 {
		t.Fatalf("attachment count %d", plan.AttachmentCount)
	}
	// Code is untouched; ambiguous and unresolved links stay as text.
	for _, s := range []string{"`[[NotALink]]`", "```\n[[AlsoNot]]\n```", "[[Setup]]", "[[Nowhere]]", "[site](https://example.com)", "[x](attachment:abc)", "[Proxmox](/docs/" + pve.ID + ")"} {
		if !strings.Contains(network.Content, s) {
			t.Fatalf("network content lacks %q: %q", s, network.Content)
		}
	}
	for path, msg := range map[string]string{"Homelab/Network.md": "ambiguous link [[Setup]] matches Homelab/A/Setup.md, Homelab/B/Setup.md"} {
		if !hasIssue(plan.Warnings, path, msg) {
			t.Fatalf("missing warning %q in %+v", msg, plan.Warnings)
		}
	}
	if !hasIssue(plan.Warnings, "Homelab/Network.md", "unresolved link [[Nowhere]]") || !hasIssue(plan.Warnings, "Homelab/Network.md", "note embed") {
		t.Fatalf("warnings %+v", plan.Warnings)
	}
	for path, msg := range map[string]string{
		"Homelab/.obsidian":              "hidden",
		"Homelab/report.docx":            "unsupported file type",
		"Homelab/attachments/evil.png":   "not an allowed type",
		"Homelab/attachments/unused.png": "not embedded",
	} {
		if !hasIssue(plan.Skipped, path, msg) {
			t.Fatalf("missing skip %s %q in %+v", path, msg, plan.Skipped)
		}
	}
	if len(plan.Skipped) != 4 {
		t.Fatalf("hidden folder reported once: %+v", plan.Skipped)
	}
}

func TestImageEmbedSizeAlias(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{
			name: "width",
			link: "![[img.png|alt text|100]]",
			want: "![alt text](attachment:img.png)",
		},
		{
			name: "width and height",
			link: "![[img.png|alt text|100x50]]",
			want: "![alt text](attachment:img.png)",
		},
		{
			name: "size only",
			link: "![[img.png|100]]",
			want: "![img.png](attachment:img.png)",
		},
		{
			name: "numeric alias before size",
			link: "![[img.png|123|100]]",
			want: "![123](attachment:img.png)",
		},
		{
			name: "size only width and height",
			link: "![[img.png|100x50]]",
			want: "![img.png](attachment:img.png)",
		},
		{
			name: "no alias",
			link: "![[img.png]]",
			want: "![img.png](attachment:img.png)",
		},
		{
			name: "nonnumeric final pipe",
			link: "![[img.png|alt|caption]]",
			want: "![alt|caption](attachment:img.png)",
		},
		{
			name: "only final numeric pipe stripped",
			link: "![[img.png|alt|caption|100]]",
			want: "![alt|caption](attachment:img.png)",
		},
		{
			name: "invalid dimension",
			link: "![[img.png|alt|100x]]",
			want: "![alt|100x](attachment:img.png)",
		},
		{
			name: "whitespace around suffix",
			link: "![[img.png| alt text | 100 ]]",
			want: "![alt text](attachment:img.png)",
		},
		{
			name: "ordinary wiki image link",
			link: "[[img.png|linked image|100]]",
			want: "[linked image|100](attachment:img.png)",
		},
		{
			name: "markdown image link",
			link: "![Markdown image](img.png)",
			want: "![Markdown image](attachment:img.png)",
		},
		{
			name: "PDF link",
			link: "[[manual.pdf|download|100]]",
			want: "[download|100](attachment:manual.pdf)",
		},
		{
			name: "PDF embed",
			link: "![[manual.pdf|preview|100]]",
			want: "[preview|100](attachment:manual.pdf)",
		},
		{
			name: "note alias",
			link: "[[Target|note alias]]",
			want: "[note alias](/docs/Target.md)",
		},
		{
			name: "note alias with numeric pipe",
			link: "[[Target|note alias|100]]",
			want: "[note alias|100](/docs/Target.md)",
		},
	}
	lines := make([]string, 0, len(tests))
	for _, tt := range tests {
		lines = append(lines, tt.link)
	}
	plan := analyze(t, buildZip(t,
		entry{"Use.md", md(strings.Join(lines, "\n"))},
		entry{"Target.md", md("target")},
		entry{"img.png", pngBytes},
		entry{"manual.pdf", md("%PDF-1.7\n%test\n")},
	), DefaultLimits())
	docs := byPath(plan)
	gotLines := strings.Split(docs["Use.md"].Content, "\n")
	for _, a := range docs["Use.md"].Attachments {
		for i := range gotLines {
			gotLines[i] = strings.ReplaceAll(gotLines[i], "attachment:"+a.ID, "attachment:"+a.Path)
		}
	}
	for i := range gotLines {
		gotLines[i] = strings.ReplaceAll(gotLines[i], "/docs/"+docs["Target.md"].ID, "/docs/Target.md")
	}
	if len(gotLines) != len(tests) {
		t.Fatalf("got %d rewritten lines, want %d: %q", len(gotLines), len(tests), docs["Use.md"].Content)
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotLines[i] != tt.want {
				t.Fatalf("got %q, want %q", gotLines[i], tt.want)
			}
		})
	}
}

func TestResolutionPrefersLocalThenShortest(t *testing.T) {
	plan := analyze(t, buildZip(t,
		entry{"Setup.md", md("root")},
		entry{"deep/Setup.md", md("deep")},
		entry{"deep/Use.md", md("[[Setup]] [[deep/Setup]]")},
		entry{"Other.md", md("[[Setup]] [[Setup.md]]")},
	), DefaultLimits())
	docs := byPath(plan)
	use, other := docs["deep/Use.md"], docs["Other.md"]
	if use.Content != "[Setup](/docs/"+docs["deep/Setup.md"].ID+") [deep/Setup](/docs/"+docs["deep/Setup.md"].ID+")" {
		t.Fatalf("same folder wins: %q", use.Content)
	}
	if other.Content != "[Setup](/docs/"+docs["Setup.md"].ID+") [Setup.md](/docs/"+docs["Setup.md"].ID+")" {
		t.Fatalf("shortest path wins: %q", other.Content)
	}
}

func TestFrontMatterEdgeCases(t *testing.T) {
	plan := analyze(t, buildZip(t,
		entry{"bad.md", md("---\ntitle: [unclosed\n---\nbody")},
		entry{"unknown.md", md("---\nconnector: nope\n---\n# Heading\n")},
		entry{"plain.md", md("---\nnot front matter")},
		entry{"index.md", md("root index stays a doc")},
	), DefaultLimits())
	docs := byPath(plan)
	if docs["bad.md"].Content != "body" || !hasIssue(plan.Warnings, "bad.md", "not valid YAML") {
		t.Fatalf("bad yaml %+v %+v", docs["bad.md"], plan.Warnings)
	}
	if docs["unknown.md"].ServiceID != "" || docs["unknown.md"].Title != "Heading" || !hasIssue(plan.Warnings, "unknown.md", `unknown connector "nope"`) {
		t.Fatalf("unknown connector %+v", docs["unknown.md"])
	}
	if docs["plain.md"].Title != "plain" || docs["index.md"].Title != "index" {
		t.Fatal("filename titles")
	}
}

func TestDepthIsClampedToFive(t *testing.T) {
	plan := analyze(t, buildZip(t, entry{"a/b/c/d/e/f/g.md", md("deep")}), DefaultLimits())
	docs := byPath(plan)
	if docs["a/b/c/d/e/f"].ParentID != docs["a/b/c/d"].ID || docs["a/b/c/d/e/f/g.md"].ParentID != docs["a/b/c/d"].ID {
		t.Fatal("deep docs not reattached at depth five")
	}
	if !hasIssue(plan.Warnings, "a/b/c/d/e/f/g.md", "deeper than 5") {
		t.Fatalf("warnings %+v", plan.Warnings)
	}
}

func TestArchiveGuards(t *testing.T) {
	for _, name := range []string{"../evil.md", "/abs.md", "C:/win.md", "a/../../x.md", `..\evil.md`} {
		if _, err := OpenArchive(buildZip(t, entry{name, md("x")}), DefaultLimits()); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("%s: zip slip not rejected: %v", name, err)
		}
	}
	bomb := buildZip(t, entry{"bomb.md", make([]byte, 4<<20)})
	if _, err := OpenArchive(bomb, DefaultLimits()); !errors.Is(err, ErrCompressionRatio) {
		t.Fatalf("zip bomb not rejected: %v", err)
	}
	limits := DefaultLimits()
	limits.MaxEntries = 2
	if _, err := OpenArchive(buildZip(t, entry{"a.md", nil}, entry{"b.md", nil}, entry{"c.md", nil}), limits); !errors.Is(err, ErrTooManyEntries) {
		t.Fatalf("entry limit: %v", err)
	}
	limits = DefaultLimits()
	limits.MaxBytes = 10
	if _, err := OpenArchive(buildZip(t, entry{"a.md", md("0123456789abc")}), limits); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("declared size limit: %v", err)
	}
	if _, err := OpenArchive(buildZip(t, entry{"a.md", nil}, entry{"a.md", nil}), DefaultLimits()); !errors.Is(err, ErrDuplicatePath) {
		t.Fatalf("duplicate: %v", err)
	}
	// Bytes are also counted while reading, whatever the headers declare.
	a, err := OpenArchive(buildZip(t, entry{"a.md", md("0123456789")}, entry{"b.md", md("0123456789")}), DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	a.limits.MaxBytes = 15
	if _, err := Analyze(a, nil); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("counted read limit: %v", err)
	}
	limits = DefaultLimits()
	limits.MaxNoteBytes = 4
	plan := analyze(t, buildZip(t, entry{"big.md", md("12345")}, entry{"bin.md", []byte("a\x00b")}), limits)
	if len(plan.Docs) != 0 || !hasIssue(plan.Skipped, "big.md", "larger than") || !hasIssue(plan.Skipped, "bin.md", "not UTF-8") {
		t.Fatalf("note limits %+v", plan)
	}
}

func TestPulledArchiveSkipsOnlyCompressionRatio(t *testing.T) {
	compressed := buildZip(t, entry{"note.md", md(strings.Repeat("x", (1<<20)+1))})
	if _, err := OpenArchive(compressed, DefaultLimits()); !errors.Is(err, ErrCompressionRatio) {
		t.Fatalf("strict reader error = %v, want compression ratio", err)
	}
	archive, err := OpenPulledArchive(compressed, DefaultLimits())
	if err != nil {
		t.Fatalf("pull reader rejected compressible note: %v", err)
	}
	if _, ok, err := archive.readAll("note.md", DefaultLimits().MaxNoteBytes); err != nil || !ok {
		t.Fatalf("read pulled note: ok=%v err=%v", ok, err)
	}
	archive.read = 0
	archive.limits.MaxBytes = 1 << 20
	rc, err := archive.Open("note.md")
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(io.Discard, rc)
	_ = rc.Close()
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("actual-read size error = %v, want size limit", err)
	}

	limits := DefaultLimits()
	limits.MaxBytes = 32
	if _, err := OpenPulledArchive(buildZip(t, entry{"large.md", md(strings.Repeat("x", 33))}), limits); !errors.Is(err, ErrTooLarge) {
		t.Errorf("pull reader size error = %v, want size limit", err)
	}
	limits = DefaultLimits()
	limits.MaxEntries = 1
	if _, err := OpenPulledArchive(buildZip(t, entry{"a.md", nil}, entry{"b.md", nil}), limits); !errors.Is(err, ErrTooManyEntries) {
		t.Errorf("pull reader entry error = %v, want entry limit", err)
	}
	if _, err := OpenPulledArchive(buildZip(t, entry{"../unsafe.md", nil}), DefaultLimits()); !errors.Is(err, ErrUnsafePath) {
		t.Errorf("pull reader path error = %v, want unsafe path", err)
	}
	if _, err := OpenPulledArchive(buildZip(t, entry{"same.md", nil}, entry{"same.md", nil}), DefaultLimits()); !errors.Is(err, ErrDuplicatePath) {
		t.Errorf("pull reader duplicate error = %v, want duplicate path", err)
	}
}

func TestStageClaimAndSweep(t *testing.T) {
	stage := NewStage(t.TempDir())
	id := "0b8f2a5e-6a3c-4f1e-9d2b-5c7e8f9a0b1c"
	dir, err := stage.Create(id)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := SavePlan(dir, &Plan{ID: id, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := stage.Create("../escape"); !errors.Is(err, ErrNotStaged) {
		t.Fatal("unsafe id accepted")
	}
	_, _, release, _, err := stage.Claim(id, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, err := stage.Claim(id, now); !errors.Is(err, ErrNotStaged) {
		t.Fatal("double claim")
	}
	release()
	if _, _, _, _, err := stage.Claim(id, now.Add(TTL+time.Second)); !errors.Is(err, ErrNotStaged) {
		t.Fatal("expired plan claimed")
	}
	if _, err := os.Stat(filepath.Join(stage.Dir, id)); !os.IsNotExist(err) {
		t.Fatal("expired plan not removed on claim")
	}
	fresh, _ := stage.Create("1b8f2a5e-6a3c-4f1e-9d2b-5c7e8f9a0b1c")
	old, _ := stage.Create("2b8f2a5e-6a3c-4f1e-9d2b-5c7e8f9a0b1c")
	past := now.Add(-2 * TTL)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := stage.Sweep(now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("fresh import swept")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("expired import kept")
	}
	if err := NewStage(filepath.Join(t.TempDir(), "missing")).Sweep(now); err != nil {
		t.Fatal(err)
	}
}

func TestStageReadPlanDoesNotClaim(t *testing.T) {
	stage := NewStage(t.TempDir())
	id := "0b8f2a5e-6a3c-4f1e-9d2b-5c7e8f9a0b1c"
	dir, err := stage.Create(id)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := SavePlan(dir, &Plan{ID: id, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if plan, err := stage.ReadPlan(id, now); err != nil || plan.ID != id {
			t.Fatalf("read = %v %v", plan, err)
		}
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatal("read removed or moved the staged import")
	}
	if _, _, release, _, err := stage.Claim(id, now); err != nil {
		t.Fatalf("claim after reads = %v", err)
	} else {
		release()
	}
	if _, err := stage.ReadPlan(id, now.Add(TTL+time.Second)); !errors.Is(err, ErrNotStaged) {
		t.Fatal("expired plan read")
	}
	if _, err := stage.ReadPlan("../escape", now); !errors.Is(err, ErrNotStaged) {
		t.Fatal("unsafe id read")
	}
	if _, err := stage.ReadPlan("1b8f2a5e-6a3c-4f1e-9d2b-5c7e8f9a0b1c", now); !errors.Is(err, ErrNotStaged) {
		t.Fatal("missing plan read")
	}
}

func TestSymlinkEntriesAreSkipped(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link.md"}
	h.SetMode(os.ModeSymlink | 0o777)
	f, err := w.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("/etc/passwd"))
	f, _ = w.Create("ok.md")
	_, _ = f.Write([]byte("ok"))
	_ = w.Close()
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	plan := analyze(t, zr, DefaultLimits())
	if len(plan.Docs) != 1 || plan.Docs[0].Path != "ok.md" || !hasIssue(plan.Skipped, "link.md", "symbolic") {
		t.Fatalf("plan %+v", plan)
	}
}

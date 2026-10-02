package docexport

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestExportAttachmentRewriteFlagsAndGitCap(t *testing.T) {
	s := apitest.NewStore(t)
	ctx := context.Background()
	blobs := blobstore.New(t.TempDir(), 0)
	b, err := blobs.Put(strings.NewReader("# attachment"))
	if err != nil {
		t.Fatal(err)
	}
	d := &store.DocRecord{Title: "note", Kind: "lab", Origin: store.DocOriginHuman}
	if err := s.CreateDoc(ctx, d); err != nil {
		t.Fatal(err)
	}
	a := &store.DocAttachment{DocID: d.ID, SHA256: b.SHA256, Filename: "note.md", ContentType: b.ContentType, Size: b.Size}
	if err := s.CreateDocAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	content := "<!-- wl:gen key=\"meta\" h=\"0123456789ab\" -->\n[note](attachment:" + a.ID + ") [other](attachment:unowned)\n<!-- /wl:gen -->"
	e := NewExporter(s)
	e.ConfigureAttachments(blobs, true, 4)
	dir := t.TempDir()
	got, err := e.exportContent(ctx, dir, d.ID, content)
	if err != nil {
		t.Fatal(err)
	}
	want := "attachments/" + b.SHA256 + ".txt"
	if !strings.Contains(got, want) || strings.Contains(got, "wl:gen") || !strings.Contains(got, "attachment:unowned") {
		t.Fatal(got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, want))
	if err != nil || string(raw) != "# attachment" {
		t.Fatalf("file %q %v", raw, err)
	}
	e.ConfigureAttachments(blobs, false, 4)
	got, err = e.exportContent(ctx, t.TempDir(), d.ID, content)
	if err != nil || !strings.Contains(got, "attachment:"+a.ID) {
		t.Fatalf("disabled %s %v", got, err)
	}
	e.ConfigureAttachments(blobs, true, 4)
	e.git = &gitTarget{}
	dir = t.TempDir()
	got, err = e.exportContent(ctx, dir, d.ID, content)
	if err != nil || !strings.Contains(got, "attachment:"+a.ID) {
		t.Fatalf("git cap %s %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, want)); !os.IsNotExist(err) {
		t.Fatal("large git attachment exported")
	}
}

func TestPruneAttachmentFilesKeepsOperatorFiles(t *testing.T) {
	dir := t.TempDir()
	attachmentDir := filepath.Join(dir, "attachments")
	if err := os.Mkdir(attachmentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := strings.Repeat("a", 64) + ".png"
	keep := strings.Repeat("b", 64) + ".pdf"
	for _, name := range []string{stale, keep, "manual.txt"} {
		if err := os.WriteFile(filepath.Join(attachmentDir, name), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := pruneAttachmentFiles(dir, map[string]bool{"attachments/" + keep: true})
	if err != nil || len(removed) != 1 || removed[0] != "attachments/"+stale {
		t.Fatalf("prune %v %v", removed, err)
	}
	for _, name := range []string{keep, "manual.txt"} {
		if _, err := os.Stat(filepath.Join(attachmentDir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

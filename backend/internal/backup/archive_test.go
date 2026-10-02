package backup_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/backup"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func attachmentArchive(t *testing.T) ([]byte, *backup.Bundle, *blobstore.Store) {
	t.Helper()
	ctx := context.Background()
	s := newTestStore(t)
	blobs := blobstore.New(t.TempDir(), 0)
	b, err := blobs.Put(strings.NewReader("%PDF-1.7\nattachment"))
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"first", "dedup"} {
		d := &store.DocRecord{Title: title, Kind: "lab", Origin: store.DocOriginHuman}
		if err := s.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
		a := &store.DocAttachment{DocID: d.ID, SHA256: b.SHA256, Filename: "test.pdf", ContentType: b.ContentType, Size: b.Size, CreatedBy: "author"}
		if err := s.CreateDocAttachment(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	bundle, err := backup.Export(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := backup.WriteArchive(ctx, s, blobs, &out, bundle); err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), bundle, blobs
}

func TestArchiveRoundTripAndV1Compatibility(t *testing.T) {
	data, b, _ := attachmentArchive(t)
	ctx := context.Background()
	target := newTestStore(t)
	dir := t.TempDir()
	z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(z.File) != 3 {
		t.Fatalf("expected bundle, manifest and deduplicated blob, got %d", len(z.File))
	}
	opts := backup.ArchiveOptions{BlobDir: dir}
	result, err := backup.ImportStream(ctx, target, bytes.NewReader(data), opts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Attachments.Imported != 2 || result.Docs.Imported != 2 {
		t.Fatalf("result %+v", result)
	}
	got, err := target.ListDocAttachments(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].SHA256 != b.Attachments[0].SHA256 {
		t.Fatalf("attachments %+v", got)
	}
	f, err := blobstore.New(dir, 0).Open(got[0].SHA256)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil || string(raw) != "%PDF-1.7\nattachment" {
		t.Fatalf("bytes %q %v", raw, err)
	}
	result, err = backup.ImportStream(ctx, target, bytes.NewReader(data), opts)
	if err != nil || result.Attachments.Skipped != 2 {
		t.Fatalf("idempotency %+v %v", result, err)
	}
	b.Version = 1
	b.Attachments = nil
	jsonData, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	result, err = backup.ImportStream(ctx, newTestStore(t), bytes.NewReader(jsonData), backup.ArchiveOptions{})
	if err != nil || result.Docs.Imported != 2 {
		t.Fatalf("v1 %+v %v", result, err)
	}
}

func TestArchiveRejectsTamperPathsAndBounds(t *testing.T) {
	data, _, _ := attachmentArchive(t)
	mutate := func(change func(string, []byte) (string, []byte)) []byte {
		t.Helper()
		z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		w := zip.NewWriter(&out)
		for _, entry := range z.File {
			r, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			raw, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatal(err)
			}
			name, raw := change(entry.Name, raw)
			f, err := w.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = f.Write(raw); err != nil {
				t.Fatal(err)
			}
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		return out.Bytes()
	}
	for _, tc := range []struct {
		name   string
		change func(string, []byte) (string, []byte)
	}{
		{"checksum", func(name string, b []byte) (string, []byte) {
			if strings.HasPrefix(name, "attachments/") {
				b = append(b, '!')
			}
			return name, b
		}},
		{"zip slip", func(name string, b []byte) (string, []byte) {
			if name == "bundle.json" {
				name = "../bundle.json"
			}
			return name, b
		}},
		{"missing blob", func(name string, b []byte) (string, []byte) {
			if strings.HasPrefix(name, "attachments/") {
				name = "attachments/" + strings.Repeat("f", 64)
			}
			return name, b
		}},
		{"mime mismatch", func(name string, b []byte) (string, []byte) {
			if name == "bundle.json" {
				b = bytes.ReplaceAll(b, []byte("application/pdf"), []byte("image/png"))
			}
			return name, b
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			if _, err := backup.ImportStream(context.Background(), s, bytes.NewReader(mutate(tc.change)), backup.ArchiveOptions{BlobDir: t.TempDir()}); err == nil {
				t.Fatal("invalid archive accepted")
			}
			b, err := backup.Export(context.Background(), s)
			if err != nil || len(b.Docs) != 0 || len(b.Attachments) != 0 {
				t.Fatalf("partial import %+v %v", b, err)
			}
		})
	}
	if _, err := backup.ImportStream(context.Background(), newTestStore(t), bytes.NewReader(data), backup.ArchiveOptions{MaxImportBytes: 10}); !errors.Is(err, backup.ErrImportTooLarge) {
		t.Fatal(err)
	}
	if _, err := backup.ImportStream(context.Background(), newTestStore(t), bytes.NewReader(data), backup.ArchiveOptions{BlobDir: t.TempDir(), MaxAttachmentBytes: 4}); !errors.Is(err, blobstore.ErrTooLarge) {
		t.Fatal(err)
	}
}

func TestArchiveFileVerification(t *testing.T) {
	_, b, blobs := attachmentArchive(t)
	s := newTestStore(t)
	if _, err := backup.Import(context.Background(), s, b); err != nil {
		t.Fatal(err)
	}
	run, err := backup.ExportToFile(context.Background(), s, t.TempDir(), backup.ArchiveOptions{BlobDir: blobs.Dir})
	if err != nil {
		t.Fatal(err)
	}
	result := backup.VerifyBundleFile(context.Background(), run.FilePath)
	if result.Status != "pass" || result.ActualCounts["attachments"] != 2 {
		t.Fatalf("verify %+v", result)
	}
}

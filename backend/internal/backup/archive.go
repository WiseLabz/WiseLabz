package backup

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// DefaultMaxImportBytes bounds compressed and expanded backup imports to 1 GiB.
const DefaultMaxImportBytes int64 = 1 << 30

// ErrImportTooLarge reports an archive exceeding configured import limits.
var ErrImportTooLarge = errors.New("backup exceeds import limit")

// ArchiveOptions configures blob storage and backup staging limits.
type ArchiveOptions struct {
	BlobDir            string
	MaxImportBytes     int64
	MaxAttachmentBytes int64
}

func archiveOptions(options []ArchiveOptions) ArchiveOptions {
	var opts ArchiveOptions
	if len(options) != 0 {
		opts = options[0]
	}
	if opts.MaxImportBytes <= 0 {
		opts.MaxImportBytes = DefaultMaxImportBytes
	}
	if opts.MaxAttachmentBytes <= 0 {
		opts.MaxAttachmentBytes = blobstore.DefaultMaxBytes
	}
	return opts
}
func checksumFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	n, err := io.Copy(hash, f)
	return hex.EncodeToString(hash.Sum(nil)), n, err
}

// WriteArchive streams deduplicated blobs and a checksummed bundle into a v2 ZIP.
func WriteArchive(ctx context.Context, s *store.Store, blobs *blobstore.Store, w io.Writer, b *Bundle) error {
	if b == nil {
		var err error
		b, err = Export(ctx, s)
		if err != nil {
			return err
		}
	}
	zw := zip.NewWriter(w)
	checksums := map[string]string{}
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	entry, err := zw.Create("bundle.json")
	if err != nil {
		return err
	}
	if _, err := entry.Write(data); err != nil {
		return err
	}
	checksums["bundle.json"] = ChecksumBytes(data)
	for _, a := range b.Attachments {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := "attachments/" + a.SHA256
		if _, ok := checksums[name]; ok {
			continue
		}
		f, err := blobs.Open(a.SHA256)
		if err != nil {
			return fmt.Errorf("open backup blob: %w", err)
		}
		entry, err := zw.Create(name)
		if err != nil {
			_ = f.Close()
			return err
		}
		hash := sha256.New()
		n, copyErr := io.Copy(io.MultiWriter(entry, hash), f)
		closeErr := f.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		got := hex.EncodeToString(hash.Sum(nil))
		if got != a.SHA256 || n != a.Size {
			return errors.New("backup blob checksum or size mismatch")
		}
		checksums[name] = got
	}
	entry, err = zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(entry).Encode(checksums); err != nil {
		return err
	}
	return zw.Close()
}

// ImportStream stages the bounded stream and validates all ZIP entries before publishing metadata.
func ImportStream(ctx context.Context, s *store.Store, r io.Reader, options ArchiveOptions) (Result, error) {
	opts := archiveOptions([]ArchiveOptions{options})
	f, err := os.CreateTemp("", "wiselabz-import-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	n, err := io.Copy(f, io.LimitReader(r, opts.MaxImportBytes+1))
	if err != nil {
		return Result{}, err
	}
	if n > opts.MaxImportBytes {
		return Result{}, ErrImportTooLarge
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Result{}, err
	}
	magic := make([]byte, 4)
	_, err = io.ReadFull(f, magic)
	if err != nil {
		return Result{}, fmt.Errorf("read backup header: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Result{}, err
	}
	var b Bundle
	if string(magic) != "PK\x03\x04" {
		decoder := json.NewDecoder(f)
		if err := decoder.Decode(&b); err != nil {
			return Result{}, err
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			return Result{}, errors.New("trailing backup data")
		}
		if len(b.Attachments) != 0 {
			return Result{}, errors.New("attachments require a v2 ZIP backup")
		}
		return Import(ctx, s, &b)
	}
	zr, err := zip.NewReader(f, n)
	if err != nil {
		return Result{}, err
	}
	entries := map[string]*zip.File{}
	var expanded uint64
	for _, entry := range zr.File {
		valid := entry.Name == "bundle.json" || entry.Name == "manifest.json"
		if strings.HasPrefix(entry.Name, "attachments/") {
			valid = blobstore.ValidHash(strings.TrimPrefix(entry.Name, "attachments/"))
		}
		if !valid || entries[entry.Name] != nil {
			return Result{}, errors.New("invalid or duplicate backup path")
		}
		if entry.UncompressedSize64 > uint64(opts.MaxImportBytes)-expanded {
			return Result{}, ErrImportTooLarge
		}
		expanded += entry.UncompressedSize64
		entries[entry.Name] = entry
	}
	readJSON := func(name string, out any) error {
		entry := entries[name]
		if entry == nil {
			return fmt.Errorf("missing %s", name)
		}
		r, err := entry.Open()
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		// Metadata has a separate cap so a ZIP can't allocate gigabytes of JSON.
		data, err := io.ReadAll(io.LimitReader(r, 16<<20+1))
		if err != nil {
			return err
		}
		if len(data) > 16<<20 {
			return ErrImportTooLarge
		}
		return json.Unmarshal(data, out)
	}
	if err := readJSON("bundle.json", &b); err != nil {
		return Result{}, err
	}
	if b.Version != BundleVersion {
		return Result{}, errors.New("ZIP backup must be version 2")
	}
	if err := ValidateBundle(&b); err != nil {
		return Result{}, err
	}
	checksums := map[string]string{}
	if err := readJSON("manifest.json", &checksums); err != nil {
		return Result{}, err
	}
	if len(checksums) != len(entries)-1 {
		return Result{}, errors.New("incomplete backup manifest")
	}
	return restoreArchive(ctx, s, &b, entries, checksums, opts)
}

func restoreArchive(ctx context.Context, s *store.Store, b *Bundle, entries map[string]*zip.File, checksums map[string]string, opts ArchiveOptions) (Result, error) {
	stage, err := os.MkdirTemp("", "wiselabz-blobs-*")
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	staged := blobstore.New(stage, opts.MaxAttachmentBytes)
	for name, entry := range entries {
		if name == "manifest.json" {
			continue
		}
		r, err := entry.Open()
		if err != nil {
			return Result{}, err
		}
		hash := sha256.New()
		var readErr error
		if name == "bundle.json" {
			_, readErr = io.Copy(hash, io.LimitReader(r, opts.MaxImportBytes+1))
		} else {
			var blob blobstore.Blob
			blob, readErr = staged.Put(io.TeeReader(r, hash))
			if readErr == nil && blob.SHA256 != strings.TrimPrefix(name, "attachments/") {
				readErr = errors.New("blob hash mismatch")
			}
		}
		closeErr := r.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return Result{}, err
		}
		if checksums[name] != hex.EncodeToString(hash.Sum(nil)) {
			return Result{}, errors.New("backup manifest checksum mismatch")
		}
	}
	used := map[string]bool{}
	for _, a := range b.Attachments {
		entry := entries["attachments/"+a.SHA256]
		if entry == nil || entry.UncompressedSize64 != uint64(a.Size) {
			return Result{}, errors.New("missing or wrong-size attachment")
		}
		f, err := staged.Open(a.SHA256)
		if err != nil {
			return Result{}, err
		}
		head := make([]byte, 512)
		count, readErr := f.Read(head)
		_ = f.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return Result{}, readErr
		}
		if http.DetectContentType(head[:count]) != a.ContentType {
			return Result{}, errors.New("attachment MIME mismatch")
		}
		used["attachments/"+a.SHA256] = true
	}
	for name := range entries {
		if strings.HasPrefix(name, "attachments/") && !used[name] {
			return Result{}, errors.New("unreferenced backup blob")
		}
	}
	blobstore.PublicationMu.Lock()
	defer blobstore.PublicationMu.Unlock()
	blobs := blobstore.New(opts.BlobDir, opts.MaxAttachmentBytes)
	for name := range used {
		f, err := staged.Open(strings.TrimPrefix(name, "attachments/"))
		if err != nil {
			return Result{}, err
		}
		_, putErr := blobs.Put(f)
		closeErr := f.Close()
		if err := errors.Join(putErr, closeErr); err != nil {
			return Result{}, err
		}
	}
	return Import(ctx, s, b)
}

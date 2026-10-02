// Package docimport turns a Markdown or Obsidian vault zip into a plan of
// human docs and attachments that the docs API previews and then commits.
package docimport

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
)

// Limits bounds what an uploaded archive may cost to read.
type Limits struct {
	MaxEntries         int    // zip entries, directories included
	MaxBytes           int64  // uncompressed bytes, declared and counted while reading
	MaxRatio           uint64 // uncompressed:compressed ratio per entry
	MaxNoteBytes       int64  // one Markdown note
	MaxAttachmentBytes int64  // one attachment, normally the blobstore limit
}

// MaxUploadBytes caps the uploaded zip itself.
const MaxUploadBytes int64 = 100 << 20

// ratioFloor exempts small entries, which compress well without being bombs.
const ratioFloor = 1 << 20

// DefaultLimits returns the roadmap limits: 2000 entries, 500 MB unzipped,
// a 100:1 ratio, 5 MiB notes and 25 MiB attachments.
func DefaultLimits() Limits {
	return Limits{MaxEntries: 2000, MaxBytes: 500 << 20, MaxRatio: 100, MaxNoteBytes: 5 << 20, MaxAttachmentBytes: 25 << 20}
}

var (
	// ErrUnsafePath rejects absolute, drive-qualified or parent-relative entry names.
	ErrUnsafePath = errors.New("archive entry has an unsafe path")
	// ErrTooManyEntries rejects archives beyond Limits.MaxEntries.
	ErrTooManyEntries = errors.New("archive has too many entries")
	// ErrTooLarge rejects archives that expand beyond Limits.MaxBytes.
	ErrTooLarge = errors.New("archive expands beyond the size limit")
	// ErrCompressionRatio rejects entries that look like a zip bomb.
	ErrCompressionRatio = errors.New("archive entry exceeds the compression ratio limit")
	// ErrDuplicatePath rejects archives naming the same file twice.
	ErrDuplicatePath = errors.New("archive names the same file twice")
)

// Archive is a zip whose entry names and sizes passed the guards.
type Archive struct {
	files  map[string]*zip.File
	paths  []string
	limits Limits
	read   int64
}

// OpenArchive validates every entry before any content is used.
func OpenArchive(zr *zip.Reader, limits Limits) (*Archive, error) {
	if len(zr.File) > limits.MaxEntries {
		return nil, ErrTooManyEntries
	}
	a := &Archive{files: map[string]*zip.File{}, limits: limits}
	var declared uint64
	for _, f := range zr.File {
		name, err := cleanName(f.Name)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(f.Name, "/") || f.FileInfo().IsDir() || name == "." {
			continue
		}
		size := f.UncompressedSize64
		if size > ratioFloor && size/max(f.CompressedSize64, 1) > limits.MaxRatio {
			return nil, fmt.Errorf("%w: %s", ErrCompressionRatio, name)
		}
		if size > uint64(limits.MaxBytes)-declared {
			return nil, ErrTooLarge
		}
		declared += size
		if a.files[name] != nil {
			return nil, fmt.Errorf("%w: %s", ErrDuplicatePath, name)
		}
		a.files[name] = f
		a.paths = append(a.paths, name)
	}
	sort.Strings(a.paths)
	return a, nil
}

func cleanName(name string) (string, error) {
	n := strings.ReplaceAll(name, "\\", "/")
	unsafe := n == "" || strings.HasPrefix(n, "/") || strings.ContainsRune(n, 0) ||
		(len(n) >= 2 && n[1] == ':')
	for _, seg := range strings.Split(n, "/") {
		unsafe = unsafe || seg == ".."
	}
	if unsafe {
		return "", fmt.Errorf("%w: %q", ErrUnsafePath, name)
	}
	return path.Clean(n), nil
}

// Paths lists the archive's files, sorted.
func (a *Archive) Paths() []string { return a.paths }

// Size returns an entry's declared uncompressed size.
func (a *Archive) Size(p string) int64 {
	if f := a.files[p]; f != nil {
		return int64(f.UncompressedSize64)
	}
	return 0
}

// Open streams an entry, counting its bytes against Limits.MaxBytes.
func (a *Archive) Open(p string) (io.ReadCloser, error) {
	f := a.files[p]
	if f == nil {
		return nil, fmt.Errorf("archive entry %q not found", p)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	return &countingReader{rc: rc, a: a}, nil
}

// readAll reads an entry of at most limit bytes; ok is false when it is larger.
func (a *Archive) readAll(p string, limit int64) (data []byte, ok bool, err error) {
	rc, err := a.Open(p)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = rc.Close() }()
	data, err = io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, false, err
	}
	return data, int64(len(data)) <= limit, nil
}

type countingReader struct {
	rc io.ReadCloser
	a  *Archive
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.a.read += int64(n)
	if c.a.read > c.a.limits.MaxBytes {
		return n, ErrTooLarge
	}
	return n, err
}

func (c *countingReader) Close() error { return c.rc.Close() }

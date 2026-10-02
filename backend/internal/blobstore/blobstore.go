// Package blobstore stores bounded, sniffed attachments by their SHA256 digest.
package blobstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// DefaultMaxBytes limits each attachment to 25 MiB.
const DefaultMaxBytes int64 = 25 << 20

// DefaultDir is the persistent attachment directory.
const DefaultDir = "/data/attachments"

// ErrTooLarge reports an attachment beyond the configured byte limit.
var ErrTooLarge = errors.New("attachment exceeds size limit")

// ErrUnsupported reports bytes outside the sniffed MIME allowlist.
var ErrUnsupported = errors.New("unsupported attachment content")

// PublicationMu protects publication through metadata commit against orphan collection.
var PublicationMu sync.Mutex
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Store holds the on-disk blob directory and upload limit.
type Store struct {
	Dir      string
	MaxBytes int64
}

// Blob describes the immutable bytes published by Put.
type Blob struct {
	SHA256      string
	ContentType string
	Size        int64
}

// New returns a blob store with defaults for empty settings.
func New(dir string, maxBytes int64) *Store {
	if dir == "" {
		dir = DefaultDir
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Store{Dir: dir, MaxBytes: maxBytes}
}

// ValidHash reports whether hash is a lowercase SHA256 digest.
func ValidHash(hash string) bool { return hashPattern.MatchString(hash) }
func (s *Store) path(hash string) (string, error) {
	if !ValidHash(hash) {
		return "", errors.New("invalid blob hash")
	}
	return filepath.Join(s.Dir, hash[:2], hash[2:4], hash), nil
}

// Put streams, validates and atomically publishes bytes under their SHA256 digest.
func (s *Store) Put(r io.Reader) (Blob, error) {
	var blob Blob
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return blob, fmt.Errorf("create blob directory: %w", err)
	}
	f, err := os.CreateTemp(s.Dir, ".upload-*")
	if err != nil {
		return blob, fmt.Errorf("create upload: %w", err)
	}
	defer func() { _ = f.Close(); _ = os.Remove(f.Name()) }()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(r, s.MaxBytes+1))
	if err != nil {
		return blob, fmt.Errorf("stream upload: %w", err)
	}
	if n > s.MaxBytes {
		return blob, ErrTooLarge
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return blob, fmt.Errorf("rewind upload: %w", err)
	}
	head := make([]byte, 512)
	count, err := f.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return blob, fmt.Errorf("sniff upload: %w", err)
	}
	ct := http.DetectContentType(head[:count])
	if !Allowed(ct, head[:count]) {
		return blob, ErrUnsupported
	}
	blob = Blob{SHA256: hex.EncodeToString(hash.Sum(nil)), ContentType: ct, Size: n}
	path, err := s.path(blob.SHA256)
	if err != nil {
		return blob, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return blob, fmt.Errorf("create hash directory: %w", err)
	}
	if err := f.Sync(); err != nil {
		return blob, fmt.Errorf("sync upload: %w", err)
	}
	if err := f.Close(); err != nil {
		return blob, fmt.Errorf("close upload: %w", err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return blob, fmt.Errorf("publish upload: %w", err)
	}
	return blob, nil
}

// Allowed checks sniffed MIME types and rejects SVG masquerading as text.
func Allowed(ct string, head []byte) bool {
	switch ct {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "application/pdf":
		return true
	case "text/plain; charset=utf-8":
		// SVG is sometimes sniffed as text, particularly without an XML declaration.
		lower := bytes.ToLower(head)
		return !bytes.Contains(lower, []byte("<svg")) && !bytes.Contains(lower, []byte("<?xml"))
	default:
		return false
	}
}

// Extension returns a safe filename extension for a sniffed content type.
func Extension(ct string) string {
	switch strings.Split(ct, ";")[0] {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	default:
		return ".txt"
	}
}

// Open opens a published blob by digest.
func (s *Store) Open(hash string) (*os.File, error) {
	path, err := s.path(hash)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// Delete removes a blob; already absent blobs succeed.
func (s *Store) Delete(hash string) error {
	path, err := s.path(hash)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Sweep calls referenced for each published blob, retaining all referenced bytes.
// The caller must hold PublicationMu through its database snapshot and sweep.
func (s *Store) Sweep(referenced func(string) (bool, error)) error {
	err := filepath.WalkDir(s.Dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !ValidHash(d.Name()) {
			return nil
		}
		expected, _ := s.path(d.Name())
		if filepath.Clean(path) != filepath.Clean(expected) {
			return nil
		}
		keep, err := referenced(d.Name())
		if err != nil {
			return err
		}
		if keep {
			return nil
		}
		return s.Delete(d.Name())
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

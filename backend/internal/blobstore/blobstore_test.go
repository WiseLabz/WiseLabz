package blobstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPutSniffSizeAndHash(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		mime   string
		reject bool
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nrest"), "image/png", false},
		{"jpeg", []byte("\xff\xd8\xffrest"), "image/jpeg", false},
		{"gif", []byte("GIF89arest"), "image/gif", false},
		{"webp", []byte("RIFFxxxxWEBPVP8 rest"), "image/webp", false},
		{"pdf", []byte("%PDF-1.7\nrest"), "application/pdf", false},
		{"markdown", []byte("# note\nhello"), "text/plain; charset=utf-8", false},
		{"spoofed png", []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), "", true},
		{"xml", []byte("<?xml version=\"1.0\"?><svg/>"), "", true},
		{"html", []byte("<!DOCTYPE html><html></html>"), "", true},
		{"binary", []byte{0, 1, 2, 3}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(t.TempDir(), 1024)
			b, err := s.Put(bytes.NewReader(tc.data))
			if tc.reject {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatalf("got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(tc.data)
			want := hex.EncodeToString(sum[:])
			if b.SHA256 != want || b.Size != int64(len(tc.data)) || b.ContentType != tc.mime {
				t.Fatalf("blob = %+v", b)
			}
			if _, err := os.Stat(filepath.Join(s.Dir, want[:2], want[2:4], want)); err != nil {
				t.Fatal(err)
			}
			f, err := s.Open(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(f)
			_ = f.Close()
			if err != nil || !bytes.Equal(got, tc.data) {
				t.Fatalf("roundtrip %q, %v", got, err)
			}
			again, err := s.Put(bytes.NewReader(tc.data))
			if err != nil || again != b {
				t.Fatalf("dedup %+v %v", again, err)
			}
			if err := s.Delete(want); err != nil {
				t.Fatal(err)
			}
			if err := s.Delete(want); err != nil {
				t.Fatal(err)
			}
		})
	}
	s := New(t.TempDir(), 4)
	if _, err := s.Put(strings.NewReader("12345")); !errors.Is(err, ErrTooLarge) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary files left: %v %v", entries, err)
	}
	if _, err := s.Put(strings.NewReader("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open("../../secret"); err == nil {
		t.Fatal("invalid hash accepted")
	}
}

func TestSweepRetainsReferences(t *testing.T) {
	s := New(t.TempDir(), 0)
	keep, err := s.Put(strings.NewReader("keep"))
	if err != nil {
		t.Fatal(err)
	}
	orphan, err := s.Put(strings.NewReader("orphan"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Sweep(func(hash string) (bool, error) { return hash == keep.SHA256, nil }); err != nil {
		t.Fatal(err)
	}
	f, err := s.Open(keep.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := s.Open(orphan.SHA256); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	sentinel := errors.New("database unavailable")
	if err := s.Sweep(func(string) (bool, error) { return false, sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestSignedURLExpiryTamperAndKeySeparation(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := NewSigner("secret")
	exp := "1700000900"
	sig := s.signature("attachment-id", exp)
	if !s.Valid("attachment-id", exp, sig, now) {
		t.Fatal("valid signature rejected")
	}
	for _, tc := range []struct {
		id, exp, sig string
		now          time.Time
	}{
		{"other", exp, sig, now}, {"attachment-id", "1700000899", sig, now},
		{"attachment-id", exp, "00", now}, {"attachment-id", exp, sig, now.Add(URLTTL)},
		{"attachment-id", exp, sig, now.Add(-time.Second)}, {"attachment-id", "bad", sig, now},
	} {
		if s.Valid(tc.id, tc.exp, tc.sig, tc.now) {
			t.Fatalf("invalid signature accepted: %+v", tc)
		}
	}
	if NewSigner("different").Valid("attachment-id", exp, sig, now) {
		t.Fatal("wrong key accepted")
	}
}

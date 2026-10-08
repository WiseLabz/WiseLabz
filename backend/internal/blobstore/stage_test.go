package blobstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestStageSurvivesSweepUntilPublish(t *testing.T) {
	s := New(t.TempDir(), 0)
	const data = "# staged attachment"
	staged, err := s.Stage(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = staged.Close() }()
	sum := sha256.Sum256([]byte(data))
	hash := hex.EncodeToString(sum[:])
	if _, err := s.Open(hash); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged bytes already published: %v", err)
	}
	if err := s.Sweep(func(string) (bool, error) {
		t.Fatal("sweep treated a staged upload as published")
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
	blob, err := staged.Publish()
	if err != nil {
		t.Fatal(err)
	}
	if blob.SHA256 != hash || blob.Size != int64(len(data)) || blob.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("blob = %+v", blob)
	}
	if err := staged.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := s.Open(hash)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	got, err := io.ReadAll(f)
	if err != nil || string(got) != data {
		t.Fatalf("published bytes = %q, %v", got, err)
	}
}

func TestStageCleanup(t *testing.T) {
	sentinel := errors.New("reader failed")
	for _, tc := range []struct {
		name   string
		reader io.Reader
		want   error
	}{
		{name: "discard", reader: strings.NewReader("note")},
		{name: "too large", reader: strings.NewReader("123456789"), want: ErrTooLarge},
		{name: "unsupported", reader: strings.NewReader("<svg/>"), want: ErrUnsupported},
		{name: "read error", reader: failingReader{err: sentinel}, want: sentinel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New(t.TempDir(), 8)
			staged, err := s.Stage(tc.reader)
			if !errors.Is(err, tc.want) {
				t.Fatalf("stage error = %v, want %v", err, tc.want)
			}
			if staged != nil {
				if err := staged.Close(); err != nil {
					t.Fatal(err)
				}
			}
			entries, err := os.ReadDir(s.Dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary files left: %v, %v", entries, err)
			}
		})
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

// Package docexport writes every generated doc to a local directory as
// Markdown on a schedule, so documentation survives outside the tool (and
// doubles as a lightweight secondary backup). It reuses the content the doc
// engine already renders and persists (store.DocRecord.Content) rather than
// re-rendering anything itself.
//
// Optionally (see ConfigureGit) the directory is a persistent clone of a Git
// remote: each run fetches, hard-resets to the remote branch, re-exports,
// commits the difference and pushes it (never forcing).
package docexport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// DefaultCronExpr is the cron expression the scheduled export job registers
// with by default (daily, off-hours). Operators can override it via
// config.DocExportSettings.CronExpr.
const DefaultCronExpr = "0 2 * * *"

// exportPageSize matches internal/backup's paginated doc export page size.
const exportPageSize = 1000

// Exporter renders every doc in the store to Markdown files in a target
// directory.
type Exporter struct {
	blobs              *blobstore.Store
	includeAttachments bool
	maxAttachmentBytes int64
	store              *store.Store

	git *gitTarget // nil: local-directory mode

	mu sync.Mutex // serializes runs (the Git worktree isn't concurrency-safe)
}

// NewExporter creates a new Exporter backed by s.
func NewExporter(s *store.Store) *Exporter {
	return &Exporter{store: s, blobs: blobstore.New("", 0), includeAttachments: true, maxAttachmentBytes: blobstore.DefaultMaxBytes}
}

// Result summarizes a completed export run.
type Result struct {
	Dir     string   `json:"dir"`
	Count   int      `json:"count"`
	Files   []string `json:"files"`
	Removed []string `json:"removed,omitempty"`
}

// ExportAll fetches every doc from the store and writes each as a Markdown
// file under dir, creating dir if needed. Filenames are derived from the
// doc's title (slugified) plus a short ID suffix to keep them unique and
// stable across runs even if two docs share a title. Any *.md file left
// over in dir from a previous run whose doc was renamed or deleted is
// removed, so dir always mirrors the current set of docs.
func (e *Exporter) ExportAll(ctx context.Context, dir string) (Result, error) {
	if dir == "" {
		return Result{}, fmt.Errorf("export directory must not be empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create export directory: %w", err)
	}

	docs, err := fetchAllDocs(ctx, e.store)
	if err != nil {
		return Result{}, fmt.Errorf("fetch docs: %w", err)
	}

	files := make([]string, 0, len(docs))
	seen := make(map[string]bool, len(docs))
	attachmentFiles := map[string]bool{}
	for _, d := range docs {
		name := fileName(d)
		if seen[name] {
			// Extremely unlikely (would require a colliding slug+ID
			// prefix); disambiguate rather than overwrite silently.
			name = d.ID + ".md"
		}
		seen[name] = true

		content, err := e.exportContent(ctx, dir, d.ID, d.Content, attachmentFiles)
		if err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return Result{}, fmt.Errorf("write doc %q: %w", d.ID, err)
		}
		files = append(files, name)
	}

	removed, err := pruneStale(dir, seen)
	if err != nil {
		return Result{}, fmt.Errorf("prune stale files: %w", err)
	}

	staleAttachments, err := pruneAttachmentFiles(dir, attachmentFiles)
	if err != nil {
		return Result{}, fmt.Errorf("prune stale attachments: %w", err)
	}
	removed = append(removed, staleAttachments...)
	return Result{Dir: dir, Count: len(files), Files: files, Removed: removed}, nil
}

// fetchAllDocs pages through every doc in the store, mirroring
// internal/backup.exportDocs.
func fetchAllDocs(ctx context.Context, s *store.Store) ([]store.DocRecord, error) {
	docs := []store.DocRecord{}
	for offset := 0; ; offset += exportPageSize {
		page, total, err := s.ListAllDocsWithContent(ctx, "", offset, exportPageSize)
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			return docs, nil
		}
		docs = append(docs, page...)
		if len(docs) >= total {
			return docs, nil
		}
	}
}

// generatedName matches the filenames fileName (and its collision fallback,
// a bare UUID) produces. Only these are ever pruned, so anything else an
// operator keeps next to the export (README.md, notes.md, …) survives.
var generatedName = regexp.MustCompile(`^(?:[a-z0-9-]+-[0-9a-f]{8}|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.md$`)

// IsGeneratedName reports whether name looks like a file ExportAll writes,
// i.e. one pruneStale is allowed to delete.
func IsGeneratedName(name string) bool { return generatedName.MatchString(name) }

// pruneStale removes any top-level generated doc file in dir that isn't in
// keep, leaving other files (e.g. a README the operator dropped in) untouched.
func pruneStale(dir string, keep map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !IsGeneratedName(name) || keep[name] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return removed, fmt.Errorf("remove stale file %q: %w", name, err)
		}
		removed = append(removed, name)
	}
	return removed, nil
}

// fileName derives a stable, filesystem-safe Markdown filename for a doc:
// a slug of its title, plus the first 8 characters of its ID so renamed
// titles don't collide and unrelated docs with the same title don't
// overwrite each other.
func fileName(d store.DocRecord) string {
	slug := slugify(d.Title)
	if slug == "" {
		slug = "untitled"
	}
	idSuffix := d.ID
	if len(idSuffix) > 8 {
		idSuffix = idSuffix[:8]
	}
	return fmt.Sprintf("%s-%s.md", slug, idSuffix)
}

// slugify lowercases s and replaces every run of characters that aren't
// ASCII letters, digits, or '-' with a single '-', trimming leading and
// trailing separators.
func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// RunExportOnce runs a single export pass and logs the outcome, returning
// any error so the scheduler's centralized health tracking (#384) can
// persist it and fire a system.job_failed notification on an ok<->failing
// transition — see scheduler.Runner.SetHealthTracking. It's the function
// the scheduled "docexport" cron job (wired in cmd/server/main.go) calls. In
// Git mode dir is the persistent clone and the docs go to its configured
// subdirectory.
func RunExportOnce(ctx context.Context, e *Exporter, dir string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	var err error
	if e.git != nil {
		err = e.runGit(ctx, dir, logger)
	} else {
		var res Result
		res, err = e.ExportAll(ctx, dir)
		if err == nil {
			logger.Info("doc export: completed", "dir", res.Dir, "count", res.Count, "removed", len(res.Removed))
		}
	}

	if err != nil {
		logger.Error("doc export: failed", "dir", dir, "error", err)
		return fmt.Errorf("doc export: %w", err)
	}
	return nil
}

// ConfigureAttachments applies the configured inclusion flag and Git size cap.
func (e *Exporter) ConfigureAttachments(blobs *blobstore.Store, include bool, maxBytes int64) {
	e.blobs = blobs
	e.includeAttachments = include
	if maxBytes > 0 {
		e.maxAttachmentBytes = maxBytes
	}
}

var attachmentLink = regexp.MustCompile(`attachment:([a-zA-Z0-9-]+)`)

func (e *Exporter) exportContent(ctx context.Context, dir, docID, content string, keep ...map[string]bool) (string, error) {
	content = doc.StripMarkers(content)
	if !e.includeAttachments {
		return content, nil
	}
	attachments, err := e.store.ListDocAttachments(ctx, docID)
	if err != nil {
		return "", err
	}
	paths := map[string]string{}
	for _, a := range attachments {
		if e.git != nil && a.Size > e.maxAttachmentBytes {
			continue
		}
		name := "attachments/" + a.SHA256 + blobstore.Extension(a.ContentType)
		if err := os.MkdirAll(filepath.Join(dir, "attachments"), 0o755); err != nil {
			return "", err
		}
		source, err := e.blobs.Open(a.SHA256)
		if err != nil {
			return "", fmt.Errorf("open export attachment: %w", err)
		}
		target, err := os.Create(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			_ = source.Close()
			return "", err
		}
		_, copyErr := io.Copy(target, source)
		err = errors.Join(copyErr, target.Close(), source.Close())
		if err != nil {
			return "", err
		}
		paths[a.ID] = name
		if len(keep) > 0 {
			keep[0][name] = true
		}
	}
	return attachmentLink.ReplaceAllStringFunc(content, func(link string) string {
		if path, ok := paths[strings.TrimPrefix(link, "attachment:")]; ok {
			return path
		}
		return link
	}), nil
}

var generatedAttachmentName = regexp.MustCompile(`^[a-f0-9]{64}\.(png|jpg|gif|webp|pdf|txt)$`)

func pruneAttachmentFiles(dir string, keep map[string]bool) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "attachments"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	removed := []string{}
	for _, entry := range entries {
		name := "attachments/" + entry.Name()
		if entry.IsDir() || !generatedAttachmentName.MatchString(entry.Name()) || keep[name] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil {
			return nil, err
		}
		removed = append(removed, name)
	}
	return removed, nil
}

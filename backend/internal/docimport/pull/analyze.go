package pull

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"

	"github.com/WiseLabz/wiselabz/internal/docimport"
)

// StagedSource reads the source name the manager stored before analysis.
func StagedSource(dir string) docimport.Source {
	data, err := os.ReadFile(filepath.Join(dir, "source"))
	if err == nil && docimport.Source(data) == docimport.SourceWikiJS {
		return docimport.SourceWikiJS
	}
	return docimport.SourceMarkdown
}

// IsStagedPull reports whether the server-written origin marker names a
// supported pull provider. Callers must not infer pull origin from ZIP content
// or a request's parser selection.
func IsStagedPull(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "source"))
	if err != nil {
		return false
	}
	switch docimport.Source(data) {
	case docimport.SourceBookStack, docimport.SourceWikiJS:
		return true
	default:
		return false
	}
}

// Analyze stages normalized lab-scope notes without connector inference.
func Analyze(ctx context.Context, dir string, maxAttachmentBytes int64) (*docimport.Plan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	zr, err := zip.OpenReader(docimport.UploadPath(dir))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	limits := docimport.DefaultLimits()
	if maxAttachmentBytes > 0 {
		limits.MaxAttachmentBytes = maxAttachmentBytes
	}
	var archive *docimport.Archive
	if IsStagedPull(dir) {
		archive, err = docimport.OpenPulledArchive(&zr.Reader, limits)
	} else {
		archive, err = docimport.OpenArchive(&zr.Reader, limits)
	}
	if err != nil {
		return nil, err
	}
	plan, err := docimport.AnalyzeSource(StagedSource(dir), archive, nil)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}

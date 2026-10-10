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
	archive, err := docimport.OpenArchive(&zr.Reader, limits)
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

package docs

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// ImportNode is one doc in an import preview tree.
type ImportNode struct {
	DocID           string       `json:"docId"`
	Title           string       `json:"title"`
	Path            string       `json:"path"`
	ServiceID       string       `json:"serviceId"`
	Folder          bool         `json:"folder"`
	AttachmentCount int          `json:"attachmentCount"`
	Children        []ImportNode `json:"children"`
}

// ImportCollision is a doc whose title is already used by a sibling.
type ImportCollision struct {
	Path     string `json:"path"`
	Title    string `json:"title"`
	NewTitle string `json:"newTitle"`
}

// ImportPreview is the response to staging an import.
type ImportPreview struct {
	ID              string              `json:"id"`
	ExpiresAt       time.Time           `json:"expiresAt"`
	Tree            []ImportNode        `json:"tree"`
	DocCount        int                 `json:"docCount"`
	AttachmentCount int                 `json:"attachmentCount"`
	Mappings        []docimport.Mapping `json:"mappings"`
	Warnings        []docimport.Issue   `json:"warnings"`
	Skipped         []docimport.Issue   `json:"skipped"`
	Collisions      []ImportCollision   `json:"collisions"`
}

// ImportedDoc is a doc created by an import commit.
type ImportedDoc struct {
	DocID     string `json:"docId"`
	Title     string `json:"title"`
	ParentID  string `json:"parentId"`
	ServiceID string `json:"serviceId"`
}

func (h *Handler) importStage() docimport.Stage {
	return docimport.NewStage(h.Settings.Config.Attachments.ImportDir)
}

func (h *Handler) importLimits() docimport.Limits {
	limits := docimport.DefaultLimits()
	limits.MaxAttachmentBytes = h.attachmentStore().MaxBytes
	return limits
}

func importError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, docimport.ErrTooLarge):
		httputil.Error(w, http.StatusRequestEntityTooLarge, "archive_too_large", err.Error())
	case errors.Is(err, docimport.ErrUnsafePath), errors.Is(err, docimport.ErrTooManyEntries),
		errors.Is(err, docimport.ErrCompressionRatio), errors.Is(err, docimport.ErrDuplicatePath),
		errors.Is(err, zip.ErrFormat), errors.Is(err, zip.ErrAlgorithm), errors.Is(err, zip.ErrChecksum):
		httputil.Error(w, http.StatusBadRequest, "invalid_archive", err.Error())
	default:
		httputil.Errorf(w, err)
	}
}

// StageImport handles POST /api/docs/import: it stages a Markdown/Obsidian
// vault zip or a Wiki.js export zip (multipart field source) and returns a preview of the docs it would create.
func (h *Handler) StageImport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocOperator(w, r, "") {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, docimport.MaxUploadBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_upload", "Expected multipart file upload")
		return
	}
	id := uuid.NewString()
	dir, err := h.importStage().Create(id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	staged := false
	defer func() {
		if !staged {
			_ = os.RemoveAll(dir)
		}
	}()
	// The file is copied to disk as it arrives, so the source field may come
	// before or after it.
	var source string
	gotFile := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				h.importUploadError(w, err)
			} else {
				httputil.Error(w, http.StatusBadRequest, "invalid_upload", "Expected multipart file upload")
			}
			return
		}
		switch {
		case part.FormName() == "file" && part.FileName() != "" && !gotFile:
			gotFile = true
			err = saveImportUpload(dir, part)
		case part.FormName() == "source":
			var v []byte
			v, err = io.ReadAll(io.LimitReader(part, 64))
			source = string(v)
		}
		_ = part.Close()
		if err != nil {
			h.importUploadError(w, err)
			return
		}
	}
	if !gotFile {
		httputil.Error(w, http.StatusBadRequest, "invalid_upload", "Expected file field")
		return
	}
	src, err := docimport.ParseSource(source)
	if err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid_source", "Unknown import source")
		return
	}
	plan, err := h.analyzeUpload(r.Context(), dir, src)
	if err != nil {
		h.importUploadError(w, err)
		return
	}
	plan.ID, plan.CreatedAt = id, time.Now().UTC()
	if err := docimport.SavePlan(dir, plan); err != nil {
		httputil.Errorf(w, err)
		return
	}
	preview, err := h.importPreview(r.Context(), plan)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	staged = true
	httputil.JSON(w, http.StatusCreated, preview)
}

var errUploadTooLarge = errors.New("import upload too large")

func (h *Handler) importUploadError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) || errors.Is(err, errUploadTooLarge) {
		httputil.Error(w, http.StatusRequestEntityTooLarge, "request_too_large", "Import archives are limited to 100 MB")
		return
	}
	importError(w, err)
}

func saveImportUpload(dir string, part io.Reader) error {
	f, err := os.Create(docimport.UploadPath(dir))
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(part, docimport.MaxUploadBytes+1))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if n > docimport.MaxUploadBytes {
		return errUploadTooLarge
	}
	return nil
}

func (h *Handler) analyzeUpload(ctx context.Context, dir string, src docimport.Source) (*docimport.Plan, error) {
	zr, err := zip.OpenReader(docimport.UploadPath(dir))
	if err != nil {
		return nil, err
	}
	defer func() { _ = zr.Close() }()
	archive, err := docimport.OpenArchive(&zr.Reader, h.importLimits())
	if err != nil {
		return nil, err
	}
	names, err := h.Store.ListConnectorNames(ctx)
	if err != nil {
		return nil, err
	}
	connectors := make([]docimport.Connector, len(names))
	for i, c := range names {
		connectors[i] = docimport.Connector{ID: c.ID, Name: c.Name}
	}
	return docimport.AnalyzeSource(src, archive, connectors)
}

// importPreview nests the plan and predicts title collisions; the commit
// transaction recomputes them authoritatively.
func (h *Handler) importPreview(ctx context.Context, plan *docimport.Plan) (*ImportPreview, error) {
	p := &ImportPreview{ID: plan.ID, ExpiresAt: plan.CreatedAt.Add(docimport.TTL), Tree: []ImportNode{}, DocCount: len(plan.Docs),
		AttachmentCount: plan.AttachmentCount, Mappings: plan.Mappings, Warnings: plan.Warnings, Skipped: plan.Skipped, Collisions: []ImportCollision{}}
	used := map[[3]string]bool{}
	children := map[string][]int{}
	for i, d := range plan.Docs {
		title, err := store.ImportedTitle(d.Title, func(t string) (bool, error) {
			if used[[3]string{d.ServiceID, d.ParentID, t}] {
				return true, nil
			}
			if d.ParentID != "" {
				return false, nil // parents are new docs with no other children
			}
			return h.Store.SiblingTitleTaken(ctx, d.ServiceID, "", t)
		})
		if err != nil {
			return nil, err
		}
		if title != d.Title {
			p.Collisions = append(p.Collisions, ImportCollision{Path: d.Path, Title: d.Title, NewTitle: title})
			plan.Docs[i].Title = title
		}
		used[[3]string{d.ServiceID, d.ParentID, title}] = true
		children[d.ParentID] = append(children[d.ParentID], i)
	}
	var build func(parent string) []ImportNode
	build = func(parent string) []ImportNode {
		nodes := []ImportNode{}
		for _, i := range children[parent] {
			d := plan.Docs[i]
			nodes = append(nodes, ImportNode{DocID: d.ID, Title: d.Title, Path: d.Path, ServiceID: d.ServiceID,
				Folder: d.Folder, AttachmentCount: len(d.Attachments), Children: build(d.ID)})
		}
		return nodes
	}
	p.Tree = build("")
	return p, nil
}

// CommitImport handles POST /api/docs/import/{id}/commit: it publishes the
// staged attachments and creates every doc in one transaction.
func (h *Handler) CommitImport(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocOperator(w, r, "") {
		return
	}
	id := r.PathValue("id")
	plan, dir, release, done, err := h.importStage().Claim(id, time.Now())
	if err != nil {
		httputil.Error(w, http.StatusNotFound, "not_found", "Import not found or expired")
		return
	}
	committed := false
	defer func() {
		if committed {
			done()
		} else {
			release()
		}
	}()
	for _, d := range plan.Docs {
		if d.ServiceID == "" {
			continue
		}
		if _, err := h.Store.GetConnector(r.Context(), d.ServiceID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				httputil.Error(w, http.StatusConflict, "connector_missing", "A connector named by the import no longer exists; upload the archive again")
			} else {
				httputil.Errorf(w, err)
			}
			return
		}
	}
	docs, attachments, err := h.commitImport(r.Context(), dir, plan)
	if err != nil {
		if errors.Is(err, store.ErrDocHierarchy) {
			h.lifecycleError(w, err)
			return
		}
		importError(w, err)
		return
	}
	committed = true
	if err := h.Store.RecordAuditFromContext(r.Context(), "docs.import", "doc_import", id, map[string]int{"docs": len(docs), "attachments": attachments}); err != nil {
		slog.Error("audit doc import", "importId", id, "error", err)
	}
	go func(ctx context.Context) {
		for _, d := range docs {
			h.SyncEmbeddings(ctx, d.ID, d.Content)
		}
	}(context.WithoutCancel(r.Context()))
	out := make([]ImportedDoc, len(docs))
	for i, d := range docs {
		out[i] = ImportedDoc{DocID: d.ID, Title: d.Title, ParentID: d.ParentID, ServiceID: d.ServiceID}
	}
	httputil.JSON(w, http.StatusCreated, out)
}

func (h *Handler) commitImport(ctx context.Context, dir string, plan *docimport.Plan) ([]store.DocRecord, int, error) {
	zr, err := zip.OpenReader(docimport.UploadPath(dir))
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = zr.Close() }()
	archive, err := docimport.OpenArchive(&zr.Reader, h.importLimits())
	if err != nil {
		return nil, 0, err
	}
	user := auth.UserIDFromContext(ctx)
	blobstore.PublicationMu.Lock()
	defer blobstore.PublicationMu.Unlock()
	blobs := h.attachmentStore()
	published := map[string]blobstore.Blob{}
	docs := make([]store.DocRecord, 0, len(plan.Docs))
	var attachments []store.DocAttachment
	for _, d := range plan.Docs {
		docs = append(docs, store.DocRecord{ID: d.ID, Title: d.Title, ServiceID: d.ServiceID, ParentID: d.ParentID, Content: d.Content, CreatedBy: user})
		for _, a := range d.Attachments {
			blob, ok := published[a.Path]
			if !ok {
				rc, err := archive.Open(a.Path)
				if err != nil {
					return nil, 0, err
				}
				blob, err = blobs.Put(rc)
				_ = rc.Close()
				if err != nil {
					return nil, 0, err
				}
				published[a.Path] = blob
			}
			attachments = append(attachments, store.DocAttachment{ID: a.ID, DocID: d.ID, SHA256: blob.SHA256, Filename: a.Filename,
				ContentType: blob.ContentType, Size: blob.Size, CreatedBy: user})
		}
	}
	if err := h.Store.ImportDocs(ctx, docs, attachments); err != nil {
		return nil, 0, err
	}
	return docs, len(attachments), nil
}

package docs

import (
	"errors"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func (h *Handler) attachmentStore() *blobstore.Store {
	cfg := h.Settings.Config.Attachments
	return blobstore.New(cfg.Dir, cfg.MaxBytes)
}
func (h *Handler) attachmentSigner() *blobstore.Signer {
	return blobstore.NewSigner(h.Settings.Config.Auth.Secret)
}
func (h *Handler) signedAttachments(r *http.Request, id string) ([]store.DocAttachment, error) {
	attachments, err := h.Store.ListDocAttachments(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if len(attachments) == 0 {
		return attachments, nil
	}
	signer := h.attachmentSigner()
	for i := range attachments {
		attachments[i].URL = signer.URL(attachments[i].ID, time.Now())
	}
	return attachments, nil
}
func (h *Handler) writeDoc(w http.ResponseWriter, r *http.Request, d *store.DocRecord) {
	attachments, err := h.signedAttachments(r, d.ID)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, struct {
		*store.DocRecord
		Attachments []store.DocAttachment `json:"attachments"`
	}{DocRecord: d, Attachments: attachments})
}

// ListAttachments returns doc-owned signed metadata after viewer authorization.
func (h *Handler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !h.loadDocForViewer(w, r, id) {
		return
	}
	attachments, err := h.signedAttachments(r, id)
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusOK, attachments)
}
func (h *Handler) attachmentWriteDoc(w http.ResponseWriter, r *http.Request) (*store.DocRecord, bool) {
	d, err := h.Store.GetDoc(r.Context(), r.PathValue("id"))
	if err != nil {
		httputil.HandleStoreError(w, err)
		return nil, false
	}
	if !h.requireDocEditor(w, r, d) {
		return nil, false
	}
	return d, true
}

// UploadAttachment streams an allowlisted file after doc editor authorization.
func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	d, ok := h.attachmentWriteDoc(w, r)
	if !ok {
		return
	}
	blobs := h.attachmentStore()
	r.Body = http.MaxBytesReader(w, r.Body, blobs.MaxBytes+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		httputil.Error(w, 400, "invalid_upload", "Expected multipart file upload")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" || part.FileName() == "" {
		httputil.Error(w, 400, "invalid_upload", "Expected file field")
		return
	}
	defer func() { _ = part.Close() }()
	blobstore.PublicationMu.Lock()
	defer blobstore.PublicationMu.Unlock()
	blob, err := blobs.Put(part)
	if err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.Is(err, blobstore.ErrTooLarge), errors.As(err, &maxErr):
			httputil.Error(w, 413, "request_too_large", "Attachment exceeds size limit")
		case errors.Is(err, blobstore.ErrUnsupported):
			httputil.Error(w, 415, "unsupported_content", "Unsupported attachment type")
		default:
			httputil.Errorf(w, err)
		}
		return
	}
	a := store.DocAttachment{DocID: d.ID, SHA256: blob.SHA256, Filename: filepath.Base(strings.ReplaceAll(part.FileName(), "\\", "/")),
		ContentType: blob.ContentType, Size: blob.Size, CreatedBy: auth.UserIDFromContext(r.Context())}
	// Recheck active doc inside the transaction after streaming an upload.
	err = h.Store.WithinTransaction(r.Context(), func(tx *store.Store) error {
		if _, err := tx.GetDoc(r.Context(), d.ID); err != nil {
			return err
		}
		return tx.CreateDocAttachment(r.Context(), &a)
	})
	if err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	a.URL = h.attachmentSigner().URL(a.ID, time.Now())
	httputil.JSON(w, http.StatusCreated, a)
}

// DeleteAttachment removes metadata only from the authorized owning doc.
func (h *Handler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	d, ok := h.attachmentWriteDoc(w, r)
	if !ok {
		return
	}
	if err := h.Store.DeleteDocAttachment(r.Context(), d.ID, r.PathValue("aid")); err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RawAttachment serves active-doc bytes to holders of a valid signed URL.
func (h *Handler) RawAttachment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("aid")
	if !h.attachmentSigner().Valid(id, r.URL.Query().Get("exp"), r.URL.Query().Get("sig"), time.Now()) {
		httputil.Error(w, 403, "invalid_signature", "Attachment URL is invalid or expired")
		return
	}
	a, err := h.Store.GetDocAttachment(r.Context(), id)
	if err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	if _, err := h.Store.GetDoc(r.Context(), a.DocID); err != nil {
		httputil.HandleStoreError(w, err)
		return
	}
	file, err := h.attachmentStore().Open(a.SHA256)
	if err != nil {
		httputil.Error(w, 404, "not_found", "Attachment not found")
		return
	}
	defer func() { _ = file.Close() }()
	disposition := "attachment"
	if strings.HasPrefix(a.ContentType, "image/") || a.ContentType == "application/pdf" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": a.Filename}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	// Chrome refuses to render PDFs inside a CSP-sandboxed document, so the inline
	// preview needs the sandbox dropped. PDFs are sniffed, served nosniff and never
	// parsed as HTML, so frame-ancestors alone is enough.
	csp := "sandbox; frame-ancestors 'self'"
	if a.ContentType == "application/pdf" {
		csp = "frame-ancestors 'self'"
	}
	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, a.Filename, time.Time{}, file)
}

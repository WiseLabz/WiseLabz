package docs

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/labbook"
)

// Export downloads only the caller's viewable docs as a portable Lab Book.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format != "html" && format != "md.zip" {
		httputil.Error(w, 400, "invalid_format", "Format must be html or md.zip")
		return
	}
	docs, err := labbook.Viewable(r.Context(), h.Store, auth.UserIDFromContext(r.Context()))
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	blobs := h.attachmentStore()
	book, err := labbook.Load(r.Context(), h.Store, docs, func(hash string) (io.ReadCloser, error) { return blobs.Open(hash) })
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	// Stage on disk so blob/build failures still return an HTTP error before download headers.
	f, err := os.CreateTemp("", "lab-book-*")
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	defer os.Remove(f.Name()) //nolint:errcheck
	defer f.Close()           //nolint:errcheck
	if err = book.Write(f, format); err != nil {
		httputil.Errorf(w, err)
		return
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		httputil.Errorf(w, err)
		return
	}
	mime := "text/html; charset=utf-8"
	if format == "md.zip" {
		mime = "application/zip"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", `attachment; filename="lab-book-`+time.Now().UTC().Format("2006-01-02")+`.`+format+`"`)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "lab-book."+format, time.Time{}, f)
}

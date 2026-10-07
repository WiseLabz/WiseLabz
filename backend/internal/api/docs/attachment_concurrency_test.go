package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/store"
)

type pausedAttachmentReader struct {
	prefix    *bytes.Reader
	remainder *bytes.Reader
	stalled   chan struct{}
	resume    chan struct{}
	once      sync.Once
}

func (r *pausedAttachmentReader) Read(p []byte) (int, error) {
	if r.prefix.Len() > 0 {
		return r.prefix.Read(p)
	}
	r.once.Do(func() { close(r.stalled) })
	<-r.resume
	return r.remainder.Read(p)
}

func startPausedAttachment(
	t *testing.T,
	h *Handler,
	request *http.Request,
) (*httptest.ResponseRecorder, <-chan struct{}, func()) {
	t.Helper()
	data, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	_ = request.Body.Close()
	// Give MultipartReader the complete headers and one payload byte before pausing.
	split := bytes.Index(data, []byte("\r\n\r\n")) + 5
	reader := &pausedAttachmentReader{
		prefix: bytes.NewReader(data[:split]), remainder: bytes.NewReader(data[split:]),
		stalled: make(chan struct{}), resume: make(chan struct{}),
	}
	request.Body = io.NopCloser(reader)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.UploadAttachment(response, request)
	}()
	var once sync.Once
	resume := func() { once.Do(func() { close(reader.resume) }) }
	t.Cleanup(func() {
		resume()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("paused upload did not exit during cleanup")
		}
	})
	select {
	case <-reader.stalled:
	case <-done:
		t.Fatalf("upload exited before streaming: %d %s", response.Code, response.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("upload did not reach paused body read")
	}
	return response, done, resume
}

func TestAttachmentSlowUploadAllowsUploadAndGC(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	h.Settings.Config.Auth.Secret = "test-secret"
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	user := apitest.NewUser(t, h.Store, "operator")
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "operator")
	slow, slowDone, resume := startPausedAttachment(t, h,
		asUser(uploadRequest(t, d.ID, "slow.txt", "slow attachment"), user, false))
	fast := httptest.NewRecorder()
	fastRequest := asUser(uploadRequest(t, d.ID, "fast.txt", "fast attachment"), user, false)
	fastDone := make(chan struct{})
	go func() {
		defer close(fastDone)
		h.UploadAttachment(fast, fastRequest)
	}()
	t.Cleanup(func() {
		resume()
		select {
		case <-fastDone:
		case <-time.After(5 * time.Second):
			t.Error("fast upload did not exit during cleanup")
		}
	})
	select {
	case <-fastDone:
	case <-time.After(5 * time.Second):
		t.Fatal("fast upload blocked behind the paused upload")
	}
	if fast.Code != http.StatusCreated {
		t.Fatalf("fast upload: %d %s", fast.Code, fast.Body.String())
	}
	gcDone := make(chan struct{})
	var gcErr error
	go func() {
		defer close(gcDone)
		blobstore.PublicationMu.Lock()
		defer blobstore.PublicationMu.Unlock()
		gcErr = h.attachmentStore().Sweep(func(hash string) (bool, error) {
			return h.Store.BlobReferenced(context.Background(), hash)
		})
	}()
	t.Cleanup(func() {
		resume()
		select {
		case <-gcDone:
		case <-time.After(5 * time.Second):
			t.Error("attachment GC did not exit during cleanup")
		}
	})
	select {
	case <-gcDone:
		if gcErr != nil {
			t.Fatal(gcErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("attachment GC blocked behind the paused upload")
	}
	resume()
	select {
	case <-slowDone:
	case <-time.After(5 * time.Second):
		t.Fatal("resumed upload did not finish")
	}
	if slow.Code != http.StatusCreated {
		t.Fatalf("slow upload: %d %s", slow.Code, slow.Body.String())
	}
	for _, result := range []struct {
		response *httptest.ResponseRecorder
		content  string
	}{
		{response: fast, content: "fast attachment"},
		{response: slow, content: "slow attachment"},
	} {
		var attachment store.DocAttachment
		if err := json.Unmarshal(result.response.Body.Bytes(), &attachment); err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, attachment.URL, nil)
		request.SetPathValue("aid", attachment.ID)
		response := httptest.NewRecorder()
		h.RawAttachment(response, request)
		if response.Code != http.StatusOK || response.Body.String() != result.content {
			t.Fatalf("raw attachment after GC: %d %q", response.Code, response.Body.String())
		}
	}
}

func TestAttachmentDocDeletedWhileUploadStalled(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	user := apitest.NewUser(t, h.Store, "operator")
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "operator")
	response, done, resume := startPausedAttachment(t, h,
		asUser(uploadRequest(t, d.ID, "note.txt", "deleted attachment"), user, false))
	if err := h.Store.SoftDeleteDoc(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	resume()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("resumed upload did not finish")
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("deleted doc upload: %d %s", response.Code, response.Body.String())
	}
	attachments, err := h.Store.ListDocAttachments(context.Background(), d.ID)
	if err != nil || len(attachments) != 0 {
		t.Fatalf("deleted doc attachments: %+v, %v", attachments, err)
	}
	files, err := filepath.Glob(filepath.Join(h.Settings.Config.Attachments.Dir, ".upload-*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary upload files: %v, %v", files, err)
	}
}

package docs

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func uploadRequest(t *testing.T, docID, filename, data string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/docs/"+docID+"/attachments", &body)
	r.SetPathValue("id", docID)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	return r
}

func TestAttachmentACLAndServing(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	h.Settings.Config.Auth.Secret = "test-secret"
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	user := apitest.NewUser(t, h.Store, "viewer")
	upload := func(admin bool) *httptest.ResponseRecorder {
		rr := httptest.NewRecorder()
		h.UploadAttachment(rr, asUser(uploadRequest(t, d.ID, "photo.png", "%PDF-1.7\nhello"), user, admin))
		return rr
	}
	if rr := upload(false); rr.Code != 404 {
		t.Fatalf("hidden status %d %s", rr.Code, rr.Body.String())
	}
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "viewer")
	if rr := upload(false); rr.Code != 403 {
		t.Fatalf("viewer status %d", rr.Code)
	}
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "operator")
	rr := upload(false)
	if rr.Code != 201 {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var a store.DocAttachment
	if err := json.Unmarshal(rr.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.ContentType != "application/pdf" || a.CreatedBy != user {
		t.Fatalf("metadata %+v", a)
	}
	r := httptest.NewRequest("GET", a.URL, nil)
	r.SetPathValue("aid", a.ID)
	rr = httptest.NewRecorder()
	h.RawAttachment(rr, r)
	if rr.Code != 200 || rr.Body.String() != "%PDF-1.7\nhello" {
		t.Fatalf("raw %d %s", rr.Code, rr.Body.String())
	}
	for key, want := range map[string]string{"X-Content-Type-Options": "nosniff", "Content-Security-Policy": "frame-ancestors 'self'", "Cache-Control": "private, no-store"} {
		if rr.Header().Get(key) != want {
			t.Fatalf("%s=%s", key, rr.Header().Get(key))
		}
	}
	if !strings.HasPrefix(rr.Header().Get("Content-Disposition"), "inline") {
		t.Fatal("PDF not inline")
	}
	q := r.URL.Query()
	q.Set("sig", "tampered")
	r.URL.RawQuery = q.Encode()
	rr = httptest.NewRecorder()
	h.RawAttachment(rr, r)
	if rr.Code != 403 {
		t.Fatal(rr.Code)
	}
	r = httptest.NewRequest("GET", a.URL, nil)
	r.SetPathValue("aid", a.ID)
	if err := h.Store.SoftDeleteDoc(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	rr = httptest.NewRecorder()
	h.RawAttachment(rr, r)
	if rr.Code != 404 {
		t.Fatal(rr.Code)
	}
}

func TestAttachmentLabOriginACLAndLimits(t *testing.T) {
	for _, origin := range []string{store.DocOriginHuman, store.DocOriginGenerated} {
		t.Run(origin, func(t *testing.T) {
			h := newTestHandler(t)
			h.Settings.Config.Attachments.Dir = t.TempDir()
			h.Settings.Config.Attachments.MaxBytes = 8
			d := &store.DocRecord{Title: "lab", Kind: "lab", Origin: origin}
			if err := h.Store.CreateDoc(context.Background(), d); err != nil {
				t.Fatal(err)
			}
			user := apitest.NewUser(t, h.Store, "viewer")
			r := httptest.NewRequest("GET", "/", nil)
			r.SetPathValue("id", d.ID)
			r = asUser(r, user, false)
			rr := httptest.NewRecorder()
			h.ListAttachments(rr, r)
			want := 200
			if origin == store.DocOriginGenerated {
				want = 404
			}
			if rr.Code != want {
				t.Fatal(rr.Code)
			}
			rr = httptest.NewRecorder()
			h.UploadAttachment(rr, asUser(uploadRequest(t, d.ID, "x.txt", "ok"), user, false))
			want = 403
			if origin == store.DocOriginGenerated {
				want = 404
			}
			if rr.Code != want {
				t.Fatal(rr.Code)
			}
			for _, tc := range []struct {
				data   string
				status int
			}{{"123456789", 413}, {"<svg/>", 415}, {"hello", 201}} {
				rr = httptest.NewRecorder()
				h.UploadAttachment(rr, asUser(uploadRequest(t, d.ID, "photo.png", tc.data), user, true))
				if rr.Code != tc.status {
					t.Fatalf("%d %s", rr.Code, rr.Body.String())
				}
			}
		})
	}
}

func TestShareAttachmentScope(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Auth.Secret = "secret"
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	other := seedDoc(t, h.Store, conn.ID)
	for _, owner := range []*store.DocRecord{d, other} {
		a := &store.DocAttachment{DocID: owner.ID, SHA256: strings.Repeat("a", 64), Filename: "note.txt", ContentType: "text/plain; charset=utf-8", Size: 4}
		if err := h.Store.CreateDocAttachment(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	scope := &shareLinkScope{node: shareLinkNode{kind: "doc", docID: d.ID}}
	for _, id := range []string{d.ID, other.ID} {
		r := httptest.NewRequest("GET", "/", nil)
		r.SetPathValue("docId", id)
		r = r.WithContext(context.WithValue(r.Context(), shareLinkContextKey{}, scope))
		rr := httptest.NewRecorder()
		h.ShareLinkDoc(rr, r)
		if id == other.ID {
			if rr.Code != 404 {
				t.Fatal(rr.Code)
			}
			continue
		}
		var response struct {
			Attachments []store.DocAttachment `json:"attachments"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Attachments) != 1 || response.Attachments[0].DocID != d.ID {
			t.Fatalf("leaked scope %+v", response)
		}
		a := response.Attachments[0]
		u, err := url.Parse(a.URL)
		if err != nil {
			t.Fatal(err)
		}
		if !blobstore.NewSigner("secret").Valid(a.ID, u.Query().Get("exp"), u.Query().Get("sig"), time.Now()) {
			t.Fatal("invalid signed URL")
		}
	}
}

func TestAttachmentDeleteOwnershipAndPurgeGC(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	ctx := context.Background()
	if _, err := h.Store.DB().ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	other := seedDoc(t, h.Store, conn.ID)
	user := apitest.NewUser(t, h.Store, "viewer")
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "operator")
	b, err := h.attachmentStore().Put(strings.NewReader("shared bytes"))
	if err != nil {
		t.Fatal(err)
	}
	a := &store.DocAttachment{DocID: d.ID, SHA256: b.SHA256, Filename: "note.txt", ContentType: b.ContentType, Size: b.Size}
	if err := h.Store.CreateDocAttachment(ctx, a); err != nil {
		t.Fatal(err)
	}
	second := *a
	second.ID = ""
	second.DocID = other.ID
	if err := h.Store.CreateDocAttachment(ctx, &second); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("DELETE", "/", nil)
	r.SetPathValue("id", other.ID)
	r.SetPathValue("aid", a.ID)
	r = asUser(r, user, false)
	rr := httptest.NewRecorder()
	h.DeleteAttachment(rr, r)
	if rr.Code != 404 {
		t.Fatalf("wrong owning doc: %d", rr.Code)
	}
	r.SetPathValue("id", d.ID)
	rr = httptest.NewRecorder()
	h.DeleteAttachment(rr, r)
	if rr.Code != 204 {
		t.Fatalf("delete %d %s", rr.Code, rr.Body.String())
	}
	if err := h.Store.SoftDeleteDoc(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	sweep := func() {
		t.Helper()
		if err := h.attachmentStore().Sweep(func(hash string) (bool, error) { return h.Store.BlobReferenced(ctx, hash) }); err != nil {
			t.Fatal(err)
		}
	}
	sweep()
	f, err := h.attachmentStore().Open(b.SHA256)
	if err != nil {
		t.Fatalf("trash reference lost: %v", err)
	}
	_ = f.Close()
	if _, err := h.Store.PurgeDeletedDocs(ctx, time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.GetDocAttachment(ctx, second.ID); err == nil {
		t.Fatal("attachment metadata did not cascade")
	}
	sweep()
	if f, err := h.attachmentStore().Open(b.SHA256); err == nil {
		_ = f.Close()
		t.Fatal("orphan survived GC")
	}
}

func TestRawAttachmentSandboxesNonPDF(t *testing.T) {
	h := newTestHandler(t)
	h.Settings.Config.Attachments.Dir = t.TempDir()
	h.Settings.Config.Auth.Secret = "test-secret"
	conn := seedConnector(t, h.Store)
	d := seedDoc(t, h.Store, conn.ID)
	user := apitest.NewUser(t, h.Store, "operator")
	apitest.GrantConnectorRole(t, h.Store, user, conn.ID, "operator")
	rr := httptest.NewRecorder()
	h.UploadAttachment(rr, asUser(uploadRequest(t, d.ID, "note.txt", "hello"), user, false))
	if rr.Code != 201 {
		t.Fatalf("upload %d %s", rr.Code, rr.Body.String())
	}
	var a store.DocAttachment
	if err := json.Unmarshal(rr.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", a.URL, nil)
	r.SetPathValue("aid", a.ID)
	rr = httptest.NewRecorder()
	h.RawAttachment(rr, r)
	if got := rr.Header().Get("Content-Security-Policy"); got != "sandbox; frame-ancestors 'self'" {
		t.Fatalf("csp %q", got)
	}
	if got := rr.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Fatalf("disposition %q", got)
	}
}

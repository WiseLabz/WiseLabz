package docs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/api/apitest"
	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/store"
)

func TestDocLifecycleAuthzMatrix(t *testing.T) {
	for _, scope := range []string{"lab-human", "lab-generated", "service"} {
		for _, role := range []string{"admin", "viewer", "operator", "none"} {
			t.Run(scope+"/"+role, func(t *testing.T) {
				h := newTestHandler(t)
				ctx := context.Background()
				conn := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "http://example.com"}
				if err := h.Store.CreateConnector(ctx, conn); err != nil {
					t.Fatal(err)
				}
				d := &store.DocRecord{Title: "private nebula", Content: "body", Origin: store.DocOriginHuman}
				if scope == "lab-generated" {
					d.Origin = store.DocOriginGenerated
				}
				if scope == "service" {
					d.ServiceID = conn.ID
					d.Kind = "service"
				}
				if err := h.Store.CreateDoc(ctx, d); err != nil {
					t.Fatal(err)
				}
				user := apitest.NewUser(t, h.Store, "viewer")
				if scope == "service" && (role == "operator" || role == "viewer") {
					apitest.GrantConnectorRole(t, h.Store, user, conn.ID, role)
				}
				caller := auth.ContextWithUser(ctx, user, role == "admin")
				call := func(method, body string, fn http.HandlerFunc) *httptest.ResponseRecorder {
					r := httptest.NewRequest(method, "/api/docs/"+d.ID, strings.NewReader(body)).WithContext(caller)
					r.SetPathValue("id", d.ID)
					rr := httptest.NewRecorder()
					fn(rr, r)
					return rr
				}
				canRead := scope == "lab-human" || role == "admin" && scope == "lab-generated" || scope == "service" && (role == "viewer" || role == "operator")
				canWrite := scope != "service" && role == "admin" || scope == "service" && role == "operator"
				rr := call("GET", "", h.Get)
				expected := 404
				if canRead {
					expected = 200
				}
				if rr.Code != expected {
					t.Fatalf("read: %d want %d %s", rr.Code, expected, rr.Body.String())
				}
				rr = call("GET", "", h.Tree)
				if strings.Contains(rr.Body.String(), d.ID) != canRead {
					t.Fatalf("tree leak/filter: %s", rr.Body.String())
				}
				for _, mutation := range []struct {
					method, body string
					fn           http.HandlerFunc
					success      int
				}{
					{"PATCH", `{"title":"renamed"}`, h.Patch, 200},
					{"PUT", `{"content":"edit"}`, h.Save, 200},
					{"DELETE", "", h.Delete, 204},
				} {
					rr = call(mutation.method, mutation.body, mutation.fn)
					expected = 403
					if canWrite {
						expected = mutation.success
					} else if !canRead && mutation.method != "PUT" {
						// PATCH/DELETE must not confirm that a hidden doc exists.
						expected = 404
					}
					if rr.Code != expected {
						t.Fatalf("%s: %d want %d %s", mutation.method, rr.Code, expected, rr.Body.String())
					}
				}
			})
		}
	}
}

func TestCreateNestDeleteRestore(t *testing.T) {
	h := newTestHandler(t)
	user := apitest.NewUser(t, h.Store, "viewer")
	ctx := auth.ContextWithUser(context.Background(), user, true)
	call := func(method, path, body string, fn http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/docs/"+path, strings.NewReader(body)).WithContext(ctx)
		r.SetPathValue("id", path)
		rr := httptest.NewRecorder()
		fn(rr, r)
		return rr
	}
	rr := call("POST", "", `{"title":"Handbook","content":"intro"}`, h.Create)
	if rr.Code != 201 {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var root store.DocRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &root); err != nil {
		t.Fatal(err)
	}
	if root.Origin != store.DocOriginHuman || root.CreatedBy != user || root.Kind != "lab" {
		t.Fatalf("metadata: %+v", root)
	}
	rr = call("POST", "", `{"title":"Child","parentId":"`+root.ID+`"}`, h.Create)
	if rr.Code != 201 {
		t.Fatalf("child: %d %s", rr.Code, rr.Body.String())
	}
	var child store.DocRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &child); err != nil {
		t.Fatal(err)
	}
	rr = call("PATCH", root.ID, `{"parentId":"`+child.ID+`"}`, h.Patch)
	if rr.Code != 400 {
		t.Fatalf("cycle: %d %s", rr.Code, rr.Body.String())
	}
	rr = call("GET", "", "", h.Tree)
	var tree DocTreeNode
	if err := json.Unmarshal(rr.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	if len(tree.Children) != 1 || len(tree.Children[0].Children) != 1 || len(tree.Children[0].Children[0].Children) != 1 {
		t.Fatalf("tree: %s", rr.Body.String())
	}
	rr = call("DELETE", root.ID, "", h.Delete)
	if rr.Code != 204 {
		t.Fatal(rr.Body.String())
	}
	for _, id := range []string{root.ID, child.ID} {
		rr = call("GET", id, "", h.Get)
		if rr.Code != 404 {
			t.Fatalf("deleted get %s: %d", id, rr.Code)
		}
	}
	rr = call("GET", "trash", "", h.Trash)
	var trash []store.DocRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &trash); err != nil || len(trash) != 2 {
		t.Fatalf("trash: %s %v", rr.Body.String(), err)
	}
	rr = call("POST", root.ID, "", h.RestoreDeleted)
	if rr.Code != 200 {
		t.Fatalf("restore: %d %s", rr.Code, rr.Body.String())
	}
	rr = call("GET", child.ID, "", h.Get)
	if rr.Code != 200 {
		t.Fatalf("restored child: %d", rr.Code)
	}
	ordinary := auth.ContextWithUser(context.Background(), user, false)
	r := httptest.NewRequest("GET", "/api/docs/trash", nil).WithContext(ordinary)
	rr = httptest.NewRecorder()
	h.Trash(rr, r)
	if rr.Code != 403 {
		t.Fatalf("ordinary trash: %d", rr.Code)
	}
	r = httptest.NewRequest("POST", "/api/docs/restore", nil).WithContext(ordinary)
	r.SetPathValue("id", root.ID)
	rr = httptest.NewRecorder()
	h.RestoreDeleted(rr, r)
	if rr.Code != 403 {
		t.Fatalf("ordinary restore: %d", rr.Code)
	}
}

func TestCreateDocAuthz(t *testing.T) {
	h := newTestHandler(t)
	conn := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "http://example.com"}
	if err := h.Store.CreateConnector(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"admin", "viewer", "operator", "none"} {
		for _, scope := range []string{"", "service"} {
			t.Run(role+scope, func(t *testing.T) {
				user := apitest.NewUser(t, h.Store, "viewer")
				if role == "viewer" || role == "operator" {
					apitest.GrantConnectorRole(t, h.Store, user, conn.ID, role)
				}
				body := `{"title":"Note"}`
				if scope != "" {
					body = `{"title":"Note","serviceId":"` + conn.ID + `"}`
				}
				r := httptest.NewRequest("POST", "/api/docs", strings.NewReader(body)).WithContext(auth.ContextWithUser(context.Background(), user, role == "admin"))
				rr := httptest.NewRecorder()
				h.Create(rr, r)
				want := 403
				if scope == "" && role == "admin" || scope != "" && role == "operator" {
					want = 201
				}
				if rr.Code != want {
					t.Fatalf("create: %d want %d %s", rr.Code, want, rr.Body.String())
				}
			})
		}
	}
}

func TestNestedDocNodesKeepsOrderAndNesting(t *testing.T) {
	docs := []store.DocRecord{
		{ID: "c2", Title: "C2", ParentID: "a"},
		{ID: "a", Title: "A"},
		{ID: "orphan", Title: "Orphan", ParentID: "hidden"},
		{ID: "c1", Title: "C1", ParentID: "a"},
		{ID: "g", Title: "G", ParentID: "c1"},
		{ID: "b", Title: "B"},
	}
	got := nestedDocNodes(docs, "")
	want := []DocTreeNode{
		{ID: "a", Title: "A", Children: []DocTreeNode{
			{ID: "c2", Title: "C2", ParentID: "a", Children: []DocTreeNode{}},
			{ID: "c1", Title: "C1", ParentID: "a", Children: []DocTreeNode{
				{ID: "g", Title: "G", ParentID: "c1", Children: []DocTreeNode{}},
			}},
		}},
		{ID: "orphan", Title: "Orphan", ParentID: "hidden", Children: []DocTreeNode{}},
		{ID: "b", Title: "B", Children: []DocTreeNode{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nesting:\n got %+v\nwant %+v", got, want)
	}
	if sub := nestedDocNodes(docs, "c1"); len(sub) != 1 || sub[0].ID != "g" {
		t.Fatalf("subtree: %+v", sub)
	}
}

func TestPatchParentInOtherScopeLooksMissing(t *testing.T) {
	h := newTestHandler(t)
	ctx := context.Background()
	conn := &store.ConnectorRecord{Name: "svc", Category: "virtualization", Type: "proxmox", URL: "http://example.com"}
	if err := h.Store.CreateConnector(ctx, conn); err != nil {
		t.Fatal(err)
	}
	hidden := &store.DocRecord{Title: "hidden", ServiceID: conn.ID, Kind: "service", Origin: store.DocOriginHuman}
	note := &store.DocRecord{Title: "note", Origin: store.DocOriginHuman}
	for _, d := range []*store.DocRecord{hidden, note} {
		if err := h.Store.CreateDoc(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	admin := auth.ContextWithUser(ctx, apitest.NewUser(t, h.Store, "viewer"), true)
	patch := func(parent string) string {
		r := httptest.NewRequest("PATCH", "/api/docs/"+note.ID, strings.NewReader(`{"parentId":"`+parent+`"}`)).WithContext(admin)
		r.SetPathValue("id", note.ID)
		rr := httptest.NewRecorder()
		h.Patch(rr, r)
		if rr.Code != 400 {
			t.Fatalf("patch %s: %d %s", parent, rr.Code, rr.Body.String())
		}
		return rr.Body.String()
	}
	if cross, missing := patch(hidden.ID), patch("does-not-exist"); cross != missing {
		t.Fatalf("cross-scope parent distinguishable from missing:\n%s\n%s", cross, missing)
	}
}

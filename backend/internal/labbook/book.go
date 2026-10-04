// Package labbook writes portable documentation for downloads and reports.
package labbook

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
	"github.com/WiseLabz/wiselabz/internal/doc"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

//go:embed book.html.tmpl assets/mermaid.min.js
var assets embed.FS

var bookTemplate = template.Must(template.ParseFS(assets, "book.html.tmpl"))
var attachmentLink = regexp.MustCompile(`attachment:([a-zA-Z0-9-]+)`)
var docLink = regexp.MustCompile(`/docs/([a-zA-Z0-9-]+)`)

// Book contains an already-authorized doc set. OpenBlob opens immutable attachment bytes.
type Book struct {
	Docs        []store.DocRecord
	Attachments map[string][]store.DocAttachment
	Connectors  map[string]string
	OpenBlob    func(string) (io.ReadCloser, error)
}

// Load resolves metadata only for the supplied docs; it never widens their scope.
func Load(ctx context.Context, s *store.Store, docs []store.DocRecord, open func(string) (io.ReadCloser, error)) (*Book, error) {
	b := &Book{Docs: docs, Attachments: map[string][]store.DocAttachment{}, Connectors: map[string]string{}, OpenBlob: open}
	names, err := s.ListConnectorNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range names {
		b.Connectors[c.ID] = c.Name
	}
	for _, d := range docs {
		a, err := s.ListDocAttachments(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		b.Attachments[d.ID] = a
	}
	return b, nil
}

// Viewable pages the same ACL-filtered content used by the docs API.
func Viewable(ctx context.Context, s *store.Store, userID string) ([]store.DocRecord, error) {
	var docs []store.DocRecord
	for offset := 0; ; offset += 1000 {
		page, total, err := s.ListViewableDocsWithContent(ctx, userID, "", offset, 1000)
		if err != nil {
			return nil, err
		}
		docs = append(docs, page...)
		if len(page) == 0 || len(docs) >= total {
			return docs, nil
		}
	}
}

// ReportDocs includes selected connectors plus lab-wide docs, or everything for an empty filter.
func ReportDocs(ctx context.Context, s *store.Store, ids []string) ([]store.DocRecord, error) {
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	var docs []store.DocRecord
	for offset := 0; ; offset += 1000 {
		page, total, err := s.ListAllDocsWithContent(ctx, "", offset, 1000)
		if err != nil {
			return nil, err
		}
		for _, d := range page {
			if len(ids) == 0 || d.ServiceID == "" || selected[d.ServiceID] {
				docs = append(docs, d)
			}
		}
		if len(page) == 0 || offset+len(page) >= total {
			return docs, nil
		}
	}
}

type node struct {
	ID, Title string
	Doc       store.DocRecord
	Children  []*node
	HTML      template.HTML
	Files     []string
}

func anchor(id string) string { return "doc-" + hex.EncodeToString([]byte(id)) }
func component(title, id string) string {
	var b strings.Builder
	for _, r := range title {
		if b.Len() >= 100 {
			break
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		name = "doc"
	}
	return name + "-" + hex.EncodeToString([]byte(id))
}
func (b *Book) tree() []*node {
	groups := map[string]*node{"": {Title: "Lab"}}
	nodes := map[string]*node{}
	for _, d := range b.Docs {
		nodes[d.ID] = &node{ID: anchor(d.ID), Title: d.Title, Doc: d}
	}
	for _, d := range b.Docs {
		n := nodes[d.ID]
		if parent := nodes[d.ParentID]; parent != nil && parent.Doc.ServiceID == d.ServiceID {
			parent.Children = append(parent.Children, n)
			continue
		}
		if groups[d.ServiceID] == nil {
			name := b.Connectors[d.ServiceID]
			if name == "" {
				name = d.ServiceID
			}
			groups[d.ServiceID] = &node{Title: name}
		}
		groups[d.ServiceID].Children = append(groups[d.ServiceID].Children, n)
	}
	roots := []*node{groups[""]}
	for id, n := range groups {
		if id != "" {
			roots = append(roots, n)
		}
	}
	sort.Slice(roots[1:], func(i, j int) bool { return roots[i+1].Title < roots[j+1].Title })
	var sortChildren func(*node)
	sortChildren = func(n *node) {
		sort.Slice(n.Children, func(i, j int) bool {
			a, c := n.Children[i], n.Children[j]
			if a.Title == c.Title {
				return a.ID < c.ID
			}
			return a.Title < c.Title
		})
		for _, c := range n.Children {
			sortChildren(c)
		}
	}
	for _, n := range roots {
		sortChildren(n)
	}
	return roots
}

// Write selects a supported portable format and writes it to w.
func (b *Book) Write(w io.Writer, format string) error {
	switch format {
	case "html":
		return b.writeHTML(w)
	case "md.zip":
		return b.writeZip(w)
	default:
		return fmt.Errorf("unsupported lab book format %q", format)
	}
}
func (b *Book) readBlob(a store.DocAttachment) ([]byte, error) {
	if b.OpenBlob == nil {
		return nil, errors.New("attachment blob opener is missing")
	}
	r, err := b.OpenBlob(a.SHA256)
	if err != nil {
		return nil, fmt.Errorf("open attachment %q: %w", a.Filename, err)
	}
	data, err := io.ReadAll(r)
	return data, errors.Join(err, r.Close())
}
func (b *Book) writeHTML(w io.Writer) error {
	nodes := b.tree()
	ids := map[string]bool{}
	for _, d := range b.Docs {
		ids[d.ID] = true
	}
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	var render func(*node) error
	render = func(n *node) error {
		if n.ID != "" {
			attachments := map[string]store.DocAttachment{}
			for _, a := range b.Attachments[n.Doc.ID] {
				attachments[a.ID] = a
				if !strings.HasPrefix(a.ContentType, "image/") {
					n.Files = append(n.Files, a.Filename)
				}
			}
			source := []byte(doc.StripMarkers(n.Doc.Content))
			tree := md.Parser().Parse(text.NewReader(source))
			err := ast.Walk(tree, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
				if !entering {
					return ast.WalkContinue, nil
				}
				switch v := current.(type) {
				case *ast.Image:
					a, ok := attachments[strings.TrimPrefix(string(v.Destination), "attachment:")]
					if !ok || !strings.HasPrefix(a.ContentType, "image/") {
						label := nodeText(v, source)
						if ok {
							label = a.Filename
						}
						current.Parent().ReplaceChild(current.Parent(), current, ast.NewString([]byte(label)))
						return ast.WalkSkipChildren, nil
					}
					data, err := b.readBlob(a)
					if err != nil {
						return ast.WalkStop, err
					}
					v.Destination = []byte("data:" + a.ContentType + ";base64," + base64.StdEncoding.EncodeToString(data))
				case *ast.Link:
					dest := string(v.Destination)
					if strings.HasPrefix(dest, "attachment:") {
						a, ok := attachments[strings.TrimPrefix(dest, "attachment:")]
						label := nodeText(v, source)
						if ok {
							label = a.Filename
						}
						current.Parent().ReplaceChild(current.Parent(), current, ast.NewString([]byte(label)))
						return ast.WalkSkipChildren, nil
					}
					if strings.HasPrefix(dest, "/docs/") {
						id, _, _ := strings.Cut(strings.TrimPrefix(dest, "/docs/"), "#")
						if ids[id] {
							v.Destination = []byte("#" + anchor(id))
						} else {
							v.Destination = nil
						}
					}
				}
				return ast.WalkContinue, nil
			})
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			if err := md.Renderer().Render(&buf, source, tree); err != nil {
				return err
			}
			n.HTML = template.HTML(buf.String()) // #nosec G203 -- Goldmark omits raw HTML and rejects unsafe URLs.
		}
		for _, c := range n.Children {
			if err := render(c); err != nil {
				return err
			}
		}
		return nil
	}
	for _, n := range nodes {
		if err := render(n); err != nil {
			return err
		}
	}
	js, err := assets.ReadFile("assets/mermaid.min.js")
	if err != nil {
		return err
	}
	return bookTemplate.Execute(w, struct {
		Nodes   []*node
		Mermaid template.JS
	}{nodes, template.JS(strings.ReplaceAll(string(js), "</script", "<\\/script"))}) // #nosec G203 -- embedded, vendored script only.
}
func (b *Book) writeZip(w io.Writer) error {
	z := zip.NewWriter(w)
	paths := map[string]string{}
	nodes := b.tree()
	var assign func(*node, string)
	assign = func(n *node, dir string) {
		if n.ID != "" {
			dir = path.Join(dir, component(n.Title, n.Doc.ID))
			paths[n.Doc.ID] = path.Join(dir, "index.md")
		}
		for _, c := range n.Children {
			assign(c, dir)
		}
	}
	// Scope lives in front matter; connector grouping is HTML-only to avoid synthetic imported parents.
	for _, group := range nodes {
		for _, n := range group.Children {
			assign(n, "")
		}
	}
	for _, d := range b.Docs {
		target := paths[d.ID]
		dir := path.Dir(target)
		replacements := map[string]string{}
		for _, a := range b.Attachments[d.ID] {
			name := component(strings.TrimSuffix(a.Filename, path.Ext(a.Filename)), a.ID) + blobstore.Extension(a.ContentType)
			ap := path.Join(dir, "attachments", name)
			data, err := b.readBlob(a)
			if err != nil {
				return errors.Join(err, z.Close())
			}
			f, err := z.Create(ap)
			if err != nil {
				return errors.Join(err, z.Close())
			}
			if _, err = f.Write(data); err != nil {
				return errors.Join(err, z.Close())
			}
			replacements[a.ID] = "attachments/" + name
		}
		content := attachmentLink.ReplaceAllStringFunc(doc.StripMarkers(d.Content), func(s string) string {
			if p := replacements[strings.TrimPrefix(s, "attachment:")]; p != "" {
				return p
			}
			return s
		})
		content = docLink.ReplaceAllStringFunc(content, func(s string) string {
			if p := paths[strings.TrimPrefix(s, "/docs/")]; p != "" {
				return relative(dir, p)
			}
			return s
		})
		title, _ := json.Marshal(d.Title)
		scope, _ := json.Marshal(d.ServiceID)
		content = fmt.Sprintf("---\ntitle: %s\nconnector: %s\n---\n\n%s", title, scope, content)
		f, err := z.Create(target)
		if err != nil {
			return errors.Join(err, z.Close())
		}
		if _, err = f.Write([]byte(content)); err != nil {
			return errors.Join(err, z.Close())
		}
	}
	return z.Close()
}
func relative(from, to string) string {
	a, c := strings.Split(from, "/"), strings.Split(to, "/")
	if from == "." {
		a = nil
	}
	for len(a) > 0 && len(c) > 0 && a[0] == c[0] {
		a = a[1:]
		c = c[1:]
	}
	return strings.Repeat("../", len(a)) + strings.Join(c, "/")
}

func nodeText(node ast.Node, source []byte) string {
	var out strings.Builder
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			switch v := n.(type) {
			case *ast.Text:
				out.Write(v.Value(source))
			case *ast.String:
				out.Write(v.Value)
			}
		}
		return ast.WalkContinue, nil
	})
	return out.String()
}

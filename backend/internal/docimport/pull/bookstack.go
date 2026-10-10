package pull

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/docimport"
	"github.com/WiseLabz/wiselabz/internal/docimport/htmlmd"
)

// BookStackMinVersion is the first release with portable ZIP API exports.
// https://www.bookstackapp.com/blog/bookstack-release-v25-07/
// v24.12 introduced the format but did not include API export endpoints.
const BookStackMinVersion = "v25.07"

// BookStack fetches readable content through portable ZIP API exports.
type BookStack struct {
	client remoteClient
	limits docimport.Limits
}

// NewBookStack validates the URL and constructs a guarded, verified-by-default source.
func NewBookStack(rawURL, tokenID, tokenSecret string, skipTLS bool, limits docimport.Limits) (*BookStack, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}
	if tokenID == "" || tokenSecret == "" || strings.ContainsAny(tokenID+tokenSecret, "\r\n") {
		return nil, errors.New("BookStack token ID and token secret are required")
	}
	return &BookStack{client: remoteClient{base: u,
		http:          newHTTPClient(skipTLS),
		authorization: "Token " + tokenID + ":" + tokenSecret}, limits: limits}, nil
}

type entity struct {
	ID              int      `json:"id"`
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	BookID          int      `json:"book_id"`
	Description     string   `json:"description"`
	DescriptionHTML string   `json:"description_html"`
	Books           []entity `json:"books"`
}

type portableNode struct {
	ID              int            `json:"id"`
	Name            string         `json:"name"`
	DescriptionHTML string         `json:"description_html"`
	Markdown        string         `json:"markdown"`
	HTML            string         `json:"html"`
	Priority        int            `json:"priority"`
	Chapters        []portableNode `json:"chapters"`
	Pages           []portableNode `json:"pages"`
	Images          []portableFile `json:"images"`
	Attachments     []portableFile `json:"attachments"`
}

type portableFile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
	Link string `json:"link"`
}

type outputNote struct{ name, title, body string }
type bookMapper struct {
	notes     []outputNote
	files     map[string][]byte
	targets   map[string]string
	linkRefs  map[string]bool
	siteLinks map[string]string
	warnings  []docimport.Issue
	entries   int
	bytes     int64
	limits    docimport.Limits
	base      *url.URL
}

// list uses upstream count/offset pagination and rejects inconsistent results.
func (b *BookStack) list(ctx context.Context, kind string) ([]entity, error) {
	out := []entity{}
	for {
		raw, err := b.client.get(ctx, fmt.Sprintf("/api/%s?count=500&offset=%d&sort=%%2Bid", kind, len(out)), 10<<20)
		if err != nil {
			return nil, err
		}
		var result struct {
			Data  []entity `json:"data"`
			Total int      `json:"total"`
		}
		if json.Unmarshal(raw, &result) != nil {
			return nil, errors.New("invalid BookStack listing response")
		}
		if result.Total > b.limits.MaxEntries || len(out)+len(result.Data) > b.limits.MaxEntries {
			return nil, docimport.ErrTooManyEntries
		}
		if len(result.Data) == 0 && len(out) < result.Total {
			return nil, errors.New("incomplete BookStack listing")
		}
		out = append(out, result.Data...)
		if len(out) >= result.Total {
			return out, nil
		}
	}
}

// Fetch writes BookStack shelves, books, chapters and pages as a staged Markdown zip.
func (b *BookStack) Fetch(ctx context.Context, destination string, progress func(Progress)) ([]docimport.Issue, error) {
	defer b.client.http.CloseIdleConnections()
	shelves, err := b.list(ctx, "shelves")
	if err != nil {
		return nil, err
	}
	books, err := b.list(ctx, "books")
	if err != nil {
		return nil, err
	}
	pages, err := b.list(ctx, "pages")
	if err != nil {
		return nil, err
	}
	mapper := &bookMapper{notes: []outputNote{}, files: map[string][]byte{}, targets: map[string]string{},
		siteLinks: map[string]string{}, linkRefs: map[string]bool{}, warnings: []docimport.Issue{}, limits: b.limits, base: b.client.base}
	parents := map[int]string{}
	for _, s := range shelves {
		data, err := b.client.get(ctx, "/api/shelves/"+strconv.Itoa(s.ID), 10<<20)
		if err != nil {
			return nil, err
		}
		var shelf entity
		if json.Unmarshal(data, &shelf) != nil {
			return nil, errors.New("invalid BookStack shelf response")
		}
		shelfPath := fmt.Sprintf("shelf-%d", s.ID)
		body := shelf.Description
		if shelf.DescriptionHTML != "" {
			body, err = convertHTML(shelf.DescriptionHTML)
			if err != nil {
				return nil, errors.New("could not convert BookStack shelf HTML")
			}
		}
		if err := mapper.addNote(shelfPath+"/index.md", shelf.Name, body); err != nil {
			return nil, err
		}
		for _, book := range shelf.Books {
			if first, exists := parents[book.ID]; exists {
				mapper.warnings = append(mapper.warnings, docimport.Issue{Path: fmt.Sprintf("book-%d", book.ID),
					Message: fmt.Sprintf("book belongs to multiple shelves; imported under %s (first shelf), rather than %s", first, shelfPath)})
				continue
			}
			parents[book.ID] = shelfPath
		}
	}
	bookPaths := map[int]string{}
	for _, book := range books {
		bookPaths[book.ID] = path.Join(parents[book.ID], fmt.Sprintf("book-%d", book.ID))
	}
	for _, page := range pages {
		_, exists := bookPaths[page.BookID]
		if !exists {
			continue
		}
		// The export determines the chapter parent; these temporary targets are
		// resolved to the final paths after every book is mapped.
		target := "page:" + strconv.Itoa(page.ID)
		mapper.siteLinks["/link/"+strconv.Itoa(page.ID)] = target
		for _, book := range books {
			if book.ID == page.BookID {
				mapper.siteLinks["/books/"+book.Slug+"/page/"+page.Slug] = target
				break
			}
		}
	}
	progress(Progress{Total: len(books)})
	for i, book := range books {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := b.client.get(ctx, fmt.Sprintf("/api/books/%d/export/zip", book.ID), docimport.MaxUploadBytes)
		if errors.Is(err, errNotFound) {
			return nil, errors.New("BookStack ZIP export API requires " + BookStackMinVersion + " or newer; check the version and token export permissions")
		}
		if err != nil {
			return nil, err
		}
		if err := mapper.readBook(raw, bookPaths[book.ID], book.ID); err != nil {
			return nil, err
		}
		progress(Progress{Done: i + 1, Total: len(books)})
	}
	if err := mapper.write(ctx, destination); err != nil {
		return nil, err
	}
	return mapper.warnings, nil
}

func (m *bookMapper) addNote(name, title, body string) error {
	if len(m.notes)+len(m.files) >= m.limits.MaxEntries {
		return docimport.ErrTooManyEntries
	}
	if int64(len(body)) > m.limits.MaxNoteBytes {
		return errors.New("BookStack page exceeds note size limit")
	}
	m.notes = append(m.notes, outputNote{name: name, title: title, body: body})
	return nil
}

func (m *bookMapper) readBook(raw []byte, folder string, bookID int) error {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return errors.New("invalid BookStack ZIP export")
	}
	archive, err := docimport.OpenArchive(zr, m.limits)
	if err != nil {
		return errors.New("BookStack ZIP export violates import archive limits")
	}
	m.entries += len(zr.File)
	if m.entries > m.limits.MaxEntries {
		return docimport.ErrTooManyEntries
	}
	data, err := m.read(archive, "data.json", m.limits.MaxBytes)
	if err != nil {
		return err
	}
	var export struct {
		Book *portableNode `json:"book"`
	}
	if json.Unmarshal(data, &export) != nil || export.Book == nil {
		return errors.New("invalid BookStack book export metadata")
	}
	// Validate and count every file, even if the export does not reference it.
	for _, filename := range archive.Paths() {
		if filename == "data.json" {
			continue
		}
		data, err := m.read(archive, filename, m.limits.MaxBytes)
		if err != nil {
			return err
		}
		key := assetPath(bookID, filename)
		m.files[key] = data
	}
	book := export.Book
	m.targets["book:"+strconv.Itoa(bookID)] = folder + "/index.md"
	return m.node(*book, folder, "book", bookID)
}

// Markdown assets are attachments upstream, never page input. Give them an
// unsupported extension so the shared analyzer reports them as skipped instead
// of interpreting their contents/front-matter as docs.
func assetPath(bookID int, name string) string {
	key := path.Join("assets", strconv.Itoa(bookID), name)
	ext := strings.ToLower(path.Ext(name))
	if ext == ".md" || ext == ".markdown" {
		key += ".unsupported"
	}
	return key
}

func (m *bookMapper) read(a *docimport.Archive, name string, limit int64) ([]byte, error) {
	rc, err := a.Open(name)
	if err != nil {
		return nil, errors.New("BookStack export references a missing file")
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(io.LimitReader(rc, min(limit, m.limits.MaxBytes-m.bytes)+1))
	if err != nil {
		return nil, errors.New("could not read BookStack export file")
	}
	m.bytes += int64(len(data))
	if m.bytes > m.limits.MaxBytes || int64(len(data)) > limit {
		return nil, docimport.ErrTooLarge
	}
	return data, nil
}

func (m *bookMapper) node(n portableNode, folder, kind string, bookID int) error {
	filename := folder + "/index.md"
	if kind == "page" {
		filename = folder + ".md"
	}
	if n.ID > 0 {
		m.targets[kind+":"+strconv.Itoa(n.ID)] = filename
	}
	body := n.Markdown
	if body == "" {
		html := n.HTML
		if kind != "page" {
			html = n.DescriptionHTML
		}
		if html != "" {
			converted, err := convertHTML(html)
			if err != nil {
				return errors.New("could not convert BookStack HTML")
			}
			body = converted
		}
	}
	for _, group := range []struct {
		kind  string
		files []portableFile
	}{
		{kind: "image", files: n.Images}, {kind: "attachment", files: n.Attachments},
	} {
		for _, file := range group.files {
			ref := group.kind + ":" + strconv.Itoa(file.ID)
			if file.Link != "" {
				u, err := url.Parse(file.Link)
				if err != nil {
					return errors.New("invalid BookStack link attachment")
				}
				base := *m.base
				base.Path = strings.TrimRight(base.Path, "/") + "/"
				base.RawPath = ""
				resolved := base.ResolveReference(u)
				if !safeLinkScheme(resolved.Scheme) {
					m.warnings = append(m.warnings, docimport.Issue{Path: filename, Message: "unsafe link attachment skipped"})
					continue
				}
				// The URL becomes a Markdown destination, so parentheses must not
				// close it early and let the remainder inject Markdown.
				m.targets[ref] = markdownSafeURL(resolved.String())
				m.linkRefs[ref] = true
				if group.kind == "attachment" {
					body += "\n\n[" + escapeLabel(file.Name) + "]([[bsexport:" + ref + "]])"
				}
				continue
			}
			// OpenArchive already validates archive paths. Exact lookup prevents a
			// JSON reference from escaping its own files directory.
			expected := "files/" + file.File
			key := assetPath(bookID, expected)
			if file.File == "" || path.Clean(expected) != expected || !strings.HasPrefix(expected, "files/") {
				return errors.New("invalid BookStack file reference")
			}
			if _, exists := m.files[key]; !exists {
				return errors.New("BookStack export references a missing file")
			}
			m.targets[ref] = key
			if group.kind == "attachment" {
				body += "\n\n[" + escapeLabel(file.Name) + "]([[bsexport:" + ref + "]])"
			}
		}
	}
	if err := m.addNote(filename, n.Name, body); err != nil {
		return err
	}
	type child struct {
		node portableNode
		kind string
	}
	children := []child{}
	for _, chapter := range n.Chapters {
		children = append(children, child{node: chapter, kind: "chapter"})
	}
	for _, page := range n.Pages {
		children = append(children, child{node: page, kind: "page"})
	}
	sort.SliceStable(children, func(i, j int) bool { return children[i].node.Priority < children[j].node.Priority })
	for i, child := range children {
		next := path.Join(folder, fmt.Sprintf(
			"%06d-%06d-%s-%d",
			child.node.Priority,
			i,
			child.kind,
			child.node.ID,
		))
		if err := m.node(child.node, next, child.kind, bookID); err != nil {
			return err
		}
	}
	return nil
}

var bsReference = regexp.MustCompile(`\[\[bsexport:(page|chapter|book|image|attachment):(\d+)\]\]`)
var markdownDestination = regexp.MustCompile(`\]\((<?)([^\s<>()]+)(>?)([^\n)]*)\)`)

var bsPlaceholder = regexp.MustCompile(`#bsexport/(page|chapter|book|image|attachment)/(\d+)`)

// convertHTML shields export references from htmlmd, which drops link and image
// URLs whose scheme is not http(s), mailto or tel. References become relative
// fragments for the conversion and are restored afterwards.
func convertHTML(src string) (string, error) {
	shielded := bsReference.ReplaceAllString(src, "#bsexport/$1/$2")
	out, err := htmlmd.Convert(shielded)
	if err != nil {
		return "", err
	}
	return bsPlaceholder.ReplaceAllString(out, "[[bsexport:$1:$2]]"), nil
}

// safeLinkScheme is the allowlist for link attachments; relative links resolve
// to the wiki's own http(s) scheme before this check.
func safeLinkScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "mailto", "tel":
		return true
	}
	return false
}

func markdownSafeURL(u string) string {
	// Brackets, backslashes and backticks are encoded too, so a URL cannot carry
	// a [[bsexport:...]] placeholder into the second rewrite pass.
	return strings.NewReplacer("(", "%28", ")", "%29", " ", "%20", "<", "%3C", ">", "%3E",
		"[", "%5B", "]", "%5D", "\\", "%5C", "`", "%60").Replace(u)
}

func escapeLabel(s string) string { return strings.NewReplacer("[", "\\[", "]", "\\]").Replace(s) }

func (m *bookMapper) reference(ref string) (string, bool) {
	parts := bsReference.FindStringSubmatch(ref)
	if parts == nil {
		return "", false
	}
	target, exists := m.targets[parts[1]+":"+parts[2]]
	if exists && !m.linkRefs[parts[1]+":"+parts[2]] {
		target = "/" + target
	}
	return target, exists
}

func (m *bookMapper) rewrite(note outputNote) string {
	// Resolve placeholders used as Markdown destinations first, retaining the
	// existing label and image marker. Bare references become Markdown links.
	body := markdownDestination.ReplaceAllStringFunc(note.body, func(link string) string {
		parts := markdownDestination.FindStringSubmatch(link)
		if target, ok := m.reference(parts[2]); ok {
			return "](" + target + parts[4] + ")"
		}
		u, err := url.Parse(parts[2])
		if err != nil {
			return link
		}
		sameHost := u.Host == "" || strings.EqualFold(u.Host, m.base.Host)
		if !sameHost || (u.Scheme != "" && u.Scheme != m.base.Scheme) {
			return link
		}
		remotePath := strings.TrimPrefix(u.Path, m.base.Path)
		ref, exists := m.siteLinks[remotePath]
		if !exists {
			return link
		}
		target, exists := m.targets[ref]
		if !exists {
			return link
		}
		return "](/" + target + parts[4] + ")"
	})
	return bsReference.ReplaceAllStringFunc(body, func(ref string) string {
		target, exists := m.reference(ref)
		if !exists {
			m.warnings = append(m.warnings, docimport.Issue{Path: note.name, Message: "unresolved BookStack export reference"})
			return ref
		}
		return "[" + escapeLabel(ref[2:len(ref)-2]) + "](" + target + ")"
	})
}

func (m *bookMapper) write(ctx context.Context, destination string) error {
	f, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("could not create staged wiki archive")
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	var total int64
	add := func(name string, data []byte) error {
		total += int64(len(data))
		if total > m.limits.MaxBytes {
			return docimport.ErrTooLarge
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	for _, note := range m.notes {
		if err := ctx.Err(); err != nil {
			return err
		}
		title, _ := json.Marshal(note.title) // JSON quoted strings are also valid YAML scalars.
		data := []byte("---\ntitle: " + string(title) + "\n---\n" + m.rewrite(note))
		if err := add(note.name, data); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(m.files))
	for name := range m.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := add(name, m.files[name]); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > docimport.MaxUploadBytes {
		return docimport.ErrTooLarge
	}
	return f.Close()
}

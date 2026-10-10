package docimport

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/WiseLabz/wiselabz/internal/docimport/htmlmd"
)

// Source names the kind of archive an import reads.
type Source string

const (
	// SourceMarkdown is a folder of Markdown files or an Obsidian vault.
	SourceMarkdown Source = "markdown"
	// SourceBookStack identifies server-staged BookStack pulls.
	SourceBookStack Source = "bookstack"
	// SourceWikiJS is a Wiki.js 2.x storage export (the disk target layout).
	SourceWikiJS Source = "wikijs"
)

// ErrUnknownSource rejects an import source this server does not know.
var ErrUnknownSource = errors.New("unknown import source")

// ParseSource maps the upload's source field to a Source; empty means Markdown.
func ParseSource(s string) (Source, error) {
	switch Source(s) {
	case "", SourceMarkdown:
		return SourceMarkdown, nil
	case SourceWikiJS:
		return SourceWikiJS, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnknownSource, s)
}

// AnalyzeSource builds the import plan for an archive of the given source.
func AnalyzeSource(src Source, a *Archive, connectors []Connector) (*Plan, error) {
	if src == SourceWikiJS {
		return AnalyzeWikiJS(a)
	}
	return Analyze(a, connectors)
}

// page is one Wiki.js page file after its metadata block was split off.
type page struct {
	file      string // archive path, with extension
	key       string // site path: the file path without extension
	title     string
	body      string
	published bool
}

var (
	wikiLocaleRe = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})?$`)
	// wikiImageSize matches Wiki.js image sizing, ![alt](/path =200x).
	wikiImageSize = regexp.MustCompile(`(!\[[^\[\]\n]*\]\([^()\s]+)\s+=\d*x\d*\)`)
	wikiMetaKeys  = map[string]bool{"title": true, "description": true, "published": true, "date": true, "tags": true, "editor": true, "datecreated": true}
)

func pageKind(p string) (kind string) {
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".markdown":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".adoc", ".asciidoc":
		return "asciidoc"
	}
	return ""
}

// AnalyzeWikiJS reads a Wiki.js 2.x storage export. Pages are <path>.md or
// <path>.html, each starting with a metadata block; assets sit at their site
// path. Path segments become parent docs, and a page next to a folder of the
// same name is that folder's parent.
func AnalyzeWikiJS(a *Archive) (*Plan, error) {
	p := &planner{archive: a, plan: &Plan{Docs: []Doc{}, Mappings: []Mapping{}, Warnings: []Issue{}, Skipped: []Issue{}},
		notes: map[string]*note{}, attachments: map[string]bool{}, referenced: map[string]bool{},
		docs: map[string]*Doc{}, sources: map[string]string{}, depth: map[string]int{}, byID: map[string]*Doc{}}
	pages := map[string]*page{} // by lower-case site path
	err := p.readFiles(func(fp string) (bool, error) {
		switch pageKind(fp) {
		case "":
			return false, nil
		case "asciidoc":
			p.skip(fp, "AsciiDoc pages are not supported")
			return true, nil
		}
		pg, err := p.readPage(fp)
		if err != nil || pg == nil {
			return true, err
		}
		lk := strings.ToLower(pg.key)
		if other := pages[lk]; other != nil {
			p.skip(fp, fmt.Sprintf("another file already provides the page %q (%s)", pg.key, other.file))
			return true, nil
		}
		pages[lk] = pg
		return true, nil
	})
	if err != nil {
		return nil, err
	}
	site := p.wikiLayout(pages)
	p.rewriteWiki(site)
	for _, ap := range sortedKeys(p.attachments) {
		if !p.referenced[ap] {
			p.skip(ap, "attachment is not embedded by any page")
		}
	}
	return p.plan, nil
}

// readPage reads one page file, converting HTML to Markdown.
func (p *planner) readPage(fp string) (*page, error) {
	data, ok, err := p.archive.readAll(fp, p.archive.limits.MaxNoteBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		p.skip(fp, fmt.Sprintf("page is larger than %d MiB", p.archive.limits.MaxNoteBytes>>20))
		return nil, nil
	}
	if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
		p.skip(fp, "page is not UTF-8 text")
		return nil, nil
	}
	isHTML := pageKind(fp) == "html"
	meta, body := splitWikiMeta(string(data), isHTML)
	if isHTML {
		if body, err = htmlmd.Convert(body); err != nil {
			p.skip(fp, "HTML could not be converted: "+err.Error())
			return nil, nil
		}
	}
	pg := &page{file: fp, key: strings.TrimSuffix(fp, path.Ext(fp)), title: meta["title"], body: body, published: true}
	if v, ok := meta["published"]; ok && strings.EqualFold(v, "false") {
		pg.published = false
		p.warn(fp, "page is not published in Wiki.js; imported anyway")
	}
	return pg, nil
}

// splitWikiMeta separates the metadata block Wiki.js writes before the
// content: between --- lines in Markdown, inside <!-- --> in HTML. Values are
// raw text, not YAML, so each line is a key, a colon and the rest of the line.
func splitWikiMeta(s string, isHTML bool) (map[string]string, string) {
	s = strings.ReplaceAll(strings.TrimPrefix(s, "\ufeff"), "\r\n", "\n")
	open, closer := "---", "---"
	if isHTML {
		open, closer = "<!--", "-->"
	}
	lines := strings.Split(s, "\n")
	if strings.TrimSpace(lines[0]) != open {
		return map[string]string{}, s
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == closer {
			end = i
			break
		}
	}
	if end < 0 {
		return map[string]string{}, s
	}
	meta := map[string]string{}
	for _, l := range lines[1:end] {
		k, v, ok := strings.Cut(l, ":")
		if k = strings.ToLower(strings.TrimSpace(k)); ok && wikiMetaKeys[k] {
			meta[k] = strings.TrimSpace(v)
		}
	}
	if len(meta) == 0 {
		return meta, s // a horizontal rule or comment, not a metadata block
	}
	return meta, strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")
}

// wikiLayout allocates docs for pages and the folders between them, parents
// first, and returns the lower-case site path -> doc key index.
func (p *planner) wikiLayout(pages map[string]*page) map[string]string {
	type entry struct {
		key string
		pg  *page
	}
	byKey := map[string]*entry{}
	for _, pg := range pages {
		byKey[strings.ToLower(pg.key)] = &entry{pg.key, pg}
	}
	parents := map[string]bool{} // lower-case keys that have children
	for _, pg := range pages {
		for d := path.Dir(pg.key); d != "."; d = path.Dir(d) {
			ld := strings.ToLower(d)
			parents[ld] = true
			if byKey[ld] == nil {
				byKey[ld] = &entry{key: d}
			}
		}
	}
	entries := make([]*entry, 0, len(byKey))
	for _, e := range byKey {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		di, dj := strings.Count(entries[i].key, "/"), strings.Count(entries[j].key, "/")
		if di != dj {
			return di < dj
		}
		return entries[i].key < entries[j].key
	})
	// Fixed capacity keeps the pointers stored below valid.
	p.plan.Docs = make([]Doc, 0, len(entries))
	site := map[string]string{}
	for _, e := range entries {
		name := path.Base(e.key)
		d := Doc{ID: uuid.NewString(), Path: e.key, Folder: parents[strings.ToLower(e.key)], Attachments: []Attachment{}}
		if e.pg != nil {
			d.Path, d.Content = e.pg.file, e.pg.body
			d.Title = firstNonEmpty(e.pg.title, firstH1(e.pg.body), name)
			p.sources[d.ID] = e.pg.file
		} else {
			d.Title = name
		}
		d.Title = truncateRunes(d.Title, maxTitleRunes)
		lk := strings.ToLower(e.key) // docs are keyed by lower-case site path
		d, depth := p.nest(d, lk, "")
		p.store(d, depth, lk)
		site[lk] = lk
	}
	return site
}

func (p *planner) rewriteWiki(site map[string]string) {
	assets := newPathIndex(sortedKeys(p.attachments))
	for i := range p.plan.Docs {
		d := &p.plan.Docs[i]
		src := p.sources[d.ID]
		if src == "" || d.Content == "" {
			continue
		}
		r := rewriter{planner: p, doc: d, src: src, attachments: assets, site: site}
		d.Content = r.rewrite(d.Content, r.wikiText)
	}
}

func (r *rewriter) wikiText(s string) string {
	s = wikiImageSize.ReplaceAllString(s, "$1)")
	return mdLinkRe.ReplaceAllStringFunc(s, r.siteLink)
}

// siteLink rewrites an absolute site link: /path and /<locale>/path become
// doc links, and an asset path resolved from the export root an attachment.
func (r *rewriter) siteLink(m string) string {
	sub := mdLinkRe.FindStringSubmatch(m)
	embed, text, dest := sub[1] == "!", sub[2], strings.Trim(sub[3], "<>")
	if !strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "//") || schemeRe.MatchString(dest) {
		return m
	}
	dest, _, _ = strings.Cut(dest, "#")
	dest, _, _ = strings.Cut(dest, "?")
	if decoded, err := url.PathUnescape(dest); err == nil {
		dest = decoded
	}
	dest = strings.Trim(path.Clean(dest), "/")
	if dest == "" || dest == "." {
		return m
	}
	lower := strings.ToLower(dest)
	if isAttachmentPath(dest) {
		if ap, ok := r.attachments.byPath[lower]; ok {
			return r.attachmentLink(m, ap, firstNonEmpty(text, path.Base(ap)), embed)
		}
		r.unresolved(m, nil)
		return m
	}
	candidates := []string{lower}
	if first, rest, ok := strings.Cut(lower, "/"); ok && wikiLocaleRe.MatchString(first) {
		candidates = append(candidates, rest)
	}
	for _, c := range candidates {
		if key, ok := r.site[c]; ok {
			return r.docLink(m, key, firstNonEmpty(text, path.Base(key)))
		}
	}
	r.unresolved(m, nil)
	return m
}

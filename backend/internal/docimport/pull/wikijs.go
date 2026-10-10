package pull

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/docimport"
)

// WikiJS fetches pages and assets from a Wiki.js 2.x GraphQL API and writes the
// storage export layout that docimport.AnalyzeWikiJS parses. The API key needs
// the read:pages, read:source and read:assets permissions.
//
// Schema source (requarks/wiki v2.5.x, server/graph/schemas/page.graphql and
// asset.graphql): pages.list, pages.single(id), assets.folders(parentFolderId)
// and assets.list(folderId, kind). Page.isPublished and Page.editor need
// write:pages upstream, so publication state comes from the list item instead.
type WikiJS struct {
	client remoteClient
	limits docimport.Limits
}

// NewWikiJS validates the URL and constructs a guarded, verified-by-default source.
func NewWikiJS(rawURL, apiKey string, skipTLS bool, limits docimport.Limits) (*WikiJS, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return nil, err
	}
	if apiKey == "" || strings.ContainsAny(apiKey, "\r\n ") {
		return nil, errors.New("the Wiki.js API key is required")
	}
	return &WikiJS{client: remoteClient{base: u, http: newHTTPClient(skipTLS),
		authorization: "Bearer " + apiKey}, limits: limits}, nil
}

// wikiGraphQLLimit is a variable so tests can shrink it.
var wikiGraphQLLimit int64 = 10 << 20

type wikiListItem struct {
	ID          int    `json:"id"`
	Path        string `json:"path"`
	Locale      string `json:"locale"`
	ContentType string `json:"contentType"`
	IsPublished bool   `json:"isPublished"`
}

type wikiPage struct {
	Path        string `json:"path"`
	Locale      string `json:"locale"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Tags        []struct {
		Tag string `json:"tag"`
	} `json:"tags"`
}

type wikiAsset struct {
	Filename string `json:"filename"`
	FileSize int64  `json:"fileSize"`
	folder   string // site path of the containing folder, without a leading slash
}

// query runs one GraphQL operation. Error text never carries the remote's own
// messages: they can echo request details.
func (w *WikiJS) query(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return errors.New("could not encode Wiki.js request")
	}
	raw, err := w.client.postJSON(ctx, "/graphql", body, wikiGraphQLLimit)
	if errors.Is(err, errNotFound) {
		return errors.New("the Wiki.js GraphQL endpoint was not found; check the URL")
	}
	if err != nil {
		return err
	}
	var envelope struct {
		Data   json.RawMessage   `json:"data"`
		Errors []json.RawMessage `json:"errors"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return errors.New("invalid Wiki.js GraphQL response")
	}
	if len(envelope.Errors) > 0 {
		return errors.New("the Wiki.js GraphQL request failed; check that the API key has read:pages, read:source and read:assets")
	}
	if json.Unmarshal(envelope.Data, out) != nil {
		return errors.New("invalid Wiki.js GraphQL response")
	}
	return nil
}

func (w *WikiJS) listPages(ctx context.Context) ([]wikiListItem, error) {
	var result struct {
		Pages struct {
			List []wikiListItem `json:"list"`
		} `json:"pages"`
	}
	err := w.query(ctx, `query { pages { list(orderBy: ID, orderByDirection: ASC) { id path locale contentType isPublished } } }`, nil, &result)
	if err != nil {
		return nil, err
	}
	if len(result.Pages.List) > w.limits.MaxEntries {
		return nil, docimport.ErrTooManyEntries
	}
	return result.Pages.List, nil
}

func (w *WikiJS) page(ctx context.Context, id int) (*wikiPage, error) {
	var result struct {
		Pages struct {
			Single *wikiPage `json:"single"`
		} `json:"pages"`
	}
	err := w.query(ctx, `query ($id: Int!) { pages { single(id: $id) { path locale title description content contentType createdAt updatedAt tags { tag } } } }`,
		map[string]any{"id": id}, &result)
	return result.Pages.Single, err
}

// listAssets walks the folder tree breadth first. Folder slugs form the site path.
func (w *WikiJS) listAssets(ctx context.Context) ([]wikiAsset, error) {
	type folder struct {
		id   int
		path string
	}
	queue, seen, assets, folders := []folder{{}}, map[int]bool{0: true}, []wikiAsset{}, 0
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current := queue[0]
		queue = queue[1:]
		var files struct {
			Assets struct {
				List []wikiAsset `json:"list"`
			} `json:"assets"`
		}
		err := w.query(ctx, `query ($id: Int!) { assets { list(folderId: $id, kind: ALL) { filename fileSize } } }`,
			map[string]any{"id": current.id}, &files)
		if err != nil {
			return nil, err
		}
		for _, a := range files.Assets.List {
			a.folder = current.path
			assets = append(assets, a)
		}
		var subfolders struct {
			Assets struct {
				Folders []struct {
					ID   int    `json:"id"`
					Slug string `json:"slug"`
				} `json:"folders"`
			} `json:"assets"`
		}
		err = w.query(ctx, `query ($id: Int!) { assets { folders(parentFolderId: $id) { id slug } } }`,
			map[string]any{"id": current.id}, &subfolders)
		if err != nil {
			return nil, err
		}
		for _, f := range subfolders.Assets.Folders {
			if seen[f.ID] || !safeSegment(f.Slug) {
				continue
			}
			seen[f.ID] = true
			if folders++; folders+len(assets) > w.limits.MaxEntries {
				return nil, docimport.ErrTooManyEntries
			}
			queue = append(queue, folder{id: f.ID, path: path.Join(current.path, f.Slug)})
		}
		if len(assets) > w.limits.MaxEntries {
			return nil, docimport.ErrTooManyEntries
		}
	}
	return assets, nil
}

// safeSegment accepts a single path component: remote folder slugs and file
// names must not collapse into, or climb out of, their parent when joined.
func safeSegment(s string) bool {
	return s != "" && s != "." && s != ".." && !strings.ContainsAny(s, "/\\")
}

// safeSitePath cleans a remote-provided relative path and rejects traversal.
func safeSitePath(p string) (string, bool) {
	p = strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
	if p == "" || strings.ContainsFunc(p, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return "", false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", false
		}
	}
	return p, true
}

// wikiMetaValue keeps a value on one line: the export metadata is line based.
func wikiMetaValue(s string) string {
	return strings.TrimSpace(strings.NewReplacer("\r", " ", "\n", " ").Replace(s))
}

func exportExt(contentType string) string {
	switch contentType {
	case "markdown":
		return ".md"
	case "html":
		return ".html"
	case "asciidoc":
		return ".adoc"
	}
	return ""
}

// renderPage reproduces Wiki.js helpers/page.js injectPageMetadata.
func renderPage(p *wikiPage, published bool) []byte {
	tags := make([]string, 0, len(p.Tags))
	for _, t := range p.Tags {
		tags = append(tags, wikiMetaValue(t.Tag))
	}
	meta := fmt.Sprintf("title: %s\ndescription: %s\npublished: %t\ndate: %s\ntags: %s\neditor: %s\ndateCreated: %s",
		wikiMetaValue(p.Title), wikiMetaValue(p.Description), published, wikiMetaValue(p.UpdatedAt),
		strings.Join(tags, ", "), wikiMetaValue(p.ContentType), wikiMetaValue(p.CreatedAt))
	if p.ContentType == "html" {
		return []byte("<!--\n" + meta + "\n-->\n\n" + p.Content)
	}
	return []byte("---\n" + meta + "\n---\n\n" + p.Content)
}

// defaultLocale is the locale with the most pages (lowest code on a tie); the
// Wiki.js disk export leaves its pages unprefixed.
func defaultLocale(pages []wikiListItem) string {
	counts := map[string]int{}
	for _, p := range pages {
		counts[p.Locale]++
	}
	best := ""
	for locale, n := range counts {
		if best == "" || n > counts[best] || (n == counts[best] && locale < best) {
			best = locale
		}
	}
	return best
}

// Fetch writes every readable page and asset as a Wiki.js storage export zip.
func (w *WikiJS) Fetch(ctx context.Context, destination string, progress func(Progress)) ([]docimport.Issue, error) {
	defer w.client.http.CloseIdleConnections()
	pages, err := w.listPages(ctx)
	if err != nil {
		return nil, err
	}
	progress(Progress{Total: len(pages)})
	assets, err := w.listAssets(ctx)
	if err != nil {
		return nil, err
	}
	if len(pages)+len(assets) > w.limits.MaxEntries {
		return nil, docimport.ErrTooManyEntries
	}
	total := len(pages) + len(assets)
	progress(Progress{Total: total})

	f, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, errors.New("could not create staged wiki archive")
	}
	defer func() { _ = f.Close() }()
	zw := zip.NewWriter(f)
	warnings := []docimport.Issue{}
	warn := func(p, msg string) { warnings = append(warnings, docimport.Issue{Path: p, Message: msg}) }
	var written int64
	names := map[string]bool{}
	// add skips a repeated name (compared case-insensitively): OpenArchive
	// rejects archives with duplicate entries.
	add := func(name string, data []byte) error {
		key := strings.ToLower(name)
		if names[key] {
			warn(name, "duplicate path skipped")
			return nil
		}
		names[key] = true
		written += int64(len(data))
		if written > w.limits.MaxBytes {
			return docimport.ErrTooLarge
		}
		entry, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = entry.Write(data)
		return err
	}

	sort.SliceStable(pages, func(i, j int) bool { return pages[i].ID < pages[j].ID })
	def, done := defaultLocale(pages), 0
	for _, item := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		done++
		label := fmt.Sprintf("page-%d", item.ID)
		sitePath, ok := safeSitePath(item.Path)
		ext := exportExt(item.ContentType)
		if !ok || ext == "" {
			warn(label, "page skipped: unsupported path or content type")
			progress(Progress{Done: done, Total: total})
			continue
		}
		page, err := w.page(ctx, item.ID)
		if errors.Is(err, errResponseTooLarge) {
			warn(sitePath, "page exceeds note size limit; skipped")
			progress(Progress{Done: done, Total: total})
			continue
		}
		if err != nil {
			return nil, err
		}
		if page == nil {
			warn(sitePath, "page could not be read; skipped")
			progress(Progress{Done: done, Total: total})
			continue
		}
		if int64(len(page.Content)) > w.limits.MaxNoteBytes {
			warn(sitePath, "page exceeds note size limit; skipped")
			progress(Progress{Done: done, Total: total})
			continue
		}
		if item.Locale != "" && item.Locale != def && safeLocale(item.Locale) {
			sitePath = item.Locale + "/" + sitePath
		}
		if err := add(sitePath+ext, renderPage(page, item.IsPublished)); err != nil {
			return nil, err
		}
		progress(Progress{Done: done, Total: total})
	}
	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		done++
		rel, ok := "", safeSegment(asset.Filename)
		if ok {
			rel, ok = safeSitePath(path.Join(asset.folder, asset.Filename))
		}
		switch {
		case !ok:
			warn(asset.Filename, "asset skipped: unsupported file name")
		case asset.FileSize > w.limits.MaxAttachmentBytes:
			warn(rel, fmt.Sprintf("asset is larger than %d MiB; skipped", w.limits.MaxAttachmentBytes>>20))
		default:
			data, err := w.client.get(ctx, "/"+escapePath(rel), w.limits.MaxAttachmentBytes)
			if errors.Is(err, errNotFound) {
				warn(rel, "asset could not be downloaded; skipped")
			} else if errors.Is(err, errResponseTooLarge) {
				warn(rel, fmt.Sprintf("asset is larger than %d MiB; skipped", w.limits.MaxAttachmentBytes>>20))
			} else if err != nil {
				return nil, err
			} else if err := add(rel, data); err != nil {
				return nil, err
			}
		}
		progress(Progress{Done: done, Total: total})
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() > docimport.MaxUploadBytes {
		return nil, docimport.ErrTooLarge
	}
	return warnings, f.Close()
}

// safeLocale accepts the language codes Wiki.js uses as folder names.
func safeLocale(l string) bool {
	_, ok := safeSitePath(l)
	return ok && !strings.Contains(l, "/")
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

package docimport

import (
	"bytes"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	"github.com/WiseLabz/wiselabz/internal/blobstore"
)

// maxDepth mirrors the doc hierarchy limit enforced by the store.
const maxDepth = 5

// maxTitleRunes leaves room for a collision suffix under the 500-rune title limit.
const maxTitleRunes = 480

// Connector is an existing connector a note can name in front-matter.
type Connector struct {
	ID   string
	Name string
}

// Plan is a staged import: docs with allocated IDs and rewritten content,
// parents listed before their children.
type Plan struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"createdAt"`
	Docs            []Doc     `json:"docs"`
	Mappings        []Mapping `json:"mappings"`
	Warnings        []Issue   `json:"warnings"`
	Skipped         []Issue   `json:"skipped"`
	AttachmentCount int       `json:"attachmentCount"`
}

// Doc is one doc to create.
type Doc struct {
	ID          string       `json:"docId"`
	ParentID    string       `json:"parentId"`
	ServiceID   string       `json:"serviceId"`
	Title       string       `json:"title"`
	Path        string       `json:"path"`
	Folder      bool         `json:"folder"`
	Content     string       `json:"content"`
	Attachments []Attachment `json:"attachments"`
}

// Attachment is an archive file attached to the doc that embeds it.
type Attachment struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Filename string `json:"filename"`
}

// Mapping records one rewritten link.
type Mapping struct {
	Source string `json:"source"`
	Link   string `json:"link"`
	Target string `json:"target"`
}

// Issue is a warning or a skipped file, keyed by archive path.
type Issue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

var attachmentExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".pdf": true, ".txt": true}

func isNote(p string) bool {
	ext := strings.ToLower(path.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func isAttachmentPath(p string) bool { return attachmentExts[strings.ToLower(path.Ext(p))] }

type note struct {
	path      string
	body      string
	title     string // front-matter title
	connector string // front-matter connector, as written
}

type planner struct {
	archive     *Archive
	connectors  []Connector
	plan        *Plan
	notes       map[string]*note // by archive path
	attachments map[string]bool  // sniffed attachment paths
	referenced  map[string]bool
	docs        map[string]*Doc // by archive path of the note or folder
	depth       map[string]int  // by doc ID
	byID        map[string]*Doc
	notePaths   []string          // every note path, for link resolution
	sources     map[string]string // doc ID -> note supplying its content
}

// Analyze reads the archive and builds an import plan.
func Analyze(a *Archive, connectors []Connector) (*Plan, error) {
	p := &planner{archive: a, connectors: connectors, plan: &Plan{Docs: []Doc{}, Mappings: []Mapping{}, Warnings: []Issue{}, Skipped: []Issue{}},
		notes: map[string]*note{}, attachments: map[string]bool{}, referenced: map[string]bool{},
		docs: map[string]*Doc{}, sources: map[string]string{}, depth: map[string]int{}, byID: map[string]*Doc{}}
	if err := p.read(); err != nil {
		return nil, err
	}
	p.layout()
	p.rewriteAll()
	for _, ap := range sortedKeys(p.attachments) {
		if !p.referenced[ap] {
			p.skip(ap, "attachment is not embedded by any note")
		}
	}
	return p.plan, nil
}

func (p *planner) skip(path, msg string) { p.plan.Skipped = append(p.plan.Skipped, Issue{path, msg}) }
func (p *planner) warn(path, msg string) { p.plan.Warnings = append(p.plan.Warnings, Issue{path, msg}) }

func hiddenRoot(p string) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ".") || s == "__MACOSX" {
			return strings.Join(segs[:i+1], "/")
		}
	}
	return ""
}

// read classifies archive files, reading notes and sniffing attachments.
func (p *planner) read() error {
	hiddenSeen := map[string]bool{}
	for _, l := range p.archive.links {
		p.skip(l, "symbolic links are not imported")
	}
	for _, fp := range p.archive.Paths() {
		if h := hiddenRoot(fp); h != "" {
			if !hiddenSeen[h] {
				hiddenSeen[h] = true
				p.skip(h, "hidden file or folder")
			}
			continue
		}
		switch {
		case isNote(fp):
			data, ok, err := p.archive.readAll(fp, p.archive.limits.MaxNoteBytes)
			if err != nil {
				return err
			}
			if !ok {
				p.skip(fp, fmt.Sprintf("note is larger than %d MiB", p.archive.limits.MaxNoteBytes>>20))
				continue
			}
			if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
				p.skip(fp, "note is not UTF-8 text")
				continue
			}
			n := &note{path: fp}
			n.body, n.title, n.connector = p.frontMatter(fp, string(data))
			p.notes[fp] = n
		case isAttachmentPath(fp):
			if p.archive.Size(fp) > p.archive.limits.MaxAttachmentBytes {
				p.skip(fp, fmt.Sprintf("attachment is larger than %d MiB", p.archive.limits.MaxAttachmentBytes>>20))
				continue
			}
			head, _, err := p.archive.readAll(fp, 512)
			if err != nil {
				return err
			}
			if !blobstore.Allowed(http.DetectContentType(head[:min(len(head), 512)]), head) {
				p.skip(fp, "attachment content is not an allowed type (SVG and spoofed extensions are rejected)")
				continue
			}
			p.attachments[fp] = true
		default:
			p.skip(fp, "unsupported file type")
		}
	}
	return nil
}

// frontMatter strips a leading YAML block, returning its title and connector.
func (p *planner) frontMatter(fp, s string) (body, title, connector string) {
	s = strings.TrimPrefix(s, "\ufeff")
	normalized := strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return s, "", ""
	}
	rest := normalized[4:]
	end, closeLen := -1, 0
	for i := 0; i <= len(rest); {
		j := strings.IndexByte(rest[i:], '\n')
		line := rest[i:]
		if j >= 0 {
			line = rest[i : i+j]
		}
		if line == "---" || line == "..." {
			end, closeLen = i, len(line)
			break
		}
		if j < 0 {
			break
		}
		i += j + 1
	}
	if end < 0 {
		return s, "", ""
	}
	body = strings.TrimLeft(rest[end+closeLen:], "\n")
	var meta map[string]any
	if err := yaml.Unmarshal([]byte(rest[:end]), &meta); err != nil {
		p.warn(fp, "front-matter is not valid YAML and was dropped")
		return body, "", ""
	}
	return body, scalar(meta["title"]), scalar(meta["connector"])
}

func scalar(v any) string {
	switch v.(type) {
	case nil, map[string]any, []any:
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func (p *planner) connectorID(fp, ref string) string {
	if ref == "" {
		return ""
	}
	for _, c := range p.connectors {
		if c.ID == ref {
			return c.ID
		}
	}
	for _, c := range p.connectors {
		if strings.EqualFold(c.Name, ref) {
			return c.ID
		}
	}
	p.warn(fp, fmt.Sprintf("unknown connector %q; imported into the lab", ref))
	return ""
}

func (p *planner) connectorName(id string) string {
	for _, c := range p.connectors {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

// folderNote returns the note supplying a folder doc's content, if any.
func (p *planner) folderNote(dir string) *note {
	base := strings.ToLower(path.Base(dir))
	for _, name := range []string{"index", "readme", base} {
		for _, ext := range []string{".md", ".markdown"} {
			for fp, n := range p.notes {
				if path.Dir(fp) == dir && strings.ToLower(path.Base(fp)) == name+ext {
					return n
				}
			}
		}
	}
	return nil
}

// layout allocates doc IDs and places notes and folders in the tree.
func (p *planner) layout() {
	folders := map[string]bool{}
	for fp := range p.notes {
		for d := path.Dir(fp); d != "."; d = path.Dir(d) {
			folders[d] = true
		}
	}
	consumed := map[string]string{} // note path -> folder path
	type entry struct {
		key    string
		folder bool
		n      *note
	}
	var entries []entry
	for dir := range folders {
		n := p.folderNote(dir)
		if n != nil {
			consumed[n.path] = dir
		}
		entries = append(entries, entry{dir, true, n})
	}
	for fp, n := range p.notes {
		if _, ok := consumed[fp]; !ok {
			entries = append(entries, entry{fp, false, n})
		}
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
	for _, e := range entries {
		d := Doc{ID: uuid.NewString(), Path: e.key, Folder: e.folder, Attachments: []Attachment{}}
		name := path.Base(e.key)
		if !e.folder {
			name = strings.TrimSuffix(name, path.Ext(name))
		}
		var own string
		if e.n != nil {
			p.sources[d.ID] = e.n.path
			d.Content = e.n.body
			d.Title = firstNonEmpty(e.n.title, firstH1(e.n.body), name)
			own = p.connectorID(e.n.path, e.n.connector)
		} else {
			d.Title = name
		}
		d.Title = truncateRunes(d.Title, maxTitleRunes)
		depth := 1
		if parent := p.docs[path.Dir(e.key)]; parent != nil {
			d.ParentID, d.ServiceID = parent.ID, parent.ServiceID
			depth = p.depth[parent.ID] + 1
		}
		if own != "" && own != d.ServiceID {
			if d.ParentID != "" {
				p.warn(e.key, fmt.Sprintf("connector doc placed at the root of %s because its folder belongs to another scope", p.connectorName(own)))
			}
			d.ParentID, d.ServiceID, depth = "", own, 1
		}
		if depth > maxDepth {
			anc := p.byID[d.ParentID]
			for p.depth[anc.ID] >= maxDepth {
				anc = p.byID[anc.ParentID]
			}
			p.warn(e.key, fmt.Sprintf("nested deeper than %d levels; placed under %q", maxDepth, anc.Title))
			d.ParentID, depth = anc.ID, p.depth[anc.ID]+1
		}
		p.plan.Docs = append(p.plan.Docs, d)
		stored := &p.plan.Docs[len(p.plan.Docs)-1]
		p.docs[e.key] = stored
		if e.n != nil && e.folder {
			p.docs[e.n.path] = stored
		}
		p.byID[d.ID] = stored
		p.depth[d.ID] = depth
	}
	p.notePaths = sortedKeys(p.notes)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// firstH1 returns the first ATX level-one heading outside code fences.
func firstH1(body string) string {
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if f := fenceMarker(trimmed); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if fence == "" && strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimRight(trimmed[2:], "# "))
		}
	}
	return ""
}

func fenceMarker(trimmed string) string {
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, f) {
			return f
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

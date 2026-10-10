package docimport

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
)

var (
	wikiLinkRe = regexp.MustCompile(`(!?)\[\[([^\[\]\n]+?)\]\]`)
	mdLinkRe   = regexp.MustCompile(`(!?)\[([^\[\]\n]*)\]\((<[^<>\n]+>|[^()\s]+)(\s+"[^"\n]*")?\)`)
	schemeRe   = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
	imageSize  = regexp.MustCompile(`^\d+(x\d+)?$`)
)

// pathIndex resolves link targets case-insensitively, as Obsidian does.
type pathIndex struct {
	byPath map[string]string
	byBase map[string][]string
}

func newPathIndex(paths []string) pathIndex {
	ix := pathIndex{byPath: map[string]string{}, byBase: map[string][]string{}}
	for _, p := range paths {
		lp := strings.ToLower(p)
		ix.byPath[lp] = p
		ix.byBase[path.Base(lp)] = append(ix.byBase[path.Base(lp)], p)
	}
	return ix
}

// resolve applies Obsidian's rule: a target with a path matches the vault
// path, the path relative to the source note, then the shortest path ending
// with it; a bare name matches by basename. Several equally good matches are
// returned as ambiguous candidates.
func (ix pathIndex) resolve(srcDir string, variants []string) (string, []string) {
	var ambiguous []string
	for _, v := range variants {
		lv := strings.ToLower(v)
		var matches []string
		if strings.Contains(v, "/") {
			if p, ok := ix.byPath[lv]; ok {
				return p, nil
			}
			if p, ok := ix.byPath[strings.ToLower(path.Join(srcDir, v))]; ok {
				return p, nil
			}
			for lp, p := range ix.byPath {
				if strings.HasSuffix(lp, "/"+lv) {
					matches = append(matches, p)
				}
			}
		} else {
			matches = ix.byBase[lv]
		}
		best, candidates := pick(matches, srcDir)
		if best != "" {
			return best, nil
		}
		if ambiguous == nil {
			ambiguous = candidates
		}
	}
	return "", ambiguous
}

func pick(matches []string, srcDir string) (string, []string) {
	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		return matches[0], nil
	}
	var local []string
	for _, m := range matches {
		if path.Dir(m) == srcDir {
			local = append(local, m)
		}
	}
	if len(local) == 1 {
		return local[0], nil
	}
	shortest, best := -1, []string{}
	for _, m := range matches {
		depth := strings.Count(m, "/")
		switch {
		case shortest < 0 || depth < shortest:
			shortest, best = depth, []string{m}
		case depth == shortest:
			best = append(best, m)
		}
	}
	if len(best) == 1 {
		return best[0], nil
	}
	sort.Strings(best)
	return "", best
}

func noteVariants(t string) []string {
	if isNote(t) {
		return []string{t}
	}
	return []string{t, t + ".md", t + ".markdown"}
}

func isImage(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}

var linkTextEscaper = strings.NewReplacer(`[`, `\[`, `]`, `\]`)

// rewriteAll is the second pass: every doc ID exists, so links can point at them.
func (p *planner) rewriteAll() {
	notes := newPathIndex(p.notePaths)
	attachments := newPathIndex(sortedKeys(p.attachments))
	for i := range p.plan.Docs {
		d := &p.plan.Docs[i]
		src := p.sources[d.ID]
		if src == "" || d.Content == "" {
			continue
		}
		r := rewriter{planner: p, doc: d, src: src, notes: notes, attachments: attachments}
		d.Content = r.rewrite(d.Content, r.rewriteText)
	}
}

type rewriter struct {
	*planner
	doc                *Doc
	src                string
	notes, attachments pathIndex
	site               map[string]string // Wiki.js: lower-case site path -> doc key
}

// rewrite applies fn to every part of content outside code fences and spans.
func (r *rewriter) rewrite(content string, fn func(string) string) string {
	var b strings.Builder
	fence := ""
	for _, line := range strings.SplitAfter(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if f := fenceMarker(trimmed); f != "" {
			if fence == "" {
				fence = f
			} else if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			b.WriteString(line)
			continue
		}
		if fence != "" {
			b.WriteString(line)
			continue
		}
		b.WriteString(outsideCode(line, fn))
	}
	return b.String()
}

// outsideCode applies fn to the parts of line that are not inline code spans.
func outsideCode(line string, fn func(string) string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(line, '`')
		if i < 0 {
			b.WriteString(fn(line))
			return b.String()
		}
		b.WriteString(fn(line[:i]))
		n := i
		for n < len(line) && line[n] == '`' {
			n++
		}
		ticks := line[i:n]
		j := strings.Index(line[n:], ticks)
		if j < 0 {
			b.WriteString(ticks)
			line = line[n:]
			continue
		}
		b.WriteString(line[i : n+j+len(ticks)])
		line = line[n+j+len(ticks):]
	}
}

func (r *rewriter) rewriteText(s string) string {
	s = wikiLinkRe.ReplaceAllStringFunc(s, r.wikiLink)
	return mdLinkRe.ReplaceAllStringFunc(s, r.mdLink)
}

func (r *rewriter) srcDir() string { return path.Dir(r.src) }

func (r *rewriter) unresolved(link string, candidates []string) {
	if len(candidates) > 1 {
		r.warn(r.src, fmt.Sprintf("ambiguous link %s matches %s; left as text", link, strings.Join(candidates, ", ")))
		return
	}
	r.warn(r.src, fmt.Sprintf("unresolved link %s; left as text", link))
}

func (r *rewriter) mapped(link, target string) {
	r.plan.Mappings = append(r.plan.Mappings, Mapping{Source: r.src, Link: link, Target: target})
}

// attach returns the doc's attachment ID for an archive file, adding it once.
func (r *rewriter) attach(ap string) string {
	for _, a := range r.doc.Attachments {
		if a.Path == ap {
			return a.ID
		}
	}
	a := Attachment{ID: uuid.NewString(), Path: ap, Filename: path.Base(ap)}
	r.doc.Attachments = append(r.doc.Attachments, a)
	r.plan.AttachmentCount++
	r.referenced[ap] = true
	return a.ID
}

func (r *rewriter) attachmentLink(link, ap, text string, embed bool) string {
	target := "attachment:" + r.attach(ap)
	r.mapped(link, target)
	out := "[" + linkTextEscaper.Replace(text) + "](" + target + ")"
	if embed && isImage(ap) {
		return "!" + out
	}
	return out
}

func (r *rewriter) docLink(link, notePath, text string) string {
	target := "/docs/" + r.docs[notePath].ID
	r.mapped(link, target)
	return "[" + linkTextEscaper.Replace(text) + "](" + target + ")"
}

func (r *rewriter) wikiLink(m string) string {
	sub := wikiLinkRe.FindStringSubmatch(m)
	embed := sub[1] == "!"
	target, alias, _ := strings.Cut(sub[2], "|")
	target = strings.TrimSpace(strings.TrimSuffix(target, `\`))
	alias = strings.TrimSpace(alias)
	name, _, _ := strings.Cut(target, "#")
	name = strings.TrimSpace(name)
	if name == "" {
		return m
	}
	if isAttachmentPath(name) {
		ap, candidates := r.attachments.resolve(r.srcDir(), []string{name})
		if ap == "" {
			r.unresolved(m, candidates)
			return m
		}
		if imageSize.MatchString(alias) {
			alias = ""
		}
		if embed && isImage(ap) {
			if i := strings.LastIndexByte(alias, '|'); i >= 0 && imageSize.MatchString(strings.TrimSpace(alias[i+1:])) {
				alias = strings.TrimSpace(alias[:i])
			}
		}
		text := path.Base(ap)
		if alias != "" {
			text = alias
		}
		return r.attachmentLink(m, ap, text, embed)
	}
	np, candidates := r.notes.resolve(r.srcDir(), noteVariants(name))
	if np == "" || r.docs[np] == nil {
		r.unresolved(m, candidates)
		return m
	}
	if embed {
		r.warn(r.src, fmt.Sprintf("note embed %s converted to a link", m))
	}
	return r.docLink(m, np, firstNonEmpty(alias, target))
}

func (r *rewriter) mdLink(m string) string {
	sub := mdLinkRe.FindStringSubmatch(m)
	embed, text, dest := sub[1] == "!", sub[2], strings.Trim(sub[3], "<>")
	if strings.HasPrefix(dest, "#") || schemeRe.MatchString(dest) {
		return m
	}
	dest, _, _ = strings.Cut(dest, "#")
	dest, _, _ = strings.Cut(dest, "?")
	if decoded, err := url.PathUnescape(dest); err == nil {
		dest = decoded
	}
	candidates := []string{path.Join(r.srcDir(), dest)}
	if strings.HasPrefix(dest, "/") {
		candidates = []string{strings.TrimPrefix(path.Clean(dest), "/")}
	}
	switch {
	case isNote(dest):
		for _, c := range candidates {
			if np, ok := r.notes.byPath[strings.ToLower(c)]; ok && r.docs[np] != nil {
				return r.docLink(m, np, firstNonEmpty(text, path.Base(np)))
			}
		}
	case isAttachmentPath(dest):
		for _, c := range candidates {
			if ap, ok := r.attachments.byPath[strings.ToLower(c)]; ok {
				return r.attachmentLink(m, ap, firstNonEmpty(text, path.Base(ap)), embed)
			}
		}
	default:
		return m
	}
	r.unresolved(m, nil)
	return m
}

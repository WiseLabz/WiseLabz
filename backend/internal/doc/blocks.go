package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Generated-block markers (#478). Content sync owns is wrapped as
//
//	<!-- wl:gen key="snap.containers" h="3fa9c0d1e2b4" -->
//	…body…
//	<!-- /wl:gen -->
//
// h is the hash of the body as last generated, so a body that no longer
// matches it was edited by a human. Everything outside blocks is human-owned.
var openMarkerRe = regexp.MustCompile(`^<!-- wl:gen key="([A-Za-z0-9._-]+)" h="([0-9a-f]{12})" -->$`)

const closeMarker = "<!-- /wl:gen -->"

// Block is one generated section of a doc.
type Block struct {
	Key  string
	Hash string // hash of the body when it was last generated
	Body string
}

// NewBlock builds a freshly generated block, neutralising marker lookalikes
// in body so generated text can never close or nest a block.
func NewBlock(key, body string) Block {
	body = strings.NewReplacer("<!-- wl:gen", "<!-- wl-gen", "<!-- /wl:gen", "<!-- /wl-gen").Replace(body)
	return Block{Key: key, Hash: HashBody(body), Body: body}
}

// Edited reports whether the body was changed since it was generated.
func (b Block) Edited() bool { return HashBody(b.Body) != b.Hash }

// HashBody is the 12-hex-char marker hash of a block body.
func HashBody(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])[:12]
}

// Segment is either human-owned text or a generated block.
type Segment struct {
	Text  string
	Block *Block
}

// ParseBlocks splits content into text and block segments. It is lossless:
// RenderSegments(ParseBlocks(x)) == x for any x. Anything that isn't a
// well-formed block (unclosed, nested, empty, CRLF markers) stays text, so a
// mangled marker can never cause sync to delete human content.
func ParseBlocks(content string) []Segment {
	var segs []Segment
	textStart := 0
	for p := 0; p < len(content); {
		eol := lineEnd(content, p)
		m := openMarkerRe.FindStringSubmatch(content[p:eol])
		if m == nil || eol == len(content) {
			p = eol + 1
			continue
		}
		bodyStart := eol + 1
		closeAt, closeEnd := -1, -1
		for q := bodyStart; q <= len(content); {
			qe := lineEnd(content, q)
			line := content[q:qe]
			if line == closeMarker && q > bodyStart {
				closeAt, closeEnd = q, qe
				break
			}
			if openMarkerRe.MatchString(line) || qe == len(content) {
				break
			}
			q = qe + 1
		}
		if closeAt < 0 {
			p = eol + 1
			continue
		}
		if p > textStart {
			segs = append(segs, Segment{Text: content[textStart:p]})
		}
		segs = append(segs, Segment{Block: &Block{Key: m[1], Hash: m[2], Body: content[bodyStart : closeAt-1]}})
		textStart = closeEnd
		p = closeEnd + 1
	}
	if textStart < len(content) {
		segs = append(segs, Segment{Text: content[textStart:]})
	}
	return segs
}

func lineEnd(s string, from int) int {
	if i := strings.IndexByte(s[from:], '\n'); i >= 0 {
		return from + i
	}
	return len(s)
}

// RenderSegments is the inverse of ParseBlocks.
func RenderSegments(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		if s.Block == nil {
			b.WriteString(s.Text)
			continue
		}
		fmt.Fprintf(&b, "<!-- wl:gen key=%q h=%q -->\n%s\n%s", s.Block.Key, s.Block.Hash, s.Block.Body, closeMarker)
	}
	return b.String()
}

// renderFresh lays out a newly generated doc: blocks separated by a blank
// line, ending with a newline.
func renderFresh(blocks []Block) string {
	segs := make([]Segment, 0, 2*len(blocks))
	for i := range blocks {
		if i > 0 {
			segs = append(segs, Segment{Text: "\n\n"})
		}
		segs = append(segs, Segment{Block: &blocks[i]})
	}
	if len(blocks) > 0 {
		segs = append(segs, Segment{Text: "\n"})
	}
	return RenderSegments(segs)
}

// blockKeys returns the keys of blocks in order.
func blockKeys(blocks []Block) []string {
	keys := make([]string, len(blocks))
	for i, b := range blocks {
		keys[i] = b.Key
	}
	return keys
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugKeys turns titles into "<prefix>.<slug>" keys, de-duplicating repeats
// with -2, -3… so each key is unique within one render.
func slugKeys(prefix string, titles []string) []string {
	seen := map[string]int{}
	keys := make([]string, len(titles))
	for i, t := range titles {
		slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(t), "-"), "-")
		if slug == "" {
			slug = "section"
		}
		seen[slug]++
		if n := seen[slug]; n > 1 {
			slug = fmt.Sprintf("%s-%d", slug, n)
		}
		keys[i] = prefix + "." + slug
	}
	return keys
}

// topologyMarkerLine matches the topology edge fingerprint comment together
// with the blank line that separates it from the generated body.
var topologyMarkerLine = regexp.MustCompile(`\n?` + regexp.QuoteMeta(topologyDocMarker) + `[^\n]*-->[ \t]*\n?`)

// StripTopologyMarker removes only the topology edge fingerprint comment,
// leaving wl:gen block markers intact. Served doc content goes through it so
// the comment, which exists for regeneration bookkeeping, is never shown.
func StripTopologyMarker(content string) string {
	return topologyMarkerLine.ReplaceAllString(content, "")
}

// StripMarkers returns content with the wl:gen marker lines and the topology
// edge fingerprint comment removed, i.e. what a reader sees. For a fresh
// render it equals the plain preview.
func StripMarkers(content string) string {
	content = topologyMarkerLine.ReplaceAllString(content, "")
	var b strings.Builder
	for _, s := range ParseBlocks(content) {
		if s.Block != nil {
			b.WriteString(s.Block.Body)
		} else {
			b.WriteString(s.Text)
		}
	}
	return b.String()
}

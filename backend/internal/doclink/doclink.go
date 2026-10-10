// Package doclink resolves document wikilinks and extracts internal Markdown links.
package doclink

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// Target is a visible document or entity destination and its saved label.
type Target struct {
	Type  string
	ID    string
	Label string
}

// Lookup returns visible candidates for an unqualified title or kind/ref.
type Lookup func(string) ([]Target, error)

var (
	wikiLinkRE = regexp.MustCompile(`!?\[\[((?:\\.|[^\[\]\n]|\[[^\]\n]*\])*)\]\]`)
	openGenRE  = regexp.MustCompile(`<!--\s*wl:gen(?:\s|--)`)
)

// RewriteOutsideCode applies fn outside fenced code blocks and inline code spans.
func RewriteOutsideCode(content string, fn func(string) string) string {
	return rewriteOutside(content, fn, false)
}

// Resolve rewrites resolvable [[target]] and [[target|label]] links.
func Resolve(content string, lookup Lookup) (string, []string, error) {
	warnings := []string{}
	var lookupErr error
	resolved := rewriteOutside(content, func(text string) string {
		return replaceWikiLinks(text, func(link string, target string) string {
			if lookupErr != nil {
				return link
			}
			if strings.HasPrefix(link, "!") { // Obsidian embeds are not wikilinks.
				return link
			}
			targetText, alias, hasAlias := strings.Cut(target, "|")
			targetText = strings.TrimSpace(targetText)
			targetText, _, _ = strings.Cut(targetText, "#")
			targetText = strings.TrimSpace(targetText)
			if targetText == "" {
				warnings = append(warnings, fmt.Sprintf("unresolved link %s", link))
				return link
			}
			matches, err := lookup(targetText)
			if err != nil {
				lookupErr = err
				return link
			}
			if len(matches) != 1 {
				if len(matches) > 1 {
					warnings = append(warnings, fmt.Sprintf("ambiguous link %s", link))
				} else {
					warnings = append(warnings, fmt.Sprintf("unresolved link %s", link))
				}
				return link
			}
			t := matches[0]
			if (t.Type != "doc" && t.Type != "entity") || !validUUID(t.ID) {
				warnings = append(warnings, fmt.Sprintf("unresolved link %s", link))
				return link
			}
			label := t.Label
			if hasAlias {
				label = strings.TrimSpace(alias)
			}
			route := "/docs/"
			if t.Type == "entity" {
				route = "/entities/"
			}
			return "[" + escapeLabel(label) + "](" + route + t.ID + ")"
		})
	}, true)
	if lookupErr != nil {
		return content, nil, lookupErr
	}
	return resolved, warnings, nil
}

// Extract returns distinct internal destinations from actual Markdown links,
// including links in generated sections. Images and code nodes are not links.
func Extract(content string) []Target {
	targets := []Target{}
	seen := map[string]bool{}
	source := []byte(content)
	tree := goldmark.DefaultParser().Parse(text.NewReader(source))
	_ = ast.Walk(tree, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := n.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		dest := string(link.Destination)
		typ, prefix := "doc", "/docs/"
		if strings.HasPrefix(dest, "/entities/") {
			typ, prefix = "entity", "/entities/"
		}
		if !strings.HasPrefix(dest, prefix) {
			return ast.WalkContinue, nil
		}
		rawID, _, _ := strings.Cut(strings.TrimPrefix(dest, prefix), "#")
		id, err := uuid.Parse(rawID)
		if err != nil || len(rawID) != 36 {
			return ast.WalkContinue, nil
		}
		canonical := id.String()
		key := typ + "\x00" + canonical
		if !seen[key] {
			targets = append(targets, Target{Type: typ, ID: canonical})
			seen[key] = true
		}
		return ast.WalkContinue, nil
	})
	return targets
}

type span struct{ start, end int }

func codeSpans(content string) []span {
	source := []byte(content)
	tree := goldmark.DefaultParser().Parse(text.NewReader(source))
	blocked := []span{}
	_ = ast.Walk(tree, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.FencedCodeBlock:
			if node.Info != nil {
				blocked = append(blocked, span{node.Info.Segment.Start, node.Info.Segment.Stop})
			}
			lines := n.Lines()
			if lines.Len() > 0 {
				blocked = append(blocked, span{lines.At(0).Start, lines.At(lines.Len() - 1).Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeBlock:
			lines := n.Lines()
			if lines.Len() > 0 {
				blocked = append(blocked, span{lines.At(0).Start, lines.At(lines.Len() - 1).Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.CodeSpan:
			first, last := n.FirstChild(), n.LastChild()
			if firstText, ok := first.(*ast.Text); ok {
				if lastText, ok := last.(*ast.Text); ok {
					blocked = append(blocked, span{firstText.Segment.Start, lastText.Segment.Stop})
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return blocked
}

func rewriteOutside(content string, fn func(string) string, skipGenerated bool) string {
	blocked := codeSpans(content)
	sort.Slice(blocked, func(i, j int) bool { return blocked[i].start < blocked[j].start })
	codeRanges := blocked
	codeIndex := 0
	if skipGenerated {
		// Marker lines are excluded only outside code, so a marker in an example
		// cannot hide the remainder of a human section from the resolver.
		generated := false
		for start := 0; start < len(content); {
			end := strings.IndexByte(content[start:], '\n')
			if end < 0 {
				end = len(content)
			} else {
				end += start + 1
			}
			line := content[start:end]
			for codeIndex < len(codeRanges) && codeRanges[codeIndex].end <= start {
				codeIndex++
			}
			inCode := codeIndex < len(codeRanges) && codeRanges[codeIndex].start < end
			if !inCode && openGenRE.MatchString(line) {
				generated = true
			}
			if generated {
				blocked = append(blocked, span{start, end})
				if strings.Contains(line, "<!-- /wl:gen -->") {
					generated = false
				}
			}
			start = end
		}
	}
	sort.Slice(blocked, func(i, j int) bool { return blocked[i].start < blocked[j].start })
	var b strings.Builder
	position := 0
	for _, excluded := range blocked {
		if excluded.start > position {
			writeLines(&b, content[position:excluded.start], fn)
		}
		if excluded.end > position {
			b.WriteString(content[max(position, excluded.start):excluded.end])
			position = excluded.end
		}
	}
	if position < len(content) {
		writeLines(&b, content[position:], fn)
	}
	return b.String()
}

// Preserve the importer's line-by-line rewrite order, including attachment IDs.
func writeLines(b *strings.Builder, content string, fn func(string) string) {
	for _, line := range strings.SplitAfter(content, "\n") {
		b.WriteString(fn(line))
	}
}

func replaceWikiLinks(text string, rewrite func(string, string) string) string {
	var b strings.Builder
	position := 0
	for _, match := range wikiLinkRE.FindAllStringSubmatchIndex(text, -1) {
		start, end := match[0], match[1]
		if isEscaped(text, start) {
			continue
		}
		b.WriteString(text[position:start])
		b.WriteString(rewrite(text[start:end], text[match[2]:match[3]]))
		position = end
	}
	b.WriteString(text[position:])
	return b.String()
}

func isEscaped(text string, position int) bool {
	backslashes := 0
	for i := position - 1; i >= 0 && text[i] == '\\'; i-- {
		backslashes++
	}
	return backslashes%2 == 1
}

func validUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func escapeLabel(s string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(s)
}

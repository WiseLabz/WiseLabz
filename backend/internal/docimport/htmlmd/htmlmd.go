// Package htmlmd converts the HTML bodies of imported wiki pages to Markdown.
package htmlmd

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
)

var conv = converter.NewConverter(converter.WithPlugins(
	base.NewBasePlugin(),
	commonmark.NewCommonmarkPlugin(),
	strikethrough.NewStrikethroughPlugin(),
	table.NewTablePlugin(),
))

// safeSchemes are the URL schemes an imported link or image may keep.
var safeSchemes = map[string]bool{"http": true, "https": true, "mailto": true, "tel": true}

// droppedTags are removed with their content, in any namespace (so a script
// nested in an svg element goes too).
var droppedTags = map[string]bool{"script": true, "style": true, "iframe": true, "object": true, "embed": true}

// Convert turns an HTML fragment into Markdown. Scripts, styles and unknown
// elements are dropped rather than passed through as raw HTML, and link or
// image URLs with a scheme other than http, https, mailto or tel (javascript:,
// data:, vbscript:, ...) are removed.
func Convert(src string) (string, error) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return "", err
	}
	sanitize(doc)
	out, err := conv.ConvertNode(doc)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func sanitize(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && droppedTags[c.Data] {
			n.RemoveChild(c)
		} else {
			if c.Type == html.ElementNode {
				switch c.Data {
				case "a":
					stripUnsafeAttr(c, "href")
				case "img":
					stripUnsafeAttr(c, "src")
				}
			}
			sanitize(c)
		}
		c = next
	}
}

func stripUnsafeAttr(n *html.Node, key string) {
	kept := n.Attr[:0]
	for _, a := range n.Attr {
		if a.Key == key && !safeURL(a.Val) {
			continue
		}
		kept = append(kept, a)
	}
	n.Attr = kept
}

// safeURL reports whether v has no scheme or an allowed one. Browsers ignore
// ASCII control characters and whitespace inside a URL scheme, so they are
// stripped before the scheme is read.
func safeURL(v string) bool {
	v = strings.Map(func(r rune) rune {
		if r <= ' ' || r == 0x7f {
			return -1
		}
		return r
	}, v)
	i := strings.IndexAny(v, ":/?#")
	if i < 0 || v[i] != ':' {
		return true // relative, /absolute, ?query or #fragment
	}
	return safeSchemes[strings.ToLower(v[:i])]
}

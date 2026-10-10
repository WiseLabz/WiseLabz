// Package htmlmd converts the HTML bodies of imported wiki pages to Markdown.
package htmlmd

import (
	"strings"

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

// Convert turns an HTML fragment into Markdown. Scripts, styles and unknown
// elements are dropped rather than passed through as raw HTML.
func Convert(html string) (string, error) {
	out, err := conv.ConvertString(html)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

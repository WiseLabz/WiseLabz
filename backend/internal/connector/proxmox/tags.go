package proxmox

import (
	"slices"
	"strings"
	"unicode"
)

// parseTags splits a Proxmox tag list (separated by ';', ',' or whitespace)
// into a sorted, de-duplicated slice. It returns an empty, non-nil slice when
// there are no tags so the attribute is present for every guest. Case is kept.
func parseTags(raw string) []string {
	tags := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == ',' || unicode.IsSpace(r) })
	slices.Sort(tags)
	return slices.Compact(append([]string{}, tags...))
}

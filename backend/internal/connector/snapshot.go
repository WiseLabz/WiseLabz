package connector

import (
	"errors"
	"strings"
)

// ErrorSection marks data that was not observed and preserves the error type.
func ErrorSection(title string, err error) SnapshotSection {
	return SnapshotSection{Title: title, Content: "_" + title + " unavailable: " + err.Error() + "_", Error: err.Error(), cause: err}
}

// FinalizeSnapshot normalizes legacy malformed-response renderers and rejects
// a fetch where no section could be observed. Empty, successful lists are valid.
func FinalizeSnapshot(sn *ServiceSnapshot, err error) (*ServiceSnapshot, error) {
	if err != nil || sn == nil {
		return sn, err
	}
	var first error
	failed := 0
	for i := range sn.Sections {
		section := &sn.Sections[i]
		// Table builders still return this fixed placeholder for decode failures.
		if section.Error == "" && strings.HasPrefix(section.Content, "_") && strings.Contains(section.Content, " unavailable: malformed response:") {
			*section = ErrorSection(section.Title, NewMalformedResponseError(errors.New(section.Content)))
		}
		if section.Error == "" {
			continue
		}
		failed++
		if first == nil {
			first = section.cause
			if first == nil {
				first = errors.New(section.Error)
			}
		}
	}
	if failed > 0 && failed == len(sn.Sections) {
		return nil, first
	}
	return sn, nil
}

// SnapshotError returns the first failed section, if any.
func SnapshotError(sn *ServiceSnapshot) error {
	for _, section := range sn.Sections {
		if section.Error != "" {
			if section.cause != nil {
				return section.cause
			}
			return errors.New(section.Error)
		}
	}
	return nil
}

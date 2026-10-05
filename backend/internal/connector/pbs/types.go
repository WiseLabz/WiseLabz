package pbs

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// flexBool decodes a JSON boolean written as true/false, 0/1, or the strings
// "0", "1", "true" and "false". null decodes to false.
type flexBool bool

func (b *flexBool) UnmarshalJSON(raw []byte) error {
	switch strings.ToLower(strings.Trim(string(bytes.TrimSpace(raw)), `"`)) {
	case "true", "1":
		*b = true
	case "false", "0", "null", "":
		*b = false
	default:
		return errors.New("boolean must be true, false, 0, or 1")
	}
	return nil
}

// flexInt decodes a JSON integer written as a number (including 5.0) or as a
// numeric string. null decodes to 0.
type flexInt int64

func (n *flexInt) UnmarshalJSON(raw []byte) error {
	text := strings.Trim(string(bytes.TrimSpace(raw)), `"`)
	if text == "" || text == "null" {
		*n = 0
		return nil
	}
	if whole, err := strconv.ParseInt(text, 10, 64); err == nil {
		*n = flexInt(whole)
		return nil
	}
	fraction, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return errors.New("integer must be a number or numeric string")
	}
	*n = flexInt(fraction)
	return nil
}

// flexString decodes a JSON string, or a JSON number rendered as text.
type flexString string

func (s *flexString) UnmarshalJSON(raw []byte) error {
	trimmed := bytes.TrimSpace(raw)
	if string(trimmed) == "null" {
		*s = ""
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return err
		}
		*s = flexString(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return errors.New("value must be a string or number")
	}
	*s = flexString(number.String())
	return nil
}

// Payload types follow the PBS API schema (pbs.proxmox.com/docs/api-viewer):
// GET /admin/datastore, /admin/datastore/{store}/namespace, .../groups,
// .../snapshots, /config/verify and /config/prune. Every field is optional on
// the wire, and numbers and booleans are decoded tolerantly.

type datastore struct {
	Store       string `json:"store"`
	Comment     string `json:"comment"`
	BackendType string `json:"backend-type"`
}

type namespaceInfo struct {
	NS string `json:"ns"`
}

// backupGroup is one group of a datastore namespace. Store and Namespace are
// not in the group payload; the client fills them from the request.
type backupGroup struct {
	Store       string     `json:"-"`
	Namespace   string     `json:"-"`
	Type        string     `json:"backup-type"` // vm, ct or host
	ID          flexString `json:"backup-id"`
	BackupCount flexInt    `json:"backup-count"`
	LastBackup  flexInt    `json:"last-backup"` // Unix seconds
}

// groupRef identifies one backup group across datastores and namespaces.
type groupRef struct {
	Store, Namespace, Type, ID string
}

type snapshotInfo struct {
	BackupTime   flexInt `json:"backup-time"` // Unix seconds
	Verification *struct {
		State string `json:"state"` // ok or failed
	} `json:"verification"`
}

type verifyJob struct {
	ID             string   `json:"id"`
	Store          string   `json:"store"`
	NS             string   `json:"ns"`
	Schedule       string   `json:"schedule"`
	IgnoreVerified flexBool `json:"ignore-verified"`
	OutdatedAfter  *flexInt `json:"outdated-after"`
	MaxDepth       *flexInt `json:"max-depth"`
}

type pruneJob struct {
	ID          string   `json:"id"`
	Store       string   `json:"store"`
	NS          string   `json:"ns"`
	Schedule    string   `json:"schedule"`
	Disable     flexBool `json:"disable"`
	MaxDepth    *flexInt `json:"max-depth"`
	KeepLast    flexInt  `json:"keep-last"`
	KeepHourly  flexInt  `json:"keep-hourly"`
	KeepDaily   flexInt  `json:"keep-daily"`
	KeepWeekly  flexInt  `json:"keep-weekly"`
	KeepMonthly flexInt  `json:"keep-monthly"`
	KeepYearly  flexInt  `json:"keep-yearly"`
}

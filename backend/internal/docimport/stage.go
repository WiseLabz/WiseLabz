package docimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TTL is how long a staged import can be committed.
const TTL = time.Hour

// DefaultDir is the persistent staging directory.
const DefaultDir = "/data/imports"

// ErrNotStaged reports a missing, expired or already committed import.
var ErrNotStaged = errors.New("import not found or expired")

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const claimSuffix = ".commit"

// Stage holds staged imports as <dir>/<id>/{upload.zip,plan.json}.
type Stage struct{ Dir string }

// NewStage returns a stage rooted at dir, or DefaultDir when empty.
func NewStage(dir string) Stage {
	if dir == "" {
		dir = DefaultDir
	}
	return Stage{Dir: dir}
}

// Create makes the directory for a new import and returns its path.
func (s Stage) Create(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", ErrNotStaged
	}
	dir := filepath.Join(s.Dir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create import staging: %w", err)
	}
	return dir, nil
}

// UploadPath is where the uploaded zip lives inside a staging directory.
func UploadPath(dir string) string { return filepath.Join(dir, "upload.zip") }

// SavePlan writes the plan next to its upload.
func SavePlan(dir string, plan *Plan) error {
	data, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "plan.json"), data, 0o600)
}

// Claim takes exclusive ownership of an unexpired import for commit. The
// rename is atomic, so concurrent commits of one plan cannot both succeed.
// release puts the import back after a failed commit; done deletes it.
func (s Stage) Claim(id string, now time.Time) (plan *Plan, dir string, release, done func(), err error) {
	if !idPattern.MatchString(id) {
		return nil, "", nil, nil, ErrNotStaged
	}
	staged := filepath.Join(s.Dir, id)
	claimed := staged + claimSuffix
	if err := os.Rename(staged, claimed); err != nil {
		return nil, "", nil, nil, ErrNotStaged
	}
	done = func() { _ = os.RemoveAll(claimed) }
	data, err := os.ReadFile(filepath.Join(claimed, "plan.json"))
	if err != nil {
		done()
		return nil, "", nil, nil, ErrNotStaged
	}
	plan = &Plan{}
	if err := json.Unmarshal(data, plan); err != nil || now.Sub(plan.CreatedAt) > TTL {
		done()
		return nil, "", nil, nil, ErrNotStaged
	}
	release = func() { _ = os.Rename(claimed, staged) }
	return plan, claimed, release, done, nil
}

// Sweep removes staged imports older than TTL, judged by directory mtime.
func (s Stage) Sweep(now time.Time) error {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || !idPattern.MatchString(strings.TrimSuffix(e.Name(), claimSuffix)) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if now.Sub(info.ModTime()) > TTL {
			errs = append(errs, os.RemoveAll(filepath.Join(s.Dir, e.Name())))
		}
	}
	return errors.Join(errs...)
}

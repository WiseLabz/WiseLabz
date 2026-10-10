package docs

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/docimport/pull"
	"github.com/WiseLabz/wiselabz/internal/httputil"
)

// PullRequest carries remote credentials (for Wiki.js, the API key in tokenSecret) for one pull, never staged metadata.
type PullRequest struct {
	Source        string `json:"source"`
	URL           string `json:"url"`
	TokenID       string `json:"tokenId"`
	TokenSecret   string `json:"tokenSecret"`
	SkipTLSVerify bool   `json:"skipTlsVerify"`
}

// PullResponse exposes safe job status and the existing preview once ready.
type PullResponse struct {
	pull.Job
	Preview *ImportPreview `json:"preview,omitempty"`
}

// NewPullManager lets startup own the manager's lifecycle while preserving the
// handler's existing staging, analysis, collision and commit behavior.
func (h *Handler) NewPullManager() *pull.Manager {
	return pull.NewManager(pull.Config{Stage: h.importStage(), Analyze: h.analyzeStagedZip, Audit: h.Store})
}

func (h *Handler) pullManager() *pull.Manager {
	h.pullOnce.Do(func() {
		if h.Pull == nil {
			h.Pull = h.NewPullManager()
		}
	})
	return h.Pull
}

// StartPull handles POST /api/docs/import/pull.
func (h *Handler) StartPull(w http.ResponseWriter, r *http.Request) {
	if !auth.InstanceAdminFromContext(r.Context()) {
		httputil.Error(w, 403, "forbidden", "Instance admin required")
		return
	}
	var request PullRequest
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		httputil.Error(w, 400, "invalid_request", "Invalid documentation pull request")
		return
	}
	var source pull.Source
	var err error
	switch request.Source {
	case "bookstack":
		source, err = pull.NewBookStack(request.URL, request.TokenID, request.TokenSecret, request.SkipTLSVerify, h.importLimits())
	case "wikijs":
		// The Wiki.js API key travels in tokenSecret; there is no token ID.
		source, err = pull.NewWikiJS(request.URL, request.TokenSecret, request.SkipTLSVerify, h.importLimits())
	default:
		httputil.Error(w, 400, "invalid_source", "Unsupported pull source")
		return
	}
	if err != nil {
		httputil.Error(w, 400, "invalid_request", err.Error())
		return
	}
	u, _ := pull.ValidateURL(request.URL)
	job, err := h.pullManager().Start(source, pull.Job{Source: request.Source, Host: u.Host,
		SkipTLSVerify: request.SkipTLSVerify}, pull.Actor{UserID: auth.UserIDFromContext(r.Context()), InstanceAdmin: true})
	if err != nil {
		if errors.Is(err, pull.ErrRunning) {
			httputil.Error(w, 409, "pull_running", err.Error())
			return
		}
		httputil.Errorf(w, err)
		return
	}
	httputil.JSON(w, http.StatusAccepted, PullResponse{Job: job})
}

// GetPull handles GET /api/docs/import/pull.
func (h *Handler) GetPull(w http.ResponseWriter, r *http.Request) {
	if !auth.InstanceAdminFromContext(r.Context()) {
		httputil.Error(w, 403, "forbidden", "Instance admin required")
		return
	}
	job, ok := h.pullManager().Current()
	if !ok {
		httputil.Error(w, 404, "not_found", "No current documentation pull")
		return
	}
	response := PullResponse{Job: job}
	if job.State == "ready" {
		// Read without claiming: a poll must not race a concurrent commit.
		plan, err := h.importStage().ReadPlan(job.ID, time.Now())
		if err != nil {
			httputil.Error(w, 404, "not_found", "Import not found or expired")
			return
		}
		response.Preview, err = h.importPreview(r.Context(), plan)
		if err != nil {
			httputil.Errorf(w, err)
			return
		}
	}
	httputil.JSON(w, 200, response)
}

// CancelPull handles DELETE /api/docs/import/pull.
func (h *Handler) CancelPull(w http.ResponseWriter, r *http.Request) {
	if !auth.InstanceAdminFromContext(r.Context()) {
		httputil.Error(w, 403, "forbidden", "Instance admin required")
		return
	}
	if !h.pullManager().Cancel() {
		httputil.Error(w, 409, "no_running_pull", "No running documentation pull")
		return
	}
	job, _ := h.pullManager().Current()
	httputil.JSON(w, 202, PullResponse{Job: job})
}

// source is separate staging metadata so the source-neutral Plan stays intact.
func importSource(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "source"))
	if err == nil {
		switch string(data) {
		case "bookstack", "wikijs", "markdown":
			return string(data)
		}
	}
	return "markdown"
}

package alerts

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
)

// maxDraftDiffChars bounds how much of a change diff is quoted in a draft.
const maxDraftDiffChars = 2000

// RunbookDraft is the response of POST /alerts/{id}/draft-runbook: a
// deterministic starting point for a runbook. Nothing is persisted; the
// operator edits it and creates a runbook through the normal runbook API.
type RunbookDraft struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	TargetType  string `json:"targetType"`
	TargetValue string `json:"targetValue"`
	// ExistingRunbookID is set when a runbook is already bound to the
	// suggested target, so the UI can offer editing it instead.
	ExistingRunbookID string `json:"existingRunbookId,omitempty"`
}

// DraftRunbook handles POST /api/alerts/{id}/draft-runbook. It builds the
// draft from the alert, its linked change (summary and diff) and any runbook
// already bound to the target, with no AI involved, so it works whether or not
// an AI provider is configured. Requires operator access to the alert's
// connector.
func (h *Handler) DraftRunbook(w http.ResponseWriter, r *http.Request) {
	a, err := h.Store.GetAlert(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		httputil.Error(w, http.StatusNotFound, "not_found", "Alert not found")
		return
	}
	if err != nil {
		httputil.Errorf(w, err)
		return
	}
	if !h.operatorOrForbidden(w, r, a.ServiceID) {
		return
	}

	serviceName := ""
	if conn, err := h.Store.GetConnector(r.Context(), a.ServiceID); err == nil {
		serviceName = conn.Name
	}

	var change *store.ChangeRecord
	if a.ChangeID != "" {
		c, err := h.Store.GetChange(r.Context(), a.ChangeID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			httputil.Errorf(w, err)
			return
		}
		change = c
	}

	targetType, targetValue := "alert_severity", a.Severity
	if change != nil && change.ChangeType != "" {
		targetType, targetValue = "change_type", change.ChangeType
	}
	existing, err := h.Store.GetRunbookByTarget(r.Context(), targetType, targetValue)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		httputil.Errorf(w, err)
		return
	}

	draft := buildRunbookDraft(a, serviceName, change, existing, targetType, targetValue)
	httputil.JSON(w, http.StatusOK, draft)
}

// buildRunbookDraft renders the deterministic runbook template.
func buildRunbookDraft(a *store.AlertRecord, serviceName string, change *store.ChangeRecord, existing *store.RunbookRecord, targetType, targetValue string) RunbookDraft {
	subject := serviceName
	if subject == "" {
		subject = "service"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Trigger\n\nAlert: %s (severity: %s) on %s.\n", a.Title, a.Severity, subject)
	if a.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", a.Description)
	}

	if change != nil {
		fmt.Fprintf(&b, "\n## Related change\n\n%s (type: %s, severity: %s).\n", change.Summary, change.ChangeType, change.Severity)
		if diff := strings.TrimSpace(change.Diff); diff != "" && diff != "{}" && diff != "[]" {
			if len(diff) > maxDraftDiffChars {
				diff = diff[:maxDraftDiffChars] + "\n... (truncated)"
			}
			fmt.Fprintf(&b, "\nDiff:\n\n```\n%s\n```\n", diff)
		}
	}

	b.WriteString("\n## Investigation\n\n- [ ] Confirm the alert is still firing and note when it started.\n")
	if change != nil {
		b.WriteString("- [ ] Review the change above and confirm whether it was expected.\n")
	}
	fmt.Fprintf(&b, "- [ ] Check the health and recent syncs of %s.\n", subject)

	b.WriteString("\n## Remediation\n\n- [ ] _Describe the steps that resolve this alert._\n")
	b.WriteString("\n## Verification\n\n- [ ] Confirm the alert clears after the next sync.\n")

	if existing != nil {
		fmt.Fprintf(&b, "\n## Existing runbook\n\nA runbook already covers this target: \"%s\". Consider updating it instead of creating a new one.\n", existing.Title)
	}

	d := RunbookDraft{
		Title:       "Runbook: " + a.Title,
		Body:        b.String(),
		TargetType:  targetType,
		TargetValue: targetValue,
	}
	if existing != nil {
		d.ExistingRunbookID = existing.ID
	}
	return d
}

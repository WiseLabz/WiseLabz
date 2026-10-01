package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Doc edit proposal statuses.
const (
	ProposalPending  = "pending"
	ProposalApproved = "approved"
	ProposalRejected = "rejected"
)

// DocEditProposalTrigger is the doc_versions trigger recorded when an
// approved proposal is applied.
const DocEditProposalTrigger = "proposal"

// DocEditProposal is a suggested replacement body for a doc, awaiting review.
// It never touches the doc until a reviewer approves it, and approval only
// succeeds while the doc is still at BaseVersion. DocTitle and ServiceID are
// read-only joins from docs.
type DocEditProposal struct {
	ID          string `json:"id"`
	DocID       string `json:"docId"`
	DocTitle    string `json:"docTitle"`
	ServiceID   string `json:"serviceId"`
	BaseVersion int    `json:"baseVersion"`
	Content     string `json:"content"`
	Summary     string `json:"summary"`
	AuthorID    string `json:"authorId"`
	Status      string `json:"status"`
	ReviewerID  string `json:"reviewerId,omitempty"`
	ReviewedAt  string `json:"reviewedAt,omitempty"`
	CreatedAt   string `json:"createdAt"`
}

const docEditProposalColumns = `p.id, p.doc_id, d.title, d.service_id, p.base_version, p.content, p.summary,
	p.author_id, p.status, p.reviewer_id, p.reviewed_at, p.created_at`

func scanDocEditProposal(row rowScanner) (*DocEditProposal, error) {
	var p DocEditProposal
	var svc, reviewer, reviewedAt sql.NullString
	err := row.Scan(&p.ID, &p.DocID, &p.DocTitle, &svc, &p.BaseVersion, &p.Content, &p.Summary,
		&p.AuthorID, &p.Status, &reviewer, &reviewedAt, &p.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.ServiceID, p.ReviewerID, p.ReviewedAt = svc.String, reviewer.String, reviewedAt.String
	return &p, nil
}

// MaxPendingProposalsPerAuthor bounds how many pending proposals one author
// may hold at once, so an automated client cannot grow the table (and the
// reviewers' queue) without limit. A new proposal by the same author for the
// same doc replaces their older pending one and does not count against it.
const MaxPendingProposalsPerAuthor = 25

// ErrProposalLimit is returned by CreateDocEditProposal when the author
// already holds MaxPendingProposalsPerAuthor pending proposals on other docs.
var ErrProposalLimit = fmt.Errorf("too many pending doc edit proposals (limit %d): wait for review or reuse an existing one", MaxPendingProposalsPerAuthor)

// CreateDocEditProposal stores a pending proposal. The doc must exist. Any
// earlier pending proposal by the same author for the same doc is superseded
// (deleted) in the same transaction, and ErrProposalLimit is returned when the
// author would exceed MaxPendingProposalsPerAuthor. The count-then-insert is
// not serialized across concurrent writers, so the cap is a soft bound.
func (s *Store) CreateDocEditProposal(ctx context.Context, p *DocEditProposal) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	p.Status = ProposalPending
	return s.WithinTransaction(ctx, func(tx *Store) error {
		if _, err := tx.db.ExecContext(ctx,
			`DELETE FROM doc_edit_proposals WHERE doc_id = ? AND author_id = ? AND status = ?`,
			p.DocID, p.AuthorID, ProposalPending); err != nil {
			return fmt.Errorf("supersede doc edit proposals: %w", err)
		}
		var pending int
		if err := tx.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM doc_edit_proposals WHERE author_id = ? AND status = ?`,
			p.AuthorID, ProposalPending).Scan(&pending); err != nil {
			return fmt.Errorf("count pending doc edit proposals: %w", err)
		}
		if pending >= MaxPendingProposalsPerAuthor {
			return ErrProposalLimit
		}
		if _, err := tx.db.ExecContext(ctx, `
			INSERT INTO doc_edit_proposals (id, doc_id, base_version, content, summary, author_id, status, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, p.ID, p.DocID, p.BaseVersion, p.Content, p.Summary, p.AuthorID, p.Status, p.CreatedAt); err != nil {
			return fmt.Errorf("create doc edit proposal: %w", err)
		}
		return nil
	})
}

// GetDocEditProposal retrieves one proposal.
func (s *Store) GetDocEditProposal(ctx context.Context, id string) (*DocEditProposal, error) {
	p, err := scanDocEditProposal(s.db.QueryRowContext(ctx, `
		SELECT `+docEditProposalColumns+`
		FROM doc_edit_proposals p JOIN docs d ON d.id = p.doc_id WHERE p.id = ?`, id))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("get doc edit proposal: %w", err)
	}
	return p, nil
}

// ProposalScope narrows a proposal listing to the docs a caller may review.
// All skips the filter (admin tooling and tests); otherwise a proposal is
// included when its doc is lab-wide and LabWide is set, or its doc's
// connector is in ConnectorIDs.
type ProposalScope struct {
	All          bool
	LabWide      bool
	ConnectorIDs []string
}

// ListDocEditProposals returns one page of proposals filtered by status ("" for
// all) and scope, newest first, plus the total matching count. Filtering and
// paging happen in SQL, and Content is left empty (use GetDocEditProposal for
// the body) so a long queue of large proposals is never loaded into memory.
func (s *Store) ListDocEditProposals(ctx context.Context, status string, scope ProposalScope, limit, offset int) ([]DocEditProposal, int, error) {
	where := ` WHERE 1 = 1`
	var args []any
	if status != "" {
		where += ` AND p.status = ?`
		args = append(args, status)
	}
	if !scope.All {
		var conds []string
		if scope.LabWide {
			conds = append(conds, `d.service_id IS NULL`, `d.service_id = ''`)
		}
		if len(scope.ConnectorIDs) > 0 {
			conds = append(conds, `d.service_id IN (`+placeholders(len(scope.ConnectorIDs))+`)`)
			for _, id := range scope.ConnectorIDs {
				args = append(args, id)
			}
		}
		if len(conds) == 0 {
			return []DocEditProposal{}, 0, nil
		}
		where += ` AND (` + strings.Join(conds, ` OR `) + `)`
	}
	from := ` FROM doc_edit_proposals p JOIN docs d ON d.id = p.doc_id` + where

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count doc edit proposals: %w", err)
	}

	query := `SELECT ` + strings.Replace(docEditProposalColumns, "p.content", "'' AS content", 1) + from +
		` ORDER BY p.created_at DESC, p.id LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list doc edit proposals: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	out := []DocEditProposal{}
	for rows.Next() {
		p, err := scanDocEditProposal(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan doc edit proposal: %w", err)
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate doc edit proposals: %w", err)
	}
	return out, total, nil
}

// ApproveDocEditProposal applies a pending proposal to its doc as a new
// version (author = reviewerID) and marks it approved, atomically. It returns
// the new doc revision. ErrNotFound: no such proposal (or doc). ErrConflict:
// the proposal is no longer pending. ErrVersionConflict: the doc changed
// since the proposal's base version; nothing is changed and the proposal
// stays pending.
func (s *Store) ApproveDocEditProposal(ctx context.Context, id, reviewerID string) (int, error) {
	var rev int
	err := s.WithinTransaction(ctx, func(tx *Store) error {
		now := time.Now().UTC().Format(time.RFC3339)
		var docID, content string
		var base int
		err := tx.db.QueryRowContext(ctx, `
			UPDATE doc_edit_proposals SET status = ?, reviewer_id = ?, reviewed_at = ?
			WHERE id = ? AND status = ?
			RETURNING doc_id, base_version, content
		`, ProposalApproved, reviewerID, now, id, ProposalPending).Scan(&docID, &base, &content)
		if errors.Is(err, sql.ErrNoRows) {
			if _, getErr := tx.GetDocEditProposal(ctx, id); getErr != nil {
				return getErr
			}
			return ErrConflict
		}
		if err != nil {
			return fmt.Errorf("claim doc edit proposal: %w", err)
		}
		rev, err = tx.updateDocRev(ctx, docID, content, &base)
		if err != nil {
			return err
		}
		return tx.CreateDocVersion(ctx, &DocVersionRecord{
			DocID: docID, Rev: rev, Content: content, Author: reviewerID, Trigger: DocEditProposalTrigger,
		})
	})
	if err != nil {
		return 0, err
	}
	return rev, nil
}

// RejectDocEditProposal marks a pending proposal rejected without touching
// the doc. ErrNotFound if missing, ErrConflict if no longer pending.
func (s *Store) RejectDocEditProposal(ctx context.Context, id, reviewerID string) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE doc_edit_proposals SET status = ?, reviewer_id = ?, reviewed_at = ?
		WHERE id = ? AND status = ?
	`, ProposalRejected, reviewerID, time.Now().UTC().Format(time.RFC3339), id, ProposalPending)
	if err != nil {
		return fmt.Errorf("reject doc edit proposal: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		if _, getErr := s.GetDocEditProposal(ctx, id); getErr != nil {
			return getErr
		}
		return ErrConflict
	}
	return nil
}

package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/WiseLabz/wiselabz/internal/auth"
)

// Search hit types returned by SearchContent.
const (
	SearchHitDoc     = "doc"
	SearchHitRunbook = "runbook"
)

// SearchHit is one full-text search result over docs and runbooks.
type SearchHit struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	ServiceID string  `json:"serviceId,omitempty"`
	Snippet   string  `json:"snippet"`
	Score     float64 `json:"score"`
}

const searchSnippetChars = 200

var searchTokenRE = regexp.MustCompile(`[\p{L}\p{N}]+`)

// searchStopwords mirrors the common core of PostgreSQL's 'english' text
// search stopword list. Postgres drops these at both index and query time;
// the SQLite FTS5 index keeps them, so both engines drop them from the query
// here and a stopword-only query returns no hits (not an error) everywhere.
var searchStopwords = func() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(`i me my myself we our ours ourselves you your yours yourself yourselves
		he him his himself she her hers herself it its itself they them their theirs themselves what which who whom
		this that these those am is are was were be been being have has had having do does did doing a an the and
		but if or because as until while of at by for with about against between into through during before after
		above below to from up down in out on off over under again further then once here there when where why how
		all any both each few more most other some such no nor not only own same so than too very s t can will just
		don should now`) {
		m[w] = true
	}
	return m
}()

// searchTokens splits a free-text query into lowercase word tokens, dropping
// stopwords and capping the count. Only letters and digits survive, so the
// result is safe to embed in an FTS5 or tsquery expression.
func searchTokens(query string) []string {
	raw := searchTokenRE.FindAllString(strings.ToLower(query), -1)
	tokens := make([]string, 0, len(raw))
	for _, t := range raw {
		if searchStopwords[t] {
			continue
		}
		tokens = append(tokens, t)
		if len(tokens) == 16 {
			break
		}
	}
	return tokens
}

// pgTSQuery builds the Postgres equivalent of ftsMatchExpr: tokens ANDed, the
// last one a prefix match, for to_tsquery('english', ...). Using the same
// sanitized tokens as SQLite (instead of websearch_to_tsquery on raw input)
// keeps operator handling (OR, -, quotes) and prefix behavior identical.
func pgTSQuery(tokens []string) string {
	return strings.Join(tokens, " & ") + ":*"
}

// ftsMatchExpr turns tokens into a safe FTS5 MATCH expression: every token
// is quoted (so user input can never inject FTS syntax) and ANDed, with the
// last one treated as a prefix so type-ahead style queries work.
func ftsMatchExpr(tokens []string) string {
	parts := make([]string, len(tokens))
	for i, t := range tokens {
		parts[i] = `"` + t + `"`
	}
	parts[len(parts)-1] += "*"
	return strings.Join(parts, " ")
}

// SearchContent runs a full-text search over docs and runbooks and returns at
// most limit hits ranked by relevance. Docs are limited to those userID may
// view (same rule as ListViewableDocs: a viewer grant on the doc's connector,
// honouring API-key restrictions, plus human lab notes and admin-only inventory).
// Runbooks are global, like the runbooks REST list; their connector-bound
// steps are redacted by the caller, not matched here.
func (s *Store) SearchContent(ctx context.Context, userID, query string, limit int) ([]SearchHit, error) {
	tokens := searchTokens(query)
	if len(tokens) == 0 {
		return []SearchHit{}, nil
	}
	if limit <= 0 {
		limit = 20
	}

	docs, err := s.searchDocs(ctx, userID, tokens, limit)
	if err != nil {
		return nil, err
	}
	runbooks, err := s.searchRunbooks(ctx, tokens, limit)
	if err != nil {
		return nil, err
	}

	return interleaveHits(normalizeScores(docs), normalizeScores(runbooks), limit), nil
}

// normalizeScores rescales a result set's engine scores (bm25 on SQLite,
// ts_rank on Postgres) to 0..1 relative to its own best hit. Raw scores are
// only comparable within one query on one table, so they are never mixed
// across docs and runbooks.
func normalizeScores(hits []SearchHit) []SearchHit {
	if len(hits) == 0 {
		return hits
	}
	lo, hi := hits[0].Score, hits[0].Score
	for _, h := range hits {
		lo, hi = min(lo, h.Score), max(hi, h.Score)
	}
	for i := range hits {
		if hi == lo {
			hits[i].Score = 1
			continue
		}
		hits[i].Score = 0.1 + 0.9*(hits[i].Score-lo)/(hi-lo)
	}
	return hits
}

// interleaveHits merges two already-ranked lists by rank position (best doc,
// best runbook, second doc, ...) up to limit. Interleaving by rank, rather
// than sorting on score, avoids comparing bm25/ts_rank values that come from
// different corpora, and behaves the same on both engines.
func interleaveHits(docs, runbooks []SearchHit, limit int) []SearchHit {
	out := make([]SearchHit, 0, min(limit, len(docs)+len(runbooks)))
	for i := 0; len(out) < limit && (i < len(docs) || i < len(runbooks)); i++ {
		if i < len(docs) {
			out = append(out, docs[i])
		}
		if i < len(runbooks) && len(out) < limit {
			out = append(out, runbooks[i])
		}
	}
	return out
}

func (s *Store) searchDocs(ctx context.Context, userID string, tokens []string, limit int) ([]SearchHit, error) {
	keyFilter, keyArgs := apiKeyConnectorFilter(ctx, "d.service_id")
	viewable := `(d.service_id IN (SELECT connector_id FROM user_connector_roles WHERE user_id = ?` + keyFilter + `)`
	if auth.InstanceAdminFromContext(ctx) {
		viewable += ` OR d.service_id IS NULL OR d.service_id = ''`
	}
	if !auth.InstanceAdminFromContext(ctx) {
		viewable += ` OR ((d.service_id IS NULL OR d.service_id = '') AND d.origin = 'human')`
	}
	viewable += `) AND d.deleted_at IS NULL`

	var sqlText string
	var args []any
	if s.driver == "postgres" {
		sqlText = `SELECT d.id, d.title, d.service_id, d.content, ts_rank(d.search_tsv, q.q)
			FROM docs d, to_tsquery('english', ?) q
			WHERE d.search_tsv @@ q.q AND ` + viewable + `
			ORDER BY 5 DESC, d.id LIMIT ?`
		args = append(args, pgTSQuery(tokens), userID)
	} else {
		sqlText = `SELECT d.id, d.title, d.service_id, d.content, -bm25(docs_fts, 0.0, 5.0, 1.0)
			FROM docs_fts JOIN docs d ON d.id = docs_fts.id
			WHERE docs_fts MATCH ? AND ` + viewable + `
			ORDER BY 5 DESC, d.id LIMIT ?`
		args = append(args, ftsMatchExpr(tokens), userID)
	}
	args = append(args, keyArgs...)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("search docs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	hits := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var svc sql.NullString
		var content string
		if err := rows.Scan(&h.ID, &h.Title, &svc, &content, &h.Score); err != nil {
			return nil, fmt.Errorf("scan doc hit: %w", err)
		}
		h.Type, h.ServiceID, h.Snippet = SearchHitDoc, svc.String, snippetFor(content, tokens)
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate doc hits: %w", err)
	}
	return hits, nil
}

func (s *Store) searchRunbooks(ctx context.Context, tokens []string, limit int) ([]SearchHit, error) {
	var sqlText string
	var args []any
	if s.driver == "postgres" {
		sqlText = `SELECT r.id, r.title, r.body, ts_rank(r.search_tsv, q.q)
			FROM runbooks r, to_tsquery('english', ?) q
			WHERE r.search_tsv @@ q.q
			ORDER BY 4 DESC, r.id LIMIT ?`
		args = []any{pgTSQuery(tokens), limit}
	} else {
		sqlText = `SELECT r.id, r.title, r.body, -bm25(runbooks_fts, 0.0, 5.0, 1.0)
			FROM runbooks_fts JOIN runbooks r ON r.id = runbooks_fts.id
			WHERE runbooks_fts MATCH ?
			ORDER BY 4 DESC, r.id LIMIT ?`
		args = []any{ftsMatchExpr(tokens), limit}
	}

	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("search runbooks: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	hits := []SearchHit{}
	for rows.Next() {
		var h SearchHit
		var body string
		if err := rows.Scan(&h.ID, &h.Title, &body, &h.Score); err != nil {
			return nil, fmt.Errorf("scan runbook hit: %w", err)
		}
		h.Type, h.Snippet = SearchHitRunbook, snippetFor(body, tokens)
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate runbook hits: %w", err)
	}
	return hits, nil
}

// snippetFor returns a short excerpt of text around the first token it
// contains (or the start of the text when none is found verbatim).
func snippetFor(text string, tokens []string) string {
	lower := strings.ToLower(text)
	pos := -1
	for _, t := range tokens {
		if i := strings.Index(lower, t); i >= 0 && (pos < 0 || i < pos) {
			pos = i
		}
	}
	start := 0
	if pos > 60 {
		start = pos - 60
	}
	// Positions come from the lowercased copy; clamp and re-align to a rune
	// boundary of the original in case lowercasing changed byte lengths.
	if start > len(text) {
		start = 0
	}
	for start > 0 && !utf8.RuneStart(text[start]) {
		start--
	}
	end := start + searchSnippetChars
	if end > len(text) {
		end = len(text)
	}
	for end < len(text) && !utf8.RuneStart(text[end]) {
		end++
	}
	out := strings.Join(strings.Fields(text[start:end]), " ")
	if start > 0 {
		out = "…" + out
	}
	if end < len(text) {
		out += "…"
	}
	return out
}

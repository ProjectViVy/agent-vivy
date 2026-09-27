package bml

import (
	"context"
	"strings"
	"unicode"
)

// maxSearchHits caps search fan-out, matching limit.min(100) in
// typed_store.rs search/search_visible.
const maxSearchHits uint32 = 100

// SearchHit is a bounded FTS5 candidate. Rank is the SQLite bm25 score
// (lower is better); ranking is not an authority decision.
// (MemorySearchHit in typed_store.rs:103.)
type SearchHit struct {
	Record StoredRecord
	Rank   float64
}

// SearchQuery is a bounded FTS5 search request: free-text query, the scope
// the caller is allowed to see, tombstone visibility, and a hit cap.
type SearchQuery struct {
	Text              string
	Scope             Scope
	IncludeTombstoned bool
	Limit             uint32
}

const (
	// searchSQL is byte-faithful to typed_store.rs search() except the
	// tombstone clause, which is bound so IncludeTombstoned can lift it.
	searchSQL = `SELECT r.record_revision, r.record_json, bm25(memory_fts) AS rank
             FROM memory_fts
             JOIN memory_records r ON r.memory_id = memory_fts.memory_id
             WHERE memory_fts MATCH ?
               AND memory_fts.tenant_id = ?
               AND memory_fts.workspace_id = ?
               AND ((? IS NULL AND memory_fts.session_id IS NULL) OR memory_fts.session_id = ?)
               AND (r.tombstone = 0 OR ?)
             ORDER BY rank, r.memory_id
             LIMIT ?`
	// searchVisibleSQL is byte-faithful to search_visible() with the same
	// bound tombstone clause.
	searchVisibleSQL = `SELECT r.record_revision, r.record_json, bm25(memory_fts) AS rank
             FROM memory_fts
             JOIN memory_records r ON r.memory_id = memory_fts.memory_id
             WHERE memory_fts MATCH ?
               AND memory_fts.tenant_id = ?
               AND memory_fts.workspace_id = ?
               AND (memory_fts.session_id IS NULL OR memory_fts.session_id = ?)
               AND (r.tombstone = 0 OR ?)
             ORDER BY rank, r.effective_at DESC, r.memory_id
             LIMIT ?`
)

// Search returns FTS candidates restricted to an exact logical scope: a nil
// scope session matches only sessionless records, a set session matches only
// that session's records (typed_store.rs search).
func (s *Store) Search(ctx context.Context, q SearchQuery) ([]SearchHit, error) {
	return s.search(ctx, q, searchSQL, 2)
}

// SearchVisible returns workspace-global records plus records owned by the
// exact session — the visibility shape required by Recall. It intentionally
// does not change the exact-scope semantics of Search
// (typed_store.rs search_visible).
func (s *Store) SearchVisible(ctx context.Context, q SearchQuery) ([]SearchHit, error) {
	return s.search(ctx, q, searchVisibleSQL, 1)
}

func (s *Store) search(ctx context.Context, q SearchQuery, querySQL string, sessionBinds int) ([]SearchHit, error) {
	if q.Scope.WorkspaceID != s.workspaceID {
		return nil, workspaceMismatchErr(s.workspaceID, q.Scope.WorkspaceID)
	}
	match := fts5MatchQuery(q.Text)
	if match == "" {
		return []SearchHit{}, nil
	}
	limit := q.Limit
	if limit > maxSearchHits {
		limit = maxSearchHits
	}
	args := []any{match, q.Scope.TenantID, q.Scope.WorkspaceID}
	for i := 0; i < sessionBinds; i++ {
		args = append(args, q.Scope.SessionID)
	}
	args = append(args, boolInt(q.IncludeTombstoned), int64(limit))
	rows, err := s.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, persistenceErr(s.path, err)
	}
	defer rows.Close()
	out := []SearchHit{}
	for rows.Next() {
		var revision int64
		var recordJSON string
		var rank float64
		if err := rows.Scan(&revision, &recordJSON, &rank); err != nil {
			return nil, persistenceErr(s.path, err)
		}
		stored, err := decodeStored(revision, recordJSON)
		if err != nil {
			return nil, err
		}
		out = append(out, SearchHit{Record: stored, Rank: rank})
	}
	if err := rows.Err(); err != nil {
		return nil, persistenceErr(s.path, err)
	}
	return out, nil
}

// fts5MatchQuery renders free text as an FTS5 MATCH operand that can neither
// break nor widen the query: each whitespace-separated token that contains at
// least one letter or digit is wrapped in a quoted phrase (inner '"' doubled),
// and tokens that are pure FTS5 punctuation are dropped. Space-separated
// phrases keep the implicit-AND conjunction the Rust store relies on, while
// MATCH syntax (AND/OR/NOT, NEAR, column filters, prefix '*', '^', braces,
// parens, unbalanced quotes) is reduced to literal text. An empty result
// means nothing can match.
func fts5MatchQuery(text string) string {
	fields := strings.Fields(text)
	var b strings.Builder
	for _, f := range fields {
		if !hasWordRune(f) {
			continue
		}
		b.WriteByte('"')
		b.WriteString(strings.ReplaceAll(f, `"`, `""`))
		b.WriteString(`" `)
	}
	return strings.TrimRight(b.String(), " ")
}

func hasWordRune(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}) >= 0
}

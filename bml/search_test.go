package bml

import (
	"context"
	"testing"
	"time"
)

func sessionScopedRecord(id, workspace, sessionID, content string) Record {
	rec := storeRecord(id, workspace, content)
	rec.Scope.SessionID = &sessionID
	return rec
}

func searchQuery(text, tenant, workspace string, sessionID *string, limit uint32) SearchQuery {
	return SearchQuery{
		Text:  text,
		Scope: Scope{TenantID: tenant, WorkspaceID: workspace, SessionID: sessionID},
		Limit: limit,
	}
}

func hitIDs(hits []SearchHit) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.Record.Record.ID
	}
	return ids
}

func TestSearchReturnsRankedHits(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("a", "workspace-1", "alpha durable"), 0, nil); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if _, err := s.Put(ctx, storeRecord("b", "workspace-1", "durable durable durable beta"), 1, nil); err != nil {
		t.Fatalf("Put b: %v", err)
	}
	if _, err := s.Put(ctx, storeRecord("c", "workspace-1", "unrelated content"), 2, nil); err != nil {
		t.Fatalf("Put c: %v", err)
	}

	hits, err := s.Search(ctx, searchQuery("durable", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search hits = %d, want 2 (%v)", len(hits), hitIDs(hits))
	}
	// bm25 ranks the higher-term-frequency row first.
	if hits[0].Record.Record.ID != "b" {
		t.Fatalf("hits[0] = %q, want b (order %v)", hits[0].Record.Record.ID, hitIDs(hits))
	}
	if hits[1].Record.Record.ID != "a" {
		t.Fatalf("hits[1] = %q, want a", hits[1].Record.Record.ID)
	}
	if !(hits[0].Rank < hits[1].Rank) {
		t.Fatalf("ranks not ascending by bm25: %v then %v", hits[0].Rank, hits[1].Rank)
	}
	if hits[0].Record.Revision != 1 || hits[0].Record.Record.Content != "durable durable durable beta" {
		t.Fatalf("hit record payload wrong: %+v", hits[0].Record)
	}
}

func TestSearchExactScopeFilters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("global", "workspace-1", "needle"), 0, nil); err != nil {
		t.Fatalf("Put global: %v", err)
	}
	if _, err := s.Put(ctx, sessionScopedRecord("s1", "workspace-1", "sess-1", "needle"), 1, nil); err != nil {
		t.Fatalf("Put s1: %v", err)
	}
	if _, err := s.Put(ctx, sessionScopedRecord("s2", "workspace-1", "sess-2", "needle"), 2, nil); err != nil {
		t.Fatalf("Put s2: %v", err)
	}
	otherTenant := storeRecord("t2", "workspace-1", "needle")
	otherTenant.Scope.TenantID = "tenant-2"
	if _, err := s.Put(ctx, otherTenant, 3, nil); err != nil {
		t.Fatalf("Put t2: %v", err)
	}

	sess1 := "sess-1"
	tests := []struct {
		name      string
		sessionID *string
		wantIDs   []string
	}{
		{"nil session matches global only", nil, []string{"global"}},
		{"exact session scope", &sess1, []string{"s1"}},
	}
	for _, tt := range tests {
		hits, err := s.Search(ctx, searchQuery("needle", "tenant-1", "workspace-1", tt.sessionID, 8))
		if err != nil {
			t.Fatalf("Search %s: %v", tt.name, err)
		}
		if len(hits) != len(tt.wantIDs) {
			t.Fatalf("Search %s ids = %v, want %v", tt.name, hitIDs(hits), tt.wantIDs)
		}
		for i, want := range tt.wantIDs {
			if hits[i].Record.Record.ID != want {
				t.Fatalf("Search %s ids = %v, want %v", tt.name, hitIDs(hits), tt.wantIDs)
			}
		}
	}
}

func TestSearchWorkspaceMismatch(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	_, err := s.Search(ctx, searchQuery("x", "tenant-1", "workspace-2", nil, 8))
	if code := storeErrCode(t, err); code != ErrWorkspaceMismatch {
		t.Fatalf("Search wrong workspace: code = %q, want %q", code, ErrWorkspaceMismatch)
	}
	_, err = s.SearchVisible(ctx, searchQuery("x", "tenant-1", "workspace-2", nil, 8))
	if code := storeErrCode(t, err); code != ErrWorkspaceMismatch {
		t.Fatalf("SearchVisible wrong workspace: code = %q, want %q", code, ErrWorkspaceMismatch)
	}
}

func TestSearchVisibleIncludesGlobalAndOwnSession(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("global", "workspace-1", "needle"), 0, nil); err != nil {
		t.Fatalf("Put global: %v", err)
	}
	if _, err := s.Put(ctx, sessionScopedRecord("s1", "workspace-1", "sess-1", "needle"), 1, nil); err != nil {
		t.Fatalf("Put s1: %v", err)
	}
	if _, err := s.Put(ctx, sessionScopedRecord("s2", "workspace-1", "sess-2", "needle"), 2, nil); err != nil {
		t.Fatalf("Put s2: %v", err)
	}

	sess1 := "sess-1"
	hits, err := s.SearchVisible(ctx, searchQuery("needle", "tenant-1", "workspace-1", &sess1, 8))
	if err != nil {
		t.Fatalf("SearchVisible: %v", err)
	}
	got := hitIDs(hits)
	if len(got) != 2 {
		t.Fatalf("SearchVisible ids = %v, want global+s1 only", got)
	}
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	if !seen["global"] || !seen["s1"] {
		t.Fatalf("SearchVisible ids = %v, want global+s1", got)
	}
}

func TestSearchVisibleOrdersNewestFirstOnRankTie(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	now := time.Now().UTC()
	older := storeRecord("older", "workspace-1", "same needle")
	older.CreatedAt = now.Add(-3 * time.Hour)
	older.EffectiveAt = now.Add(-2 * time.Hour)
	newer := storeRecord("newer", "workspace-1", "same needle")
	newer.CreatedAt = now.Add(-30 * time.Minute)
	newer.EffectiveAt = now.Add(-time.Minute)

	if _, err := s.Put(ctx, older, 0, nil); err != nil {
		t.Fatalf("Put older: %v", err)
	}
	if _, err := s.Put(ctx, newer, 1, nil); err != nil {
		t.Fatalf("Put newer: %v", err)
	}

	hits, err := s.SearchVisible(ctx, searchQuery("needle", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("SearchVisible: %v", err)
	}
	if len(hits) != 2 || hits[0].Record.Record.ID != "newer" || hits[1].Record.Record.ID != "older" {
		t.Fatalf("SearchVisible order = %v, want [newer older]", hitIDs(hits))
	}
}

func TestSearchExcludesTombstonedAndSuperseded(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("victim", "workspace-1", "needle secret"), 0, nil); err != nil {
		t.Fatalf("Put victim: %v", err)
	}
	if _, err := s.Put(ctx, storeRecord("alive", "workspace-1", "needle public"), 1, nil); err != nil {
		t.Fatalf("Put alive: %v", err)
	}
	if err := s.PutTombstone(ctx, tombstoneRecord("tom", "workspace-1", "victim"), 2, "victim", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}

	hits, err := s.Search(ctx, searchQuery("needle", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if got := hitIDs(hits); len(got) != 1 || got[0] != "alive" {
		t.Fatalf("Search ids = %v, want [alive]", got)
	}
	vis, err := s.SearchVisible(ctx, searchQuery("needle", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("SearchVisible: %v", err)
	}
	if got := hitIDs(vis); len(got) != 1 || got[0] != "alive" {
		t.Fatalf("SearchVisible ids = %v, want [alive]", got)
	}
}

func TestSearchIncludeTombstonedFlag(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("target", "workspace-1", "needle body"), 0, nil); err != nil {
		t.Fatalf("Put target: %v", err)
	}
	if err := s.PutTombstone(ctx, tombstoneRecord("tom", "workspace-1", "target"), 1, "target", 1); err != nil {
		t.Fatalf("PutTombstone: %v", err)
	}
	// A tombstone record normally has no FTS row; simulate a row surviving
	// (e.g. inconsistent import) to exercise the flag at SQL level.
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO memory_fts(memory_id, tenant_id, workspace_id, session_id, content) "+
			"VALUES ('tom','tenant-1','workspace-1',NULL,'needle marker')"); err != nil {
		t.Fatalf("insert tombstone fts row: %v", err)
	}

	hits, err := s.Search(ctx, searchQuery("needle", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("Search without flag returned tombstoned rows: %v", hitIDs(hits))
	}

	q := searchQuery("needle", "tenant-1", "workspace-1", nil, 8)
	q.IncludeTombstoned = true
	hits, err = s.Search(ctx, q)
	if err != nil {
		t.Fatalf("Search IncludeTombstoned: %v", err)
	}
	if got := hitIDs(hits); len(got) != 1 || got[0] != "tom" {
		t.Fatalf("Search IncludeTombstoned ids = %v, want [tom]", got)
	}
	if hits[0].Record.Record.Tombstone == nil {
		t.Fatalf("expected tombstone record payload, got %+v", hits[0].Record)
	}
}

func TestSearchSpecialFTS5CharsDoNotErrorOrWiden(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")

	if _, err := s.Put(ctx, storeRecord("doc", "workspace-1", "alpha beta gamma"), 0, nil); err != nil {
		t.Fatalf("Put doc: %v", err)
	}

	queries := []string{
		`alpha OR beta`,    // must not widen: 'or' is a literal token
		`alpha AND delta`,  // 'and' literal; delta absent -> no match
		`NOT alpha`,        // leading NOT must not error
		`NEAR(alpha beta)`, // NEAR syntax neutralized
		`alpha*`,           // prefix star treated literally
		`content:alpha`,    // column filter neutralized
		`"unclosed`,        // unbalanced quote must not error
		`"alpha beta"`,     // quotes are literal text, not phrase syntax
		`(alpha OR beta)`,  // parens neutralized
		`alpha ^beta`,      // start-of-doc anchor neutralized
		`alpha-`,           // trailing operator char
		`{braces} alpha`,   // column-set braces
	}
	for _, q := range queries {
		hits, err := s.Search(ctx, searchQuery(q, "tenant-1", "workspace-1", nil, 8))
		if err != nil {
			t.Fatalf("Search %q errored: %v", q, err)
		}
		// 'doc' contains only alpha/beta/gamma tokens; any query whose
		// escaped form introduces tokens not in the doc must not match.
		switch q {
		case `alpha OR beta`, `alpha AND delta`, `NOT alpha`, `NEAR(alpha beta)`, `(alpha OR beta)`, `{braces} alpha`, `"unclosed`:
			if len(hits) != 0 {
				t.Fatalf("Search %q widened/mismatched: %v", q, hitIDs(hits))
			}
		}
	}

	// Sanity: a plain multi-token AND query still matches.
	hits, err := s.Search(ctx, searchQuery("alpha beta", "tenant-1", "workspace-1", nil, 8))
	if err != nil {
		t.Fatalf("Search plain: %v", err)
	}
	if got := hitIDs(hits); len(got) != 1 || got[0] != "doc" {
		t.Fatalf("plain AND query ids = %v, want [doc]", got)
	}

	// A query that is entirely FTS5 punctuation must not error or match all.
	for _, q := range []string{`*`, `"`, `(`, `)`, `:`, `^`, `"""`} {
		hits, err := s.Search(ctx, searchQuery(q, "tenant-1", "workspace-1", nil, 8))
		if err != nil {
			t.Fatalf("Search %q errored: %v", q, err)
		}
		if len(hits) != 0 {
			t.Fatalf("Search %q matched %v, want none", q, hitIDs(hits))
		}
	}
}

func TestSearchEmptyTextReturnsNoHits(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")
	if _, err := s.Put(ctx, storeRecord("doc", "workspace-1", "alpha"), 0, nil); err != nil {
		t.Fatalf("Put: %v", err)
	}
	for _, text := range []string{"", "   ", "\t\n"} {
		hits, err := s.Search(ctx, searchQuery(text, "tenant-1", "workspace-1", nil, 8))
		if err != nil {
			t.Fatalf("Search %q errored: %v", text, err)
		}
		if len(hits) != 0 {
			t.Fatalf("Search %q returned %v, want none", text, hitIDs(hits))
		}
	}
}

func TestSearchLimitIsBounded(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t, "workspace-1")
	for i, id := range []string{"r1", "r2", "r3"} {
		if _, err := s.Put(ctx, storeRecord(id, "workspace-1", "needle"), int64(i), nil); err != nil {
			t.Fatalf("Put %s: %v", id, err)
		}
	}
	hits, err := s.Search(ctx, searchQuery("needle", "tenant-1", "workspace-1", nil, 2))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search limit 2 returned %d hits", len(hits))
	}
}

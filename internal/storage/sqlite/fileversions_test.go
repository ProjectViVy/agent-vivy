package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	"agent-vivy/internal/storage"
)

func TestFileVersionChainSemantics(t *testing.T) {
	b, err := Open(context.Background(), filepath.Join(t.TempDir(), "fv.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = b.Close() }()
	ctx := context.Background()
	sid := domain.SessionID("sess-fv")

	record := func(t *testing.T, path string, old, new string) {
		t.Helper()
		if err := b.RecordFileMutation(ctx, sid, "run-fv", path, []byte(old), []byte(new)); err != nil {
			t.Fatalf("RecordFileMutation(%s): %v", path, err)
		}
	}
	versions := func(t *testing.T, path string) []string {
		t.Helper()
		rows, err := b.db.QueryContext(ctx,
			`SELECT content FROM file_versions WHERE session_id = ? AND path = ? ORDER BY version`, sid, path)
		if err != nil {
			t.Fatalf("query versions: %v", err)
		}
		defer func() { _ = rows.Close() }()
		var out []string
		for rows.Next() {
			var content []byte
			if err := rows.Scan(&content); err != nil {
				t.Fatalf("scan version: %v", err)
			}
			out = append(out, string(content))
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate versions: %v", err)
		}
		return out
	}

	// First sighting archives the pre-mutation baseline, then the new
	// content.
	record(t, "a.txt", "", "v1")
	got := versions(t, "a.txt")
	if len(got) != 2 || got[0] != "" || got[1] != "v1" {
		t.Fatalf("first sighting chain = %v, want [\"\" v1]", got)
	}

	// A chain whose latest matches the pre-mutation content appends only
	// the new content.
	record(t, "a.txt", "v1", "v2")
	got = versions(t, "a.txt")
	if len(got) != 3 || got[2] != "v2" {
		t.Fatalf("second mutation chain = %v, want 3 rows ending v2", got)
	}

	// External modification inserts the on-disk intermediate state first.
	record(t, "a.txt", "user-edited", "v3")
	got = versions(t, "a.txt")
	if len(got) != 5 || got[3] != "user-edited" || got[4] != "v3" {
		t.Fatalf("external-modification chain = %v, want 5 rows with user-edited before v3", got)
	}

	// Identical pre/post content dedupes to a no-op.
	record(t, "a.txt", "v3", "v3")
	if got := versions(t, "a.txt"); len(got) != 5 {
		t.Fatalf("dedupe chain = %d rows, want 5", len(got))
	}

	// Retention keeps the newest FileVersionRetention versions.
	prev := ""
	for i := 0; i < storage.FileVersionRetention+5; i++ {
		next := strings.Repeat("x", i+1)
		record(t, "loop.txt", prev, next)
		prev = next
	}
	if got := versions(t, "loop.txt"); len(got) != storage.FileVersionRetention {
		t.Fatalf("retention kept %d rows, want %d", len(got), storage.FileVersionRetention)
	}

	// Oversized content is skipped, not truncated: a huge pre-mutation
	// baseline is dropped while the new content still lands, and a huge
	// post-mutation content appends nothing.
	huge := strings.Repeat("y", storage.FileVersionMaxBytes+1)
	record(t, "big.txt", huge, "small")
	if got := versions(t, "big.txt"); len(got) != 1 || got[0] != "small" {
		t.Fatalf("oversize baseline chain = %v, want [small]", got)
	}
	record(t, "big.txt", "small", huge)
	if got := versions(t, "big.txt"); len(got) != 1 {
		t.Fatalf("oversize post-content chain = %d rows, want 1", len(got))
	}
}

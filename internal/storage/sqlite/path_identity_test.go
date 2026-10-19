package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenPreservesLiteralDatabasePathIdentity(t *testing.T) {
	for _, tc := range []struct{ name, first, second string }{
		{"fragment", "journal#one.db", "journal#two.db"},
		{"query", "journal?one.db", "journal?two.db"},
		{"percent", "journal%23one.db", "journal#one.db"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			firstPath, secondPath := filepath.Join(root, tc.first), filepath.Join(root, tc.second)
			if runtime.GOOS == "windows" && tc.name == "query" {
				// '?' is an invalid Windows filename. Reject it as a literal
				// name instead of opening a shortened URI-derived database.
				for _, path := range []string{firstPath, secondPath} {
					b, err := Open(ctx, path)
					if err == nil {
						_ = b.Close()
						t.Fatalf("invalid Windows filename opened: %s", path)
					}
				}
				if _, err := os.Stat(filepath.Join(root, "journal")); !os.IsNotExist(err) {
					t.Fatalf("invalid filename created a shortened database: %v", err)
				}
				return
			}
			first, err := Open(ctx, firstPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = first.Close() })
			if err := first.Snapshot().Put(ctx, "path-identity", []byte("first profile"), 0); err != nil {
				t.Fatal(err)
			}
			second, err := Open(ctx, secondPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = second.Close() })
			value, version, err := second.Snapshot().Get(ctx, "path-identity")
			if err != nil || len(value) != 0 || version != 0 {
				t.Fatalf("distinct path read first profile: value=%s version=%d err=%v", value, version, err)
			}
			for _, path := range []string{firstPath, secondPath} {
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("requested literal database was not created: %s: %v", path, err)
				}
			}
		})
	}
}

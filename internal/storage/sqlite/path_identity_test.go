package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"agent-vivy/internal/domain"
)

func TestOpenPreservesLiteralFilename(t *testing.T) {
	for _, name := range []string{"hash#part.db", "percent%23part.db", "space name.db", "question?mode=memory.db"} {
		t.Run(name, func(t *testing.T) {
			// Windows forbids '?' in filenames; the Linux owner exercises it.
			path := filepath.Join(t.TempDir(), name)
			probe, err := os.Create(path)
			if err != nil {
				t.Skipf("host rejects filename: %v", err)
			}
			probe.Close()
			ctx := context.Background()
			b, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			if err := b.CreateSession(ctx, domain.Session{ID: "literal", Title: "saved"}); err != nil {
				t.Fatal(err)
			}
			if err := b.Close(); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(path)
			if err != nil || info.Size() == 0 {
				t.Fatalf("literal database missing: %v, %v", info, err)
			}
			b, err = Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			if _, err := b.GetSession(ctx, "literal"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"agent-vivy/internal/storage"
	"agent-vivy/internal/storage/sqlite"
	"github.com/google/uuid"
)

func TestIssue58SubjectIdentityStable(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "core.db")
	owner := filepath.Join(root, "garden", "actmem")
	b, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := ResolveSubjectBinding(ctx, b.Snapshot(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(first.SubjectID); err != nil {
		t.Fatalf("subject is not UUID: %v", first)
	}
	if first.DomainID == "" || first.SubjectID == "diva" {
		t.Fatalf("invalid independent identity: %+v", first)
	}
	// Presentation and persona metadata are independent snapshot keys.
	if err := b.Snapshot().Put(ctx, "persona/presentation", []byte(`{"name":"renamed","avatar":"new"}`), 0); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	again, err := ResolveSubjectBinding(ctx, b.Snapshot(), owner)
	if err != nil {
		t.Fatal(err)
	}
	if first != again {
		t.Fatalf("restart/presentation changed subject: %+v != %+v", first, again)
	}
	const n = 8
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Go(func() {
			got, err := ResolveSubjectBinding(ctx, b.Snapshot(), owner)
			if err != nil || got != first {
				t.Errorf("concurrent resolve = %+v, %v", got, err)
			}
		})
	}
	wg.Wait()
}

func TestIssue58SubjectOwnerPathMismatch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	b, err := sqlite.Open(ctx, filepath.Join(root, "core.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	owner := filepath.Join(root, "owner")
	if err := os.Mkdir(owner, 0700); err != nil {
		t.Fatal(err)
	}
	first, err := ResolveSubjectBinding(ctx, b.Snapshot(), owner)
	if err != nil {
		t.Fatal(err)
	}
	// A symbolic link is a reparse point and is not universally permitted:
	// Windows requires SeCreateSymbolicLinkPrivilege or an effective Developer
	// Mode. Probe the host first so a refusal is an explicit skip, never a
	// pass; the canonical-alias assertion still runs wherever links work
	// (Linux CI, Developer-Mode Windows).
	probeTarget := filepath.Join(root, "probe-target")
	if err := os.Mkdir(probeTarget, 0700); err != nil {
		t.Fatal(err)
	}
	probeLink := filepath.Join(root, "probe-link")
	if err := os.Symlink(probeTarget, probeLink); err != nil {
		t.Skipf("host does not permit symbolic links, so the canonical-alias assertion cannot run: %v", err)
	}
	if err := os.Remove(probeLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(probeTarget); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(owner, alias); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveSubjectBinding(ctx, b.Snapshot(), alias)
	if err != nil || got != first {
		t.Fatalf("canonical alias forked: %+v, %v", got, err)
	}
	for _, other := range []string{filepath.Join(root, "moved"), filepath.Join(root, "other")} {
		if _, err := ResolveSubjectBinding(ctx, b.Snapshot(), other); !errors.Is(err, storage.ErrSubjectBinding) {
			t.Fatalf("owner change accepted: %v", err)
		}
	}
}

func TestIssue58SubjectIdentityConcurrentFirstRegistration(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, "core.db")
	first, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	start := make(chan struct{})
	type result struct {
		binding storage.SubjectHistoryBinding
		err     error
	}
	results := make(chan result, 2)
	for _, store := range []storage.SnapshotStore{first.Snapshot(), second.Snapshot()} {
		go func() {
			<-start
			b, e := ResolveSubjectBinding(ctx, store, filepath.Join(root, "owner"))
			results <- result{b, e}
		}()
	}
	close(start)
	a, c := <-results, <-results
	if a.err != nil || c.err != nil || a.binding != c.binding {
		t.Fatalf("absent CAS forked or failed registration: %v (%v) %v (%v)", a.binding, a.err, c.binding, c.err)
	}
}

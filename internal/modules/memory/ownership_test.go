package memory

import (
	"agent-vivy/internal/config"
	"agent-vivy/sdk/port/contextsource"
	"context"
	"fmt"
	"github.com/ProjectViVy/agent-vivy/bml"
	"path/filepath"
	"sync"
	"testing"
)

func TestIssue58MemoryOwnedRootAndRelease(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cfg := config.Config{Storage: config.Storage{DataDir: root}}
	first, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	same := cfg
	same.Storage.DataDir = filepath.Join(root, ".")
	if got, err := Open(ctx, same); err != nil || got != first {
		t.Fatalf("same normalized owner=%p %v", got, err)
	}
	foreign := cfg
	foreign.Storage.DataDir = t.TempDir()
	if got, err := Open(ctx, foreign); err == nil || got != nil {
		t.Errorf("foreign root reused ambient Home: %p %v", got, err)
	}
	if Active() != first || !first.live() {
		t.Fatal("failed foreign acquisition closed original")
	}
	closer, ok := any(first).(interface{ Close() error })
	if !ok {
		t.Fatal("service has no identity-aware release")
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := Open(ctx, foreign)
	if err != nil {
		t.Fatal(err)
	}
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	if Active() != next || !next.live() {
		t.Fatal("stale owner release closed replacement")
	}
	if out := next.Add(ctx, bml.MemoryAddRequest{Content: "replacement live"}); out.Status != bml.CrudOutcomeApplied {
		t.Fatal(out)
	}
}

func TestIssue58BoundProviderCannotSubstituteForeignActive(t *testing.T) {
	ctx := context.Background()
	svc, err := Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })
	p := NewProvider()
	binder, ok := any(p).(interface{ BindMemoryService(*Service) error })
	if !ok {
		t.Fatal("generated memory provider cannot bind its actual App Home")
	}
	if err := binder.BindMemoryService(svc); err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(); err != nil {
		t.Fatal(err)
	}
	foreign, err := Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	if out := foreign.Add(ctx, bml.MemoryAddRequest{Content: "foreign secret needle"}); out.Status != bml.CrudOutcomeApplied {
		t.Fatal(out)
	}
	if page, err := p.Query(ctx, contextsource.Request{Query: "needle", Limit: 1}); err == nil || len(page.Candidates) > 0 {
		t.Fatalf("closed bound provider read replacement: %+v %v", page, err)
	}
}

func TestIssue58MemoryConcurrentOwnedAcquireRelease(t *testing.T) {
	ctx := context.Background()
	roots := []string{t.TempDir(), t.TempDir()}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range roots {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			owner := fmt.Sprintf("actual-lease-%d", i)
			for n := 0; n < 30; n++ {
				svc, err := OpenOwned(ctx, config.Config{Storage: config.Storage{DataDir: roots[i]}}, owner)
				if err != nil {
					continue
				}
				if svc.leaseOwner != owner || svc.home.ConfigDir() != roots[i] {
					errs <- fmt.Errorf("foreign owner returned by concurrent OpenOwned")
					return
				}
				if err := svc.Close(); err != nil {
					errs <- err
					return
				}
				if err := svc.Close(); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if Active() != nil {
		t.Fatal("concurrent release left a detached Home")
	}
}

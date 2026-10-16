package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	memorymodule "agent-vivy/internal/modules/memory"
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/controlaction"
	"agent-vivy/sdk/port/observer"
	"agent-vivy/sdk/port/tool"
	"github.com/ProjectViVy/agent-vivy/bml"
	gardenmemory "github.com/ProjectViVy/laputa/garden/memory"
)

func issue58WithoutMemory(a genassembly.RuntimeAssembly) genassembly.RuntimeAssembly {
	a.Manifest.Modules = slices.DeleteFunc(slices.Clone(a.Manifest.Modules), func(id string) bool { return strings.HasPrefix(id, "vivy/memory-bml") })
	a.Tools = slices.DeleteFunc(slices.Clone(a.Tools), func(p tool.ToolProvider) bool { return strings.HasPrefix(p.Definition().ID, "memory_") })
	a.Manifest.Tools = slices.DeleteFunc(slices.Clone(a.Manifest.Tools), func(id string) bool { return strings.HasPrefix(id, "memory_") })
	a.ContextSources = slices.DeleteFunc(slices.Clone(a.ContextSources), func(p contextsource.Provider) bool { return p.ID() == memorymodule.ProviderID })
	a.Manifest.ContextSources = slices.DeleteFunc(slices.Clone(a.Manifest.ContextSources), func(id string) bool { return id == memorymodule.ProviderID })
	a.RunObservers = slices.DeleteFunc(slices.Clone(a.RunObservers), func(p observer.RunProvider) bool { return p.ID() == memorymodule.ProviderID })
	a.Manifest.RunObservers = slices.DeleteFunc(slices.Clone(a.Manifest.RunObservers), func(id string) bool { return id == memorymodule.ProviderID })
	a.ActionSets = slices.DeleteFunc(slices.Clone(a.ActionSets), func(p controlaction.ProviderSet) bool { return p.ModuleID == memorymodule.ID })
	a.Manifest.Actions = slices.DeleteFunc(slices.Clone(a.Manifest.Actions), func(id string) bool { return strings.HasPrefix(id, "vivy.memory.") })
	a.Manifest.ContextSourcePolicies = slices.DeleteFunc(slices.Clone(a.Manifest.ContextSourcePolicies), func(p generation.ContextSourcePolicy) bool { return p.ProviderID == memorymodule.ProviderID })
	a.Manifest.RunObserverPolicies = slices.DeleteFunc(slices.Clone(a.Manifest.RunObserverPolicies), func(p generation.RunObserverPolicy) bool { return p.ProviderID == memorymodule.ProviderID })
	return a
}
func TestIssue58AppOwnedMemoryLifecycle(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("VIVY_USER_HOME", t.TempDir())
	ctx := context.Background()
	cfg := newDeepSeekTestConfig(t)
	if !assemblyHasModule(genassembly.BuildDefault().Manifest.Modules, memorymodule.ID) {
		foreign, err := memorymodule.Open(ctx, config.Config{Storage: config.Storage{DataDir: t.TempDir()}})
		if err != nil {
			t.Fatal(err)
		}
		defer foreign.Close()
		noMemory, err := NewWithAssembly(ctx, cfg, genassembly.BuildDefault(), WithoutEars(), WithoutGateway())
		if err != nil {
			t.Fatal(err)
		}
		selected, ok := noMemory.cognitive.(interface{ SubjectMemory() gardenmemory.Backend })
		if ok && selected.SubjectMemory() != nil {
			t.Fatal("no-BML generation prepared ambient BML sink")
		}
		if err := noMemory.Close(); err != nil {
			t.Fatal(err)
		}
		if memorymodule.Active() != foreign || !foreign.Status(ctx).Available {
			t.Fatal("actual no-BML App closed foreign Home")
		}
		return
	}
	originalAssembly := genassembly.BuildDefault()
	a, err := NewWithAssembly(ctx, cfg, originalAssembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	prepared, ok := a.cognitive.(interface{ SubjectMemory() gardenmemory.Backend })
	if !ok || prepared.SubjectMemory() == nil {
		t.Fatal("actual App did not prepare its selected C1-bound BML adapter")
	}
	var oldSource contextsource.Provider
	for _, p := range a.assembly.ContextSources {
		if p.ID() == memorymodule.ProviderID {
			oldSource = p
		}
	}
	first := memorymodule.Active()
	if first == nil {
		t.Skip("no BML in this generated composition; separate no-memory fixture covers release")
	}
	if _, err := NewWithAssembly(ctx, cfg, genassembly.BuildDefault(), WithoutEars(), WithoutGateway()); err == nil {
		t.Fatal("duplicate same-root App acquired storage lease")
	}
	if memorymodule.Active() != first || !first.Status(ctx).Available {
		t.Fatal("duplicate App closed first Home")
	}
	sameHome := cfg
	sameHome.Storage.SQLite.Path = filepath.Join(cfg.DataDirectory(), "different-core.db")
	if got, err := NewWithAssembly(ctx, sameHome, genassembly.BuildDefault(), WithoutEars(), WithoutGateway()); err == nil {
		_ = got.Close()
		t.Fatal("different storage lease borrowed same Home")
	}
	if memorymodule.Active() != first || !first.Status(ctx).Available {
		t.Fatal("failed same-Home lease acquisition closed original")
	}
	foreign := newDeepSeekTestConfig(t)
	other, err := NewWithAssembly(ctx, foreign, genassembly.BuildDefault(), WithoutEars(), WithoutGateway())
	if err == nil {
		_ = other.Close()
		t.Error("foreign App accepted ambient Home")
	}
	if memorymodule.Active() != first || !first.Status(ctx).Available {
		t.Fatal("foreign construction closed first Home")
	}
	without := issue58WithoutMemory(genassembly.BuildDefault())
	noMemory, err := NewWithAssembly(ctx, newDeepSeekTestConfig(t), without, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	if err := noMemory.Close(); err != nil {
		t.Fatal(err)
	}
	if memorymodule.Active() != first || !first.Status(ctx).Available {
		t.Fatal("real no-memory App closed foreign Home")
	}
	if out := first.Add(ctx, bml.MemoryAddRequest{Content: "owning App still live"}); out.Status != bml.CrudOutcomeApplied {
		t.Fatal(out)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if memorymodule.Active() != nil {
		t.Fatal("owning close did not detach Home")
	}
	reopened, err := NewWithAssembly(ctx, cfg, originalAssembly, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	if got := memorymodule.Active().Search(ctx, bml.MemorySearchRequest{Query: "owning"}); len(got.Entries) != 1 {
		t.Fatalf("reopen lost native record %+v", got)
	}
	if _, err := oldSource.Query(ctx, contextsource.Request{Query: "owning", Limit: 1}); err == nil {
		t.Fatal("old App source followed reopened Home")
	}
	for _, p := range reopened.assembly.ContextSources {
		if p.ID() == memorymodule.ProviderID {
			if page, err := p.Query(ctx, contextsource.Request{Query: "owning", Limit: 1}); err != nil || len(page.Candidates) != 1 {
				t.Fatalf("fresh source cannot read own reopened Home: %+v %v", page, err)
			}
		}
	}
	_ = reopened.Close()
	failed := genassembly.BuildDefault()
	failed.CognitiveFactory = func(context.Context, cognitivecontract.FactoryInput) (cognitivecontract.Bundle, error) {
		return nil, errors.New("injected factory failure")
	}
	if _, err := NewWithAssembly(ctx, newDeepSeekTestConfig(t), failed, WithoutEars(), WithoutGateway()); err == nil {
		t.Fatal("factory failure accepted")
	}
	if memorymodule.Active() != nil {
		t.Fatal("failed constructor retained Home")
	}
}

// Competing actual constructors share one core lease; a failed acquisition must
// leave the successful App's Home live, including after the loser unwinds.
func TestIssue58AppCompetingMemoryConstructors(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("VIVY_USER_HOME", t.TempDir())
	if !assemblyHasModule(genassembly.BuildDefault().Manifest.Modules, memorymodule.ID) {
		t.Skip("generated composition has no BML owner")
	}
	cfg := newDeepSeekTestConfig(t)
	type result struct {
		app *App
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a := genassembly.BuildDefault()
			<-start
			app, err := NewWithAssembly(context.Background(), cfg, a, WithoutEars(), WithoutGateway())
			results <- result{app, err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	var winner *App
	successes, failures := 0, 0
	for r := range results {
		if r.err != nil {
			failures++
		} else {
			successes++
			if winner != nil {
				_ = r.app.Close()
			}
			winner = r.app
		}
	}
	if winner != nil {
		defer winner.Close()
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("competing constructors: successes=%d failures=%d", successes, failures)
	}
	if winner.memory == nil || memorymodule.Active() != winner.memory || !winner.memory.Status(context.Background()).Available {
		t.Fatal("failed competitor closed or replaced winner Home")
	}
}

func TestIssue58AppRunReleasesOwnedMemory(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	t.Setenv("VIVY_USER_HOME", t.TempDir())
	cfg := newDeepSeekTestConfig(t)
	a, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if a.memory != nil && memorymodule.Active() != nil {
		t.Fatal("Run left its exact owned Home active")
	}
	b, err := New(context.Background(), cfg, WithoutEars(), WithoutGateway())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if a.memory != nil && a.memory == b.memory {
		t.Fatal("Run reopen reused old service")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if b.memory != nil && (memorymodule.Active() != b.memory || !b.memory.Status(context.Background()).Available) {
		t.Fatal("old Run App close detached reopened Home")
	}
}

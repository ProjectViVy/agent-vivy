package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	nb "agent-vivy/internal/notebookcontract"
	"agent-vivy/internal/storage/sqlite"
)

// The default recipe selects vivy/notebook-core, so the generated Assembly
// must carry the sealed factory seam and App must construct a live bundle
// from it through notebookBundleForAssembly.
func TestNotebookFactorySelectedInDefaultAssembly(t *testing.T) {
	assembly := genassembly.BuildDefault()
	if !assembly.HasNotebookFactory() {
		t.Fatal("default Assembly lacks notebook factory")
	}
	if _, ok := assembly.NotebookFactoryValue().(nb.Factory); !ok {
		t.Fatalf("NotebookFactoryValue type = %T", assembly.NotebookFactoryValue())
	}

	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "nb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	bundle, err := notebookBundleForAssembly(context.Background(), &assembly, "generation-test", backend)
	if err != nil {
		t.Fatalf("notebookBundleForAssembly: %v", err)
	}
	if bundle == nil {
		t.Fatal("selected module produced nil bundle")
	}
	t.Cleanup(func() { _ = bundle.Close(context.Background()) })

	// The bound facade must reach the real N1 store: create a note in Home.
	receipt, err := bundle.Actions(nb.HomeScopeID, nb.Actor{Kind: nb.ActorHuman, Ref: "peer:test"}).CreateEntry(context.Background(), nb.OperationKeyed[nb.CreateEntryRequest]{
		OperationKey: "test-create",
		Request:      nb.CreateEntryRequest{SectionID: "section-notes", Title: "hello", Markdown: "body"},
	})
	if err != nil {
		t.Fatalf("create via bundle facade: %v", err)
	}
	if receipt.ResourceID == "" {
		t.Fatal("empty resource id")
	}
}

// A composition whose manifest omits vivy/notebook-core constructs nothing —
// the capability is physically absent, not a nil-stubbed second path.
func TestNotebookFactoryOmittedComposition(t *testing.T) {
	assembly := genassembly.BuildDefault()
	kept := assembly.Manifest.Modules[:0]
	for _, id := range assembly.Manifest.Modules {
		if !strings.HasPrefix(id, "vivy/notebook") {
			kept = append(kept, id)
		}
	}
	assembly.Manifest.Modules = kept

	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "nb-off.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	bundle, err := notebookBundleForAssembly(context.Background(), &assembly, "generation-test", backend)
	if err != nil {
		t.Fatalf("omitted composition errored: %v", err)
	}
	if bundle != nil {
		t.Fatal("omitted composition produced a bundle")
	}
}

// The scope resolver is the trusted authority: workspace scopes resolve only
// for sessions the storage registry knows; forged or unknown session ids fail
// not_found before any version is revealed.
func TestNotebookScopeResolverGatesWorkspace(t *testing.T) {
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "nb-scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	resolver := notebookScopeResolver{engine: backend}
	if got := resolver.Home(); got != nb.HomeScopeID {
		t.Fatalf("Home = %q", got)
	}
	if _, err := resolver.ForSession(context.Background(), ""); err == nil {
		t.Fatal("empty session resolved")
	}
	if _, err := resolver.ForSession(context.Background(), "sess-forged"); err == nil {
		t.Fatal("unknown session resolved a workspace scope")
	}
	if err := backend.CreateSession(context.Background(), domain.Session{ID: "sess-real", Title: "real", CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	scope, err := resolver.ForSession(context.Background(), "sess-real")
	if err != nil {
		t.Fatalf("known session failed: %v", err)
	}
	if scope != nb.WorkspaceScope("sess-real") {
		t.Fatalf("scope = %q, want %q", scope, nb.WorkspaceScope("sess-real"))
	}
}

// A selected module whose emitted binding is mistyped fails init — App never
// degrades to a second construction path.
func TestNotebookFactoryMistypedBindingFails(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.NotebookFactory = nil
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "nb-mistyped.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	if _, err := notebookBundleForAssembly(context.Background(), &assembly, "generation-test", backend); err == nil {
		t.Fatal("nil factory in selected composition must fail")
	}
}

// A closed bundle reports capability_unavailable on mutation instead of a
// silent nil panic.
func TestNotebookBundleClosedHandle(t *testing.T) {
	assembly := genassembly.BuildDefault()
	backend, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "nb-closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	bundle, err := notebookBundleForAssembly(context.Background(), &assembly, "generation-test", backend)
	if err != nil {
		t.Fatal(err)
	}
	if err := bundle.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	_, err = bundle.Actions(nb.HomeScopeID, nb.Actor{Kind: nb.ActorHuman, Ref: "peer:test"}).CreateEntry(context.Background(), nb.OperationKeyed[nb.CreateEntryRequest]{
		OperationKey: "after-close",
		Request:      nb.CreateEntryRequest{SectionID: "section-notes", Title: "x", Markdown: "y"},
	})
	if err == nil {
		t.Fatal("closed bundle accepted a mutation")
	}
}

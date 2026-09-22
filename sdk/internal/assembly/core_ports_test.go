package assembly

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

func TestPublicSourceCannotProvideCorePort(t *testing.T) {
	descriptor := withProvides(testDescriptor("fixture/action-host"), module.PortRef{Port: "core/action-host@v1", ID: "fixture.action-host"})
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT2, RootlessFixture: true}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	_, err = compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}})
	if err == nil || !strings.Contains(err.Error(), "may only be provided by build-owned T1 module vivy/action-host") {
		t.Fatalf("public core Provider error = %v", err)
	}
}

func TestCanonicalCoreOwnerCanProvideActionHost(t *testing.T) {
	descriptor := withProvides(testDescriptor("vivy/action-host"), module.PortRef{Port: "core/action-host@v1", ID: "vivy.action-host"})
	descriptor.Source.Ref = "internal:vivy/action-host"
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT1}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	if _, err := compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}}); err != nil {
		t.Fatalf("canonical action Host rejected: %v", err)
	}
}

func TestSelectedPublicProviderRequiresConditionalCoreHost(t *testing.T) {
	publicPorts := []string{
		"std/context-source@v1", "std/skill-source@v1", "std/observer/run@v1",
		"std/observer/diagnostic@v1", "std/status-source@v1", "std/ui-extension@v1",
		"std/ui-root@v1", "std/control-action@v1",
	}
	for _, publicPort := range publicPorts {
		t.Run(publicPort, func(t *testing.T) {
			selected := map[string]SourceRecord{
				"fixture/provider": {Descriptor: withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: publicPort, ID: "fixture.provider"})},
			}
			providers := map[string][]SourceRecord{}
			for _, portName := range []string{
				"core/loop-driver@v1", "core/chat-model-host@v1", "core/tool-host@v1",
				"core/storage-engine@v1", "core/checkpoint-store@v1",
				"core/credential-resolver@v1", "core/sandbox-backend@v1",
			} {
				providers[portName] = []SourceRecord{{}}
			}
			diagnostics := validateClosedInternalSelection(selected, providers)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "requires conditional Host core/") {
				t.Fatalf("diagnostics = %v", diagnostics)
			}
		})
	}
}

func TestMaskCorePortRejectsPublicProvider(t *testing.T) {
	descriptor := withProvides(testDescriptor("fixture/masks"), module.PortRef{Port: "core/mask-service@v1", ID: "fixture.mask-service"})
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT2, RootlessFixture: true}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	startGuard := &maskStartGuard{}
	_, generation, err := compileThenStart(context.Background(), compiler, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}}, startGuard)
	if generation != nil {
		t.Fatal("invalid public mask Provider unexpectedly started a generation")
	}
	if err == nil || !strings.Contains(err.Error(), "core Port core/mask-service@v1 may only be provided by build-owned T1 module vivy/masks") {
		t.Fatalf("public mask Provider error = %v", err)
	}
	if startGuard.started != 0 {
		t.Fatalf("compiler rejection started a Provider %d times", startGuard.started)
	}
}

func TestMaskCorePortRejectsDuplicate(t *testing.T) {
	descriptor := testDescriptor("vivy/masks")
	descriptor.Source.Ref = "file:internal"
	descriptor.Provides = []module.PortRef{
		{Port: "core/mask-service@v1", ID: "vivy.mask-service"},
		{Port: "core/mask-service@v1", ID: "vivy.mask-service-alt"},
	}
	storage := testDescriptor("vivy/storage")
	storage.Source.Ref = "file:internal"
	storage.Provides = []module.PortRef{{Port: "core/storage-engine@v1", ID: "vivy.storage-engine"}}
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT1}, {Descriptor: storage, Trust: TrustT1}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	startGuard := &maskStartGuard{}
	_, generation, err := compileThenStart(context.Background(), compiler, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID, storage.Module.ID}}, startGuard)
	if generation != nil {
		t.Fatal("duplicate mask Provider unexpectedly started a generation")
	}
	if err == nil || !strings.Contains(err.Error(), "closed internal Port core/mask-service@v1 allows at most one Provider") {
		t.Fatalf("duplicate mask Provider error = %v", err)
	}
	if startGuard.started != 0 {
		t.Fatalf("compiler rejection started a Provider %d times", startGuard.started)
	}
}

func TestMaskBackendDoesNotRequireUI(t *testing.T) {
	descriptor := testDescriptor("vivy/masks")
	descriptor.Source.Ref = "file:internal"
	descriptor.Provides = []module.PortRef{{Port: "core/mask-service@v1", ID: "vivy.mask-service"}}
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: descriptor, Trust: TrustT1}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	startGuard := &maskStartGuard{}
	plan, generation, err := compileThenStart(context.Background(), compiler, Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{descriptor.Module.ID}}, startGuard)
	if err != nil {
		t.Fatalf("backend-only mask selection rejected: %v", err)
	}
	if generation == nil {
		t.Fatal("backend-only mask selection returned no started generation")
	}
	if startGuard.started != 1 {
		t.Fatalf("successful mask compile started %d times, want 1", startGuard.started)
	}
	if len(plan.Modules) != 1 || descriptorProvides(plan.Modules[0].Descriptor, "std/ui-extension@v1") || descriptorProvides(plan.Modules[0].Descriptor, "std/ui-root@v1") {
		t.Fatalf("mask backend acquired a UI dependency: %#v", plan.Modules)
	}
	if err := generation.Close(context.Background()); err != nil {
		t.Fatalf("close successful mask generation: %v", err)
	}
}

func TestMaskProductionSelectionRejectedBeforeGenerationStart(t *testing.T) {
	mask := testDescriptor("vivy/masks")
	mask.Source.Ref = "file:internal"
	mask.Provides = []module.PortRef{{Port: "core/mask-service@v1", ID: "vivy.mask-service"}}
	storage := testDescriptor("vivy/storage")
	storage.Source.Ref = "file:internal"
	storage.Provides = []module.PortRef{{Port: "core/storage-engine@v1", ID: "vivy.storage-engine"}}
	catalog, err := NewSourceCatalog([]SourceRecord{
		{Descriptor: mask, Trust: TrustT1},
		{Descriptor: storage, Trust: TrustT1},
	})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: SupportedPortEvidence(), ConformanceResults: SupportedPortConformance()}
	startGuard := &maskStartGuard{}
	_, generation, err := compileThenStart(context.Background(), compiler, Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{storage.Module.ID, mask.Module.ID},
	}, startGuard)
	if generation != nil {
		t.Fatal("SPECIFIED mask Provider unexpectedly started a generation")
	}
	if err == nil || !strings.Contains(err.Error(), "core/mask-service@v1 is SPECIFIED and cannot be selected") {
		t.Fatalf("production mask selection error = %v", err)
	}
	if startGuard.started != 0 {
		t.Fatalf("production compile rejection started a Provider %d times", startGuard.started)
	}
}

func compileThenStart(ctx context.Context, compiler Compiler, recipe Recipe, owners ...module.Instance) (AssemblyPlan, *module.Generation, error) {
	plan, err := compiler.Compile(ctx, recipe)
	if err != nil {
		return AssemblyPlan{}, nil, err
	}
	generation, err := module.StartGeneration(ctx, owners)
	if err != nil {
		return plan, nil, err
	}
	return plan, generation, nil
}

// maskStartGuard is deliberately not an input to Compiler: Compile accepts
// descriptors and source metadata only. The compileThenStart pipeline passes
// it to the lifecycle only after a successful compile, making both rejection
// and successful-start counters observable.
type maskStartGuard struct{ started int }

var _ module.Instance = (*maskStartGuard)(nil)

func (guard *maskStartGuard) Start(context.Context) error {
	guard.started++
	return nil
}

func (guard *maskStartGuard) Ready(context.Context) error { return nil }
func (guard *maskStartGuard) Stop(context.Context) error  { return nil }
func (guard *maskStartGuard) Close(context.Context) error { return nil }

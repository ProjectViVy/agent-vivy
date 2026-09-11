package assembly

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port"
)

type fixtureCase struct {
	ID        string              `json:"id"`
	Expect    string              `json:"expect"`
	WantError string              `json:"wantError"`
	Recipe    Recipe              `json:"recipe"`
	Modules   []module.Descriptor `json:"modules"`
}

func TestCompilePluginV1GraphFixtures(t *testing.T) {
	casePaths := []string{
		"valid/minimal/case.json",
		"valid/ordered-tool/case.json",
		"invalid/duplicate-module/case.json",
		"invalid/missing-provider/case.json",
		"invalid/duplicate-exclusive-provider/case.json",
		"invalid/dependency-cycle/case.json",
		"invalid/unapproved-grant/case.json",
		"invalid/bad-source-hash/case.json",
	}

	for _, casePath := range casePaths {
		fixture := loadFixtureCase(t, casePath)
		t.Run(fixture.ID, func(t *testing.T) {
			compiler := fixtureCompiler(t, fixture.Modules)
			plan, err := compiler.Compile(context.Background(), fixture.Recipe)
			if fixture.Expect == "accept" {
				if err != nil {
					t.Fatalf("Compile() error = %v", err)
				}
				if len(plan.Modules) != len(fixture.Recipe.Modules) {
					t.Fatalf("compiled modules = %d, want %d", len(plan.Modules), len(fixture.Recipe.Modules))
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), fixture.WantError) {
				t.Fatalf("Compile() error = %v, want substring %q", err, fixture.WantError)
			}
		})
	}
}

func TestCompileEmbedsEffectiveGrants(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/network"), module.PortRef{Port: "std/tool@v1", ID: "fixture.network"})
	provider.RequestedGrants = []module.Grant{module.GrantNetClient}
	consumer := testDescriptor("fixture/host")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1"}, Provider: "fixture/network"}}
	compiler := fixtureCompiler(t, []module.Descriptor{provider, consumer})
	recipe := Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{"fixture/network", "fixture/host"},
		GrantApprovals: []GrantApproval{{
			Module: "fixture/network",
			Name:   module.GrantNetClient,
			Constraints: map[string][]string{
				"ports":   {"443"},
				"hosts":   {"api.example.com"},
				"schemes": {"https"},
			},
		}},
	}

	plan, err := compiler.Compile(context.Background(), recipe)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	want := []EffectiveGrant{{
		Name: module.GrantNetClient,
		Constraints: map[string][]string{
			"hosts":   {"api.example.com"},
			"ports":   {"443"},
			"schemes": {"https"},
		},
	}}
	if got := plan.Modules[0].EffectiveGrants; !reflect.DeepEqual(got, want) {
		t.Fatalf("effective grants = %#v, want %#v", got, want)
	}
}

func TestCompileRequiresAuthoritativeT2SourcePin(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "provider.go"), []byte("package provider\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := HashSourceTree(root, "")
	if err != nil {
		t.Fatal(err)
	}
	provider := withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: "std/tool@v1", ID: "fixture.tool"})
	provider.Source = module.Source{Ref: "git:fixture/provider@abc123", SHA256: digest}
	consumer := testDescriptor("fixture/consumer")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1"}, Provider: provider.Module.ID}}
	catalog, err := NewSourceCatalog([]SourceRecord{
		{Descriptor: provider, Trust: TrustT2, Root: root, Ref: provider.Source.Ref},
		{Descriptor: consumer, Trust: TrustT1},
	})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: supportedPortEvidence()}
	base := Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{provider.Module.ID, consumer.Module.ID}}
	if _, err := compiler.Compile(context.Background(), base); err == nil || !strings.Contains(err.Error(), "missing authoritative source pin") {
		t.Fatalf("missing pin error = %v", err)
	}
	mismatch := base
	mismatch.Sources = map[string]module.Source{provider.Module.ID: {Ref: provider.Source.Ref, SHA256: strings.Repeat("f", 64)}}
	if _, err := compiler.Compile(context.Background(), mismatch); err == nil || !strings.Contains(err.Error(), "source pin mismatch") {
		t.Fatalf("mismatched pin error = %v", err)
	}
	valid := base
	valid.Sources = map[string]module.Source{provider.Module.ID: provider.Source}
	if _, err := compiler.Compile(context.Background(), valid); err != nil {
		t.Fatalf("valid source pin rejected: %v", err)
	}
}

func TestCompileRejectsUnusedProviderUnresolvedOrderAndConflict(t *testing.T) {
	tests := []struct {
		name      string
		modules   []module.Descriptor
		recipe    Recipe
		wantError string
	}{
		{
			name: "unused provider",
			modules: []module.Descriptor{withProvides(testDescriptor("fixture/provider"),
				module.PortRef{Port: "std/tool@v1", ID: "fixture.tool"})},
			recipe:    Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/provider"}},
			wantError: "unused provider fixture/provider for std/tool@v1",
		},
		{
			name: "unresolved ordered contribution",
			modules: []module.Descriptor{
				withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: "std/middleware/pre-tool@v1", ID: "fixture.guard"}),
			},
			recipe: Recipe{
				APIVersion: RecipeAPIVersionV1,
				Modules:    []string{"fixture/provider"},
				Order:      map[string][]string{"std/middleware/pre-tool@v1": {"fixture/missing"}},
			},
			wantError: "unresolved order edge fixture/missing for std/middleware/pre-tool@v1",
		},
		{
			name: "unresolved lifecycle edge",
			modules: []module.Descriptor{withAfter(testDescriptor("fixture/consumer"),
				"fixture/missing")},
			recipe:    Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/consumer"}},
			wantError: "unresolved lifecycle.after fixture/missing for fixture/consumer",
		},
		{
			name: "declared conflict",
			modules: []module.Descriptor{
				withConflict(testDescriptor("fixture/a"), "fixture/b"),
				testDescriptor("fixture/b"),
			},
			recipe:    Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/a", "fixture/b"}},
			wantError: "module conflict: fixture/a conflicts with fixture/b",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compiler := fixtureCompiler(t, test.modules)
			_, err := compiler.Compile(context.Background(), test.recipe)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("Compile() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestCompileRequiresExplicitExclusiveSelection(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/face"), module.PortRef{Port: "std/face@v1", ID: "fixture.face"})
	consumer := testDescriptor("fixture/host")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/face@v1"}, Provider: "fixture/face"}}
	compiler := fixtureCompiler(t, []module.Descriptor{provider, consumer})

	_, err := compiler.Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{"fixture/face", "fixture/host"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing exclusive selection for std/face@v1") {
		t.Fatalf("Compile() error = %v, want explicit exclusive selection rejection", err)
	}
}

func TestCompileRequiresCompleteOrderForOrderedPort(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/guard"), module.PortRef{Port: "std/middleware/pre-tool@v1", ID: "fixture.guard"})
	consumer := testDescriptor("fixture/host")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/middleware/pre-tool@v1"}, Provider: "fixture/guard"}}
	compiler := fixtureCompiler(t, []module.Descriptor{provider, consumer})

	_, err := compiler.Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{"fixture/guard", "fixture/host"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing order for std/middleware/pre-tool@v1") {
		t.Fatalf("Compile() error = %v, want explicit order rejection", err)
	}
}

func TestCompileIsDeterministicAndUsesCatalogTrust(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: "std/tool@v1", ID: "fixture.tool"})
	consumer := testDescriptor("fixture/consumer")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1"}, Provider: "fixture/provider"}}
	catalog, err := NewSourceCatalog([]SourceRecord{
		{Descriptor: provider, Trust: TrustT2},
		{Descriptor: consumer, Trust: TrustT1},
	})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: supportedPortEvidence()}
	recipe := Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/consumer", "fixture/provider"}}

	first, err := compiler.Compile(context.Background(), recipe)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	second, err := compiler.Compile(context.Background(), recipe)
	if err != nil {
		t.Fatalf("Compile() second error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated Compile() differs\nfirst: %#v\nsecond: %#v", first, second)
	}
	if got := first.Modules[0].Trust; got != TrustT1 {
		t.Fatalf("consumer trust = %q, want Source Catalog assignment %q", got, TrustT1)
	}
	if got := first.LifecycleOrder; !reflect.DeepEqual(got, []string{"fixture/provider", "fixture/consumer"}) {
		t.Fatalf("lifecycle order = %v", got)
	}
}

func TestCompileRejectsPortWithoutBuildEvidence(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: "std/tool@v1", ID: "fixture.tool"})
	consumer := testDescriptor("fixture/consumer")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1"}, Provider: "fixture/provider"}}
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: provider, Trust: TrustT2}, {Descriptor: consumer, Trust: TrustT1}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = (Compiler{Ports: port.PublicCatalog(), Sources: catalog}).Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{"fixture/provider", "fixture/consumer"},
	})
	if err == nil || !strings.Contains(err.Error(), "Port std/tool@v1 is SPECIFIED and cannot be selected") {
		t.Fatalf("Compile() error = %v, want build-evidence rejection", err)
	}
}

func loadFixtureCase(t *testing.T, relative string) fixtureCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "plugin-v1", relative))
	if err != nil {
		t.Fatal(err)
	}
	var fixture fixtureCase
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", relative, err)
	}
	return fixture
}

func fixtureCompiler(t *testing.T, descriptors []module.Descriptor) Compiler {
	t.Helper()
	records := make([]SourceRecord, 0, len(descriptors))
	for _, descriptor := range descriptors {
		trust := TrustT2
		if strings.HasPrefix(descriptor.Source.Ref, "internal:") {
			trust = TrustT1
		}
		records = append(records, SourceRecord{Descriptor: descriptor, Trust: trust})
	}
	catalog, err := NewSourceCatalog(records)
	if err != nil {
		t.Fatal(err)
	}
	return Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: supportedPortEvidence()}
}

func supportedPortEvidence() map[string]port.SupportEvidence {
	references := make([]port.EvidenceReference, 0, len(port.RequiredEvidenceKinds()))
	for _, kind := range port.RequiredEvidenceKinds() {
		references = append(references, port.EvidenceReference{Kind: kind, ID: "test/" + string(kind)})
	}
	evidence := make(map[string]port.SupportEvidence)
	for _, definition := range port.PublicCatalog().Definitions() {
		evidence[definition.Ref.Port] = port.SupportEvidence{References: references, GatesPassed: true}
	}
	return evidence
}

func withProvides(descriptor module.Descriptor, refs ...module.PortRef) module.Descriptor {
	descriptor.Provides = append([]module.PortRef(nil), refs...)
	return descriptor
}

func withAfter(descriptor module.Descriptor, moduleID string) module.Descriptor {
	descriptor.Lifecycle.After = []string{moduleID}
	return descriptor
}

func withConflict(descriptor module.Descriptor, moduleID string) module.Descriptor {
	descriptor.Conflicts = []module.Conflict{{Module: moduleID}}
	return descriptor
}

func TestCompileRejectsCoreAuthorityAndDuplicateProviderIdentity(t *testing.T) {
	core := withProvides(testDescriptor("fixture/core"), module.PortRef{Port: "core/tool-host@v1", ID: "fixture.tool-host"})
	catalog, err := NewSourceCatalog([]SourceRecord{{Descriptor: core, Trust: TrustT2}})
	if err != nil {
		t.Fatal(err)
	}
	compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: P1P2PortEvidence()}
	_, err = compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/core"}})
	if err == nil || !strings.Contains(err.Error(), "core Port core/tool-host@v1 may only be provided") {
		t.Fatalf("core authority error = %v", err)
	}

	first := withProvides(testDescriptor("fixture/first"), module.PortRef{Port: "std/tool@v1", ID: "fixture.same"})
	second := withProvides(testDescriptor("fixture/second"), module.PortRef{Port: "std/tool@v1", ID: "fixture.same"})
	consumer := testDescriptor("fixture/consumer")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1", ID: "fixture.same"}, Provider: "fixture/first"}}
	compiler = fixtureCompiler(t, []module.Descriptor{first, second, consumer})
	_, err = compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/first", "fixture/second", "fixture/consumer"}})
	if err == nil || !strings.Contains(err.Error(), "duplicate provider id fixture.same") {
		t.Fatalf("provider identity error = %v", err)
	}
}

func TestCompileMatchesRequirementProviderIdentity(t *testing.T) {
	provider := withProvides(testDescriptor("fixture/provider"), module.PortRef{Port: "std/tool@v1", ID: "fixture.actual"})
	consumer := testDescriptor("fixture/consumer")
	consumer.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "std/tool@v1", ID: "fixture.expected"}, Provider: "fixture/provider"}}
	compiler := fixtureCompiler(t, []module.Descriptor{provider, consumer})
	_, err := compiler.Compile(context.Background(), Recipe{APIVersion: RecipeAPIVersionV1, Modules: []string{"fixture/provider", "fixture/consumer"}})
	if err == nil || !strings.Contains(err.Error(), `requirement id "fixture.expected"`) {
		t.Fatalf("requirement identity error = %v", err)
	}
}

func TestCompileRequiresTypedMCPHostProviderAndCoreOwner(t *testing.T) {
	toolHost := testDescriptor("vivy/tool-host")
	toolHost.Source.Ref = "internal:vivy/tool-host"
	toolHost.Provides = []module.PortRef{{Port: "core/tool-host@v1", ID: "vivy.tool-host"}}

	compile := func(t *testing.T, mcp module.Descriptor, binding GoBinding) error {
		t.Helper()
		catalog, err := NewSourceCatalog([]SourceRecord{
			{Descriptor: toolHost, Trust: TrustT1},
			{Descriptor: mcp, Trust: TrustT1, Binding: binding},
		})
		if err != nil {
			t.Fatal(err)
		}
		compiler := Compiler{Ports: port.PublicCatalog(), Sources: catalog, PortEvidence: supportedPortEvidence()}
		_, err = compiler.Compile(context.Background(), Recipe{
			APIVersion: RecipeAPIVersionV1,
			Modules:    []string{toolHost.Module.ID, mcp.Module.ID},
		})
		return err
	}

	withoutCore := testDescriptor("vivy/mcp-host")
	withoutCore.Source.Ref = "internal:vivy/mcp-host"
	withoutCore.Provides = []module.PortRef{{Port: "std/tool-world@v1", ID: "mcp"}}
	withoutCore.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: toolHost.Module.ID}}
	if err := compile(t, withoutCore, GoBinding{MCPHostProvider: true}); err == nil || !strings.Contains(err.Error(), "core/mcp-host@v1") {
		t.Fatalf("MCP world without core Host error = %v, want core/mcp-host@v1 rejection", err)
	}

	withoutTypedBinding := testDescriptor("vivy/mcp-host")
	withoutTypedBinding.Source.Ref = "internal:vivy/mcp-host"
	withoutTypedBinding.Provides = []module.PortRef{
		{Port: "core/mcp-host@v1", ID: "vivy.mcp-host"},
		{Port: "std/tool-world@v1", ID: "mcp"},
	}
	withoutTypedBinding.Requires = []module.Requirement{{PortRef: module.PortRef{Port: "core/tool-host@v1"}, Provider: toolHost.Module.ID}}
	if err := compile(t, withoutTypedBinding, GoBinding{}); err == nil || !strings.Contains(err.Error(), "typed MCPHostProvider") {
		t.Fatalf("MCP world without typed binding error = %v, want typed MCPHostProvider rejection", err)
	}

	valid := withoutTypedBinding
	if err := compile(t, valid, GoBinding{MCPHostProvider: true}); err != nil {
		t.Fatalf("typed MCPHostProvider graph rejected: %v", err)
	}
}

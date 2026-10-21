package assembly

import (
	"agent-vivy/sdk/module"
	"strings"
	"testing"
)

func TestGenerateRuntimeAssemblyNudgeFactoryAbsent(t *testing.T) {
	generated, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{{Descriptor: testDescriptor("fixture/absent"), Binding: GoBinding{ImportPath: "example.test/fixture", Package: "fixture", Constructor: "New"}}}}, "generatedfixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"HasNudgeFactory() bool { return false }", "NudgeFactoryValue() any { return nil }"} {
		if !strings.Contains(strings.Join(strings.Fields(string(generated)), " "), want) {
			t.Fatalf("missing inert factory accessor %q", want)
		}
	}
	if strings.Contains(string(generated), "agent-vivy/internal/runtime") || strings.Contains(string(generated), "exp-nudge") {
		t.Fatal("absent nudge factory imports an implementation")
	}
}

func TestGenerateRuntimeAssemblyNudgeFactorySelected(t *testing.T) {
	descriptor := testDescriptor("vivy/nudge")
	descriptor.Provides = []module.PortRef{{Port: "core/nudge-factory@v1"}}
	resolved := ResolvedModule{Descriptor: descriptor, Binding: GoBinding{ImportPath: "example.test/nudge", Package: "vivyruntime", Constructor: "New", NudgeFactory: "NewFactory"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{resolved}}
	generated, err := GenerateRuntimeAssembly(plan, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Join(strings.Fields(string(generated)), " ")
	for _, want := range []string{`vivyruntime "agent-vivy/internal/runtime"`, `vivyruntime2 "example.test/nudge"`, `NudgeFactory vivyruntime.NudgeFactory`, `NudgeFactory: vivyruntime2.NewFactory`, `return assembly.NudgeFactory`} {
		if !strings.Contains(source, want) {
			t.Fatalf("missing selected typed binding %q", want)
		}
	}
	for _, name := range []string{"", "invalid()"} {
		invalid := resolved
		invalid.Binding.NudgeFactory = name
		if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{invalid}}, "fixture"); err == nil {
			t.Fatalf("invalid binding %q accepted", name)
		}
	}
	invalid := resolved
	invalid.Descriptor.Provides = nil
	if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{invalid}}, "fixture"); err == nil {
		t.Fatal("factory without port accepted")
	}
	if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{resolved, resolved}}, "fixture"); err == nil {
		t.Fatal("duplicate factory accepted")
	}
}

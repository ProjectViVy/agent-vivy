package assembly

import (
	"agent-vivy/sdk/module"
	"strings"
	"testing"
)

func TestGenerateRuntimeAssemblyWarmingSteeringFactoriesAbsent(t *testing.T) {
	generated, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{{Descriptor: testDescriptor("fixture/absent"), Binding: GoBinding{ImportPath: "example.test/fixture", Package: "fixture", Constructor: "New"}}}}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Join(strings.Fields(string(generated)), " ")
	for _, name := range []string{"Warming", "Steering"} {
		for _, want := range []string{"Has" + name + "Factory() bool { return false }", name + "FactoryValue() any { return nil }"} {
			if !strings.Contains(source, want) {
				t.Fatalf("absent factory accessor missing: %s", want)
			}
		}
	}
	if strings.Contains(source, "agent-vivy/internal/runtime") || strings.Contains(source, "plugins/exp/") {
		t.Fatal("empty selection imports a runtime adapter/provider")
	}
}

func TestGenerateRuntimeAssemblyWarmingSteeringFactoriesSelected(t *testing.T) {
	for _, tc := range []struct {
		name, owner, port string
		set               func(*GoBinding, string)
	}{
		{"Warming", "vivy/warming", "core/warming-factory@v1", func(b *GoBinding, s string) { b.WarmingFactory = s }},
		{"Steering", "vivy/steering", "core/steering-factory@v1", func(b *GoBinding, s string) { b.SteeringFactory = s }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			descriptor := testDescriptor(tc.owner)
			descriptor.Provides = []module.PortRef{{Port: tc.port}}
			binding := GoBinding{ImportPath: "example.test/provider", Package: "fixture", Constructor: "New"}
			tc.set(&binding, "NewFactory")
			resolved := ResolvedModule{Descriptor: descriptor, Binding: binding}
			generated, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{resolved}}, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			source := strings.Join(strings.Fields(string(generated)), " ")
			for _, want := range []string{tc.name + "Factory vivyruntime." + tc.name + "Factory", tc.name + "Factory: fixture.NewFactory", "return assembly." + tc.name + "Factory"} {
				if !strings.Contains(source, want) {
					t.Fatalf("missing typed binding %q", want)
				}
			}
			for _, name := range []string{"", "invalid()"} {
				bad := resolved
				tc.set(&bad.Binding, name)
				if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{bad}}, "fixture"); err == nil {
					t.Fatalf("invalid named binding %q accepted", name)
				}
			}
			bad := resolved
			bad.Descriptor.Provides = nil
			if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{bad}}, "fixture"); err == nil {
				t.Fatal("factory without Port accepted")
			}
			if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{resolved, resolved}}, "fixture"); err == nil {
				t.Fatal("duplicate factory accepted")
			}
		})
	}
}

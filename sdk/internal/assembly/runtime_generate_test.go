package assembly

import (
	"strings"
	"testing"

	"agent-vivy/sdk/module"
)

func TestGenerateRuntimeAssemblyUsesTypedProviderConstructors(t *testing.T) {
	channelDescriptor := testDescriptor("fixture/chat")
	channelDescriptor.Provides = []module.PortRef{{Port: "std/channel@v1", ID: "fixture.chat"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{{Descriptor: channelDescriptor, Binding: GoBinding{ImportPath: "example.com/fixture/chat", Package: "chat", Constructor: "New", ProviderConstructor: "NewProvider"}, EffectiveGrants: []EffectiveGrant{{Name: module.GrantChannelPoll}}}}}
	first, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("runtime assembly generation is not stable")
	}
	for _, want := range []string{`chat "example.com/fixture/chat"`, `chat.NewProvider()`, `"fixture.chat": {{Name: module.Grant("channel.poll")`, `"channels": generation.Unconfigured`, `"mcp": generation.NotCompiled`} {
		if !strings.Contains(string(first), want) {
			t.Fatalf("generated runtime assembly missing %q:\n%s", want, first)
		}
	}
}

func TestGenerateRuntimeAssemblyComposesMultipleToolProviderSets(t *testing.T) {
	firstDescriptor := testDescriptor("fixture/tools-a")
	firstDescriptor.Provides = []module.PortRef{{Port: "std/tool@v1", ID: "fixture.tool-a"}}
	secondDescriptor := testDescriptor("fixture/tools-b")
	secondDescriptor.Provides = []module.PortRef{{Port: "std/tool@v1", ID: "fixture.tool-b"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{
		{Descriptor: firstDescriptor, Binding: GoBinding{ImportPath: "example.com/fixture/a", Package: "a", ProviderConstructor: "Providers", ProviderCollection: true}},
		{Descriptor: secondDescriptor, Binding: GoBinding{ImportPath: "example.com/fixture/b", Package: "b", ProviderConstructor: "Providers", ProviderCollection: true}},
	}}

	generated, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	if want := "append(append([]tool.ToolProvider{}, a.Providers()...), b.Providers()...)"; !strings.Contains(string(generated), want) {
		t.Fatalf("generated runtime assembly does not compose both provider sets:\n%s", generated)
	}
}

func TestGenerateRuntimeAssemblyReusesExplicitAuxiliaryToolWorldProvider(t *testing.T) {
	descriptor := testDescriptor("fixture/lsp")
	descriptor.Provides = []module.PortRef{{Port: "std/tool-world@v1", ID: "fixture.lsp"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{{
		Descriptor: descriptor,
		Binding: GoBinding{
			ImportPath:                   "example.com/fixture/lsp",
			Package:                      "lsp",
			ProviderConstructor:          "NewProvider",
			DiagnosticObserver:           true,
			LanguageServerStatusProvider: true,
		},
	}}}

	generated, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	if strings.Count(source, "lsp.NewProvider()") != 1 {
		t.Fatalf("auxiliary provider must be constructed exactly once:\n%s", source)
	}
	for _, want := range []string{
		"Worlds:                     []toolworld.Provider{provider0}",
		"DiagnosticObservers:        []toolworld.DiagnosticObserver{provider0}",
		`DiagnosticObserverWorldIDs: []string{"fixture.lsp"}`,
		"LanguageServerStatuses:     []toolworld.LanguageServerStatusProvider{provider0}",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated runtime assembly missing %q:\n%s", want, source)
		}
	}
}

func TestGenerateRuntimeAssemblyComposesTypedP4Sources(t *testing.T) {
	contextDescriptor := testDescriptor("fixture/context-source")
	contextDescriptor.Provides = []module.PortRef{{Port: "std/context-source@v1", ID: "fixture.context"}}
	skillDescriptor := testDescriptor("fixture/skill-source")
	skillDescriptor.Provides = []module.PortRef{{Port: "std/skill-source@v1", ID: "fixture.skills"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{
		{Descriptor: contextDescriptor, Binding: GoBinding{ImportPath: "example.com/fixture/context", Package: "contextfixture", ProviderConstructor: "Providers", ProviderCollection: true, ContextSourceProvider: true}},
		{Descriptor: skillDescriptor, Binding: GoBinding{ImportPath: "example.com/fixture/skill", Package: "skillfixture", ProviderConstructor: "Providers", ProviderCollection: true, SkillSourceProvider: true}},
	}}

	generated, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	compactSource := strings.Join(strings.Fields(source), " ")
	for _, want := range []string{
		`"agent-vivy/sdk/port/contextsource"`,
		`"agent-vivy/sdk/port/skillsource"`,
		`ContextSources:  append([]contextsource.Provider{}, contextfixture.Providers()...)`,
		`SkillSources:    append([]skillsource.Provider{}, skillfixture.Providers()...)`,
		`ContextSources: []string{"fixture.context"}`,
		`SkillSources:   []string{"fixture.skills"}`,
	} {
		if !strings.Contains(compactSource, strings.Join(strings.Fields(want), " ")) {
			t.Fatalf("typed P4 source output missing %q:\n%s", want, source)
		}
	}
}

func TestGenerateRuntimeAssemblyMinimalOmitsP4SourceImportsAndFields(t *testing.T) {
	minimal := testDescriptor("fixture/minimal")
	minimal.Provides = []module.PortRef{{Port: "core/tool-host@v1", ID: "fixture.tool-host"}}
	generated, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{{
		Descriptor: minimal,
		Binding:    GoBinding{ImportPath: "example.com/fixture/minimal", Package: "minimal"},
	}}}, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	source := string(generated)
	for _, omitted := range []string{"sdk/port/contextsource", "sdk/port/skillsource", "ContextSources []", "SkillSources []", "ContextSources:", "SkillSources:", "ContextSourceProviders", "SkillSourceProviders", "NewContextHost", "NewContextSource", "NewSkillHost", "NewSkillSource", "NewMCPHost"} {
		if strings.Contains(source, omitted) {
			t.Fatalf("minimal runtime assembly contains omitted P4 surface %q:\n%s", omitted, source)
		}
	}
}

func TestGenerateRuntimeAssemblyMCPStateRequiresTypedHostBinding(t *testing.T) {
	descriptor := testDescriptor("vivy/mcp-host")
	descriptor.Provides = []module.PortRef{
		{Port: "core/mcp-host@v1", ID: "vivy.mcp-host"},
		{Port: "std/tool-world@v1", ID: "mcp"},
	}
	base := ResolvedModule{Descriptor: descriptor, Binding: GoBinding{
		ImportPath:          "example.com/fixture/mcp",
		Package:             "mcpfixture",
		Constructor:         "New",
		ProviderConstructor: "NewProvider",
	}}
	if _, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{base}}, "assembly"); err == nil || !strings.Contains(err.Error(), "typed MCPHostProvider") {
		t.Fatalf("MCP world without typed binding error = %v, want typed MCPHostProvider rejection", err)
	}

	base.Binding.MCPHostProvider = true
	generated, err := GenerateRuntimeAssembly(AssemblyPlan{Modules: []ResolvedModule{base}}, "assembly")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), `"mcp": generation.Unconfigured`) {
		t.Fatalf("typed MCP binding did not produce compiled MCP signal:\n%s", generated)
	}
}

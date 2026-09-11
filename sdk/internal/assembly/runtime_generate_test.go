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

func TestGenerateRuntimeAssemblyBindsTypedActionInventory(t *testing.T) {
	descriptor := testDescriptor("fixture/actions")
	descriptor.Provides = []module.PortRef{{Port: "std/control-action@v1", ID: "fixture.action"}}
	plan := AssemblyPlan{Modules: []ResolvedModule{{
		Descriptor: descriptor,
		Binding: GoBinding{
			ImportPath:          "example.com/fixture/actions",
			Package:             "actions",
			ProviderConstructor: "NewProvider",
		},
		EffectiveGrants: []EffectiveGrant{{Name: module.GrantSecretRead, Constraints: map[string][]string{"names": {"ACTIONS_TOKEN"}}}},
	}}}

	generated, err := GenerateRuntimeAssembly(plan, "assembly")
	if err != nil {
		t.Fatalf("GenerateRuntimeAssembly() error = %v", err)
	}
	source := string(generated)
	for _, want := range []string{
		`"agent-vivy/sdk/port/controlaction"`,
		"ActionSets                 []controlaction.ProviderSet",
		"ActionSets:      []controlaction.ProviderSet{controlaction.ProviderSet{",
		`controlaction.ProviderSet{ModuleID: "fixture/actions", AllowedIDs: []string{"fixture.action"}, Providers: []controlaction.Provider{actions.NewProvider()}`,
		`EffectiveGrants: []module.GrantBinding{{Name: module.Grant("secret.read"), Constraints: map[string][]string{"names": {"ACTIONS_TOKEN"}}}}`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated runtime assembly missing %q:\n%s", want, source)
		}
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

package app

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/tools"
	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/skillsource"
)

type defaultGenerationInventory struct {
	Channels           []string          `json:"channels"`
	ProtectedTools     []string          `json:"protectedTools"`
	ToolWorldProviders []string          `json:"toolWorldProviders"`
	DefaultFace        string            `json:"defaultFace"`
	NetworkState       map[string]string `json:"networkState"`
	ProviderProfiles   []string          `json:"providerProfiles"`
}

func TestDefaultGenerationLeavesUnconfiguredNetworkInactive(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	a, err := New(context.Background(), newAnthropicTestConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.service.CancelAll(); _ = a.mcpBackend.Close(); _ = a.backend.Close() })
	if got := a.channels.Started(); len(got) != 0 {
		t.Fatalf("unconfigured channels started: %v", got)
	}
	for _, status := range a.channels.Inspect() {
		if status.Started {
			t.Fatalf("unconfigured channel started: %+v", status)
		}
	}
}

func TestAppUsesGeneratedRuntimeAssembly(t *testing.T) {
	runtime.SetEngineVersionOverride(pinnedEinoVersion)
	t.Cleanup(func() { runtime.SetEngineVersionOverride("") })
	assembly := genassembly.BuildDefault()
	assembly.Channels = nil
	assembly.ChannelGrants = map[string][]module.GrantBinding{}
	assembly.Manifest.Channels = nil

	a, err := NewWithAssembly(context.Background(), newAnthropicTestConfig(t), assembly)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.service.CancelAll(); _ = a.mcpBackend.Close(); _ = a.backend.Close() })
	if got := a.channels.Inspect(); len(got) != 0 {
		t.Fatalf("app ignored supplied runtime assembly channels: %+v", got)
	}
}

func TestDefaultGenerationBaselineInventory(t *testing.T) {
	raw, err := os.ReadFile("../../sdk/internal/testdata/default-generation.expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var want defaultGenerationInventory
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	assembly := genassembly.BuildDefault()
	manifest := assembly.Manifest
	got := defaultGenerationInventory{
		Channels: manifest.Channels, ProtectedTools: manifest.Tools,
		ToolWorldProviders: manifest.ToolWorlds, DefaultFace: manifest.Face,
		NetworkState:     map[string]string{"channels": string(manifest.NetworkStates["channels"]), "mcp": string(manifest.NetworkStates["mcp"])},
		ProviderProfiles: manifest.ProviderProfiles,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	toolIDs := make([]string, 0, len(assembly.Tools))
	for _, provider := range assembly.Tools {
		toolIDs = append(toolIDs, provider.Definition().ID)
	}
	if !reflect.DeepEqual(toolIDs, manifest.Tools) {
		t.Fatalf("tool providers = %v, manifest tools = %v", toolIDs, manifest.Tools)
	}
	worldIDs := make([]string, 0, len(assembly.Worlds))
	for _, provider := range assembly.Worlds {
		worldIDs = append(worldIDs, provider.Definition().ID)
	}
	if !reflect.DeepEqual(worldIDs, manifest.ToolWorlds) {
		t.Fatalf("tool-world providers = %v, manifest worlds = %v", worldIDs, manifest.ToolWorlds)
	}
}

func TestDefaultGenerationProviderProfilesAreGeneratedAuthority(t *testing.T) {
	assembly := genassembly.BuildDefault()
	if len(assembly.ProviderProfiles) != 2 {
		t.Fatalf("generated Provider Profiles = %d, want 2", len(assembly.ProviderProfiles))
	}
	for index, provider := range assembly.ProviderProfiles {
		if got, want := provider.Definition().ID, assembly.Manifest.ProviderProfiles[index]; got != want {
			t.Fatalf("Provider Profile %d id = %q, manifest = %q", index, got, want)
		}
	}
}

func TestDefaultGeneratedToolProvidersBindRuntimeImplementations(t *testing.T) {
	assembly := genassembly.BuildDefault()
	registry, err := bindGeneratedTools(assembly.Tools, tools.Builtin(nil))
	if err != nil {
		t.Fatal(err)
	}
	implementation, ok := registry.Lookup("ask_user")
	if !ok {
		t.Fatal("generated ask_user provider is not registered")
	}
	governed, ok := implementation.(interface{ GovernedToolID() string })
	if !ok {
		t.Fatalf("ask_user implementation = %T, want ToolHost-governed binding", implementation)
	}
	if governed.GovernedToolID() != "ask_user" {
		t.Fatalf("ask_user governed id = %q, want ask_user", governed.GovernedToolID())
	}
}

func TestDefaultGenerationComposesEstablishedOptionalHosts(t *testing.T) {
	assembly := genassembly.BuildDefault()
	for _, moduleID := range []string{"vivy/context-host", "vivy/context-source", "vivy/skill-host", "vivy/skill-source", "vivy/mcp-host", "vivy/observer-host", "vivy/status-host"} {
		if !slices.Contains(assembly.Manifest.Modules, moduleID) {
			t.Fatalf("default Generation omitted %s: %v", moduleID, assembly.Manifest.Modules)
		}
	}
	if got, ok := assembly.ContextSourceProviders().([]contextsource.Provider); !ok || len(got) != 1 || got[0].ID() != "vivy.project-context" {
		t.Fatalf("default ContextSource inventory = %#v (typed=%v)", got, ok)
	}
	if got, ok := assembly.SkillSourceProviders().([]skillsource.Provider); !ok || len(got) != 1 || got[0].ID() != "vivy.default-skills" {
		t.Fatalf("default SkillSource inventory = %#v (typed=%v)", got, ok)
	}
	if len(assembly.Worlds) != 1 || assembly.Worlds[0].Definition().ID != "mcp" {
		t.Fatalf("default MCP ToolWorld inventory = %#v", assembly.Worlds)
	}
	if err := validateRuntimeAssembly(assembly); err != nil {
		t.Fatalf("generated P4 inventory validation failed: %v", err)
	}
	if err := assembly.Start(context.Background(), assemblyHosts{}); err != nil {
		t.Fatalf("default Assembly did not start all compiled Hosts: %v", err)
	}
	if err := assembly.Start(context.Background(), assemblyHosts{}); err == nil {
		t.Fatal("default Assembly allowed a second Start")
	}
	if err := assembly.Close(context.Background()); err != nil {
		t.Fatalf("default Assembly cleanup failed: %v", err)
	}
	if err := assembly.Close(context.Background()); err != nil {
		t.Fatalf("default Assembly cleanup was not idempotent: %v", err)
	}
}

func TestValidateRuntimeAssemblyRejectsTypedSourcesWithoutHosts(t *testing.T) {
	assembly := genassembly.BuildDefault()
	assembly.Manifest.Modules = slices.DeleteFunc(assembly.Manifest.Modules, func(id string) bool {
		return id == "vivy/context-host" || id == "vivy/skill-host" || id == "vivy/mcp-host"
	})
	if err := validateRuntimeAssembly(assembly); err == nil {
		t.Fatal("validateRuntimeAssembly accepted typed Sources/ToolWorld without their Hosts")
	}
}

func TestGeneratedToolInventoryIsAuthoritative(t *testing.T) {
	registry, err := bindGeneratedTools(nil, tools.Builtin(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup("ask_user"); ok {
		t.Fatal("compiled-out protected Tool remains registered")
	}
	if _, ok := registry.Lookup("echo_info"); !ok {
		t.Fatal("kernel Tool was removed with Assembly-controlled Tools")
	}
}

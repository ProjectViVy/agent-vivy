package assembly

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"agent-vivy/sdk/module"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/skillsource"
	"agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

// These fakes intentionally depend only on public SDK packages. Keeping them
// in the compiler conformance test proves that an SCX adapter needs neither
// internal/runtime nor Eino types to implement the frozen provider contracts.
type p8ContextProvider struct{}

func (p8ContextProvider) ID() string { return "scx.context" }
func (p8ContextProvider) Query(context.Context, contextsource.Request) (contextsource.Page, error) {
	return contextsource.NewPage([]contextsource.Candidate{{
		SourceID: "scx", ContentID: "personality", MediaType: "text/plain",
		Content: "concise", Version: "v1",
	}}, ""), nil
}

type p8SkillProvider struct{}

func (p8SkillProvider) ID() string { return "scx.skill" }
func (p8SkillProvider) List(context.Context, skillsource.Request) ([]skillsource.Summary, error) {
	return []skillsource.Summary{{ID: "scx/recall", Version: "v1", Available: true}}, nil
}
func (p8SkillProvider) Get(context.Context, skillsource.Request, string) (skillsource.Skill, error) {
	return skillsource.Skill{ID: "scx/recall", Version: "v1", Content: "Recall relevant context.", Available: true}, nil
}

type p8ToolProvider struct{}

func (p8ToolProvider) Definition() tool.Definition {
	return tool.Definition{ID: "scx.retrieve", Effect: tool.EffectRead, Schema: json.RawMessage(`{"type":"object"}`)}
}
func (p8ToolProvider) Invoke(context.Context, tool.Host, json.RawMessage) (tool.Result, error) {
	return tool.Result{Text: "bounded result"}, nil
}

type p8ToolWorldProvider struct{}

func (p8ToolWorldProvider) Definition() toolworld.Definition {
	return toolworld.Definition{ID: "scx.catalog", Description: "SCX retrieval catalog"}
}
func (p8ToolWorldProvider) Discover(context.Context, toolworld.Host) ([]toolworld.ToolDefinition, error) {
	return []toolworld.ToolDefinition{{ID: "scx.catalog.retrieve", Effect: toolworld.EffectRead}}, nil
}
func (p8ToolWorldProvider) Invoke(context.Context, toolworld.Host, string, json.RawMessage) (toolworld.Result, error) {
	return toolworld.Result{Text: "bounded result"}, nil
}
func (p8ToolWorldProvider) Close(context.Context) error { return nil }

var (
	_ contextsource.Provider = p8ContextProvider{}
	_ skillsource.Provider   = p8SkillProvider{}
	_ tool.ToolProvider      = p8ToolProvider{}
	_ toolworld.Provider     = p8ToolWorldProvider{}
)

func TestP8SCXGateACompilesPublicProvidersThroughSoleHosts(t *testing.T) {
	hosts := []module.Descriptor{
		p8HostDescriptor("vivy/context-host", "core/context-host@v1"),
		p8HostDescriptor("vivy/skill-host", "core/skill-host@v1"),
		p8HostDescriptor("vivy/tool-host", "core/tool-host@v1"),
	}
	providers := []module.Descriptor{
		p8ProviderDescriptor("scx/context", "std/context-source@v1", "scx.context", "core/context-host@v1", "vivy/context-host", module.GrantFSRead, module.GrantNetClient),
		p8ProviderDescriptor("scx/skill", "std/skill-source@v1", "scx.skill", "core/skill-host@v1", "vivy/skill-host", module.GrantFSRead),
		p8ProviderDescriptor("scx/tool", "std/tool@v1", "scx.retrieve", "core/tool-host@v1", "vivy/tool-host", module.GrantFSRead),
		p8ProviderDescriptor("scx/tool-world", "std/tool-world@v1", "scx.catalog", "core/tool-host@v1", "vivy/tool-host", module.GrantNetClient),
	}
	descriptors := append(append([]module.Descriptor{}, hosts...), providers...)
	recipe := Recipe{APIVersion: RecipeAPIVersionV1}
	for _, descriptor := range descriptors {
		recipe.Modules = append(recipe.Modules, descriptor.Module.ID)
	}
	for _, provider := range providers {
		for _, grant := range provider.RequestedGrants {
			recipe.GrantApprovals = append(recipe.GrantApprovals, p8GrantApproval(provider.Module.ID, grant))
		}
	}

	plan, err := fixtureCompiler(t, descriptors).Compile(context.Background(), recipe)
	if err != nil {
		t.Fatalf("Gate A provider graph rejected: %v", err)
	}
	wantEdges := map[string]struct{ provider, consumer string }{
		"std/context-source@v1": {provider: "scx/context", consumer: "vivy/context-host"},
		"std/skill-source@v1":   {provider: "scx/skill", consumer: "vivy/skill-host"},
		"std/tool@v1":           {provider: "scx/tool", consumer: "vivy/tool-host"},
		"std/tool-world@v1":     {provider: "scx/tool-world", consumer: "vivy/tool-host"},
	}
	for portName, want := range wantEdges {
		if !p8HasPortEdge(plan, portName, want.provider, want.consumer) {
			t.Errorf("missing governed %s edge from %s to %s: %#v", portName, want.provider, want.consumer, plan.PortEdges)
		}
		if !p8LifecycleBefore(plan.LifecycleOrder, want.consumer, want.provider) {
			t.Errorf("lifecycle does not start %s before %s: %#v", want.consumer, want.provider, plan.LifecycleOrder)
		}
	}
	for _, resolved := range plan.Modules {
		if !strings.HasPrefix(resolved.Descriptor.Module.ID, "scx/") {
			continue
		}
		if resolved.Trust != TrustT2 {
			t.Errorf("%s trust = %q, want %q", resolved.Descriptor.Module.ID, resolved.Trust, TrustT2)
		}
		if len(resolved.EffectiveGrants) != len(resolved.Descriptor.RequestedGrants) {
			t.Errorf("%s effective grant count = %d, want %d", resolved.Descriptor.Module.ID, len(resolved.EffectiveGrants), len(resolved.Descriptor.RequestedGrants))
		}
		for _, grant := range resolved.Descriptor.RequestedGrants {
			want := p8GrantApproval(resolved.Descriptor.Module.ID, grant).Constraints
			got, ok := p8EffectiveGrant(resolved.EffectiveGrants, grant)
			if !ok || !reflect.DeepEqual(got.Constraints, want) {
				t.Errorf("%s grant %s = %#v, want constraints %#v", resolved.Descriptor.Module.ID, grant, got, want)
			}
		}
	}

	for _, protectedID := range []string{
		"ask_user", "bash", "execute", "list_dir", "multiedit", "patch",
		"read_file", "search_files", "skill_view", "skills_list", "write_file",
	} {
		protected := p8ProviderDescriptor("scx/protected", "std/tool@v1", protectedID, "core/tool-host@v1", "vivy/tool-host")
		_, err = fixtureCompiler(t, append(hosts, protected)).Compile(context.Background(), Recipe{
			APIVersion: RecipeAPIVersionV1,
			Modules:    []string{"vivy/context-host", "vivy/skill-host", "vivy/tool-host", "scx/protected"},
		})
		if err == nil || !strings.Contains(err.Error(), "cannot claim protected Tool id "+protectedID) {
			t.Errorf("protected Tool %s error = %v", protectedID, err)
		}
	}
}

func TestP8SCXGateARejectsPeerModuleConsumingPublicProvider(t *testing.T) {
	host := p8HostDescriptor("vivy/context-host", "core/context-host@v1")
	provider := p8ProviderDescriptor("scx/context", "std/context-source@v1", "scx.context", "core/context-host@v1", host.Module.ID)
	peer := testDescriptor("scx/peer")
	peer.Requires = []module.Requirement{{
		PortRef:  module.PortRef{Port: "std/context-source@v1", ID: "scx.context"},
		Provider: provider.Module.ID,
	}}
	_, err := fixtureCompiler(t, []module.Descriptor{host, provider, peer}).Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{host.Module.ID, provider.Module.ID, peer.Module.ID},
	})
	if err == nil || !strings.Contains(err.Error(), "public Port std/context-source@v1 may only be consumed by build-owned T1 module vivy/context-host") {
		t.Fatalf("direct Provider dependency error = %v", err)
	}
}

func TestP8SCXGateARejectsDormantPeerPublicPortRequirement(t *testing.T) {
	host := p8HostDescriptor("vivy/context-host", "core/context-host@v1")
	peer := testDescriptor("scx/peer")
	peer.Optional = []module.Requirement{{
		PortRef:  module.PortRef{Port: "std/context-source@v1", ID: "scx.context"},
		Provider: "scx/context",
	}}
	_, err := fixtureCompiler(t, []module.Descriptor{host, peer}).Compile(context.Background(), Recipe{
		APIVersion: RecipeAPIVersionV1,
		Modules:    []string{host.Module.ID, peer.Module.ID},
	})
	if err == nil || !strings.Contains(err.Error(), "public Port std/context-source@v1 may only be consumed by build-owned T1 module vivy/context-host") {
		t.Fatalf("dormant direct Provider dependency error = %v", err)
	}
}

func TestP8SCXGateAVersionedContextRepresentationNeedsNoRuntimeType(t *testing.T) {
	page, err := (p8ContextProvider{}).Query(context.Background(), contextsource.Request{
		Query: "style", SessionID: "session-1", WorkspaceID: "workspace-1", Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(page.Candidates))
	}
	candidate := page.Candidates[0]
	if candidate.SourceID != "scx" || candidate.ContentID != "personality" || candidate.MediaType != "text/plain" || candidate.Version != "v1" {
		t.Fatalf("versioned candidate identity = %#v", candidate)
	}
	if candidate.SizeHint != len(candidate.Content) {
		t.Fatalf("candidate size hint = %d, want %d", candidate.SizeHint, len(candidate.Content))
	}
}

func p8HostDescriptor(id, portName string) module.Descriptor {
	descriptor := testDescriptor(id)
	descriptor.Source.Ref = "internal:" + id
	descriptor.Provides = []module.PortRef{{Port: portName, ID: strings.ReplaceAll(id, "/", ".")}}
	return descriptor
}

func p8ProviderDescriptor(id, portName, providerID, hostPort, hostID string, grants ...module.Grant) module.Descriptor {
	descriptor := testDescriptor(id)
	descriptor.Provides = []module.PortRef{{Port: portName, ID: providerID}}
	descriptor.Requires = []module.Requirement{{PortRef: module.PortRef{Port: hostPort}, Provider: hostID}}
	descriptor.RequestedGrants = append([]module.Grant(nil), grants...)
	return descriptor
}

func p8GrantApproval(moduleID string, grant module.Grant) GrantApproval {
	approval := GrantApproval{Module: moduleID, Name: grant, Constraints: map[string][]string{}}
	if grant == module.GrantNetClient {
		approval.Constraints = map[string][]string{"hosts": {"scx.example.test"}, "schemes": {"https"}}
	}
	return approval
}

func p8EffectiveGrant(grants []EffectiveGrant, name module.Grant) (EffectiveGrant, bool) {
	for _, grant := range grants {
		if grant.Name == name {
			return grant, true
		}
	}
	return EffectiveGrant{}, false
}

func p8HasPortEdge(plan AssemblyPlan, portName, provider, consumer string) bool {
	for _, edge := range plan.PortEdges {
		if edge.Port.Port == portName && edge.Provider == provider && edge.Consumer == consumer {
			return true
		}
	}
	return false
}

func p8LifecycleBefore(order []string, before, after string) bool {
	positions := make(map[string]int, len(order))
	for index, id := range order {
		positions[id] = index
	}
	beforePosition, hasBefore := positions[before]
	afterPosition, hasAfter := positions[after]
	return hasBefore && hasAfter && beforePosition < afterPosition
}

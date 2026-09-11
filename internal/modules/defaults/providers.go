package defaults

import (
	"context"
	"encoding/json"
	"errors"

	"agent-vivy/internal/tools"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/skillsource"
	"agent-vivy/sdk/port/tool"
	"agent-vivy/sdk/port/toolworld"
)

func ProtectedToolProviders() []tool.ToolProvider {
	ids := tools.AssemblyControlledToolNames()
	out := make([]tool.ToolProvider, 0, len(ids))
	for _, id := range ids {
		out = append(out, protectedToolProvider{id: id})
	}
	return out
}

type protectedToolProvider struct{ id string }

func (p protectedToolProvider) Definition() tool.Definition { return tool.Definition{ID: p.id} }
func (p protectedToolProvider) Invoke(ctx context.Context, host tool.Host, args json.RawMessage) (tool.Result, error) {
	text, err := host.InvokeTool(ctx, p.id, args)
	return tool.Result{Text: text}, err
}

// vivyProtectedToolProvider is intentionally package-private. Third-party
// providers cannot satisfy this marker from another package, so reserved
// Tool identities remain distinguishable from public providers even when a
// minimal Assembly omits the built-in owner.
func (protectedToolProvider) vivyProtectedToolProvider() {}

type protectedToolProviderMarker interface {
	vivyProtectedToolProvider()
}

// IsProtectedToolProvider reports whether provider is one of Vivy's trusted
// built-in protected Tool providers. Identity alone is not proof of trust.
func IsProtectedToolProvider(provider tool.ToolProvider) bool {
	_, ok := provider.(protectedToolProviderMarker)
	return ok
}

// ContextSourceProviders returns the build-owned source inventory. The
// project-file Source is supplied per run by the runtime adapter because its
// contents are request/workspace data, not Generation data. This stable,
// empty source keeps the first-party ContextSource Port explicitly compiled
// while remaining side-effect free and inactive until a caller supplies a
// snapshot through ContextHost.
func ContextSourceProviders() []contextsource.Provider {
	return []contextsource.Provider{emptyContextSource{id: "vivy.project-context"}}
}

type emptyContextSource struct{ id string }

func (source emptyContextSource) ID() string { return source.id }
func (source emptyContextSource) Query(ctx context.Context, _ contextsource.Request) (contextsource.Page, error) {
	if err := ctx.Err(); err != nil {
		return contextsource.Page{}, err
	}
	return contextsource.Page{}, nil
}

// SkillSourceProviders returns the build-owned source inventory. The mutable
// local Skill backend is adapted to the same read-only Port in internal/runtime
// once the app has opened its workspace/CAS, so this source is intentionally
// empty and never carries authority or configuration.
func SkillSourceProviders() []skillsource.Provider {
	return []skillsource.Provider{emptySkillSource{id: "vivy.default-skills"}}
}

type emptySkillSource struct{ id string }

func (source emptySkillSource) ID() string { return source.id }
func (source emptySkillSource) List(ctx context.Context, _ skillsource.Request) ([]skillsource.Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
func (source emptySkillSource) Get(ctx context.Context, _ skillsource.Request, _ string) (skillsource.Skill, error) {
	if err := ctx.Err(); err != nil {
		return skillsource.Skill{}, err
	}
	return skillsource.Skill{}, errors.New("skill not found")
}

func NewMCPProvider() toolworld.Provider { return mcpProvider{} }

type mcpProvider struct{}

func (mcpProvider) Definition() toolworld.Definition {
	return toolworld.Definition{ID: "mcp", Description: "Model Context Protocol tools"}
}
func (mcpProvider) Discover(ctx context.Context, host toolworld.Host) ([]toolworld.ToolDefinition, error) {
	return nil, nil
}
func (mcpProvider) Invoke(ctx context.Context, host toolworld.Host, id string, args json.RawMessage) (toolworld.Result, error) {
	return toolworld.Result{}, toolworld.ErrInvalidArgs
}
func (mcpProvider) Close(context.Context) error { return nil }

package app

import (
	"context"
	"fmt"
	"time"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/contexthost"
	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/skillsource"
)

// bindCognitiveContextSources arms only manifested, generated T1 Sources.
// The private Runtime query capability and durable session must both match.
func bindCognitiveContextSources(assembly genassembly.RuntimeAssembly, bundle cognitivecontract.Bundle, sessions storage.SessionStore) error {
	sources, err := generatedContextSources(assembly)
	if err != nil {
		return err
	}
	for _, source := range sources {
		binder, ok := source.(interface {
			BindCognitiveContext(cognitivecontract.Bundle, func(context.Context, contextsource.Request) error) error
		})
		if !ok {
			continue
		}
		if bundle == nil || sessions == nil {
			return fmt.Errorf("app: cognitive Context Source %q has no selected owner", source.ID())
		}
		authorize := func(ctx context.Context, request contextsource.Request) error {
			if err := runtime.AuthorizeContextSourceRequest(ctx, request); err != nil {
				return err
			}
			_, err := sessions.GetSession(ctx, domain.SessionID(request.SessionID))
			return err
		}
		if err := binder.BindCognitiveContext(bundle, authorize); err != nil {
			return fmt.Errorf("app: bind cognitive Context Source %q: %w", source.ID(), err)
		}
	}
	return nil
}

// generatedContextSources and generatedSkillSources are the SDK frontend's
// typed conversion boundary. Runtime Assembly generation emits the optional
// inventory methods only for a generation that selects the corresponding
// Source Port, so a minimal overlay can physically omit source imports,
// fields, manifest edges, and code paths.
func generatedContextSources(assembly genassembly.RuntimeAssembly) ([]contextsource.Provider, error) {
	provider, ok := any(&assembly).(interface{ ContextSourceProviders() any })
	if !ok {
		return nil, nil
	}
	value := provider.ContextSourceProviders()
	if value == nil {
		return nil, nil
	}
	sources, ok := value.([]contextsource.Provider)
	if !ok {
		return nil, fmt.Errorf("app: generated ContextSource inventory has invalid type %T", value)
	}
	return append([]contextsource.Provider(nil), sources...), nil
}

func generatedSkillSources(assembly genassembly.RuntimeAssembly) ([]skillsource.Provider, error) {
	provider, ok := any(&assembly).(interface{ SkillSourceProviders() any })
	if !ok {
		return nil, nil
	}
	value := provider.SkillSourceProviders()
	if value == nil {
		return nil, nil
	}
	sources, ok := value.([]skillsource.Provider)
	if !ok {
		return nil, fmt.Errorf("app: generated SkillSource inventory has invalid type %T", value)
	}
	return append([]skillsource.Provider(nil), sources...), nil
}

func buildGeneratedContextHost(assembly genassembly.RuntimeAssembly, extra ...contextsource.Provider) (*contexthost.Host, error) {
	return buildGeneratedContextHostWithTimeout(assembly, 0, extra...)
}

func buildGeneratedContextHostWithTimeout(assembly genassembly.RuntimeAssembly, sourceTimeout time.Duration, extra ...contextsource.Provider) (*contexthost.Host, error) {
	sources, err := generatedContextSources(assembly)
	if err != nil {
		return nil, err
	}
	generatedByID := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if source != nil {
			generatedByID[source.ID()] = struct{}{}
		}
	}
	for _, source := range extra {
		if source != nil {
			sources = append(sources, source)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	required := make([]string, 0, len(assembly.Manifest.ContextSourcePolicies))
	seenPolicies := make(map[string]struct{}, len(assembly.Manifest.ContextSourcePolicies))
	for _, policy := range assembly.Manifest.ContextSourcePolicies {
		if _, ok := generatedByID[policy.ProviderID]; !ok {
			return nil, fmt.Errorf("app: sealed Context Source policy names unknown provider %q", policy.ProviderID)
		}
		if _, duplicate := seenPolicies[policy.ProviderID]; duplicate {
			return nil, fmt.Errorf("app: sealed Context Source policy repeats provider %q", policy.ProviderID)
		}
		seenPolicies[policy.ProviderID] = struct{}{}
		if policy.Required {
			required = append(required, policy.ProviderID)
		}
	}
	return contexthost.New(contexthost.Config{Sources: sources, RequiredSourceIDs: required, SourceTimeout: sourceTimeout})
}

// contextHostForAssembly combines build-owned Context Sources with an
// explicitly configured MCP Resource bridge. MCPResourceProvider is lazy and
// does not connect while this composition snapshot is built.
func contextHostForAssembly(assembly genassembly.RuntimeAssembly, mcpBackend *runtime.MCPBackend, sourceTimeout ...time.Duration) (*contexthost.Host, error) {
	// A packed generation that omits ContextHost cannot regain that Host by
	// selecting an MCP Resource bridge at runtime. The generated manifest is
	// the sealed composition boundary; an absent Host means this capability is
	// unavailable even when an MCP backend is present.
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/context-host") {
		return nil, nil
	}
	var extra contextsource.Provider
	if mcpBackend != nil {
		var err error
		extra, err = mcpBackend.MCPResourceProvider()
		if err != nil {
			return nil, err
		}
	}
	var timeout time.Duration
	if len(sourceTimeout) > 0 {
		timeout = sourceTimeout[0]
	}
	return buildGeneratedContextHostWithTimeout(assembly, timeout, extra)
}

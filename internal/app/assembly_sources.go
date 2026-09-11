package app

import (
	"fmt"

	"agent-vivy/internal/contexthost"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/runtime"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/skillsource"
)

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
	sources, err := generatedContextSources(assembly)
	if err != nil {
		return nil, err
	}
	for _, source := range extra {
		if source != nil {
			sources = append(sources, source)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	return contexthost.New(contexthost.Config{Sources: sources})
}

// contextHostForAssembly combines build-owned Context Sources with an
// explicitly configured MCP Resource bridge. MCPResourceProvider is lazy and
// does not connect while this composition snapshot is built.
func contextHostForAssembly(assembly genassembly.RuntimeAssembly, mcpBackend *runtime.MCPBackend) (*contexthost.Host, error) {
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
	return buildGeneratedContextHost(assembly, extra)
}

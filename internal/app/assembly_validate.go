package app

import (
	"fmt"
	"slices"
	"strings"

	"agent-vivy/internal/config"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/sdk/module"
)

func assemblyHasModule(moduleIDs []string, wanted string) bool {
	return slices.Contains(moduleIDs, wanted)
}

// validateRuntimeAssemblyConfig rejects configuration that names a runtime
// capability absent from the sealed Assembly. This check complements the
// generated-provider identity checks: config cannot turn an omitted Host back
// into a live feature through a settings overlay or MCP Resource bridge.
func validateRuntimeAssemblyConfig(assembly genassembly.RuntimeAssembly, cfg config.Config) error {
	mcpCompiled := assemblyHasModule(assembly.Manifest.Modules, "vivy/mcp-host")
	contextCompiled := assemblyHasModule(assembly.Manifest.Modules, "vivy/context-host")
	for _, server := range cfg.Runtime.MCPServers {
		if !mcpCompiled {
			return fmt.Errorf("app: configured MCP server %q requires compiled MCPHost", server.Name)
		}
		if server.ResourceBridge && !contextCompiled {
			return fmt.Errorf("app: MCP Resource bridge for %q requires compiled ContextHost", server.Name)
		}
	}
	return nil
}

// validateRuntimeAssembly proves that runtime Provider values still expose the
// identities sealed by the compiler. A constructor cannot redirect a compiled
// slot to another provider (including a protected kernel tool).
func validateRuntimeAssembly(assembly genassembly.RuntimeAssembly) error {
	compiledModules := make(map[string]bool, len(assembly.Manifest.Modules))
	for _, moduleID := range assembly.Manifest.Modules {
		compiledModules[moduleID] = true
	}
	contextSources, err := generatedContextSources(assembly)
	if err != nil {
		return fmt.Errorf("app: generated ContextSource inventory: %w", err)
	}
	contextIDs := make([]string, 0, len(contextSources))
	for _, source := range contextSources {
		if source == nil || source.ID() == "" {
			return fmt.Errorf("app: generated ContextSource provider has no identity")
		}
		contextIDs = append(contextIDs, source.ID())
	}
	if !slices.Equal(contextIDs, assembly.Manifest.ContextSources) {
		return fmt.Errorf("app: generated ContextSource identities %v do not match sealed manifest %v", contextIDs, assembly.Manifest.ContextSources)
	}
	if len(contextIDs) > 0 && !compiledModules["vivy/context-host"] {
		return fmt.Errorf("app: generated ContextSources are present without compiled ContextHost")
	}

	skillSources, err := generatedSkillSources(assembly)
	if err != nil {
		return fmt.Errorf("app: generated SkillSource inventory: %w", err)
	}
	skillIDs := make([]string, 0, len(skillSources))
	for _, source := range skillSources {
		if source == nil || source.ID() == "" {
			return fmt.Errorf("app: generated SkillSource provider has no identity")
		}
		skillIDs = append(skillIDs, source.ID())
	}
	if !slices.Equal(skillIDs, assembly.Manifest.SkillSources) {
		return fmt.Errorf("app: generated SkillSource identities %v do not match sealed manifest %v", skillIDs, assembly.Manifest.SkillSources)
	}
	if len(skillIDs) > 0 && !compiledModules["vivy/skill-host"] {
		return fmt.Errorf("app: generated SkillSources are present without compiled SkillHost")
	}

	toolIDs := make([]string, 0, len(assembly.Tools))
	for _, provider := range assembly.Tools {
		if provider == nil {
			return fmt.Errorf("app: nil generated Tool provider")
		}
		toolIDs = append(toolIDs, provider.Definition().ID)
	}
	if !slices.Equal(toolIDs, assembly.Manifest.Tools) {
		return fmt.Errorf("app: generated Tool identities %v do not match sealed manifest %v", toolIDs, assembly.Manifest.Tools)
	}

	worldIDs := make([]string, 0, len(assembly.Worlds))
	for _, provider := range assembly.Worlds {
		if provider == nil {
			return fmt.Errorf("app: nil generated ToolWorld provider")
		}
		worldIDs = append(worldIDs, provider.Definition().ID)
		if _, ok := assembly.ToolWorldGrants[provider.Definition().ID]; !ok {
			return fmt.Errorf("app: generated ToolWorld %q has no sealed grant binding", provider.Definition().ID)
		}
	}
	if !slices.Equal(worldIDs, assembly.Manifest.ToolWorlds) {
		return fmt.Errorf("app: generated ToolWorld identities %v do not match sealed manifest %v", worldIDs, assembly.Manifest.ToolWorlds)
	}
	if slices.Contains(worldIDs, "mcp") && !compiledModules["vivy/mcp-host"] {
		return fmt.Errorf("app: generated MCP ToolWorld is present without compiled MCPHost")
	}

	channelIDs := make([]string, 0, len(assembly.Channels))
	for _, provider := range assembly.Channels {
		if provider == nil {
			return fmt.Errorf("app: nil generated Channel provider")
		}
		providerID := provider.Definition().ID
		if _, ok := assembly.ChannelGrants[providerID]; !ok {
			return fmt.Errorf("app: generated Channel %q has no sealed grant binding", providerID)
		}
		channelIDs = append(channelIDs, strings.TrimPrefix(providerID, "vivy."))
	}
	if !slices.Equal(channelIDs, assembly.Manifest.Channels) {
		return fmt.Errorf("app: generated Channel identities %v do not match sealed manifest %v", channelIDs, assembly.Manifest.Channels)
	}

	if assembly.Manifest.Face == "kernel-headless" {
		if assembly.Face != nil {
			return fmt.Errorf("app: generated Face %q is not sealed as compiled", assembly.Face.Definition().ID)
		}
	} else if assembly.Face == nil || assembly.Face.Definition().ID != assembly.Manifest.Face {
		actual := "<nil>"
		if assembly.Face != nil {
			actual = assembly.Face.Definition().ID
		}
		return fmt.Errorf("app: generated Face identity %q does not match sealed manifest %q", actual, assembly.Manifest.Face)
	}

	if err := validateGrantKeys("ToolWorld", assembly.ToolWorldGrants, assembly.Manifest.ToolWorlds, false); err != nil {
		return err
	}
	if len(assembly.DiagnosticObservers) != len(assembly.DiagnosticObserverWorldIDs) {
		return fmt.Errorf("app: generated diagnostic observer identities do not match providers")
	}
	worldSet := make(map[string]bool, len(assembly.Manifest.ToolWorlds))
	for _, id := range assembly.Manifest.ToolWorlds {
		worldSet[id] = true
	}
	for index, id := range assembly.DiagnosticObserverWorldIDs {
		if assembly.DiagnosticObservers[index] == nil || !worldSet[id] {
			return fmt.Errorf("app: generated diagnostic observer has unsealed ToolWorld %q", id)
		}
	}
	for _, status := range assembly.LanguageServerStatuses {
		if status == nil {
			return fmt.Errorf("app: nil generated language server status provider")
		}
	}
	return validateGrantKeys("Channel", assembly.ChannelGrants, assembly.Manifest.Channels, true)
}

func validateGrantKeys(kind string, grants map[string][]module.GrantBinding, ids []string, channel bool) error {
	expected := make(map[string]bool, len(ids))
	for _, id := range ids {
		if channel {
			id = "vivy." + id
		}
		expected[id] = true
	}
	if len(grants) != len(expected) {
		return fmt.Errorf("app: generated %s grant bindings do not match sealed providers", kind)
	}
	for id := range grants {
		if !expected[id] {
			return fmt.Errorf("app: generated %s grant binding has unsealed provider %q", kind, id)
		}
	}
	return nil
}

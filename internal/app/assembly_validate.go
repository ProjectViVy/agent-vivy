package app

import (
	"fmt"
	"slices"
	"strings"

	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/sdk/module"
)

// validateRuntimeAssembly proves that runtime Provider values still expose the
// identities sealed by the compiler. A constructor cannot redirect a compiled
// slot to another provider (including a protected kernel tool).
func validateRuntimeAssembly(assembly genassembly.RuntimeAssembly) error {
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

	profileIDs := make([]string, 0, len(assembly.ProviderProfiles))
	for _, provider := range assembly.ProviderProfiles {
		if provider == nil {
			return fmt.Errorf("app: nil generated Provider Profile")
		}
		profile := provider.Definition()
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("app: generated Provider Profile %q is invalid: %w", profile.ID, err)
		}
		profileIDs = append(profileIDs, profile.ID)
	}
	if !slices.Equal(profileIDs, assembly.Manifest.ProviderProfiles) {
		return fmt.Errorf("app: generated Provider Profile identities %v do not match sealed manifest %v", profileIDs, assembly.Manifest.ProviderProfiles)
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

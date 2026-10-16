package app

import (
	genassembly "agent-vivy/internal/generated/assembly"
	memorymodule "agent-vivy/internal/modules/memory"
	"fmt"
	"slices"
)

// Copy selected firstparty providers before dispatch, preserving the caller's
// reusable Assembly and old Apps' exact bindings. No unrelated provider is copied.
func bindMemoryProviders(a *genassembly.RuntimeAssembly, service *memorymodule.Service) error {
	var err error
	a.Tools = slices.Clone(a.Tools)
	for i, p := range a.Tools {
		a.Tools[i], err = memorymodule.BindToolProvider(p, service)
		if err != nil {
			return fmt.Errorf("app: bind memory tool: %w", err)
		}
	}
	a.ContextSources = slices.Clone(a.ContextSources)
	for i, p := range a.ContextSources {
		a.ContextSources[i], err = memorymodule.BindContextSource(p, service)
		if err != nil {
			return fmt.Errorf("app: bind memory source: %w", err)
		}
	}
	a.RunObservers = slices.Clone(a.RunObservers)
	for i, p := range a.RunObservers {
		a.RunObservers[i], err = memorymodule.BindRunProvider(p, service)
		if err != nil {
			return fmt.Errorf("app: bind memory observer: %w", err)
		}
	}
	a.ActionSets = slices.Clone(a.ActionSets)
	for i := range a.ActionSets {
		a.ActionSets[i].Providers = slices.Clone(a.ActionSets[i].Providers)
		for j, p := range a.ActionSets[i].Providers {
			a.ActionSets[i].Providers[j], err = memorymodule.BindActionProvider(p, service)
			if err != nil {
				return fmt.Errorf("app: bind memory action: %w", err)
			}
		}
	}
	return nil
}

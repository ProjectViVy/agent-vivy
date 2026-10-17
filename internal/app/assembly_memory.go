package app

import (
	genassembly "agent-vivy/internal/generated/assembly"
	memorymodule "agent-vivy/internal/modules/memory"
	"agent-vivy/sdk/port/contextsource"
	"agent-vivy/sdk/port/observer"
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
	// Optional generated inventories must remain absent in minimal Recipes.
	// Replace the copied Assembly's slice, never mutate a reusable caller slice.
	if inventory, ok := any(a).(interface{ ContextSourceProviderSlots() any }); ok {
		slots, ok := inventory.ContextSourceProviderSlots().(*[]contextsource.Provider)
		if !ok || slots == nil {
			return fmt.Errorf("app: invalid generated Context Source slots")
		}
		*slots = slices.Clone(*slots)
		for i, p := range *slots {
			(*slots)[i], err = memorymodule.BindContextSource(p, service)
			if err != nil {
				return fmt.Errorf("app: bind memory source: %w", err)
			}
		}
	}
	if inventory, ok := any(a).(interface{ RunObserverProviderSlots() any }); ok {
		slots, ok := inventory.RunObserverProviderSlots().(*[]observer.RunProvider)
		if !ok || slots == nil {
			return fmt.Errorf("app: invalid generated Run Observer slots")
		}
		*slots = slices.Clone(*slots)
		for i, p := range *slots {
			(*slots)[i], err = memorymodule.BindRunProvider(p, service)
			if err != nil {
				return fmt.Errorf("app: bind memory observer: %w", err)
			}
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

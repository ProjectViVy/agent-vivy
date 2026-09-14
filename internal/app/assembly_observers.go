package app

import (
	"context"
	"fmt"
	"slices"

	"agent-vivy/internal/domain"
	genassembly "agent-vivy/internal/generated/assembly"
	"agent-vivy/internal/observerhost"
	"agent-vivy/internal/storage"
	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/port/observer"
)

func generatedRunObservers(inventory any) ([]observer.RunProvider, error) {
	provider, ok := inventory.(interface{ RunObserverProviders() any })
	if !ok {
		return nil, nil
	}
	value := provider.RunObserverProviders()
	if value == nil {
		return nil, nil
	}
	observers, ok := value.([]observer.RunProvider)
	if !ok {
		return nil, fmt.Errorf("app: generated Run Observer inventory has invalid type %T", value)
	}
	return append([]observer.RunProvider(nil), observers...), nil
}

func buildRunSubscriptions(inventory any, manifest generation.Manifest) ([]observerhost.RunSubscription, error) {
	providers, err := generatedRunObservers(inventory)
	if err != nil {
		return nil, err
	}
	providerIDs := make([]string, 0, len(providers))
	byID := make(map[string]observer.RunProvider, len(providers))
	for _, provider := range providers {
		if provider == nil || provider.ID() == "" {
			return nil, fmt.Errorf("app: generated Run Observer provider has no identity")
		}
		if _, duplicate := byID[provider.ID()]; duplicate {
			return nil, fmt.Errorf("app: generated Run Observer identity %q is duplicated", provider.ID())
		}
		providerIDs = append(providerIDs, provider.ID())
		byID[provider.ID()] = provider
	}
	if !slices.Equal(providerIDs, manifest.RunObservers) {
		return nil, fmt.Errorf("app: generated Run Observer identities %v do not match sealed manifest %v", providerIDs, manifest.RunObservers)
	}
	if len(manifest.RunObserverPolicies) != len(providers) {
		return nil, fmt.Errorf("app: sealed Run Observer policy count does not match providers")
	}
	subscriptions := make([]observerhost.RunSubscription, 0, len(providers))
	seen := make(map[string]struct{}, len(providers))
	for _, policy := range manifest.RunObserverPolicies {
		provider, ok := byID[policy.ProviderID]
		if !ok {
			return nil, fmt.Errorf("app: sealed Run Observer policy names unknown provider %q", policy.ProviderID)
		}
		if _, duplicate := seen[policy.ProviderID]; duplicate {
			return nil, fmt.Errorf("app: sealed Run Observer policy repeats provider %q", policy.ProviderID)
		}
		seen[policy.ProviderID] = struct{}{}
		subscriptions = append(subscriptions, observerhost.RunSubscription{
			Provider: provider, EventTypes: append([]string(nil), policy.EventTypes...),
			AllowedPayloadFields: append([]string(nil), policy.AllowedPayloadFields...),
		})
	}
	return subscriptions, nil
}

func observerHostForAssembly(ctx context.Context, assembly genassembly.RuntimeAssembly, backend storage.Engine) (*observerhost.Host, error) {
	subscriptions, err := buildRunSubscriptions(&assembly, assembly.Manifest)
	if err != nil {
		return nil, err
	}
	if len(subscriptions) == 0 {
		return nil, nil
	}
	if !assemblyHasModule(assembly.Manifest.Modules, "vivy/observer-host") {
		return nil, fmt.Errorf("app: generated Run Observers are present without compiled ObserverHost")
	}
	recoverRunIDs, err := terminalRunIDs(ctx, backend)
	if err != nil {
		return nil, fmt.Errorf("app: discover terminal Run Observer recovery: %w", err)
	}
	return observerhost.New(observerhost.Config{Journal: backend, Cursors: backend.Snapshot(), RunSubscriptions: subscriptions, RecoverRunIDs: recoverRunIDs})
}

func terminalRunIDs(ctx context.Context, backend storage.Engine) ([]domain.RunID, error) {
	sessions, err := backend.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	var ids []domain.RunID
	for _, session := range sessions {
		runs, err := backend.ListRunsBySession(ctx, session.ID)
		if err != nil {
			return nil, err
		}
		for _, run := range runs {
			if run.Status.Terminal() {
				ids = append(ids, run.ID)
			}
		}
	}
	return ids, nil
}

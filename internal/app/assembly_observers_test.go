package app

import (
	"context"
	"strings"
	"testing"

	"agent-vivy/sdk/generation"
	"agent-vivy/sdk/port/observer"
)

type appRunObserver struct{ id string }

func (provider appRunObserver) ID() string                                 { return provider.id }
func (appRunObserver) ObserveRun(context.Context, observer.RunEvent) error { return nil }

type appRunObserverInventory struct{ providers []observer.RunProvider }

func (inventory *appRunObserverInventory) RunObserverProviders() any { return inventory.providers }

func TestBuildRunSubscriptionsUsesSealedHostPolicy(t *testing.T) {
	inventory := &appRunObserverInventory{providers: []observer.RunProvider{appRunObserver{id: "fixture.memory"}}}
	manifest := generation.Manifest{
		RunObservers: []string{"fixture.memory"},
		RunObserverPolicies: []generation.RunObserverPolicy{{
			ProviderID: "fixture.memory", EventTypes: []string{"run.completed"}, AllowedPayloadFields: []string{"outcome", "result", "view"},
		}},
	}
	subscriptions, err := buildRunSubscriptions(inventory, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(subscriptions) != 1 || subscriptions[0].Provider.ID() != "fixture.memory" || len(subscriptions[0].EventTypes) != 1 || len(subscriptions[0].AllowedPayloadFields) != 3 {
		t.Fatalf("subscriptions = %#v", subscriptions)
	}

	manifest.RunObserverPolicies[0].ProviderID = "fixture.other"
	if _, err := buildRunSubscriptions(inventory, manifest); err == nil || !strings.Contains(err.Error(), "sealed") {
		t.Fatalf("policy/provider drift error = %v", err)
	}
}

func TestBuildRunSubscriptionsRejectsProviderWithoutSealedPolicy(t *testing.T) {
	inventory := &appRunObserverInventory{providers: []observer.RunProvider{appRunObserver{id: "fixture.memory"}}}
	manifest := generation.Manifest{RunObservers: []string{"fixture.memory"}}
	if _, err := buildRunSubscriptions(inventory, manifest); err == nil {
		t.Fatal("observer provider without sealed policy was accepted")
	}
}

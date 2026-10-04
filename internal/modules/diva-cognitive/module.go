// Package divacognitive is the build-linked T1 Module owning the DIVA
// embedded cognitive capability: one Garden domain facade bound through the
// sealed Assembly factory seam and the ledger C2-3 human control actions.
// Construction is side-effect free; the typed factory opens the single owner.
package divacognitive

import (
	"context"

	"agent-vivy/sdk/module"
	controlaction "agent-vivy/sdk/port/controlaction"
)

const (
	// ID is the sole Module identity allowed to provide the closed
	// core/cognitive-factory@v1 Port.
	ID   = "vivy/diva-cognitive"
	Port = "core/cognitive-factory@v1"
	// ProviderID is the factory's provided identity.
	ProviderID = "vivy.cognitive-factory"
)

// Ledger C2-3 declares exactly these human control-plane actions. The IDs are
// sealed into the Assembly inventory now; DN-4C owns handler dispatch. No
// action is a model tool and none reports success while unarmed.
const (
	ActionStatus              = "diva.cognitive.status"
	ActionPersonaInitialize   = "diva.cognitive.persona.initialize"
	ActionPersonaRead         = "diva.cognitive.persona.read"
	ActionPersonaSave         = "diva.cognitive.persona.save"
	ActionPersonaReviewList   = "diva.cognitive.persona.reviews.list"
	ActionPersonaReviewDecide = "diva.cognitive.persona.review.decide"
	ActionFrozenRead          = "diva.cognitive.frozen.read"
	ActionActmemRead          = "diva.cognitive.actmem.read"
	ActionActmemWorkPatch     = "diva.cognitive.actmem.work.patch"
	ActionActmemOwnerRead     = "diva.cognitive.actmem.owner.read"
	ActionActmemOwnerSave     = "diva.cognitive.actmem.owner.save"
	ActionMemorySearch        = "diva.cognitive.memory.search"
	ActionMemoryExpand        = "diva.cognitive.memory.expand"
	ActionMemoryMutate        = "diva.cognitive.memory.mutate"
	ActionMemoryReceipt       = "diva.cognitive.memory.receipt"
	ActionPolicyGet           = "diva.cognitive.policy.get"
	ActionPolicySet           = "diva.cognitive.policy.set"
	ActionTrigger             = "diva.cognitive.trigger"
	ActionCancel              = "diva.cognitive.cancel"
	ActionResultsList         = "diva.cognitive.results.list"
)

// ActionIDs is the closed action inventory in ledger order.
var ActionIDs = []string{
	ActionStatus,
	ActionPersonaInitialize,
	ActionPersonaRead,
	ActionPersonaSave,
	ActionPersonaReviewList,
	ActionPersonaReviewDecide,
	ActionFrozenRead,
	ActionActmemRead,
	ActionActmemWorkPatch,
	ActionActmemOwnerRead,
	ActionActmemOwnerSave,
	ActionMemorySearch,
	ActionMemoryExpand,
	ActionMemoryMutate,
	ActionMemoryReceipt,
	ActionPolicyGet,
	ActionPolicySet,
	ActionTrigger,
	ActionCancel,
	ActionResultsList,
}

// NewModule is the pure Assembly lifecycle owner. It is independent from the
// factory value bound under GoBinding.CognitiveFactory.
func NewModule() module.Module { return ownerModule{} }

type ownerModule struct{}

func (ownerModule) Descriptor() module.Descriptor {
	provides := []module.PortRef{{Port: Port, ID: ProviderID}}
	for _, id := range ActionIDs {
		provides = append(provides, module.PortRef{Port: controlaction.Port, ID: id})
	}
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: ID, Version: "1.0.0"},
		Source:     module.Source{Ref: "file:internal", SHA256: zeroDigest},
		Provides:   provides,
		// The durable capture subscription is delivered through ObserverHost
		// and the human actions through ActionHost; both Hosts must be
		// selected for this capability to be composable.
		Requires: []module.Requirement{
			{PortRef: module.PortRef{Port: "core/observer-host@v1"}, Provider: "vivy/observer-host"},
			{PortRef: module.PortRef{Port: "core/action-host@v1"}, Provider: "vivy/action-host"},
		},
		Lifecycle: module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

func (ownerModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return ownerInstance{}, nil
}

type ownerInstance struct{}

func (ownerInstance) Start(context.Context) error { return nil }
func (ownerInstance) Ready(context.Context) error { return nil }
func (ownerInstance) Stop(context.Context) error  { return nil }
func (ownerInstance) Close(context.Context) error { return nil }

const zeroDigest = "0000000000000000000000000000000000000000000000000000000000000000"

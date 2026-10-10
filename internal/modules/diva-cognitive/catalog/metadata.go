// Package catalog owns static module metadata without loading its optional implementation.
package catalog

const (
	// ID is the sole Module identity allowed to provide the closed
	// core/cognitive-factory@v1 Port.
	ID   = "vivy/diva-cognitive"
	Port = "core/cognitive-factory@v1"
	// ProviderID is the factory's provided identity.
	ProviderID = "vivy.cognitive-factory"
)

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

const RecallModuleID = "vivy/diva-memory"

const RecallProviderID = "vivy.memory.mentle"

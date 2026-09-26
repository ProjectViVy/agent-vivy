package memory

import (
	"context"
	"encoding/json"

	controlaction "agent-vivy/sdk/port/controlaction"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// Action IDs for the vivy/memory-bml control-action plane.
const (
	ActionList       = "vivy.memory.list"
	ActionSearch     = "vivy.memory.search"
	ActionGet        = "vivy.memory.get"
	ActionAdd        = "vivy.memory.add"
	ActionUpdate     = "vivy.memory.update"
	ActionRemove     = "vivy.memory.remove"
	ActionRulesRead  = "vivy.memory.rules.read"
	ActionRulesWrite = "vivy.memory.rules.write"
	ActionStatus     = "vivy.memory.status"
)

const (
	maxActionInput  = 32 << 10
	maxActionOutput = 256 << 10
)

type memoryAction struct {
	definition controlaction.Definition
}

func (a memoryAction) Definition() controlaction.Definition { return a.definition }

// Invoke resolves the service at call time: closed registry yields the
// explicit bml_unavailable outcome; the per-action service call sites land
// with the action implementations.
func (a memoryAction) Invoke(_ context.Context, _ controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	if len(input) > maxActionInput {
		return marshalOutcome(bml.MemoryCrudOutcome{Status: bml.CrudOutcomeFailed, Reason: bml.HomeCodeInvalidRequest})
	}
	if Active() == nil {
		return marshalOutcome(unavailableOutcome())
	}
	return marshalOutcome(bml.UnsupportedCrudOutcome(a.definition.ID))
}

// ActionProviders returns the closed action inventory owned by
// vivy/memory-bml.
func ActionProviders() []controlaction.Provider {
	return []controlaction.Provider{
		memoryAction{definition: actionDefinition(ActionList, "List visible long-term memory records", controlaction.EffectRead)},
		memoryAction{definition: actionDefinition(ActionSearch, "Search visible long-term memory", controlaction.EffectRead)},
		memoryAction{definition: actionDefinition(ActionGet, "Get one visible memory record", controlaction.EffectRead)},
		memoryAction{definition: actionDefinition(ActionAdd, "Add a user-asserted long-term memory record", controlaction.EffectWrite)},
		memoryAction{definition: actionDefinition(ActionUpdate, "Update a memory record under revision CAS", controlaction.EffectWrite)},
		memoryAction{definition: actionDefinition(ActionRemove, "Tombstone a memory record under revision CAS", controlaction.EffectWrite)},
		memoryAction{definition: actionDefinition(ActionRulesRead, "Read the MEMRULES handbook", controlaction.EffectRead)},
		memoryAction{definition: actionDefinition(ActionRulesWrite, "Replace the MEMRULES handbook", controlaction.EffectWrite)},
		memoryAction{definition: actionDefinition(ActionStatus, "Report memory service status", controlaction.EffectRead)},
	}
}

func actionDefinition(id, description string, effect controlaction.Effect) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		InputSchema: objectSchema, ResultSchema: outcomeSchema,
		MaxInputBytes: maxActionInput, MaxOutputBytes: maxActionOutput,
	}
}

func marshalOutcome(outcome bml.MemoryCrudOutcome) (json.RawMessage, error) {
	return json.Marshal(outcome)
}

var (
	objectSchema  = json.RawMessage(`{"type":"object"}`)
	outcomeSchema = json.RawMessage(`{"type":"object","required":["status"]}`)
)

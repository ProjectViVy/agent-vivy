package divacognitive

import (
	"context"
	"encoding/json"

	"agent-vivy/internal/cognitivecontract"
	controlaction "agent-vivy/sdk/port/controlaction"
)

// Ledger C2-3 bounds: one input body is at most 64KiB, one result at most
// 256KiB.
const (
	maxCognitiveInput  = 64 << 10
	maxCognitiveOutput = 256 << 10
)

// cognitiveAction is a sealed action definition; invocation resolves the
// armed dispatcher through the private ActionHost facade. Until that facade
// exists the invoke path fails closed — no action reports success.
type cognitiveAction struct {
	definition controlaction.Definition
}

func (a cognitiveAction) Definition() controlaction.Definition { return a.definition }

func (a cognitiveAction) Invoke(ctx context.Context, host controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	privateHost, ok := host.(cognitivecontract.ActionHost)
	if !ok || privateHost == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	if len(input) > maxCognitiveInput {
		return nil, cognitivecontract.ErrUnarmed
	}
	dispatcher, err := privateHost.Cognitive()
	if err != nil {
		return nil, err
	}
	return dispatcher.Invoke(ctx, a.definition.ID, append(json.RawMessage(nil), input...))
}

// ActionProviders returns the closed ledger C2-3 inventory owned by
// vivy/diva-cognitive. Every definition is emitted now so the manifest equals
// the ActionSets; handlers stay fail-closed until DN-4C.
func ActionProviders() []controlaction.Provider {
	defs := []controlaction.Definition{
		actionDef(ActionStatus, "Cognitive capability status", controlaction.EffectRead, schema(sessionProp)),
		actionDef(ActionPersonaInitialize, "Initialize the persona authority", controlaction.EffectWrite, schema(
			sessionProp, prop("initialization", `{"type":"object","additionalProperties":true}`), prop("reason", strProp))),
		actionDef(ActionPersonaRead, "Read a persona authority document", controlaction.EffectRead, schema(
			sessionProp, prop("kind", strProp))),
		actionDef(ActionPersonaSave, "Save a persona authority document", controlaction.EffectWrite, schema(
			sessionProp, prop("kind", strProp), prop("content", strProp), prop("reason", strProp))),
		actionDef(ActionPersonaReviewList, "List persona review requests", controlaction.EffectRead, schema(
			sessionProp, prop("kind", strProp), prop("state", strProp), prop("cursor", strProp), prop("limit", numProp))),
		actionDef(ActionPersonaReviewDecide, "Approve or reject a persona review", controlaction.EffectWrite, schema(
			sessionProp, prop("request_id", strProp), prop("decision", strProp), prop("reason", strProp))),
		actionDef(ActionFrozenRead, "Read the session Frozen Core", controlaction.EffectRead, schema(sessionProp)),
		actionDef(ActionActmemRead, "Read the ACTMEM authority", controlaction.EffectRead, schema(sessionProp)),
		actionDef(ActionActmemWorkPatch, "Apply an ACTMEM work patch", controlaction.EffectWrite, schema(
			sessionProp, prop("patch", `{"type":"object","additionalProperties":true}`))),
		actionDef(ActionActmemOwnerRead, "Read the owner's ACTMEM", controlaction.EffectRead, schema(sessionProp)),
		actionDef(ActionActmemOwnerSave, "Save the owner's ACTMEM", controlaction.EffectWrite, schema(
			sessionProp, prop("content", strProp), prop("reason", strProp))),
		actionDef(ActionMemorySearch, "Search memory cards", controlaction.EffectRead, schema(
			sessionProp, prop("query", strProp), prop("cursor", strProp), prop("limit", numProp))),
		actionDef(ActionMemoryExpand, "Expand a memory card to evidence", controlaction.EffectRead, schema(
			sessionProp, prop("card_id", strProp))),
		actionDef(ActionMemoryMutate, "Submit a memory mutation", controlaction.EffectWrite, schema(
			sessionProp, prop("mutation", `{"type":"object","additionalProperties":true}`))),
		actionDef(ActionMemoryReceipt, "Read a memory mutation receipt", controlaction.EffectRead, schema(
			sessionProp, prop("receipt_id", strProp))),
		actionDef(ActionPolicyGet, "Read the trigger policy", controlaction.EffectRead, schema(sessionProp)),
		actionDef(ActionPolicySet, "CAS-update the trigger policy", controlaction.EffectWrite, schema(
			sessionProp, prop("base_revision", numProp), prop("policy", `{"type":"object","additionalProperties":true}`))),
		actionDef(ActionTrigger, "Manually trigger a cognitive run", controlaction.EffectWrite, schema(
			sessionProp, prop("reason", strProp))),
		actionDef(ActionCancel, "Cancel the active cognitive run", controlaction.EffectWrite, schema(
			sessionProp, prop("run_id", strProp))),
		actionDef(ActionResultsList, "Page effect receipts", controlaction.EffectRead, schema(
			sessionProp, prop("cursor", strProp), prop("limit", numProp))),
	}
	providers := make([]controlaction.Provider, 0, len(defs))
	for _, def := range defs {
		providers = append(providers, cognitiveAction{definition: def})
	}
	return providers
}

func actionDef(id, description string, effect controlaction.Effect, input json.RawMessage) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		InputSchema: input, ResultSchema: json.RawMessage(`{"type":"object","additionalProperties":true}`),
		MaxInputBytes: maxCognitiveInput, MaxOutputBytes: maxCognitiveOutput,
	}
}

const strProp = `{"type":"string"}`
const numProp = `{"type":"integer","minimum":0}`

var sessionProp = prop("session_id", strProp)

func prop(name, schemaJSON string) string {
	return `"` + name + `":` + schemaJSON
}

func schema(props ...string) json.RawMessage {
	joined := "{"
	for i, p := range props {
		if i > 0 {
			joined += ","
		}
		joined += p
	}
	joined += "}"
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["session_id"],"properties":` + joined + `}`)
}

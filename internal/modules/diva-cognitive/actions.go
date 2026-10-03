package divacognitive

import (
	"context"
	"encoding/json"
	"strings"

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
// armed dispatcher through the private ActionHost facade.
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
		return nil, controlaction.ErrInvalidInput
	}
	dispatcher, err := privateHost.Cognitive()
	if err != nil {
		return nil, err
	}
	return dispatcher.Invoke(ctx, a.definition.ID, append(json.RawMessage(nil), input...))
}

// ActionProviders returns the closed ledger C2-3 inventory owned by
// vivy/diva-cognitive. Input schemas are strict closed objects; required
// fields follow the contract table verbatim.
func ActionProviders() []controlaction.Provider {
	anyObject := `{"type":"object","additionalProperties":true}`
	boolProp := `{"type":"boolean"}`
	defs := []controlaction.Definition{
		actionDef(ActionStatus, "Cognitive capability status", controlaction.EffectRead,
			schema(nil, sessionProp)),
		actionDef(ActionPersonaInitialize, "Initialize the persona authority", controlaction.EffectWrite,
			schema(req("initialization"), sessionProp, prop("initialization", anyObject), prop("reason", strProp))),
		actionDef(ActionPersonaRead, "Read a persona authority document", controlaction.EffectRead,
			schema(req("kind"), sessionProp, prop("kind", strProp))),
		actionDef(ActionPersonaSave, "Save a persona authority document", controlaction.EffectWrite,
			schema(req("kind", "content", "base_revision"), sessionProp, prop("kind", strProp), prop("content", strProp), prop("base_revision", numProp), prop("reason", strProp))),
		actionDef(ActionPersonaReviewList, "List persona review requests", controlaction.EffectRead,
			schema(nil, sessionProp, prop("kind", strProp), prop("state", strProp), prop("cursor", strProp), prop("limit", numProp))),
		actionDef(ActionPersonaReviewDecide, "Approve or reject a persona review", controlaction.EffectWrite,
			schema(req("review_id", "decision"), sessionProp, prop("review_id", strProp), prop("decision", `{"type":"string","enum":["accept","reject"]}`))),
		actionDef(ActionFrozenRead, "Read the session Frozen Core", controlaction.EffectRead,
			schema(nil, sessionProp)),
		actionDef(ActionActmemRead, "Read the ACTMEM authority", controlaction.EffectRead,
			schema(req("sections", "max_chars"), sessionProp, prop("sections", `{"type":"array","items":{"type":"string","enum":["pulse","recap","work"]}}`), prop("max_chars", numProp))),
		actionDef(ActionActmemWorkPatch, "Apply an ACTMEM work patch", controlaction.EffectWrite,
			schema(req("patch"), sessionProp, prop("patch", anyObject))),
		actionDef(ActionActmemOwnerRead, "Read the owner's ACTMEM", controlaction.EffectRead,
			schema(nil, sessionProp)),
		actionDef(ActionActmemOwnerSave, "Save the owner's ACTMEM", controlaction.EffectWrite,
			schema(req("markdown", "base_revision"), sessionProp, prop("markdown", strProp), prop("base_revision", numProp))),
		actionDef(ActionMemorySearch, "Search memory cards", controlaction.EffectRead,
			schema(req("query", "limit", "budget_chars"), sessionProp, prop("query", strProp), prop("collection", strProp), prop("cursor", strProp), prop("limit", numProp), prop("budget_chars", numProp))),
		actionDef(ActionMemoryExpand, "Expand a memory card to evidence", controlaction.EffectRead,
			schema(req("card_id", "expected_revision", "budget_chars"), sessionProp, prop("card_id", strProp), prop("expected_revision", numProp), prop("budget_chars", numProp))),
		actionDef(ActionMemoryMutate, "Submit a memory mutation", controlaction.EffectWrite,
			schema(req("mutation"), sessionProp, prop("mutation", anyObject))),
		actionDef(ActionMemoryReceipt, "Read a memory mutation receipt", controlaction.EffectRead,
			schema(req("operation_id"), sessionProp, prop("operation_id", strProp))),
		actionDef(ActionPolicyGet, "Read the trigger policy", controlaction.EffectRead,
			schema(nil, sessionProp)),
		actionDef(ActionPolicySet, "CAS-update the trigger policy", controlaction.EffectWrite,
			schema(req("enabled", "min_interval_ms", "base_revision"), sessionProp, prop("enabled", boolProp), prop("min_interval_ms", numProp), prop("base_revision", numProp))),
		actionDef(ActionTrigger, "Manually trigger a cognitive run", controlaction.EffectWrite,
			schema(nil, sessionProp)),
		actionDef(ActionCancel, "Cancel the active cognitive run", controlaction.EffectWrite,
			schema(req("run_id"), sessionProp, prop("run_id", strProp))),
		actionDef(ActionResultsList, "Page effect receipts", controlaction.EffectRead,
			schema(nil, sessionProp, prop("cursor", strProp), prop("limit", numProp))),
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

// req lists contract-required fields beyond session_id, which is always
// required for the closed C2-3 inventory.
func req(names ...string) []string {
	return append([]string{"session_id"}, names...)
}

func schema(required []string, props ...string) json.RawMessage {
	joined := "{"
	for i, p := range props {
		if i > 0 {
			joined += ","
		}
		joined += p
	}
	joined += "}"
	if required == nil {
		required = []string{"session_id"}
	}
	quoted := make([]string, 0, len(required))
	for _, name := range required {
		quoted = append(quoted, `"`+name+`"`)
	}
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":[` + strings.Join(quoted, ",") + `],"properties":` + joined + `}`)
}

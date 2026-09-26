package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

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

// outputLimitReason reports a marshaled result larger than MaxOutputBytes
// without inventing a store-level reason code.
const outputLimitReason = "memory action result exceeds the output limit"

type memoryAction struct {
	definition controlaction.Definition
	invoke     func(context.Context, *Service, json.RawMessage) (json.RawMessage, error)
}

func (a memoryAction) Definition() controlaction.Definition { return a.definition }

// Invoke resolves the service at call time: an oversized input or a closed
// registry reports the explicit four-state outcome rather than a Go error,
// so action callers always read the same result contract.
func (a memoryAction) Invoke(ctx context.Context, _ controlaction.Host, input json.RawMessage) (json.RawMessage, error) {
	if len(input) > maxActionInput {
		return marshalOutcome(invalidInput())
	}
	service := Active()
	if service == nil {
		return marshalOutcome(unavailableOutcome())
	}
	return a.invoke(ctx, service, input)
}

// ActionProviders returns the closed action inventory owned by
// vivy/memory-bml.
func ActionProviders() []controlaction.Provider {
	return []controlaction.Provider{
		memoryAction{definition: actionDefinition(ActionList, "List visible long-term memory records", controlaction.EffectRead, listInputSchema), invoke: invokeList},
		memoryAction{definition: actionDefinition(ActionSearch, "Search visible long-term memory", controlaction.EffectRead, searchInputSchema), invoke: invokeSearch},
		memoryAction{definition: actionDefinition(ActionGet, "Get one visible memory record", controlaction.EffectRead, getInputSchema), invoke: invokeGet},
		memoryAction{definition: actionDefinition(ActionAdd, "Add a user-asserted long-term memory record", controlaction.EffectWrite, addInputSchema), invoke: invokeAdd},
		memoryAction{definition: actionDefinition(ActionUpdate, "Update a memory record under revision CAS", controlaction.EffectWrite, updateInputSchema), invoke: invokeUpdate},
		memoryAction{definition: actionDefinition(ActionRemove, "Tombstone a memory record under revision CAS", controlaction.EffectWrite, removeInputSchema), invoke: invokeRemove},
		memoryAction{definition: actionDefinition(ActionRulesRead, "Read the MEMRULES handbook", controlaction.EffectRead, emptyInputSchema), invoke: invokeRulesRead},
		memoryAction{definition: actionDefinition(ActionRulesWrite, "Replace the MEMRULES handbook under content-digest CAS", controlaction.EffectWrite, rulesWriteInputSchema), invoke: invokeRulesWrite},
		memoryAction{definition: actionDefinition(ActionStatus, "Report memory service status", controlaction.EffectRead, emptyInputSchema), invoke: invokeStatus},
	}
}

func actionDefinition(id, description string, effect controlaction.Effect, input json.RawMessage) controlaction.Definition {
	return controlaction.Definition{
		ID: id, Owner: ID, ModuleID: ID, Description: description, Effect: effect,
		InputSchema: input, ResultSchema: outcomeSchema,
		MaxInputBytes: maxActionInput, MaxOutputBytes: maxActionOutput,
	}
}

// decodeInput enforces the closed input shape; violations surface as the
// explicit memory_invalid_request outcome rather than a Go error so every
// logical failure stays inside the four-state contract.
func decodeInput[T any](raw json.RawMessage, dst *T) *bml.MemoryCrudOutcome {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		trimmed = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		outcome := invalidInput()
		return &outcome
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		outcome := invalidInput()
		return &outcome
	}
	return nil
}

// invalidInput is the failed outcome for shape violations the decoder cannot
// express on its own (absent or empty required fields).
func invalidInput() bml.MemoryCrudOutcome {
	return failedOutcome(bml.HomeCodeInvalidRequest)
}

// validCAS reports whether a required base_revision field decoded to a
// usable revision.
func validCAS(revision *int64) bool {
	return revision != nil && *revision >= 0
}

func marshalOutcome(outcome bml.MemoryCrudOutcome) (json.RawMessage, error) {
	return marshalResult(outcome)
}

// marshalResult serializes a result and enforces the output cap; an
// oversized result collapses to an explicit failed outcome, never a
// truncated payload.
func marshalResult(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxActionOutput {
		capped, err := json.Marshal(failedOutcome(outputLimitReason))
		if err != nil {
			return nil, err
		}
		return capped, nil
	}
	return raw, nil
}

type listInput struct {
	Limit *uint32 `json:"limit"`
}

type searchInput struct {
	Query string  `json:"query"`
	Limit *uint32 `json:"limit"`
}

type getInput struct {
	ID string `json:"id"`
}

type addInput struct {
	Kind     string            `json:"kind"`
	Content  string            `json:"content"`
	Evidence []bml.EvidenceRef `json:"evidence"`
}

type updateInput struct {
	ID           string `json:"id"`
	Content      string `json:"content"`
	BaseRevision *int64 `json:"base_revision"`
}

type removeInput struct {
	ID           string `json:"id"`
	Reason       string `json:"reason"`
	BaseRevision *int64 `json:"base_revision"`
}

type rulesWriteInput struct {
	Content      string  `json:"content"`
	BaseRevision *string `json:"base_revision"`
}

func invokeList(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in listInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	return marshalOutcome(service.List(ctx, bml.MemoryListRequest{Limit: in.Limit}))
}

func invokeSearch(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in searchInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Query == "" {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Search(ctx, bml.MemorySearchRequest{Query: in.Query, Limit: in.Limit}))
}

func invokeGet(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in getInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.ID == "" {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Get(ctx, bml.MemoryGetRequest{RecordID: in.ID}))
}

func invokeAdd(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in addInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Kind == "" || in.Content == "" {
		return marshalOutcome(invalidInput())
	}
	if bml.Kind(in.Kind) != bml.KindLongTerm {
		return marshalOutcome(failedOutcome(bml.HomeCodeKindForbidden))
	}
	return marshalOutcome(service.Add(ctx, bml.MemoryAddRequest{Content: in.Content, EvidenceRefs: in.Evidence}))
}

func invokeUpdate(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in updateInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.ID == "" || !validCAS(in.BaseRevision) {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Update(ctx, bml.MemoryUpdateRequest{
		RecordID: in.ID, Content: in.Content, BaseRevision: *in.BaseRevision,
	}))
}

func invokeRemove(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in removeInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.ID == "" || in.Reason == "" || !validCAS(in.BaseRevision) {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Remove(ctx, bml.MemoryRemoveRequest{
		RecordID: in.ID, Reason: in.Reason, BaseRevision: *in.BaseRevision,
	}))
}

// rulesReadResult is the rules.read wire payload: the handbook plus the
// content-digest revision token rules.write requires as its base_revision.
type rulesReadResult struct {
	Status   bml.CrudOutcomeStatus `json:"status"`
	Content  string                `json:"content"`
	Source   string                `json:"source"`
	Revision string                `json:"revision"`
}

func invokeRulesRead(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in struct{}
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	view, outcome := service.Rules(ctx)
	if outcome.Status != bml.CrudOutcomeListed {
		return marshalOutcome(outcome)
	}
	return marshalResult(rulesReadResult{
		Status:   bml.CrudOutcomeListed,
		Content:  view.Content,
		Source:   view.Source,
		Revision: view.Revision,
	})
}

func invokeRulesWrite(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in rulesWriteInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Content == "" || in.BaseRevision == nil || *in.BaseRevision == "" {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.WriteRules(ctx, in.Content, *in.BaseRevision))
}

// statusResult is the status wire payload when the read succeeds; failures
// report the plain failed outcome like every other action.
type statusResult struct {
	Status          bml.CrudOutcomeStatus `json:"status"`
	Available       bool                  `json:"available"`
	StartupRevision uint64                `json:"startup_revision"`
	DatabasePresent bool                  `json:"database_present"`
	RulesRevision   string                `json:"rules_revision,omitempty"`
}

func invokeStatus(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in struct{}
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	status := service.Status(ctx)
	if !status.Available {
		reason := status.Reason
		if reason == "" {
			reason = bml.HomeCodeBmlUnavailable
		}
		return marshalOutcome(failedOutcome(reason))
	}
	return marshalResult(statusResult{
		Status:          bml.CrudOutcomeListed,
		Available:       true,
		StartupRevision: status.StartupRevision,
		DatabasePresent: status.DatabasePresent,
		RulesRevision:   status.RulesRevision,
	})
}

var (
	emptyInputSchema      = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	listInputSchema       = json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","minimum":0,"maximum":4294967295}},"additionalProperties":false}`)
	searchInputSchema     = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","minLength":1,"maxLength":4096},"limit":{"type":"integer","minimum":0,"maximum":4294967295}},"required":["query"],"additionalProperties":false}`)
	getInputSchema        = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128}},"required":["id"],"additionalProperties":false}`)
	addInputSchema        = json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","enum":["long_term"]},"content":{"type":"string","minLength":1},"evidence":{"type":"array","items":{"type":"object"},"maxItems":64}},"required":["kind","content"],"additionalProperties":false}`)
	updateInputSchema     = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"content":{"type":"string","minLength":1},"base_revision":{"type":"integer","minimum":0,"maximum":9007199254740991}},"required":["id","content","base_revision"],"additionalProperties":false}`)
	removeInputSchema     = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"reason":{"type":"string","minLength":1,"maxLength":1024},"base_revision":{"type":"integer","minimum":0,"maximum":9007199254740991}},"required":["id","reason","base_revision"],"additionalProperties":false}`)
	rulesWriteInputSchema = json.RawMessage(`{"type":"object","properties":{"content":{"type":"string","minLength":1},"base_revision":{"type":"string","minLength":1,"maxLength":128}},"required":["content","base_revision"],"additionalProperties":false}`)
	outcomeSchema         = json.RawMessage(`{"type":"object","required":["status"]}`)
)

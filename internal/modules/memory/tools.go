package memory

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	toolport "agent-vivy/sdk/port/tool"
	"github.com/ProjectViVy/agent-vivy/bml"
)

// Tool IDs for the vivy/memory-bml-tools std/tool@v1 plane; snake_case to
// match the upstream Diva tool family.
const (
	ToolAdd    = "memory_add"
	ToolGet    = "memory_get"
	ToolList   = "memory_list"
	ToolSearch = "memory_search"
	ToolUpdate = "memory_update"
	ToolRemove = "memory_remove"
)

// memoryTool adapts one Service method to the std/tool@v1 contract. The
// Definition is static; Invoke resolves the service at call time through
// the Active registry, exactly like the control-action plane.
type memoryTool struct {
	definition toolport.Definition
	invoke     func(context.Context, *Service, json.RawMessage) (json.RawMessage, error)
}

var _ toolport.ToolProvider = (*memoryTool)(nil)

func (t *memoryTool) Definition() toolport.Definition { return t.definition }

// Invoke enforces the input cap before decode and reports unavailable or
// rejected calls through the same four-state outcome envelope as the
// control actions, so tool callers never see a panic or a bare Go error.
func (t *memoryTool) Invoke(ctx context.Context, _ toolport.Host, args json.RawMessage) (toolport.Result, error) {
	var payload json.RawMessage
	var err error
	switch {
	case len(args) > maxActionInput:
		payload, err = marshalOutcome(invalidInput())
	default:
		service := Active()
		if service == nil {
			payload, err = marshalOutcome(unavailableOutcome())
		} else {
			payload, err = t.invoke(ctx, service, args)
		}
	}
	if err != nil {
		return toolport.Result{}, err
	}
	return toolport.Result{Text: string(payload)}, nil
}

// ToolProviders returns the closed agent-facing tool inventory owned by
// vivy/memory-bml-tools. Write-effect tools surface Readonly=false in the
// bound ToolSpec, which routes them through the runtime approval gate.
func ToolProviders() []toolport.ToolProvider {
	return []toolport.ToolProvider{
		&memoryTool{definition: toolport.Definition{
			ID:          ToolAdd,
			Description: "Directly store a user-confirmed fact or preference in long-term memory. Evidence references are advisory. Use memory_search before writing when unsure whether it already exists.",
			Effect:      toolport.EffectWrite,
			Schema:      addToolSchema,
		}, invoke: invokeToolAdd},
		&memoryTool{definition: toolport.Definition{
			ID:          ToolGet,
			Description: "Retrieve one visible long-term memory record by stable id, including its current revision.",
			Effect:      toolport.EffectRead,
			Schema:      getToolSchema,
		}, invoke: invokeToolGet},
		&memoryTool{definition: toolport.Definition{
			ID:          ToolList,
			Description: "List visible long-term memory records with current revisions. Use before memory_update or memory_remove.",
			Effect:      toolport.EffectRead,
			Schema:      listToolSchema,
		}, invoke: invokeToolList},
		&memoryTool{definition: toolport.Definition{
			ID:          ToolSearch,
			Description: "Search visible long-term memory for entries matching a query. Use when the user asks whether something was remembered, or when recall from memory is needed mid-conversation.",
			Effect:      toolport.EffectRead,
			Schema:      searchToolSchema,
		}, invoke: invokeToolSearch},
		&memoryTool{definition: toolport.Definition{
			ID:          ToolUpdate,
			Description: "Directly update one long-term memory record using its current base_revision. A stale revision is rejected without changing the store.",
			Effect:      toolport.EffectWrite,
			Schema:      updateToolSchema,
		}, invoke: invokeToolUpdate},
		&memoryTool{definition: toolport.Definition{
			ID:          ToolRemove,
			Description: "Directly soft-delete one long-term memory record using its current base_revision. The tombstone hides it immediately; a stale revision is rejected.",
			Effect:      toolport.EffectWrite,
			Schema:      removeToolSchema,
		}, invoke: invokeToolRemove},
	}
}

// toolPage is the read-side wire payload: the listed outcome's entries plus
// the decimal-offset continuation cursor when more rows remain.
type toolPage struct {
	Status     bml.CrudOutcomeStatus `json:"status"`
	Entries    []bml.MemoryEntry     `json:"entries"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// recallPageBounds parses the decimal-offset cursor and bounds the fetch
// window to the store's uint32 capacity; the second return is the fetch
// limit (offset+limit+1) the caller passes to the service.
func recallPageBounds(cursor string, limit *uint32) (offset int, fetch uint32, outcome *bml.MemoryCrudOutcome) {
	offset, err := recallOffset(cursor)
	if err != nil {
		bad := invalidInput()
		return 0, 0, &bad
	}
	page := uint32(defaultRecallLimit)
	if limit != nil {
		page = *limit
	}
	// A zero page size cannot advance: it would emit empty pages whose
	// next_cursor replays the incoming cursor forever.
	if page == 0 {
		bad := invalidInput()
		return 0, 0, &bad
	}
	if int64(offset) > math.MaxUint32-int64(page)-1 {
		bad := invalidInput()
		return 0, 0, &bad
	}
	return offset, uint32(offset + int(page) + 1), nil
}

// pageEntries slices one fetched window into the requested page and its
// continuation cursor; entries arrive in the store's deterministic order.
func pageEntries(entries []bml.MemoryEntry, offset, limit int) ([]bml.MemoryEntry, string) {
	if offset > len(entries) {
		offset = len(entries)
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	page := entries[offset:end]
	next := ""
	if len(entries) > end {
		next = strconv.Itoa(end)
	}
	return page, next
}

func pageOutcome(entries []bml.MemoryEntry, next string) toolPage {
	if entries == nil {
		entries = []bml.MemoryEntry{}
	}
	return toolPage{Status: bml.CrudOutcomeListed, Entries: entries, NextCursor: next}
}

// machineMemoryToolScope is the only scope the agent-facing tools can serve;
// scope identity is host-assigned (VIVY-MEMORY-PROFILE §6), so the payload
// may only name this closed value.
const machineMemoryToolScope = "machine-memory-home"

type listToolInput struct {
	Kind   *string `json:"kind"`
	Scope  *string `json:"scope"`
	Limit  *uint32 `json:"limit"`
	Cursor string  `json:"cursor"`
}

func invokeToolList(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in listToolInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Kind != nil && bml.Kind(*in.Kind) != bml.KindLongTerm {
		return marshalOutcome(failedOutcome(bml.HomeCodeKindForbidden))
	}
	if in.Scope != nil && *in.Scope != machineMemoryToolScope {
		return marshalOutcome(invalidInput())
	}
	offset, fetch, outcome := recallPageBounds(in.Cursor, in.Limit)
	if outcome != nil {
		return marshalOutcome(*outcome)
	}
	listed := service.List(ctx, bml.MemoryListRequest{Limit: &fetch})
	if listed.Status != bml.CrudOutcomeListed {
		return marshalOutcome(listed)
	}
	limit := int(defaultRecallLimit)
	if in.Limit != nil {
		limit = int(*in.Limit)
	}
	page, next := pageEntries(listed.Entries, offset, limit)
	return marshalResult(pageOutcome(page, next))
}

type searchToolInput struct {
	Query  string  `json:"query"`
	Limit  *uint32 `json:"limit"`
	Cursor string  `json:"cursor"`
}

func invokeToolSearch(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in searchToolInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Query == "" {
		return marshalOutcome(invalidInput())
	}
	offset, fetch, outcome := recallPageBounds(in.Cursor, in.Limit)
	if outcome != nil {
		return marshalOutcome(*outcome)
	}
	listed := service.Search(ctx, bml.MemorySearchRequest{Query: in.Query, Limit: &fetch})
	if listed.Status != bml.CrudOutcomeListed {
		return marshalOutcome(listed)
	}
	limit := int(defaultRecallLimit)
	if in.Limit != nil {
		limit = int(*in.Limit)
	}
	page, next := pageEntries(listed.Entries, offset, limit)
	return marshalResult(pageOutcome(page, next))
}

type getToolInput struct {
	ID string `json:"id"`
}

func invokeToolGet(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in getToolInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.ID == "" {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Get(ctx, bml.MemoryGetRequest{RecordID: in.ID}))
}

type addToolInput struct {
	Kind         string            `json:"kind"`
	Content      string            `json:"content"`
	Trust        *string           `json:"trust"`
	Provenance   *string           `json:"provenance"`
	EvidenceRefs []bml.EvidenceRef `json:"evidence_refs"`
}

func invokeToolAdd(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in addToolInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.Kind == "" || in.Content == "" {
		return marshalOutcome(invalidInput())
	}
	if bml.Kind(in.Kind) != bml.KindLongTerm {
		return marshalOutcome(failedOutcome(bml.HomeCodeKindForbidden))
	}
	// Trust and provenance are authority-assigned by the store; a payload
	// claiming any other value is rejected rather than silently rewritten.
	if in.Trust != nil && *in.Trust != "user_asserted" {
		return marshalOutcome(invalidInput())
	}
	if in.Provenance != nil && *in.Provenance != "user_input" {
		return marshalOutcome(invalidInput())
	}
	return marshalOutcome(service.Add(ctx, bml.MemoryAddRequest{Content: in.Content, EvidenceRefs: in.EvidenceRefs}))
}

// updateToolInput distinguishes an omitted content or evidence_refs (nil —
// preserve the record's current value) from an explicitly supplied one.
// Reason is advisory: the store records no removal-style reason on update.
type updateToolInput struct {
	ID           string             `json:"id"`
	Content      *string            `json:"content"`
	BaseRevision *int64             `json:"base_revision"`
	EvidenceRefs *[]bml.EvidenceRef `json:"evidence_refs"`
	Reason       string             `json:"reason"`
}

func invokeToolUpdate(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in updateToolInput
	if outcome := decodeInput(raw, &in); outcome != nil {
		return marshalOutcome(*outcome)
	}
	if in.ID == "" || !validCAS(in.BaseRevision) {
		return marshalOutcome(invalidInput())
	}
	content := ""
	if in.Content != nil {
		content = *in.Content
	} else {
		// An omitted content preserves the record's current body under the
		// same base_revision CAS; a missing or invisible record reports the
		// store's own failed outcome.
		current := service.Get(ctx, bml.MemoryGetRequest{RecordID: in.ID})
		if current.Status != bml.CrudOutcomeListed || len(current.Entries) == 0 {
			return marshalOutcome(current)
		}
		content = current.Entries[0].Content
	}
	var evidence []bml.EvidenceRef
	if in.EvidenceRefs != nil {
		evidence = *in.EvidenceRefs
	}
	return marshalOutcome(service.Update(ctx, bml.MemoryUpdateRequest{
		RecordID: in.ID, Content: content, BaseRevision: *in.BaseRevision,
		EvidenceRefs: evidence,
	}))
}

type removeToolInput struct {
	ID           string `json:"id"`
	BaseRevision *int64 `json:"base_revision"`
	Reason       string `json:"reason"`
}

func invokeToolRemove(ctx context.Context, service *Service, raw json.RawMessage) (json.RawMessage, error) {
	var in removeToolInput
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

var (
	addToolSchema    = json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","enum":["long_term"]},"content":{"type":"string","minLength":1,"description":"The fact or preference to remember"},"trust":{"type":"string","enum":["user_asserted"],"description":"Authority-assigned trust; only user_asserted is accepted"},"provenance":{"type":"string","enum":["user_input"],"description":"Authority-assigned provenance; only user_input is accepted"},"evidence_refs":{"type":"array","items":{"type":"object"},"maxItems":64}},"required":["kind","content"],"additionalProperties":false}`)
	getToolSchema    = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128,"description":"Stable record id"}},"required":["id"],"additionalProperties":false}`)
	listToolSchema   = json.RawMessage(`{"type":"object","properties":{"kind":{"type":"string","enum":["long_term"],"description":"Record kind; only long_term is visible"},"scope":{"type":"string","enum":["machine-memory-home"],"description":"Scope filter; only the machine memory home is served"},"limit":{"type":"integer","minimum":1,"maximum":4294967295,"description":"Maximum number of entries to return"},"cursor":{"type":"string","description":"Continuation cursor from a previous next_cursor"}},"additionalProperties":false}`)
	searchToolSchema = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","minLength":1,"maxLength":4096,"description":"Search query"},"limit":{"type":"integer","minimum":1,"maximum":4294967295},"cursor":{"type":"string","description":"Continuation cursor from a previous next_cursor"}},"required":["query"],"additionalProperties":false}`)
	updateToolSchema = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"content":{"type":"string","minLength":1,"description":"Replacement content; omitted preserves the record's current content"},"base_revision":{"type":"integer","minimum":0,"maximum":9007199254740991},"evidence_refs":{"type":"array","items":{"type":"object"},"maxItems":64},"reason":{"type":"string","maxLength":1024,"description":"Advisory note; not persisted"}},"required":["id","base_revision"],"additionalProperties":false}`)
	removeToolSchema = json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","minLength":1,"maxLength":128},"base_revision":{"type":"integer","minimum":0,"maximum":9007199254740991},"reason":{"type":"string","minLength":1,"maxLength":1024}},"required":["id","base_revision","reason"],"additionalProperties":false}`)
)

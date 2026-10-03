package divacognitive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"agent-vivy/internal/cognitivecontract"
	"agent-vivy/internal/domain"
	"agent-vivy/internal/runtime"
	"agent-vivy/internal/storage"
	controlaction "agent-vivy/sdk/port/controlaction"

	"github.com/dashimaki/garden/agentapi"
	"github.com/dashimaki/garden/memory"
	laputaevolution "github.com/dashimaki/laputa/evolution"
	"github.com/dashimaki/laputa/persona"
)

// dispatch is the bundle's armed dispatch surface. Every ledger C2-3 action
// binds a per-call HumanClient on the caller's session — session admission
// already happened in the ActionHost session guard; the capability carries
// the owner principal, never a payload claim. All results ride the
// CognitiveOutcome envelope; only malformed input and unavailable plumbing
// leave as transport errors.
type dispatch struct{ bundle *bundle }

// outcomeError and outcome are the ledger CognitiveOutcome wire shape.
type outcomeError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type outcome struct {
	Status string          `json:"status"`
	Value  json.RawMessage `json:"value,omitempty"`
	Error  *outcomeError   `json:"error,omitempty"`
}

// badInput marks strict-decode or input-shape failures; they stay transport
// errors (InvalidParams) instead of fabricating a business outcome.
type badInput struct{ err error }

func (e *badInput) Error() string { return e.err.Error() }

// sessionHead extracts the session claim; the guard has already proven it
// equals the transport-bound session, so here it only selects the bound
// capability's session binding.
type sessionHead struct {
	SessionID string `json:"session_id"`
}

func (d dispatch) Invoke(ctx context.Context, actionID string, input json.RawMessage) (json.RawMessage, error) {
	if d.bundle == nil || d.bundle.owner == nil {
		return nil, cognitivecontract.ErrUnarmed
	}
	// The head decode is a partial view: unknown fields are decided by the
	// action's own strict schema, not here.
	var head sessionHead
	if err := json.Unmarshal(input, &head); err != nil {
		return nil, fmt.Errorf("%w: %v", controlaction.ErrInvalidInput, err)
	}
	if head.SessionID == "" {
		return nil, fmt.Errorf("%w: session_id is required", controlaction.ErrInvalidInput)
	}
	human, err := d.bundle.owner.BindHumanSession(head.SessionID, "")
	if err != nil {
		return nil, fmt.Errorf("diva-cognitive: bind human session: %w", err)
	}
	handler, ok := cognitiveHandlers[actionID]
	if !ok {
		return nil, fmt.Errorf("%w: unknown action %q", controlaction.ErrInvalidInput, actionID)
	}
	value, err := handler(d, ctx, human, input)
	if err != nil {
		var bad *badInput
		if errors.As(err, &bad) {
			return nil, fmt.Errorf("%w: %v", controlaction.ErrInvalidInput, bad.err)
		}
	}
	return marshalOutcome(value, err)
}

type cognitiveHandler func(dispatch, context.Context, *agentapi.HumanClient, json.RawMessage) (any, error)

var cognitiveHandlers = map[string]cognitiveHandler{
	ActionStatus:              dispatch.status,
	ActionPersonaInitialize:   personaInitialize,
	ActionPersonaRead:         personaRead,
	ActionPersonaSave:         personaSave,
	ActionPersonaReviewList:   personaReviewList,
	ActionPersonaReviewDecide: personaReviewDecide,
	ActionFrozenRead:          frozenRead,
	ActionActmemRead:          actmemRead,
	ActionActmemWorkPatch:     actmemWorkPatch,
	ActionActmemOwnerRead:     actmemOwnerRead,
	ActionActmemOwnerSave:     actmemOwnerSave,
	ActionMemorySearch:        memorySearch,
	ActionMemoryExpand:        memoryExpand,
	ActionMemoryMutate:        memoryMutate,
	ActionMemoryReceipt:       memoryReceipt,
	ActionPolicyGet:           dispatch.policyGet,
	ActionPolicySet:           dispatch.policySet,
	ActionTrigger:             dispatch.trigger,
	ActionCancel:              dispatch.cancel,
	ActionResultsList:         resultsList,
}

func decodeTyped[T any](input json.RawMessage, out *T) (*badInput, bool) {
	if err := decodeInput(input, out); err != nil {
		return &badInput{err: err}, true
	}
	return nil, false
}

// marshalOutcome emits the CognitiveOutcome envelope: value-bearing results
// report ok; typed domain failures preserve their stable code and class;
// untyped failures report a generic code so internals never leak.
func marshalOutcome(value any, err error) (json.RawMessage, error) {
	out := outcome{Status: "ok"}
	if value != nil {
		raw, mErr := json.Marshal(value)
		if mErr != nil {
			return nil, fmt.Errorf("diva-cognitive: marshal outcome: %w", mErr)
		}
		out.Value = raw
	}
	if err != nil {
		code, status, retryable := classify(err)
		out.Status = status
		out.Error = &outcomeError{Code: code, Message: publicMessage(err, code), Retryable: retryable}
	}
	return json.Marshal(out)
}

// classify maps typed domain errors onto the closed outcome statuses.
// unavailable classes are retryable; outcome_unknown stays unknown.
func classify(err error) (code, status string, retryable bool) {
	var apiErr *agentapi.Error
	if errors.As(err, &apiErr) && apiErr != nil {
		switch apiErr.Code {
		case "unavailable", "index_health_unavailable", "capability_unavailable", "backend_unavailable", "recovery_required":
			return apiErr.Code, "unavailable", true
		}
		return apiErr.Code, "failed", false
	}
	if domainCode := laputaevolution.CodeOf(err); domainCode != "" {
		switch domainCode {
		case laputaevolution.ErrBackendUnavailable, laputaevolution.ErrCapabilityUnavailable, laputaevolution.ErrRecoveryRequired:
			return string(domainCode), "unavailable", true
		case laputaevolution.ErrOutcomeUnknown:
			return string(domainCode), "unknown", true
		}
		return string(domainCode), "failed", false
	}
	switch {
	case errors.Is(err, cognitivecontract.ErrUnarmed):
		return "capability_unavailable", "unavailable", true
	case errors.Is(err, context.DeadlineExceeded):
		return "outcome_unknown", "unknown", true
	case errors.Is(err, runtime.ErrPolicyConflict):
		return "revision_conflict", "failed", false
	case errors.Is(err, storage.ErrVersionConflict), errors.Is(err, storage.ErrConflict):
		return "revision_conflict", "failed", false
	}
	return "internal", "failed", false
}

// publicMessage preserves curated agentapi/contract messages verbatim and
// refuses to leak untyped internals.
func publicMessage(err error, code string) string {
	var apiErr *agentapi.Error
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.Message != "" {
		return apiErr.Message
	}
	if laputaevolution.CodeOf(err) != "" {
		return err.Error()
	}
	return "operation " + code
}

// --- capability status -----------------------------------------------------

// capabilityStatus mirrors the ledger CapabilityStatus shape exactly; the
// sub-blocks are projections of owning DTOs, not a second dialect.
type capabilityStatus struct {
	ProfileID     string                `json:"profile_id"`
	Scope         laputaevolution.Scope `json:"scope"`
	DestinationID string                `json:"destination_id"`
	Persona       personaBlock          `json:"persona"`
	Frozen        frozenBlock           `json:"frozen"`
	Memory        memoryBlock           `json:"memory"`
	Cognition     cognitionBlock        `json:"cognition"`
}

type personaBlock struct {
	State            string            `json:"state"`
	CurrentRevisions map[string]uint64 `json:"current_revisions"`
}

type frozenBlock struct {
	State     string            `json:"state"`
	Revisions map[string]uint64 `json:"revisions,omitempty"`
}

type memoryBlock struct {
	BackendID      string `json:"backend_id"`
	Health         string `json:"health"`
	ReasonCode     string `json:"reason_code"`
	CanonicalState string `json:"canonical_state"`
	IndexState     string `json:"index_state"`
}

type cognitionBlock struct {
	Enabled        bool                         `json:"enabled"`
	PolicyRevision uint64                       `json:"policy_revision"`
	MinIntervalMS  int64                        `json:"min_interval_ms"`
	Eligibility    *laputaevolution.Eligibility `json:"eligibility,omitempty"`
	ActiveRunID    string                       `json:"active_run_id"`
	SourceID       string                       `json:"source_id"`
	Watermark      uint64                       `json:"watermark"`
	PendingThrough uint64                       `json:"pending_through"`
	Phase          string                       `json:"phase"`
	BlockReason    string                       `json:"block_reason"`
}

func (d dispatch) status(ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	state, err := control.GetState(ctx)
	if err != nil {
		return nil, err
	}
	status := capabilityStatus{
		ProfileID:     profileID,
		Scope:         d.bundle.scope,
		DestinationID: destinationID,
		Cognition: cognitionBlock{
			Enabled:        state.Enabled,
			PolicyRevision: state.PolicyRevision,
			MinIntervalMS:  state.MinIntervalMS,
			Eligibility:    state.Eligibility,
			ActiveRunID:    state.ActiveRunID,
			SourceID:       state.SourceID,
			Watermark:      state.Watermark,
			PendingThrough: state.PendingThrough,
			Phase:          state.Phase,
			BlockReason:    state.BlockReason,
		},
	}
	view, err := human.PersonaStatus(ctx)
	if err != nil {
		return nil, err
	}
	status.Persona.State = view.Status.String()
	status.Persona.CurrentRevisions = make(map[string]uint64, len(view.Files))
	for kind, file := range view.Files {
		status.Persona.CurrentRevisions[kind] = file.Revision
	}
	status.Frozen = d.frozenStatus(ctx, human)
	status.Memory = d.memoryStatus(ctx, human)
	return status, nil
}

// frozenStatus folds the session snapshot into the closed state vocabulary:
// a missing capture is not_captured, never an error.
func (d *dispatch) frozenStatus(ctx context.Context, human *agentapi.HumanClient) frozenBlock {
	core, err := human.ReadFrozen(ctx)
	if err != nil {
		var apiErr *agentapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == "not_found" {
			return frozenBlock{State: "not_captured"}
		}
		if laputaevolution.CodeOf(err) == laputaevolution.ErrRecoveryRequired {
			return frozenBlock{State: "recovery_required"}
		}
		return frozenBlock{State: "recovery_required"}
	}
	block := frozenBlock{State: "ready", Revisions: make(map[string]uint64, len(core.Sections))}
	for _, section := range core.Sections {
		block.Revisions[string(section.Kind)] = section.SourceRevision
	}
	return block
}

// memoryStatus projects the live Mentle health probe; a failed probe still
// reports health:"unavailable" with the probe's own reason code.
func (d *dispatch) memoryStatus(ctx context.Context, human *agentapi.HumanClient) memoryBlock {
	health, err := human.IndexHealth(ctx)
	block := memoryBlock{BackendID: destinationID}
	if len(health.Reasons) > 0 {
		block.ReasonCode = health.Reasons[0]
	}
	if err != nil && health.Status == "" {
		block.Health = "unavailable"
		block.CanonicalState = "unavailable"
		block.IndexState = "unavailable"
		if block.ReasonCode == "" {
			block.ReasonCode = "index_health_unavailable"
		}
		return block
	}
	block.Health = health.Status
	if block.Health == "ok" {
		block.Health = "available"
	}
	block.CanonicalState = health.Status
	block.IndexState = health.Status
	if health.FailedJobs > 0 {
		block.IndexState = "failed"
	} else if health.PendingJobs > 0 {
		block.IndexState = "pending"
	}
	return block
}

// --- persona ---------------------------------------------------------------

func personaInitialize(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID     string                 `json:"session_id"`
		Initialization persona.Initialization `json:"initialization"`
		Reason        string                 `json:"reason"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.InitializePersona(ctx, in.Initialization, in.Reason)
}

func personaRead(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
		Kind      string `json:"kind"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	kind, err := persona.ParseKind(in.Kind)
	if err != nil {
		return nil, &badInput{err: fmt.Errorf("unknown persona kind %q", in.Kind)}
	}
	return human.ReadPersona(ctx, kind.String())
}

func personaSave(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID    string `json:"session_id"`
		Kind         string `json:"kind"`
		Content      string `json:"content"`
		BaseRevision uint64 `json:"base_revision"`
		Reason       string `json:"reason"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	kind, err := persona.ParseKind(in.Kind)
	if err != nil {
		return nil, &badInput{err: fmt.Errorf("unknown persona kind %q", in.Kind)}
	}
	return human.SavePersona(ctx, kind, in.Content, in.BaseRevision, in.Reason)
}

func personaReviewList(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
		Kind      string `json:"kind,omitempty"`
		State     string `json:"state,omitempty"`
		Limit     int    `json:"limit,omitempty"`
		Cursor    string `json:"cursor,omitempty"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	query := agentapi.ReviewQuery{PageQuery: agentapi.PageQuery{Cursor: in.Cursor, Limit: in.Limit}}
	if in.Kind != "" {
		kind, err := persona.ParseKind(in.Kind)
		if err != nil {
			return nil, &badInput{err: fmt.Errorf("unknown persona kind %q", in.Kind)}
		}
		query.Kind = &kind
	}
	if in.State != "" {
		state := persona.RequestState(in.State)
		query.State = &state
	}
	return human.ListPersonaReviews(ctx, query)
}

func personaReviewDecide(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
		ReviewID  string `json:"review_id"`
		Decision  string `json:"decision"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	decision := agentapi.ReviewDecision(in.Decision)
	if decision != agentapi.ReviewAccept && decision != agentapi.ReviewReject {
		return nil, &badInput{err: fmt.Errorf("unknown review decision %q", in.Decision)}
	}
	return human.DecidePersonaReview(ctx, in.ReviewID, decision)
}

// --- frozen / actmem -------------------------------------------------------

func frozenRead(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in sessionHead
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	core, err := human.ReadFrozen(ctx)
	if err != nil {
		var apiErr *agentapi.Error
		if errors.As(err, &apiErr) && apiErr.Code == "not_found" {
			return frozenBlock{State: "not_captured"}, nil
		}
		return nil, err
	}
	return core, nil
}

func actmemRead(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string                       `json:"session_id"`
		Sections  []laputaevolution.EntrySection `json:"sections"`
		MaxChars  uint32                       `json:"max_chars"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.ReadActivity(ctx, agentapi.ReadRequest{Sections: in.Sections, MaxChars: in.MaxChars})
}

func actmemWorkPatch(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string                  `json:"session_id"`
		Patch     laputaevolution.WorkPatch `json:"patch"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.ApplyWorkPatch(ctx, in.Patch)
}

func actmemOwnerRead(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in sessionHead
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.ReadOwnerACTMEM(ctx)
}

func actmemOwnerSave(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID    string `json:"session_id"`
		Markdown     string `json:"markdown"`
		BaseRevision uint64 `json:"base_revision"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.SaveACTMEM(ctx, in.Markdown, in.BaseRevision)
}

// --- memory ----------------------------------------------------------------

func memorySearch(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID   string `json:"session_id"`
		Query       string `json:"query"`
		Collection  string `json:"collection,omitempty"`
		Cursor      string `json:"cursor,omitempty"`
		Limit       int    `json:"limit"`
		BudgetChars int    `json:"budget_chars"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.SearchMemory(ctx, memory.AuthorizedSearch{
		Query: in.Query, Collection: in.Collection, Cursor: in.Cursor,
		Limit: in.Limit, BudgetChars: in.BudgetChars,
	})
}

func memoryExpand(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID        string `json:"session_id"`
		CardID           string `json:"card_id"`
		ExpectedRevision uint64 `json:"expected_revision"`
		BudgetChars      int    `json:"budget_chars"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.ExpandMemory(ctx, memory.AuthorizedExpansion{
		CardID: in.CardID, ExpectedRevision: in.ExpectedRevision, BudgetChars: in.BudgetChars,
	})
}

func memoryMutate(d dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string                    `json:"session_id"`
		Mutation  memory.AuthorizedMutation `json:"mutation"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	// Scope and destination are host-stamped; the input may not carry them.
	in.Mutation.Scope = d.bundle.scope
	in.Mutation.DestinationID = destinationID
	return human.MutateMemory(ctx, in.Mutation)
}

func memoryReceipt(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID   string `json:"session_id"`
		OperationID string `json:"operation_id"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.MemoryReceipt(ctx, in.OperationID)
}

// --- policy / control ------------------------------------------------------

func (d dispatch) policyGet(ctx context.Context, _ *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in sessionHead
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	state, err := control.GetState(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"enabled":         state.Enabled,
		"min_interval_ms": state.MinIntervalMS,
		"policy_revision": state.PolicyRevision,
	}, nil
}

func (d dispatch) policySet(ctx context.Context, _ *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID     string `json:"session_id"`
		Enabled       bool   `json:"enabled"`
		MinIntervalMS int64  `json:"min_interval_ms"`
		BaseRevision  uint64 `json:"base_revision"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	policy := laputaevolution.TriggerPolicy{Enabled: in.Enabled, MinIntervalMS: in.MinIntervalMS}
	state, err := control.SetPolicyCAS(ctx, policy, in.BaseRevision)
	if err != nil {
		return nil, err
	}
	d.bundle.setPolicy(policy)
	return state, nil
}

func (d dispatch) trigger(ctx context.Context, _ *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in sessionHead
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	return control.Trigger(ctx)
}

func (d dispatch) cancel(ctx context.Context, _ *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
		RunID     string `json:"run_id"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	control, err := d.bundle.armed()
	if err != nil {
		return nil, err
	}
	return control.Cancel(ctx, domain.RunID(in.RunID))
}

func resultsList(_ dispatch, ctx context.Context, human *agentapi.HumanClient, input json.RawMessage) (any, error) {
	var in struct {
		SessionID string `json:"session_id"`
		Cursor    string `json:"cursor,omitempty"`
		Limit     int    `json:"limit,omitempty"`
	}
	if bad, failed := decodeTyped(input, &in); failed {
		return nil, bad
	}
	return human.Results(ctx, agentapi.PageQuery{Cursor: in.Cursor, Limit: in.Limit})
}

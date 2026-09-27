// Provider boundary for long-memory integration — the Go port of Diva's
// `agent-diva-core/src/memory/provider.rs` MemoryProvider trait surface.
//
// Contract only: no bml implementation is wired in this task. Sync Go
// signatures take a leading context.Context where the upstream trait was
// async. Rust `PathBuf` is ported as string; Rust enums that carry payloads
// are flattened into structs whose Kind field selects the variant.

package bml

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// StartupStatusKind is the deterministic status for startup wakeup injection.
type StartupStatusKind string

const (
	// StartupStatusReady marks fresh startup content for the current wakeup.
	StartupStatusReady StartupStatusKind = "ready"
	// StartupStatusDegraded marks startup continuity that could not be
	// assembled; LastUsableWakeup stays nil when no cache is reused.
	StartupStatusDegraded StartupStatusKind = "degraded"
)

// StartupStatus is the Rust `StartupStatus` enum flattened for Go.
type StartupStatus struct {
	Kind             StartupStatusKind  `json:"kind"`
	Reason           string             `json:"reason,omitempty"`
	LastUsableWakeup *SystemPromptBlock `json:"last_usable_wakeup,omitempty"`
}

// StartupInjectionShape is the explicit shape chosen for startup injection
// into prompt assembly. Upstream currently consumes compact rendered markdown
// only.
type StartupInjectionShape string

const (
	// StartupInjectionShapeCompactRenderedMarkdown is compact rendered
	// markdown ready for direct prompt inclusion.
	StartupInjectionShapeCompactRenderedMarkdown StartupInjectionShape = "compact_rendered_markdown"
)

// PrefetchStatusKind is the deterministic status for intent-aware prefetch.
type PrefetchStatusKind string

const (
	// PrefetchStatusSkippedNoIntent: no actionable intent; recall was
	// intentionally skipped.
	PrefetchStatusSkippedNoIntent PrefetchStatusKind = "skipped_no_intent"
	// PrefetchStatusReady: recall completed with a prompt block or an
	// empty-but-successful result.
	PrefetchStatusReady PrefetchStatusKind = "ready"
	// PrefetchStatusFailed: recall was attempted but failed.
	PrefetchStatusFailed PrefetchStatusKind = "failed"
)

// PrefetchStatus is the Rust `PrefetchStatus` enum flattened for Go.
type PrefetchStatus struct {
	Kind   PrefetchStatusKind `json:"kind"`
	Reason string             `json:"reason,omitempty"`
}

// SyncTurnStatusKind is the deterministic status for post-turn sync.
type SyncTurnStatusKind string

const (
	// SyncTurnStatusPersisted: an authority write was applied.
	SyncTurnStatusPersisted SyncTurnStatusKind = "persisted"
	// SyncTurnStatusProposalCreated: a governed proposal was durably created
	// for review; not authority until approved and applied.
	SyncTurnStatusProposalCreated SyncTurnStatusKind = "proposal_created"
	// SyncTurnStatusNoop: no durable write was needed for this turn.
	SyncTurnStatusNoop SyncTurnStatusKind = "noop"
	// SyncTurnStatusFailed: a write was attempted but did not complete.
	SyncTurnStatusFailed SyncTurnStatusKind = "failed"
)

// SyncTurnStatus is the Rust `SyncTurnStatus` enum flattened for Go.
type SyncTurnStatus struct {
	Kind   SyncTurnStatusKind `json:"kind"`
	Reason string             `json:"reason,omitempty"`
}

// SessionEndStatusKind is the deterministic status for session-end handling.
type SessionEndStatusKind string

const (
	// SessionEndStatusTriggered: shutdown hook ran and triggered work.
	SessionEndStatusTriggered SessionEndStatusKind = "triggered"
	// SessionEndStatusNoop: shutdown hook intentionally performed no work.
	SessionEndStatusNoop SessionEndStatusKind = "noop"
	// SessionEndStatusAlreadyHandled: the session-end call was already
	// handled and is idempotently ignored.
	SessionEndStatusAlreadyHandled SessionEndStatusKind = "already_handled"
	// SessionEndStatusFailed: shutdown work failed.
	SessionEndStatusFailed SessionEndStatusKind = "failed"
)

// SessionEndStatus is the Rust `SessionEndStatus` enum flattened for Go.
type SessionEndStatus struct {
	Kind   SessionEndStatusKind `json:"kind"`
	Reason string               `json:"reason,omitempty"`
}

// RecallTurnOutcome is the terminal outcome of a live turn that may have
// consumed recalled records.
type RecallTurnOutcome string

const (
	RecallTurnOutcomeSucceeded RecallTurnOutcome = "succeeded"
	RecallTurnOutcomeFailed    RecallTurnOutcome = "failed"
)

// SystemPromptRequest is the input for startup wakeup-style prompt
// generation. Implementations return only the markdown block to splice into
// the system prompt, not transport envelopes or backend rows.
type SystemPromptRequest struct {
	// WorkspaceRoot is the workspace root for the active agent session.
	WorkspaceRoot string `json:"workspace_root"`
}

// SystemPromptBlock is the startup block injected into the system prompt.
type SystemPromptBlock struct {
	// Shape is the explicitly chosen injection shape consumed by prompt
	// assembly.
	Shape StartupInjectionShape `json:"shape"`
	// Markdown is suitable for direct prompt inclusion.
	Markdown string `json:"markdown"`
}

// SystemPromptResponse is the startup wakeup result consumed by prompt
// assembly.
type SystemPromptResponse struct {
	Status      StartupStatus      `json:"status"`
	PromptBlock *SystemPromptBlock `json:"prompt_block"`
}

// ReadySystemPromptResponse mirrors `SystemPromptResponse::ready`.
func ReadySystemPromptResponse(block SystemPromptBlock) SystemPromptResponse {
	return SystemPromptResponse{
		Status:      StartupStatus{Kind: StartupStatusReady},
		PromptBlock: &block,
	}
}

// DegradedSystemPromptResponse mirrors `SystemPromptResponse::degraded`: the
// status carries the untrimmed reason, the rendered block trims it, and no
// cached wakeup is reused.
func DegradedSystemPromptResponse(reason string) SystemPromptResponse {
	return SystemPromptResponse{
		Status: StartupStatus{Kind: StartupStatusDegraded, Reason: reason},
		PromptBlock: &SystemPromptBlock{
			Shape:    StartupInjectionShapeCompactRenderedMarkdown,
			Markdown: renderDegradedStartupMarkdown(reason),
		},
	}
}

// SystemPromptRefreshRequest is the input for refreshing a cached startup
// projection after an authority write. The authority revision is durable and
// workspace-scoped; providers may use it to coalesce duplicate or
// out-of-order refresh notifications.
type SystemPromptRefreshRequest struct {
	WorkspaceRoot     string `json:"workspace_root"`
	AuthorityRevision uint64 `json:"authority_revision"`
}

// SystemPromptRefreshResponse is the result of refreshing a cached startup
// projection.
type SystemPromptRefreshResponse struct {
	// AuthorityRevision is the latest authority revision observed.
	AuthorityRevision uint64 `json:"authority_revision"`
	// ProjectionChanged reports whether the rendered startup projection
	// changed and the in-process prompt revision advanced.
	ProjectionChanged bool `json:"projection_changed"`
}

// WakeupPackSummary is the minimal provider-facing summary of a wakeup-style
// state bundle: the durable, domain-oriented content needed from a
// Laputa-style wakeup without depending on Laputa crate types.
type WakeupPackSummary struct {
	Identity          string   `json:"identity"`
	RecentState       string   `json:"recent_state"`
	LatestCapsule     *string  `json:"latest_capsule"`
	KeyRelations      []string `json:"key_relations"`
	UnresolvedThreads []string `json:"unresolved_threads"`
	GeneratedAt       *string  `json:"generated_at"`
}

// StartupContextSnapshot is structured startup support data that renders
// into the provider-consumable prompt block shape.
type StartupContextSnapshot struct {
	// LaputaStateRoot is the optional `.laputa` state root for the session.
	LaputaStateRoot *string `json:"laputa_state_root"`
	// SoulMarkdown is an optional rendered SOUL projection.
	SoulMarkdown *string `json:"soul_markdown"`
	// WakeupMarkdown is an optional rendered WAKEUP projection.
	WakeupMarkdown *string `json:"wakeup_markdown"`
	// WakeupPack is the optional structured wakeup summary used when markdown
	// projections are not available.
	WakeupPack *WakeupPackSummary `json:"wakeup_pack"`
	// MemoryMarkdown is an optional fallback block from existing core outputs.
	MemoryMarkdown *string `json:"memory_markdown"`
}

// IntoSystemPromptBlock renders the snapshot into the prompt seam shape,
// returning nil when nothing renderable is present.
func (s StartupContextSnapshot) IntoSystemPromptBlock() *SystemPromptBlock {
	markdown := s.renderCompactMarkdown()
	if markdown == "" {
		return nil
	}
	return &SystemPromptBlock{
		Shape:    StartupInjectionShapeCompactRenderedMarkdown,
		Markdown: markdown,
	}
}

// renderCompactMarkdown mirrors `StartupContextSnapshot::render_compact_markdown`.
// Rhythm and report content is intentionally never injected: reports are not
// memories and stay opt-in reads.
func (s StartupContextSnapshot) renderCompactMarkdown() string {
	var sections []string
	if memory := trimmedMarkdown(s.MemoryMarkdown); memory != "" {
		sections = append(sections, memory)
	}
	if soul := trimmedMarkdown(s.SoulMarkdown); soul != "" {
		sections = append(sections, "## Soul Projection\n"+soul)
	}
	if wakeup := trimmedMarkdown(s.WakeupMarkdown); wakeup != "" {
		sections = append(sections, "## Wakeup Projection\n"+wakeup)
	} else if s.WakeupPack != nil {
		sections = append(sections, renderWakeupPackSummary(s.WakeupPack))
	}
	return strings.Join(sections, "\n\n")
}

// renderDegradedStartupMarkdown mirrors `render_degraded_startup_markdown`.
func renderDegradedStartupMarkdown(reason string) string {
	return fmt.Sprintf(
		"## Memory Startup Status\n- status: degraded\n- reason: %s\n- last_usable_wakeup: omitted (no cache reuse)\n",
		strings.TrimSpace(reason),
	)
}

// trimmedMarkdown mirrors `trimmed_markdown`: trim, empty-check.
func trimmedMarkdown(markdown *string) string {
	if markdown == nil {
		return ""
	}
	return strings.TrimSpace(*markdown)
}

// renderWakeupPackSummary mirrors `render_wakeup_pack_summary`.
func renderWakeupPackSummary(pack *WakeupPackSummary) string {
	latestCapsule := "None"
	if pack.LatestCapsule != nil {
		latestCapsule = *pack.LatestCapsule
	}
	renderList := func(items []string) string {
		if len(items) == 0 {
			return "- None"
		}
		lines := make([]string, len(items))
		for i, item := range items {
			lines[i] = "- " + strings.TrimSpace(item)
		}
		return strings.Join(lines, "\n")
	}

	var b strings.Builder
	b.WriteString("## Wakeup Summary")
	if generatedAt := trimmedMarkdown(pack.GeneratedAt); generatedAt != "" {
		b.WriteString("\nGenerated: " + generatedAt)
	}
	b.WriteString("\n\n### Identity\n" + strings.TrimSpace(pack.Identity))
	b.WriteString("\n\n### Recent State\n" + strings.TrimSpace(pack.RecentState))
	b.WriteString("\n\n### Latest Capsule\n" + strings.TrimSpace(latestCapsule))
	b.WriteString("\n\n### Key Relations\n" + renderList(pack.KeyRelations))
	b.WriteString("\n\n### Unresolved Threads\n" + renderList(pack.UnresolvedThreads))
	return b.String()
}

// PrefetchRequest is the input for optional intent-aware recall during a live
// turn — the `laputa_recall_intent(intent, current_room)` shape.
type PrefetchRequest struct {
	WorkspaceRoot string  `json:"workspace_root"`
	Intent        string  `json:"intent"`
	CurrentRoom   *string `json:"current_room"`
	UserMessage   *string `json:"user_message"`
}

// PrefetchResponse is optional memory material for mid-turn recall.
type PrefetchResponse struct {
	Status      PrefetchStatus `json:"status"`
	PromptBlock *string        `json:"prompt_block"`
}

// SyncTurnRequest is the input for post-successful-turn synchronization: it
// carries distilled memory updates and a persisted history/evidence entry
// without leaking backend-specific write commands.
type SyncTurnRequest struct {
	WorkspaceRoot        string  `json:"workspace_root"`
	MemoryUpdateMarkdown *string `json:"memory_update_markdown"`
	HistoryEntry         *string `json:"history_entry"`
}

// SyncTurnResponse is the result of turn synchronization.
type SyncTurnResponse struct {
	Status SyncTurnStatus `json:"status"`
}

// RecallOutcomeRequest is the payload-free terminal signal correlating
// prefetch selections with their consuming turn.
type RecallOutcomeRequest struct {
	WorkspaceRoot string            `json:"workspace_root"`
	RequestID     string            `json:"request_id"`
	Outcome       RecallTurnOutcome `json:"outcome"`
	Corrected     bool              `json:"corrected"`
}

// SessionEndRequest is the input for session shutdown rhythm handling.
type SessionEndRequest struct {
	WorkspaceRoot string  `json:"workspace_root"`
	SessionID     *string `json:"session_id"`
}

// SessionEndResponse is the result of session shutdown handling.
type SessionEndResponse struct {
	Status SessionEndStatus `json:"status"`
}

// MemoryCrudContext is the workspace-scoped request shared by all CRUD
// operations.
type MemoryCrudContext struct {
	WorkspaceRoot string `json:"workspace_root"`
}

// MemoryAddRequest is the input for a low-risk immediate memory write
// (user-requested fact or preference).
type MemoryAddRequest struct {
	Content      string        `json:"content"`
	EvidenceRefs []EvidenceRef `json:"evidence_refs"`
}

// MarshalJSON emits nil slices as empty arrays, matching the serde contract.
func (r MemoryAddRequest) MarshalJSON() ([]byte, error) {
	type plain MemoryAddRequest
	p := plain(r)
	if p.EvidenceRefs == nil {
		p.EvidenceRefs = []EvidenceRef{}
	}
	return marshalCanonical(p)
}

// MemoryListRequest is the input for listing the applied memory projection.
type MemoryListRequest struct {
	Limit *uint32 `json:"limit"`
}

// MemoryGetRequest is the input for retrieving one visible long-term record
// by id.
type MemoryGetRequest struct {
	RecordID string `json:"record_id"`
}

// MemorySearchRequest is the input for searching applied memory.
type MemorySearchRequest struct {
	Query string  `json:"query"`
	Limit *uint32 `json:"limit"`
}

// MemoryUpdateRequest is the input for a compare-and-swap update applied
// directly to BML.
type MemoryUpdateRequest struct {
	RecordID     string        `json:"record_id"`
	Content      string        `json:"content"`
	BaseRevision int64         `json:"base_revision"`
	EvidenceRefs []EvidenceRef `json:"evidence_refs"`
}

// MarshalJSON emits nil slices as empty arrays, matching the serde contract.
func (r MemoryUpdateRequest) MarshalJSON() ([]byte, error) {
	type plain MemoryUpdateRequest
	p := plain(r)
	if p.EvidenceRefs == nil {
		p.EvidenceRefs = []EvidenceRef{}
	}
	return marshalCanonical(p)
}

// MemoryRemoveRequest is the input for a compare-and-swap soft deletion
// applied directly to BML.
type MemoryRemoveRequest struct {
	RecordID     string `json:"record_id"`
	Reason       string `json:"reason"`
	BaseRevision int64  `json:"base_revision"`
}

// MemoryDistillRequest is the input for proactive experience distillation
// into a skill artifact at `<workspace>/skills/<skill_name>/SKILL.md`.
type MemoryDistillRequest struct {
	SkillName string  `json:"skill_name"`
	Content   string  `json:"content"`
	Evidence  *string `json:"evidence"`
}

// MemoryDistillResponse is the distill outcome. Upstream `memory_distill`
// returns `MemoryCrudOutcome`; the alias keeps the wire contract identical.
type MemoryDistillResponse = MemoryCrudOutcome

// MemoryEntry is a single entry of the applied memory projection.
type MemoryEntry struct {
	ID           string        `json:"id"`
	Content      string        `json:"content"`
	Trust        string        `json:"trust"`
	Provenance   *string       `json:"provenance"`
	EvidenceRefs []EvidenceRef `json:"evidence_refs"`
	Revision     int64         `json:"revision"`
	CreatedAt    string        `json:"created_at"` // RFC 3339
	UpdatedAt    string        `json:"updated_at"` // RFC 3339
}

// MarshalJSON emits nil slices as empty arrays, matching the serde contract.
func (e MemoryEntry) MarshalJSON() ([]byte, error) {
	type plain MemoryEntry
	p := plain(e)
	if p.EvidenceRefs == nil {
		p.EvidenceRefs = []EvidenceRef{}
	}
	return marshalCanonical(p)
}

// CrudOutcomeStatus is the serde `status` tag of a MemoryCrudOutcome.
type CrudOutcomeStatus string

const (
	// CrudOutcomeApplied: the write reached the durable authority.
	CrudOutcomeApplied CrudOutcomeStatus = "applied"
	// CrudOutcomeListed: a read projection of the applied authority.
	CrudOutcomeListed CrudOutcomeStatus = "listed"
	// CrudOutcomeProposalCreated: a governed proposal was durably created;
	// not authority until approved.
	CrudOutcomeProposalCreated CrudOutcomeStatus = "proposal_created"
	// CrudOutcomeFailed: the operation did not succeed.
	CrudOutcomeFailed CrudOutcomeStatus = "failed"
)

// MemoryCrudOutcome is the deterministic outcome of a memory CRUD operation —
// the Rust internally tagged enum `MemoryCrudOutcome` flattened for Go. The
// fields outside Status are variant payloads: Entry/EvidenceAdvisory for
// applied, Entries for listed, ProposalID for proposal_created, Reason for
// failed.
type MemoryCrudOutcome struct {
	Status CrudOutcomeStatus `json:"status"`
	// Entry is the entry as written (applied only); serialized as
	// `"entry":null` when absent, matching `Option<MemoryEntry>` without
	// skip_serializing_if.
	Entry *MemoryEntry `json:"-"`
	// EvidenceAdvisory is an advisory note when the write lacked tool-result
	// evidence (applied only; omitted when absent, matching
	// skip_serializing_if).
	EvidenceAdvisory *string       `json:"-"`
	Entries          []MemoryEntry `json:"-"`
	ProposalID       string        `json:"-"`
	Reason           string        `json:"-"`
}

// UnsupportedCrudOutcome mirrors `MemoryCrudOutcome::unsupported`: providers
// that do not implement an operation report Failed with a stable reason
// instead of silently succeeding.
func UnsupportedCrudOutcome(operation string) MemoryCrudOutcome {
	return MemoryCrudOutcome{
		Status: CrudOutcomeFailed,
		Reason: operation + " not supported by this memory provider",
	}
}

// MarshalJSON emits the serde internally tagged form `{"status": ...}` with
// only the active variant's fields.
func (o MemoryCrudOutcome) MarshalJSON() ([]byte, error) {
	switch o.Status {
	case CrudOutcomeApplied:
		return marshalCanonical(struct {
			Status           CrudOutcomeStatus `json:"status"`
			Entry            *MemoryEntry      `json:"entry"`
			EvidenceAdvisory *string           `json:"evidence_advisory,omitempty"`
		}{o.Status, o.Entry, o.EvidenceAdvisory})
	case CrudOutcomeListed:
		entries := o.Entries
		if entries == nil {
			entries = []MemoryEntry{}
		}
		return marshalCanonical(struct {
			Status  CrudOutcomeStatus `json:"status"`
			Entries []MemoryEntry     `json:"entries"`
		}{o.Status, entries})
	case CrudOutcomeProposalCreated:
		return marshalCanonical(struct {
			Status     CrudOutcomeStatus `json:"status"`
			ProposalID string            `json:"proposal_id"`
		}{o.Status, o.ProposalID})
	case CrudOutcomeFailed:
		return marshalCanonical(struct {
			Status CrudOutcomeStatus `json:"status"`
			Reason string            `json:"reason"`
		}{o.Status, o.Reason})
	default:
		return marshalCanonical(struct {
			Status CrudOutcomeStatus `json:"status"`
		}{o.Status})
	}
}

// UnmarshalJSON decodes the serde internally tagged form.
func (o *MemoryCrudOutcome) UnmarshalJSON(data []byte) error {
	var probe struct {
		Status CrudOutcomeStatus `json:"status"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}
	*o = MemoryCrudOutcome{Status: probe.Status}
	switch probe.Status {
	case CrudOutcomeApplied:
		var v struct {
			Entry            *MemoryEntry `json:"entry"`
			EvidenceAdvisory *string      `json:"evidence_advisory"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		o.Entry, o.EvidenceAdvisory = v.Entry, v.EvidenceAdvisory
	case CrudOutcomeListed:
		var v struct {
			Entries []MemoryEntry `json:"entries"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		o.Entries = v.Entries
	case CrudOutcomeProposalCreated:
		var v struct {
			ProposalID string `json:"proposal_id"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		o.ProposalID = v.ProposalID
	case CrudOutcomeFailed:
		var v struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		o.Reason = v.Reason
	}
	return nil
}

// ActmemReadTarget selects the bounded ACTMEM projection to read.
type ActmemReadTarget string

const (
	ActmemReadTargetPulse    ActmemReadTarget = "pulse"
	ActmemReadTargetRecap    ActmemReadTarget = "recap"
	ActmemReadTargetWork     ActmemReadTarget = "work"
	ActmemReadTargetHead     ActmemReadTarget = "head"
	ActmemReadTargetCapsules ActmemReadTarget = "capsules"
	ActmemReadTargetCapsule  ActmemReadTarget = "capsule"
)

// ActmemReadRequest is the input for reading a bounded ACTMEM projection.
type ActmemReadRequest struct {
	Target      ActmemReadTarget `json:"target"`
	CapsuleName *string          `json:"capsule_name"`
}

// ActmemReadResponse is the bounded ACTMEM projection content.
type ActmemReadResponse struct {
	Revision uint64 `json:"revision"`
	Content  string `json:"content"`
}

// ActmemEditWorkRequest replaces one registered ACTMEM Work subsection with
// CAS.
type ActmemEditWorkRequest struct {
	Section      string `json:"section"`
	Replacement  string `json:"replacement"`
	BaseRevision uint64 `json:"base_revision"`
}

// ActmemItemRequest targets one ACTMEM item for complete or drop with CAS.
type ActmemItemRequest struct {
	Section      string `json:"section"`
	ItemIndex    uint64 `json:"item_index"`
	BaseRevision uint64 `json:"base_revision"`
}

// ActmemMutationResponse reports the post-mutation revision.
type ActmemMutationResponse struct {
	Revision  uint64 `json:"revision"`
	UpdatedAt string `json:"updated_at"`
}

// MemoryRulesResponse is the complete memory writing handbook for write-time
// injection.
type MemoryRulesResponse struct {
	Content string `json:"content"`
	Source  string `json:"source"`
}

// L0MemoryPolicy is the L0 memory management policy injected into the system
// prompt (upstream `L0_MEMORY_POLICY`): action-verified writes, no volatile
// state in long-term memory, and minimal-pointer rendering.
const L0MemoryPolicy = `You manage memory in layers:
1. Action-Verified writes: persist facts confirmed by tool results or explicit user statements; never write speculation as authority.
2. No volatile state: session progress belongs in the working checkpoint, not in long-term memory.
3. Minimal pointer principle: startup injection carries only a compact index; retrieve full entries on demand with memory_search or memory_list.`

// SessionCheckpointBlockRequest is the input for rendering the current
// session's checkpoint block (upstream `SessionCheckpointRequest`).
type SessionCheckpointBlockRequest struct {
	WorkspaceRoot string `json:"workspace_root"`
	// SessionID is the session-scoped key (for example `channel:chat_id`).
	SessionID string `json:"session_id"`
}

// SessionCheckpointBlockResponse is the checkpoint block for live-turn
// assembly (upstream `SessionCheckpointResponse`).
type SessionCheckpointBlockResponse struct {
	// PromptBlock is the markdown to inject after the system prompt, when
	// non-empty.
	PromptBlock *string `json:"prompt_block"`
}

// SessionCheckpointWriteRequest is the input for writing the session working
// checkpoint. The checkpoint is volatile, session-scoped, cleared on session
// end, and never becomes long-term authority.
type SessionCheckpointWriteRequest struct {
	WorkspaceRoot string   `json:"workspace_root"`
	SessionID     string   `json:"session_id"`
	KeyInfo       string   `json:"key_info"`
	RelatedSops   []string `json:"related_sops"`
	Content       string   `json:"content"`
}

// MarshalJSON emits nil slices as empty arrays, matching the serde contract.
func (r SessionCheckpointWriteRequest) MarshalJSON() ([]byte, error) {
	type plain SessionCheckpointWriteRequest
	p := plain(r)
	if p.RelatedSops == nil {
		p.RelatedSops = []string{}
	}
	return marshalCanonical(p)
}

// SessionCheckpointWriteResponse is the checkpoint write outcome. Upstream
// `session_checkpoint_write` returns `MemoryCrudOutcome`; the alias keeps the
// wire contract identical.
type SessionCheckpointWriteResponse = MemoryCrudOutcome

// RenderSessionCheckpointBlock renders a checkpoint into the structured
// session checkpoint markdown block (upstream `render_session_checkpoint_block`).
func RenderSessionCheckpointBlock(keyInfo string, relatedSops []string, content string) string {
	var b strings.Builder
	b.WriteString("## Session Checkpoint\n")
	if info := strings.TrimSpace(keyInfo); info != "" {
		b.WriteString("Key info:\n" + info + "\n")
	}
	if len(relatedSops) > 0 {
		lines := make([]string, len(relatedSops))
		for i, sop := range relatedSops {
			lines[i] = "- " + strings.TrimSpace(sop)
		}
		b.WriteString("Related SOPs:\n" + strings.Join(lines, "\n") + "\n")
	}
	if state := strings.TrimSpace(content); state != "" {
		b.WriteString("State:\n" + state + "\n")
	}
	return b.String()
}

// Errors returned by the default (unsupported) provider surface.
var (
	// ErrActmemUnavailable is the default for actmem_* methods.
	ErrActmemUnavailable = errors.New("actmem unavailable")
	// ErrMemoryRulesUnavailable is the default for MemoryRules.
	ErrMemoryRulesUnavailable = errors.New("memory rules unavailable")
)

// Provider is the isolation layer between the runtime and any long-memory
// backend — the sync Go equivalent of Diva's `MemoryProvider` trait.
//
// Contract rules carried over from upstream:
//   - SystemPromptBlock is startup-only and side-effect free: synchronous
//     cached state only, no I/O, blocking, or refresh work.
//   - Prefetch is recall-only: transient prompt context, no durable writes
//     or session-end work.
//   - SyncTurn is persistence-only: not for backfilling startup prompt state
//     or live-turn recall.
//   - OnSessionEnd is shutdown-only and idempotent for duplicate hooks.
//   - Markdown memory is the legacy authority only when no Laputa workspace
//     exists; if a `.laputa` workspace exists but cannot be opened,
//     implementations must return an explicit degraded status rather than
//     silently falling back to a Markdown second source of truth.
//   - All request/response types are domain structs; do not leak transport
//     schemas, CLI arguments, or backend model types.
type Provider interface {
	// SystemPromptBlock builds the startup memory block for system prompt
	// assembly. Must be side-effect free from the caller's perspective.
	SystemPromptBlock(ctx context.Context, req SystemPromptRequest) (SystemPromptResponse, error)

	// SystemPromptRevision returns the in-process version of the cached
	// startup prompt projection. Must not perform file system access,
	// blocking work, or I/O.
	SystemPromptRevision(ctx context.Context, req SystemPromptRequest) uint64

	// RefreshSystemPromptProjection refreshes a cached startup projection
	// after an external authority write. Providers without a refreshable
	// projection may acknowledge the revision without changing their prompt
	// revision.
	RefreshSystemPromptProjection(ctx context.Context, req SystemPromptRefreshRequest) (SystemPromptRefreshResponse, error)

	// Prefetch performs optional intent-aware recall for a live turn.
	// Recoverable misses or backend failures prefer PrefetchStatus over a
	// top-level error so the turn can continue without recall context.
	Prefetch(ctx context.Context, req PrefetchRequest) (PrefetchResponse, error)

	// SyncTurn persists evidence after a successful turn. Report
	// SyncTurnStatusProposalCreated for governed proposals and
	// SyncTurnStatusPersisted for direct authority writes; prefer
	// SyncTurnStatusFailed over a top-level error when the session can
	// continue safely.
	SyncTurn(ctx context.Context, req SyncTurnRequest) (SyncTurnResponse, error)

	// RecordRecallOutcome persists a payload-free outcome for the
	// immediately preceding recall.
	RecordRecallOutcome(ctx context.Context, req RecallOutcomeRequest) error

	// MemoryAdd is a low-risk immediate memory write (user-requested fact or
	// preference).
	MemoryAdd(ctx context.Context, c MemoryCrudContext, req MemoryAddRequest) (MemoryCrudOutcome, error)

	// MemoryList lists the applied memory projection.
	MemoryList(ctx context.Context, c MemoryCrudContext, req MemoryListRequest) (MemoryCrudOutcome, error)

	// MemoryGet retrieves one visible long-term record by stable id.
	MemoryGet(ctx context.Context, c MemoryCrudContext, req MemoryGetRequest) (MemoryCrudOutcome, error)

	// MemorySearch searches applied memory.
	MemorySearch(ctx context.Context, c MemoryCrudContext, req MemorySearchRequest) (MemoryCrudOutcome, error)

	// MemoryUpdate is a compare-and-swap update applied directly to BML.
	MemoryUpdate(ctx context.Context, c MemoryCrudContext, req MemoryUpdateRequest) (MemoryCrudOutcome, error)

	// MemoryRemove is a compare-and-swap soft deletion applied directly to
	// BML.
	MemoryRemove(ctx context.Context, c MemoryCrudContext, req MemoryRemoveRequest) (MemoryCrudOutcome, error)

	// ActmemRead reads a bounded ACTMEM projection.
	ActmemRead(ctx context.Context, req ActmemReadRequest) (ActmemReadResponse, error)

	// ActmemEditWork replaces one registered ACTMEM Work subsection with CAS.
	ActmemEditWork(ctx context.Context, req ActmemEditWorkRequest) (ActmemMutationResponse, error)

	// ActmemComplete completes one open item with CAS.
	ActmemComplete(ctx context.Context, req ActmemItemRequest) (ActmemMutationResponse, error)

	// ActmemDrop drops one ACTMEM item with CAS.
	ActmemDrop(ctx context.Context, req ActmemItemRequest) (ActmemMutationResponse, error)

	// MemoryRules returns the complete memory writing handbook for
	// write-time injection.
	MemoryRules(ctx context.Context) (MemoryRulesResponse, error)

	// RecordUserPulse records one safe interactive user utterance in ACTMEM
	// Pulse.
	RecordUserPulse(ctx context.Context, sessionID, content string) error

	// RecordAssistantRecap records one completed assistant response in ACTMEM
	// Recap.
	RecordAssistantRecap(ctx context.Context, sessionID, content string) error

	// FoldActmemSession folds the named idle session's Pulse and Recap into
	// bounded capsules.
	FoldActmemSession(ctx context.Context, sessionID string) error

	// MemoryDistill performs proactive experience distillation into a skill
	// artifact.
	MemoryDistill(ctx context.Context, req MemoryDistillRequest) (MemoryDistillResponse, error)

	// SessionCheckpointBlock renders the current session's checkpoint block
	// for live-turn assembly. Working memory is volatile and session-scoped;
	// providers without a checkpoint surface return an empty block so the
	// turn proceeds without recall context.
	SessionCheckpointBlock(ctx context.Context, req SessionCheckpointBlockRequest) (SessionCheckpointBlockResponse, error)

	// SessionCheckpointWrite writes the session working checkpoint. The
	// checkpoint is volatile, not authoritative, cleared on session end, and
	// applied immediately rather than through a governed proposal.
	SessionCheckpointWrite(ctx context.Context, req SessionCheckpointWriteRequest) (SessionCheckpointWriteResponse, error)

	// OnSessionEnd triggers shutdown/session-end rhythm work if needed. It
	// should not compensate for missed SyncTurn writes unless the provider
	// documents that behavior.
	OnSessionEnd(ctx context.Context, req SessionEndRequest) (SessionEndResponse, error)
}

// NopProvider is the embeddable no-op Provider. It mirrors the upstream
// MemoryProvider trait default methods exactly: revision 0, echoed refresh,
// nil side effects, unsupported CRUD outcomes, and actmem/rules unavailability.
// Methods that upstream leaves without a default (system_prompt_block,
// prefetch, sync_turn, on_session_end) return inert success values matching
// the upstream test NoopMemoryProvider.
type NopProvider struct{}

// SystemPromptBlock returns a ready response with an empty markdown block.
func (NopProvider) SystemPromptBlock(_ context.Context, _ SystemPromptRequest) (SystemPromptResponse, error) {
	return ReadySystemPromptResponse(SystemPromptBlock{
		Shape: StartupInjectionShapeCompactRenderedMarkdown,
	}), nil
}

// SystemPromptRevision returns 0: no refreshable in-process projection.
func (NopProvider) SystemPromptRevision(_ context.Context, _ SystemPromptRequest) uint64 {
	return 0
}

// RefreshSystemPromptProjection echoes the authority revision without
// advancing the projection.
func (NopProvider) RefreshSystemPromptProjection(_ context.Context, req SystemPromptRefreshRequest) (SystemPromptRefreshResponse, error) {
	return SystemPromptRefreshResponse{
		AuthorityRevision: req.AuthorityRevision,
		ProjectionChanged: false,
	}, nil
}

// Prefetch reports PrefetchStatusSkippedNoIntent.
func (NopProvider) Prefetch(_ context.Context, _ PrefetchRequest) (PrefetchResponse, error) {
	return PrefetchResponse{Status: PrefetchStatus{Kind: PrefetchStatusSkippedNoIntent}}, nil
}

// SyncTurn reports SyncTurnStatusNoop.
func (NopProvider) SyncTurn(_ context.Context, _ SyncTurnRequest) (SyncTurnResponse, error) {
	return SyncTurnResponse{Status: SyncTurnStatus{Kind: SyncTurnStatusNoop}}, nil
}

// RecordRecallOutcome is a no-op.
func (NopProvider) RecordRecallOutcome(_ context.Context, _ RecallOutcomeRequest) error {
	return nil
}

// MemoryAdd is unsupported.
func (NopProvider) MemoryAdd(_ context.Context, _ MemoryCrudContext, _ MemoryAddRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_add"), nil
}

// MemoryList is unsupported.
func (NopProvider) MemoryList(_ context.Context, _ MemoryCrudContext, _ MemoryListRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_list"), nil
}

// MemoryGet is unsupported.
func (NopProvider) MemoryGet(_ context.Context, _ MemoryCrudContext, _ MemoryGetRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_get"), nil
}

// MemorySearch is unsupported.
func (NopProvider) MemorySearch(_ context.Context, _ MemoryCrudContext, _ MemorySearchRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_search"), nil
}

// MemoryUpdate is unsupported.
func (NopProvider) MemoryUpdate(_ context.Context, _ MemoryCrudContext, _ MemoryUpdateRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_update"), nil
}

// MemoryRemove is unsupported.
func (NopProvider) MemoryRemove(_ context.Context, _ MemoryCrudContext, _ MemoryRemoveRequest) (MemoryCrudOutcome, error) {
	return UnsupportedCrudOutcome("memory_remove"), nil
}

// ActmemRead fails with ErrActmemUnavailable.
func (NopProvider) ActmemRead(_ context.Context, _ ActmemReadRequest) (ActmemReadResponse, error) {
	return ActmemReadResponse{}, ErrActmemUnavailable
}

// ActmemEditWork fails with ErrActmemUnavailable.
func (NopProvider) ActmemEditWork(_ context.Context, _ ActmemEditWorkRequest) (ActmemMutationResponse, error) {
	return ActmemMutationResponse{}, ErrActmemUnavailable
}

// ActmemComplete fails with ErrActmemUnavailable.
func (NopProvider) ActmemComplete(_ context.Context, _ ActmemItemRequest) (ActmemMutationResponse, error) {
	return ActmemMutationResponse{}, ErrActmemUnavailable
}

// ActmemDrop fails with ErrActmemUnavailable.
func (NopProvider) ActmemDrop(_ context.Context, _ ActmemItemRequest) (ActmemMutationResponse, error) {
	return ActmemMutationResponse{}, ErrActmemUnavailable
}

// MemoryRules fails with ErrMemoryRulesUnavailable.
func (NopProvider) MemoryRules(_ context.Context) (MemoryRulesResponse, error) {
	return MemoryRulesResponse{}, ErrMemoryRulesUnavailable
}

// RecordUserPulse is a no-op.
func (NopProvider) RecordUserPulse(_ context.Context, _, _ string) error {
	return nil
}

// RecordAssistantRecap is a no-op.
func (NopProvider) RecordAssistantRecap(_ context.Context, _, _ string) error {
	return nil
}

// FoldActmemSession is a no-op.
func (NopProvider) FoldActmemSession(_ context.Context, _ string) error {
	return nil
}

// MemoryDistill is unsupported.
func (NopProvider) MemoryDistill(_ context.Context, _ MemoryDistillRequest) (MemoryDistillResponse, error) {
	return UnsupportedCrudOutcome("memory_distill"), nil
}

// SessionCheckpointBlock returns an empty block so the turn proceeds without
// recall context.
func (NopProvider) SessionCheckpointBlock(_ context.Context, _ SessionCheckpointBlockRequest) (SessionCheckpointBlockResponse, error) {
	return SessionCheckpointBlockResponse{}, nil
}

// SessionCheckpointWrite is unsupported.
func (NopProvider) SessionCheckpointWrite(_ context.Context, _ SessionCheckpointWriteRequest) (SessionCheckpointWriteResponse, error) {
	return UnsupportedCrudOutcome("session_checkpoint_write"), nil
}

// OnSessionEnd reports SessionEndStatusNoop.
func (NopProvider) OnSessionEnd(_ context.Context, _ SessionEndRequest) (SessionEndResponse, error) {
	return SessionEndResponse{Status: SessionEndStatus{Kind: SessionEndStatusNoop}}, nil
}

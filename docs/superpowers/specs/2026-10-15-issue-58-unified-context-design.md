# Issue #58: unified subject continuity design

Status: **planning proposal for review; no implementation or product acceptance**.
Revision: `issue58-design-1`, 2026-10-15. Parent requirements:
[unified context contract](../../architecture/VIVY-UNIFIED-CONTEXT-MEMORY.md),
[Issue #58](https://github.com/ProjectViVy/agent-vivy/issues/58), and #57's worker boundaries.
[Execution index](../plans/issue-58/index.md) owns readiness and dependencies.

## Intent, authority, and limits

After several context rotations, a LIFE conversation in session B must continue
session A's corrections, promises, unfinished matters, and tool-verified outcomes.
The actual provider request must contain the committed Recap and exact authorized
recent originals. Saving a document or returning a plausible answer is insufficient.

| Requirement | Required outcome | Story |
|---|---|---|
| R1 | One subject, one Laputa-owned free-text Recap; atomic revision/coverage/CAS/history/idempotency | S1 |
| R2 | Continuously durable original user/assistant/tool evidence and attachment references, ordered complete interactions, no replay/fork duplication | S2 |
| R3 | Shared authorized recent window, actual model admission, safe continuous-range compaction, explicit capacity failures | S3 |
| R4 | Full inspect/edit/conflict/clear/history/restore/source UI and equivalent headless/deferred tools | S4 |
| R5 | BML first; one selected projection sink; provider-native CRUD/search; legacy preservation | S5 |
| R6 | Coding/worker/workspace isolation, persona ownership, no forced onboarding | S1–S5 |
| R7 | Same-candidate independent review, CI, real model/UI/install/Windows/restart acceptance | S6 |

Owner requirements above are distinct from engineering proposals below. Proposed
budgets, persistence layout, activation boundary, identity binding and LIFE selector
are not individually user-approved. Review this package before execution. Do not
implement #66, a role wizard, main-chat redesign, robots, multimodal generation, a
standalone Garden import, a second runtime/scheduler/compactor/transcript database,
automatic persona evolution, or user-data migration. Development execution must use
**GPT-6.1 Sol, High**; GPT-6 Sol is not the requested model. Stage product acceptance
centrally; do not send the user a testing request after every Story.

## Verified baseline and gap matrix

Vivy main: `8088f5bcf0c034ea5a4ae6df83e497430d6cec94`.
Laputa: `4b2bec2cc2ab3374b7ec1ab8d448e612c1e4db24`, pseudo-version
`v0.0.0-20261010120923-4b2bec2cc2ab`; root `go.mod` replaces its three modules with
`../laputa/{laputa,garden,mentle}`. That exact detached checkout was inspected.
Eino `v0.9.13`: `c5e6aef927cca02bea934541f8dff2ea711b2ca7` inspected from tag checkout.
EinoExt pins: OpenAI `v0.1.13`, Claude `v0.1.25`, MCP `v0.0.9`; no adapter change proposed.
The October 13 closure draft was not found in the fetched tree/history; this design
does not claim to have read it. Existing [memory plans](../plans/memory/index.md)
remain delivery history; this package implements their linked #58 follow-up.

Paths marked `Laputa:` are relative to the separate pinned repository. All paths in
this matrix exist at the baseline; all new paths in Story plans are labeled proposed.

| Existing owner/seam | Inspected evidence | Gap / disposition |
|---|---|---|
| Laputa ACTMEM | `Laputa: laputa/actmem/{store,format,ops,capture_pair,scope}.go` | Pulse/Recap/Work v2 ring; mutex per instance, no exclusive directory owner; `Open` just returns `New`; plain rename. Evolve this same Store, retain legacy reads. |
| Garden boundary | `Laputa: garden/agentapi/{open,actmem,bound_reads}.go`, `garden/memory/contract.go` | Scope-bound owner and capability-aware backend exist. Add narrow Recap facade; no second owner or full Garden merge. |
| Host composition | `internal/cognitivecontract/ports.go`, `internal/modules/diva-cognitive/{factory,dispatch,actions}.go` | Same existing bundle owns ACTMEM; currently `profileID` doubles as SubjectID. Separate subject identity, keep profile as storage/persona binding only. |
| Original messages | `internal/runtime/message_projector.go`, `internal/storage/{contracts,history}.go`, `internal/storage/{sqlite,postgres}/messages.go` | Journal replay + deterministic message IDs already exist. Current capture in `cognitive_capture_source.go` has user text plus bounded assistant projection, not complete experience. |
| Tool original | `internal/runtime/tooladapter.go:804–838`, `internal/runtime/tool_operation.go` | Result compaction is inside invoke, before coordinator persistence. Persist exact result/reference before shortening; Journal-only changes would lose original output. |
| Durable context | `internal/runtime/{context,service,compaction_service,compaction_middleware,engine}.go` | Newest-first trim, history-read fallback, timestamp `TailFrom`, 200 KiB summary-input break, and in-run reduction can lose uncovered context. Selected policy must bypass these retirements, reuse their admission/model seams. |
| Retention | `internal/storage/{sqlite,postgres}/sessions.go`, `internal/runtime/service.go` | DeleteSession physically deletes messages/runs/Journal; LIFE list removal needs non-destructive archive path. |
| BML | `bml/{provider,home,store,search}.go`, `internal/modules/memory/{provider,service,actions,tools}.go` | Existing completion observer is an 8 KiB projection, not originals. Preserve native CRUD, extend source export/projection semantics. |
| UI | `plugins/vivy-memory/ui/vivy-memory/src/{page,view,dialogs}.tsx`, `memory-client.ts` | Existing module manages BML records/rules only. Add actual owner Recap tab, never local authority. |
| Deferred tools | `internal/runtime/{toolmount_middleware,tool_activation,tool_search_e2e_test}.go` | Reuse deferred catalog and tool_search activation, no always-on Recap schemas. |
| Persona | `internal/runtime/service.go:1180`, `prompts/persona-uninitialized.md`, `internal/modules/diva-cognitive/factory.go` | Truly uninitialized chat already allowed unnamed; initialized Frozen Core is sole persona owner. Preserve both. |

Baseline test/environment evidence is in [verification](../../logs/2026-10-15-issue-58-plan/verification.md).
These are source findings, not evidence that this design is delivered.

## Chosen architecture and economics

Extend the existing Laputa ACTMEM Store with a Recap policy format and an atomic
head. Core Storage retains originals and adds only an ordered metadata index of
complete interactions. The existing Service/ContextHost/compaction path consumes
that index and calls the selected owner. BML or Mentle receives a rebuildable
search projection through the existing observer mechanism. This adds a small
owner API, a core index, and host adapters; no new daemon, queue, database or model
loop. A second transcript store would duplicate evidence and deletion semantics;
putting Recap in BML would create a second authority and couple continuity to search.
Neither alternative meets the contract.

The hot path reads one Recap head plus a bounded indexed suffix; it does not scan all
sessions on every turn. Revision history grows with commits, originals with events;
there is no default expiry under this proposal. Index repair is paged and resumes
from durable coordinates. No latency or cost improvement is claimed without S6
measurements. Model calls occur only at admission pressure or explicit compact.

### Eino capability check

Inspected `adk/middlewares/summarization/summarization.go` exposes `New`, `Summarize`,
`GenModelInput`, `Finalize`, `Callback`, and token triggers. `Summarize` calls
Finalize/Callback before `BeforeModelRewriteState` replaces `state.Messages`;
errors abort replacement. `adk/handler.go` provides `BeforeModelRewriteState` and
`adk/chatmodel.go` has resume `HistoryModifier`. Existing Vivy already uses these
and `reduction.New` in `compaction_middleware.go`.

Reuse `summarization.New(...).Summarize` for the selected complete segment. Supply
`GenModelInput` with exact previous Recap + selected originals and a Recap-specific
instruction, and a custom `Finalize` that validates output and commits the owner
CAS **before** returning replacement messages. Do not use `DefaultFinalize`'s
post-processing to transform the free text. Reuse `observeSummaryModel` for calls,
usage and cancellation. Do not register legacy reduction/summarization for this
policy, since they can retire context before the owner commit. A thin Vivy
coordinator is justified only for trusted scope, durable coordinates, CAS and
retention, which Eino does not supply. It is not another summarizer or agent loop.
Keep all Eino imports inside runtime/provider. Re-evaluate this adapter if upstream
adds a durable authorized subject handoff; do not duplicate that capability.

## Shared contracts (proposed, authoritative revision 1)

### Identity and explicit LIFE selection

Core owns `SubjectID`, generated once as a UUID in its existing SnapshotStore key
`organism/subject/v1` using absent-version CAS; that same record pins the canonical
ACTMEM owner directory and an authority UUID. Read and verify all three on restart
and before opening an owner. A changed profile/config path cannot open a second
empty Recap stream for the same subject; return binding mismatch, no fallback or
automatic path migration. Resolve symlinks before registration. It is
an organism identity, not a persona name, ProfileID, role, or session ID. Trusted
composition binds this ID to the one existing ACTMEM directory and its one sharing
domain. Store an immutable binding record in the Recap head; mismatch fails closed.
Existing profile paths are not moved. Nonempty legacy ACTMEM remains blocked from
selection pending separate human-reviewed replacement; no automatic adoption.

**Decision D1 requiring consolidated product review:** there is no LIFE selector in
main (`Face` is web/tui/code/headless, and `RunMode`/PolicyProfile are execution
controls). Recommended minimal choice: add a persisted session `ContextPurpose`
enum `unspecified|life|coding`, set through existing authenticated `session/create`
or the memory page's explicit “Enable LIFE continuity for this session” action
before its first run. This is a continuity selector, not the #66 mode UI. Existing
and unspecified sessions remain unchanged. A session selected as LIFE gets
`life-recap-v1` **enabled by default**; it needs no second memory switch. Code face,
explicit coding tasks, private/child/strategy runs and restricted workspaces veto
the policy regardless of selector. Public/model input cannot assert SubjectID,
sharing membership or override the veto. Face alone never implies LIFE.

This selector is a new user-facing session contract, so S0 cannot be accepted and
S1–S5 cannot be released until D1 is reviewed. Alternative: wait for a separately
delivered LIFE admission selector; that adds an external blocker and does not
authorize treating all chat as LIFE. Free-text intent classification is not a
security boundary: coding tasks must use the explicit coding task/face admission.
The future LIFE entry must surface that rule; silently detecting task type from
message prose is out of scope. Normal unnamed first-arrival chat remains available.

One shared domain contains only host-authorized personal LIFE sessions. Restricted
workspace content never contributes to the head or window. Since a single free-text
Recap cannot be safely filtered per fact, a caller lacking the entire domain grant
receives no Recap. Do not produce partial subject heads. Scope membership is fixed
for phase one; dynamic removal/reclassification of already-covered sources is not
implemented as a silent edit. Fail closed and require an explicit reviewed reset if
the configured domain changes. All source reads recheck authorization.

### C1: Laputa Recap owner

Proposed additions in `Laputa: laputa/actmem/recap.go`; methods are on existing
`*Store`, with no second store constructor. `SubjectID`, `DomainID`, sequence and
source identifiers are opaque strings/uint64 values supplied by the trusted host.

```go
type RecapBinding struct { SubjectID, DomainID string }
type SourceRange struct { After, Through uint64; Digest string }
type RecapHead struct {
    Binding RecapBinding
    Revision uint64
    Text string
    CoveredThrough uint64
    InvalidationEpoch uint64
    OperationID, OperationDigest, PreviousDigest, Digest string
    Cause string // summary | edit | clear | restore
    Sources SourceRange
    RestoredFrom uint64
    UpdatedAt time.Time
}
type RecapCommit struct {
    Binding RecapBinding
    BaseRevision uint64
    OperationID, Text, Cause string
    Sources SourceRange
    RestoredFrom uint64
}
func (s *Store) ReadRecap(context.Context, RecapBinding) (RecapHead, error)
func (s *Store) CommitRecap(context.Context, RecapCommit) (RecapHead, error)
func (s *Store) LookupRecapOperation(context.Context, RecapBinding, string) (RecapHead, bool, error)
func (s *Store) ListRecapHistory(context.Context, RecapBinding, uint64, int) ([]RecapHead, error)
func (s *Store) ReadRecapRevision(context.Context, RecapBinding, uint64) (RecapHead, error)
func (s *Store) Close() error
```

`summary` requires `Sources.After == current.CoveredThrough`, nonempty contiguous
range and verified host range digest; `Through` becomes the next cursor. Every
other cause preserves current coverage. `restore` reads the selected historical
text inside the owner and creates a new version, preserving current coverage;
caller text is forbidden. `clear` produces empty text and increments
InvalidationEpoch. Edit/restore/summary preserve that epoch. `edit` may be empty
but does not acquire clear's admitted-context invalidation semantics; UI uses clear
for that explicit action. Same OperationID and canonical request digest returns the
original committed receipt even if later heads exist; a different digest conflicts.
Idempotency lookup precedes stale-base rejection, but never serves uncommitted files.
Errors: `recap_conflict`, `recap_operation_conflict`, `recap_scope_denied`,
`recap_legacy_nonempty`, `recap_invalid_range`, `recap_unavailable`,
`recap_owner_busy`, `recap_too_large`, `recap_commit_unknown`.

Persistence stays under existing `actmem/`: immutable Markdown revisions in
`recap-history/` hold full text, metadata and previous digest; `ACTMEM.MD` is the
single atomic current head including text+coverage+operation identity. Write and
flush the revision first, then atomically replace and flush head; only the chain
reachable from the committed head is history. Orphans are never promoted on startup.
A response lost after commit is resolved with OperationID. A sync failure after
replacement is an unknown commit: freeze further writes, reopen/read and resolve
the operation; never pretend rollback. No destructive cleanup in this scope.

Acquire a directory-scoped OS lock for Store lifetime (including two instances in
one process), canonicalize real path, fail on symlink alias collision, and release
on Close. Use the existing dependency-compatible `gofrs/flock` implementation where
available, adding an explicit Laputa module pin if necessary, not a PID-file lock.
Platform atomic replacement must support Windows replacement without unlinking the
old head first; isolate platform helpers and prove crash behavior. Revision history
is retained indefinitely by default. Do not add a SQLite ACTMEM store beside it.

### C2: core evidence and ordered interaction index

Proposed `internal/storage/subject_history.go` extends existing Storage; original
bodies remain messages/Journal/tool operation artifacts. The index stores only
subject/domain, monotonic sequence, canonical interaction ID, source coordinates,
digest, scope and terminal/incomplete status. No transcript body or Recap text.

```go
type SubjectHistoryBinding struct { SubjectID, DomainID string }
type SubjectInteraction struct {
    Sequence uint64
    ID string // canonical original run + final Journal event identity
    SessionID domain.SessionID
    RunID domain.RunID
    TerminalSeq domain.EventSeq
    SourceDigest string
}
type SubjectHistoryStore interface {
    SealSubjectInteraction(context.Context, SubjectHistoryBinding, SubjectInteraction) (SubjectInteraction, error)
    ListSubjectInteractions(context.Context, SubjectHistoryBinding, uint64, uint64, int) ([]SubjectInteraction, error)
    ReadSubjectInteraction(context.Context, SubjectHistoryBinding, string) ([]domain.Message, error)
    ArchiveRetainedSession(context.Context, domain.SessionID) error
}
```

The index sequence is allocated only when a terminal interaction's exact evidence
is durable and validated. Concurrency orders by durable seal, never wall clock.
Crashes before seal delay visibility; replay repairs idempotently. Sequence allocation
and index insertion are one SQL transaction under subject serialization; no holes
are silently skipped. Every page records a captured high watermark. Summary ranges
are prefixes from coverage to that watermark, split only between whole interactions.
One enormous interaction causes explicit capacity pause, not a partial fold.

An interaction includes all admitted user messages, final assistant messages,
assistant tool calls, corresponding exact results/errors and durable references.
Interrupted/failed runs retain partial evidence but are marked with their actual
outcome, never a successful promise. Excluded automatic-ingest/private sources are
not index eligible. Current active run stays outside the sealed index, appended once
by canonical message ID. Forked copies point to original IDs and never seal again;
new branch activity gets new identities. Rewind appends a source disposition event,
does not rewind the subject cursor or erase experience, and future summaries must
respect that supersession. No newly created historical fork becomes a second event.

Persist user evidence at admission; preserve assistant deltas + completion digest;
persist full tool result or immutable artifact reference before `compactToolResult`.
Use existing ToolOperationStore/BlobStore/reference seams, with deterministic
result references and content digests. Never persist secrets removed by existing
credential/policy boundaries. Do not introduce generic core pattern redaction or
reuse legacy capture's shortened/redacted projection as original text. “Original”
means authorized admitted evidence, not bypassing those
boundaries. Attachments use message ID plus attachment ordinal/hash and existing
durable bytes, not local temporary paths. Projection repair must succeed before seal.

Paired append-only migrations add index/identity/retained-session metadata. LIFE
sources become retention-protected at admission, before terminal sealing, so failed
and unfinished interactions cannot disappear through hard deletion. LIFE
list removal archives the session; low-level hard deletion rejects retained sources
even if called outside UI. Recovery and exact source reads include archived sessions
under the same grant. Coding legacy deletion remains unchanged. Physical source or
Recap-history deletion is excluded; a backend tombstone never claims that deletion.

### C3: admission, compaction and activation

Runtime resolves policy and binding before any persona/Recap/history preparation.
Child/worker paths are denied by default including inherited hooks, tools and
instructions; an explicit scoped grant permits only its named source read, not
automatic resident Recap/persona. Coding policy continues its existing pipeline.

Selected LIFE input order: existing governed instructions/Frozen Core; a delimited
non-authoritative Recap data block with revision; shared recent original interactions
in sequence order; current interaction once. On-demand backend results remain
untrusted evidence in ContextHost's existing remaining budget. WORLD stays tool-only.
Reserve the Recap block before selecting optional retrieval. No synthetic legacy
SessionCompaction summary is inserted for LIFE. Deduplication uses original IDs,
not content equality (two equal user messages can be different experiences).

Proposed initial tunable seeds: Recap target 2,048 tokens, hard maximum 3,072,
recent raw target 8,192. Reuse the budget seam, but correct selected-policy accounting
to include full serialized tool schemas, message framing, reference/attachment
representations and output reserve. Use the provider tokenizer when available;
otherwise use a conservative UTF-8 byte upper bound for textual tokens plus the
provider's declared non-text charge, and fail capacity resolution if that charge is
unknown. The existing bytes/4 estimate alone is insufficient for Chinese or large
schemas. Clearly label estimates. These
are neither performance claims nor prior owner-approved limits. If total request
budget cannot fit instructions, Recap, latest complete interaction and current run,
pause with `context_capacity_required`; do not truncate. At admission or explicit
compact, select the oldest contiguous prefix leaving a complete suffix within the
raw target. The exact summarizer request must fit its real model capacity; subdivide
the prefix at complete-interaction boundaries. Never use the existing 200 KiB break.

```text
load authorized head and durable index watermark
assemble all uncovered complete interactions + current interaction
if pressure: select oldest complete prefix; summarize(head.text, exact prefix)
validate free text, output budget and unchanged scope
commit owner(base revision, operation ID, contiguous range+digest)
  conflict -> reread head, rebuild remaining range, regenerate (at most one fresh retry)
  unknown -> lookup same operation; unresolved -> pause
  failure before commit attempt -> retain old head and every source; no offload
  error/cancel after Finalize begins -> resolve operation, reload committed head
reload committed head; assemble its remaining exact suffix + current once
admit only after final capacity, permission and clear-epoch checks
```

One retry is a cost bound, not a forced success. Further conflict pauses explicitly.
Eino can fail in Callback or after-summary event emission after Finalize committed.
Track commit-attempt/receipt outside mutable Eino state and resolve the same
OperationID after any such error. A committed head remains committed; rebuild from
its cursor before retry/resume. Do not claim the old head survived a post-commit
error. The receipt is operation metadata, not another Recap authority.
Inference is outside owner locks. A confirmed durable commit (returned or resolved
by OperationID) is the only retirement boundary. No cross-database transaction is needed: evidence precedes
owner CAS, and context retirement is derived from the committed cursor on reload.
The index is not updated with a duplicate cursor after owner commit.

Ordinary edit/restore/other-session consolidation affects the next run admission.
Clear also invalidates already-admitted snapshots at the next safe model boundary:
compare epoch, full request capacity and permission before every model call/resume
using the existing Eino middleware. An oversized unsealed current interaction pauses
explicitly, since it cannot yet enter a complete summary range; no in-run fallback
may trim it. On an epoch mismatch, stop and discard the stale checkpoint admission, retain all evidence,
and require a newly admitted continuation. An already running provider request
cannot have its bytes recalled. Report this honestly in UI. No resume hook may
restore a pre-clear Recap. Do not automatically replay previously covered originals
after clear/restore; coverage does not regress. Later explicit authorized recall
may retrieve retained evidence, and clear is not a promise of permanent forgetting.

### C4: actions, tools and memory UI

The existing cognitive bundle exposes C1 through ActionHost; `vivy/memory` is the
consumer. Proposed action namespace `vivy.recap.{read,edit,clear,history,revision,
restore,sources}` with full-text payload, base_revision and operation_id for writes.
Subject/domain are derived server-side from the authenticated session; never accepted
from UI. Source action returns exact original messages plus stable references and
explicit denied/unavailable/partial outcomes. History cursor is revision-based.
Wire result: `{status: applied|listed|conflict|failed, head?, revisions?, sources?,
reason?, activation: next_run|next_safe_boundary}`. Conflict includes latest authorized
head and preserves caller draft. Do not silently retry a human's edit against a new base.

Deferred `actmem_read`, `actmem_update`, `actmem_clear`, `actmem_history`,
`actmem_restore` call the same governed owner methods through ToolHost. `tool_search`
discovers/activates them using existing exposure semantics. Default model requests
contain none of their full schemas; a worker cannot discover or invoke unauthorized
tools by guessing names. Clear/update/restore require the existing governed mutation
authorization; schemas cannot change persona slots. Must-not-forget guidance links
the existing human-controlled Persona/Frozen Core flow, never automatic promotion.

Reviewable wireframe proposal (within existing memory module; no main chat change):

| Page region | Content and behavior |
|---|---|
| Header | Memory; tabs **Active Recap / History / Long-term records**; server policy availability |
| Active status | Full revision, saved time, covered source range, “Applies next run”; AI consolidation state |
| Main editor | Entire actual text; edit mode has separate in-memory draft, Unsaved indicator, Save/Cancel |
| Footer | Clear active Recap; History; View covered sources; “Must not forget?” link to Persona review |
| History panel | Revision/time/cause/source range; full preview; Restore creates a new revision |
| Source panel | Exact ordered messages, tool-result status, attachment/reference open; unavailable reason, no fabricated excerpt |
| Conflict dialog | Your draft and current saved revision side by side; Copy draft / Reload / edit against latest; no force overwrite |
| Clear dialog | “Clears active Recap. History and original sources remain. Active calls finish; subsequent calls pause at a safe boundary.” |
| LIFE selection | D1 proposal: eligible current session can opt into LIFE before its first run; disabled with a clear reason otherwise |

State flow: read → edit(draft/base) → save → saved; stale save → conflict(draft
retained) → explicit reload/reconcile → save. Clear and restore are explicit actions
with the same conflict path. Refresh never discards a dirty draft. UI shows actual
loaded revision separately when an active run is pinned to an older one. No Recap
localStorage authority, hidden secondary summary or partial editor.

### C5: provider and legacy compatibility

BML stays the first validated backend. Its native long-term CRUD and search remain
authoritative for its own records. Extend the existing capability-aware boundary
for exact source export/expansion; unsupported operations fail explicitly. Core
originals are read through authorized core export, not reconstructed from BML text.
Mentle/Garden supplies its native search/ranking/results through the same semantic
contract; “contract-compatible” is separate from owner-led DIVA acceptance.

For selected LIFE, exactly one configured sink consumes the canonical index through
existing ObserverHost receipts/cursors. It can index source references/Recap revision
history as historical material, never promote overflow to current facts. Core Journal
remains primary evidence even during index lag/outage; no synchronous dual write.
Default BML composition must not also activate Garden/Mentle capture for those runs.
Do not keep the old BML completion observer and new subject projection both writing
the same experience. Other policies retain their existing subscriptions.

| Legacy surface | Selected LIFE disposition | Other policies / historical data |
|---|---|---|
| AppendCaptured Pulse/Recap ring | Disabled | Existing behavior only in untouched v1/v2 directories; all writers reject a v3 head |
| Work patch/reconcile/capture summary consumers | Disabled with explicit policy-unavailable result | Legacy reads retained; legacy mutations only on v1/v2 directories |
| FoldSession capsules | No new LIFE fold writer | Existing capsules remain readable |
| SessionCompaction and Eino reduction | No LIFE retirement; route existing compact action to C3 | Coding unchanged |
| Nonempty v1/v2 ACTMEM | Block selection, no overwrite/conversion | Human-reviewed replacement is separate work |
| Persona/Frozen Core, WORLD rule | Keep | No automatic mutation/injection changes |

Disable new admission to roll back; retain v3 heads/history/originals. An older binary
that cannot read v3 must fail closed, not recreate v2 over it. Do not automatically
re-enable old writers against a v3 directory or downgrade schema. Library release
and exact Vivy pin update land before consumer activation.

The v3 exclusion is directory-wide, including pending old observer deliveries and
non-LIFE callbacks. It cannot be implemented as a per-run flag around v2 writes.
Coding model inputs, tools, context selection and compaction strategy remain unchanged;
optional legacy ACTMEM writes to a now-v3 owner return explicit policy-unavailable.
This compatibility effect must be demonstrated and reviewed at S5: if an installed
Coding composition actually requires those writes to operate, block activation and
present that concrete conflict rather than weakening either owner uniqueness or the
Coding requirement. Do not open another directory to make both writers succeed.

## Acceptance and remaining review

S6 uses Chinese experience A then a distinct same-subject session B after multiple
folds: corrected dates/decisions, a promise, unfinished work and verified tool result
must survive without invented facts. Collect exact provider request bodies in a
controlled test environment plus request digest/Recap revision/source range. Evaluate
quality, omissions and model token/call costs; no simulated model proves real quality.

Test concurrent summaries, edit, clear, cancellation, fail-before-commit, commit with
lost response, reopen, Windows replacement/lock, index lag, archive removal and exact
source navigation; test Coding/worker/unauthorized domain negative cases. UI and
headless must operate on the same head. DIVA/Mentle remains pending until its actual
selected installed composition passes, even if BML passes. One candidate commit and
exact dependency/artifact identity must own all claimed gates; a fix invalidates
affected evidence and requires a rerun on the new candidate.

D1 is the one product decision requiring review; all other engineering proposals are
concrete choices in this design. The current environment lacks Go/just/PowerShell,
so runtime validation is pending. No Story is Ready for development this turn.

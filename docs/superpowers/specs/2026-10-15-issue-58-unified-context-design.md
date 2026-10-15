# Issue #58: unified subject continuity design

Status: **development design; revision 3 supersession amendment adopted by controller;
no implementation or product acceptance implied**.
Revision: `issue58-design-3`, 2026-10-15. Parent requirements:
[unified context contract](../../architecture/VIVY-UNIFIED-CONTEXT-MEMORY.md),
[Issue #58](https://github.com/ProjectViVy/agent-vivy/issues/58), and #57's worker boundaries.
[Execution index](../plans/issue-58/index.md) owns readiness and dependencies.

## Intent, authority, and limits

LIFE is one continuous subject conversation, with no user-facing session model.
After context rotations or restart, that same conversation must retain corrections,
promises, unfinished matters, and tool-verified outcomes. Underlying Session/Run
records may be reused without exposing LIFE creation, switching or conversion.
The actual provider request must contain the committed Recap and exact authorized
recent originals. Saving a document or returning a plausible answer is insufficient.

| Requirement | Required outcome | Story |
|---|---|---|
| R1 | One subject, one Laputa-owned free-text Recap; atomic revision/coverage/CAS/history/idempotency | S1 |
| R2 | Continuously durable original user/assistant/tool evidence and attachment references, ordered complete interactions, no replay/fork duplication | S2 |
| R3 | Shared authorized recent window, actual model admission, safe continuous-range compaction, explicit capacity failures | S3 |
| R4 | Full inspect/edit/conflict/clear/history/restore/source UI and equivalent headless/deferred tools | S4 |
| R5 | BML first; one selected projection sink; provider-native CRUD/search; legacy preservation | S5 |
| R6 | Ordinary-session policy isolation, worker/workspace permissions, persona ownership, no forced onboarding | S1–S5 |
| R7 | Same-candidate independent review, CI, real model/UI/install/Windows/restart acceptance | S6 |

Owner requirements above are distinct from engineering proposals below. Proposed
budgets, persistence layout, activation boundary and identity binding mechanics
are not individually user-approved. The singleton LIFE product shape below is
owner-confirmed; the withdrawn D1 selector is not an open approval question. Review this package before execution. Do not
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
| Retention | `internal/storage/{sqlite,postgres}/sessions.go`, `internal/runtime/service.go` | DeleteSession physically deletes messages/runs/Journal; retained internal sources need non-destructive archive protection. The pinned LIFE conversation is not a removable session-list item. |
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

The hot path reads one Recap head, a bounded indexed suffix, and a complete
authorized-domain disposition metadata snapshot. It does not replay covered bodies
as new experience. Revision history grows with commits, originals with events;
there is no default expiry. Pending projection repair precedes capture. Model calls
occur at admission pressure, explicit compact, or required covered-source
reconciliation. No latency or cost improvement is claimed without S6 measurements.

Revision 3 resolves covered-source supersession with the smallest coherent extension:
core Storage captures the complete disposition set; the SAME ACTMEM owner commits
its host fingerprint atomically with corrected text using a coverage-preserving
`reconcile` transition. Typed disposition events in the existing C2 sequence are a
material alternative, but would redefine terminal-interaction ordering, extend all
marker producers and require backfill while still needing a restore/edit policy.
The chosen approach costs a complete metadata scan and sometimes correction inference;
it avoids a second cursor, authority, scheduler, or synthetic interaction. This is a
controller architecture ruling implementing existing supersession requirements, not
a new user product choice. The focused investigation is
[supersession design check](../../../.superpowers/sdd/S3/supersession-design-check.md);
the contracts below, rather than that investigation, govern implementation.

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

### 2026-10-15 native-fix capability amendment

The inspected design baseline above remains Eino v0.9.13. The current dependency
pin is official v0.9.20 (`b9539ec114a14c32aa568f05eda5cde1e83fcfc1`), selected
for the native enhanced-stream converter race fix, with EinoExt and published
Laputa 4c7 pins unchanged. Reinspection confirms `summarization.New`, `Summarize`,
`GenModelInput`, `Finalize`, `Callback`, `BeforeModelRewriteState`,
`WithHistoryModifier` and runner checkpoint/resume remain available. Finalize
still precedes Callback and after-summary events; message replacement follows
successful Summarize. The durable CAS/receipt recovery obligation above is unchanged.
Native canceled tool streams now checkpoint completed output and rerun unfinished
tools through the existing compose/ADK path. Do not add another scheduler or
checkpoint mechanism. Exact source/API and final gate evidence is in
[the remediation report](../../logs/2026-10-15-issue58-subject-binding/eino-upgrade-report.md).

## Shared contracts (authoritative revision 3; implementation status is in the index)

### Identity and the trusted singleton LIFE binding

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

**Owner correction, superseding D1:** LIFE is a special, unique persistent main
conversation, permanently entered from the left sidebar above Sessions, showing
the current persona avatar and name (for example, Vivy). It has no user-facing
session create/switch/convert flow. Remove residual visible Coding/LIFE selector
or toggle buttons; entering the pinned conversation is the product entry, not a
mode switch on an ordinary session. Avatar/name changes are presentation updates,
never a subject rekey, new conversation or memory reset. Uninitialized users can
continue unnamed chat; the older forced-setup requirement is superseded.

[Issue #66](https://github.com/ProjectViVy/agent-vivy/issues/66) owns the sidebar/main
conversation producer and its trusted policy hook. That UI is an **external
producer dependency**, not implementation scope of #58. #58 consumes its one
persistent main-conversation binding and enables `life-recap-v1` by default there.
It must not add `ContextPurpose`, a memory-page enable action, session/create flags,
or a conversion path as a substitute. Ordinary sessions keep their traditional
Coding context/compaction policy. They are still the subject's own sessions and may
share its separately governed Persona under #57; they are not delegated workers.

LIFE retains executable tools and coding ability. A request to write code inside
LIFE does not change its identity or silently select ordinary-session policy.
Different compaction and tool exposure are trusted policy/composition choices in
the same engine. Reuse existing Recipe/Module/ToolHost inventory and authorization;
do not invent an approved tool allowlist, remove coding tools by prose intent, or
introduce another loop. Worker/private/restricted scope rules still apply.

Proposed #58 consumer seam, to agree with the #66 producer before integration:

```go
type MainConversationBinding struct {
    SubjectID, DomainID, ConversationID string
    ActiveSessionID domain.SessionID // internal storage coordinate, not subject identity
    PolicyID, ToolPolicyRef string // trusted Generation policy references
    SchemaVersion int
    Grant storage.SubjectGrantSnapshot // actual Host decision, reference and version
}
type MainConversationResolver interface {
    ResolveMainConversation(context.Context) (MainConversationBinding, error)
}
```

The proposed shared type belongs at `internal/cognitivecontract/main_conversation.go`
under the #66 producer's ownership. Its resolver is supplied by trusted composition,
not constructed from model/client fields. Admission validates the target internal
conversation/session against this binding, pins the policy and uses the existing
Service.Run. Main conversation identity survives restart and any authorized internal
storage segmentation; SubjectID remains independently stable. A missing/mismatched
binding fails explicitly for LIFE, never falls back to an arbitrary ordinary session.
Headless/test composition may supply the same governed singleton binding without
implementing the sidebar or creating an alternate runtime. Installed integration
remains blocked until the real #66 producer and consumer seam are accepted.

The #58 consumer additionally requires `SchemaVersion == 1` and a
`SubjectGrantSnapshot` containing stable Host `Reference`, `Version`, the actual
immutable decision `Snapshot` JSON, and `WholeDomainRead`. These additions remain
subject to #66 agreement; #58 does not implement that producer. Contribution is
separate from resident read authority. Trusted composition may supply
`SubjectSourceResolver(context.Context, domain.SessionID)` for explicitly
approved ordinary personal sources. That callback uses the existing Host
Policy/Grant checks, returns no grant for private/restricted/child/strategy or
foreign sources, and is never accepted from model/client JSON. An ordinary
source's snapshot must not grant `WholeDomainRead` or `life-recap-v1`.

Session source membership and each Run's conversation/policy/schema/grant
snapshot commit inside the existing admission transaction. Membership is
immutable; changed policy, domain, grant reference/version/decision or missing
resolver fails closed for a persisted source. Unshared ordinary admission stays
unchanged. A second internal segment may share the existing main conversation
identity; segments do not become ordinary Session-list entries. These metadata
pins are evidence, not a permission engine or caller authorization token. Every
resident/source reader must recheck the current trusted Host authorization.

The existing `FactoryInput.ResolveRecapBinding` callback carries trusted
`RecapBinding.SubjectID` independently of `ProfileID`, at the actual existing
ACTMEM `Store.Root`. There is no second SubjectID resolver, ProfileID fallback,
or migration of legacy evolution/persona keys.


One shared domain contains only host-authorized subject-owned personal sources.
The general cross-session Recap/recent-original contract remains: internal segments
and other explicitly authorized subject sources may contribute without creating
multiple visible LIFE conversations or granting ordinary sessions resident LIFE
context. Ordinary sessions are unshared by default; explicit source eligibility
controls exact evidence capture, sealing and retention independently of resident
LIFE injection. Such an authorized source keeps its existing Coding request strategy.
Restricted workspace content never contributes to the head or window. Since a single free-text
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
    Cause string // summary | reconcile | edit | clear | restore
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
InvalidationEpoch. Edit/restore/summary/reconcile preserve that epoch. `edit` may be empty
but does not acquire clear's admitted-context invalidation semantics; UI uses clear
for that explicit action. Same OperationID and canonical request digest returns the
original committed receipt even if later heads exist; a different digest conflicts.
Idempotency lookup precedes stale-base rejection, but never serves uncommitted files.
Errors: `recap_conflict`, `recap_operation_conflict`, `recap_scope_denied`,
`recap_legacy_nonempty`, `recap_invalid_range`, `recap_unavailable`,
`recap_owner_busy`, `recap_too_large`, `recap_commit_unknown`.

`reconcile` requires `Sources.After == Sources.Through == current.CoveredThrough`,
nonempty opaque host receipt, and `RestoredFrom == 0`. It corrects covered-source
supersession without advancing coverage or epoch. Ordinary `summary` still requires
a nonempty contiguous interaction range. Edit/clear/restore still require empty
Sources; no reconciliation may masquerade as a user edit or fake completed run.

For new summary/reconcile commits the trusted host encodes a canonical versioned
envelope in the existing opaque `Sources.Digest` string:

```text
{v:1, binding, range:{after,through,exact_originals_digest},
 dispositions:{digest}, correction_evidence_digest}
```

The outer range fields are the real interaction coordinates. The host verifies
the original-range digest, complete C2 disposition snapshot and exact correction
references; Laputa does not parse or independently verify source evidence. The
whole string participates in existing request/head digests, CAS and durable replay.
It is provenance attached to the current text, not a second progress cursor: no
numeric marker coverage or SQL acknowledgement is introduced. Historical plain
receipts remain valid owner data but cannot certify the current disposition set.
Edit/restore/clear reset the certificate through their existing empty Sources;
never inherit it from an older revision after such a mutation. Empty cleared text
must not be reconstructed from correction references.

This is a semantic C1 contract extension using unchanged Go DTOs/port signatures,
not a proposed public RPC/schema expansion or new ACTMEM format number. Existing
`laputa/actmem/{recap,recap_format,recap_history}.go` must all validate the new cause
and its transition. Strict older decoders reject unknown causes: preserve that
fail-closed behavior, publish/pin a compatible owner before S3 activation, and never
roll back an extended head to an incompatible reader/writer. Existing Garden
`agentapi/recap.go`, Vivy `internal/cognitivecontract/recap.go` aliases and bound
`internal/modules/diva-cognitive/recap.go` must pass through the same contract and
failures with no second DTO/owner. Receipt-envelope version 1 is host-defined;
unknown envelope versions cannot authorize S3 retirement/reconciliation skipping.

Persistence stays under existing `actmem/`: immutable Markdown revisions in
`recap-history/` hold full text, metadata and previous digest; `ACTMEM.MD` is the
single atomic current head including text+coverage+operation identity. Write and
flush the revision first, then atomically replace and flush head; only the chain
reachable from the committed head is history. Orphans are never promoted on startup.
A response lost after commit is resolved with OperationID only after owner-level
durability confirmation. Namespace visibility and matching hashes are not a receipt.
A failure after replacement is attempted freezes all authoritative C1 reads,
positive/negative lookup, history, revision access and replay with zero data and
`recap_commit_unknown` until Close/reopen and successful recovery. The first bound
C1 use of every newly acquired owner validates the selected reachable chain and
republishes the exact selected head bytes through the platform's staged replacement
and flush protocol before acknowledging it. Recovery changes no logical revision,
operation identity, digest, timestamp, text or coverage, and never promotes orphans.
Confirmed unchanged heads need no repeated full durability flush. No destructive
cleanup or second owner/receipt database is introduced.

Missing-head negative resolution also needs a qualified namespace absence barrier.
Unix may return not-found after successful directory persistence. Windows currently
has no qualified absence barrier: missing-head lookup/history/revision fail closed
with `recap_commit_unknown`. Revision-zero ReadRecap is only an **uninitialized
current view**, with zero coverage; it is not a committed receipt or proof that a
pending operation was absent. Under the same owner, a new or exact-retry base-zero
Commit may publish its actual requested initial head through qualified write-through
replacement and return that positive receipt. A visible nonempty selected head must
always be validated/recovered/replayed, never treated as empty. Native Windows
recovery acceptance remains required; cross-compilation is not durability evidence.

Acquire a directory-scoped OS lock for Store lifetime (including two instances in
one process), canonicalize real path, fail on symlink alias collision, and release
on Close. `Open` acquires it immediately. `New` construction and legacy Read/missing
legacy reads remain filesystem-free; first authoritative C1 use may lazily acquire
and recover this same Store. An unowned reader cannot bypass another owner's lock.
When Garden's trusted binding resolver is configured, construction uses this same
`actmem.Open` owner and passes its frozen canonical Root to the callback before
state/ingestion startup; nil resolver keeps legacy New behavior. Partial Open errors
and Close release that owner. Use the existing dependency-compatible `gofrs/flock`
implementation where available, adding an explicit Laputa module pin if necessary, not a PID-file lock.
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

The same backend additionally exposes
`CaptureSubjectHistory(context.Context, SubjectHistoryBinding) (uint64, error)`.
It atomically reads the committed sequence head once. `ListSubjectInteractions`
arguments are `(binding, after, capturedThrough, limit)` and every page uses that
same immutable `capturedThrough`. Zero is a captured empty prefix, never a request
to follow the moving head; no second coverage cursor is introduced.
`GetSubjectSource(ctx, sessionID)` and `GetRunSubjectSource(ctx, runID)` return the
immutable admission snapshots for authorization comparison and observer routing.
`SubjectSourceStore` groups these metadata methods with the unchanged four C2
signatures. Its `SubjectSourceBinding` contains the subject/domain, Session,
conversation identity (empty for ordinary contribution), schema version,
PolicyID/ToolPolicyRef, personal scope and the actual grant snapshot above.

Revision 3 adds a complete disposition capture to this SAME `SubjectSourceStore`;
the original four signatures, `CaptureSubjectHistory`, and terminal sequence meaning
remain unchanged:

```go
CaptureSubjectContext(context.Context, SubjectHistoryBinding) (SubjectContextSnapshot, error)
type SubjectContextSnapshot struct {
    Through uint64 // existing terminal-interaction ceiling
    Dispositions []SubjectDisposition
    DispositionDigest string
}
type SubjectDisposition struct {
    SessionID domain.SessionID
    MarkerID uint64 // existing SQL row identity, never a progress cursor
    Marker SessionTruncation
    Targets []SubjectDispositionTarget
}
type SubjectDispositionTarget struct {
    InteractionID string
    Sequence uint64
    SourceSessionID domain.SessionID // canonical original's session
    RunID domain.RunID
    SourceDigest string
    MessageIDs []string // affected canonical messages; no copied bodies
    ViewMessageIDs []string // same-length exact mapping in the marker's Session
}
```

Capture reads the ceiling, persisted eligible source/run pins, rewind/edit markers
affecting indexed originals through that ceiling, and exact target identities in
ONE consistent SQL snapshot. SQLite and Postgres must each qualify their isolation;
multiple Postgres READ COMMITTED queries are not a coherent capture. The complete
authorized domain includes archived ordinary A while main reader B has another
Session. Admission-edit markers count even while their replacement run is unsealed.
An active/unpinned/NoContext/private run does not thereby become an eligible target.

Resolve closed cutoff/tail ranges against durable message order and exact fork-copy
metadata. Missing anchors or inconsistent canonical target mapping fail closed;
do not inherit `ApplySessionTruncations`' stale-anchor fail-open behavior. Fork
provenance markers never supersede. A rewind/edit belongs to its marker Session's
view. `ViewMessageIDs[i]` identifies that view's affected row and `MessageIDs[i]`
its exact canonical original. Rewinding an inherited copy supersedes its use in
the copied view only; it cannot globally retract the parent's historical occurrence
or change the parent's disposition. Authorize both view source and canonical source
pins before returning this mapping. A private/unbound fork cannot invalidate an
authorized parent. No content matching, duplicate experience, or parent mutation
is inferred from copy provenance. Union semantics apply within the scoped view.
The digest is
versioned canonical serialization of binding, marker identities/exact fields,
target interaction identities/digests and affected message IDs. Stable sorting
defines serialization only, not chronology. Neither CreatedAt, per-run Journal Seq,
nor MAX(marker.id) proves cross-session order or committed-prefix visibility. The
ceiling is not itself hashed unless its change changes the affected target set.

The Service wrapper `CaptureSubjectContext(ctx, readerSessionID)` first calls
`AuthorizeSubjectHistory`, then compares participating source/run pins with the
current trusted Host resolver, including archived sources, before returning the
authorized snapshot or expanding bodies. Denial/error blocks the domain capture;
do not omit a denied source and claim a complete fingerprint. A later recapture
compares the whole digest and grants. No snapshot/acknowledgement is persisted in
SQL. Use `ReadSubjectOriginal` for immutable originals, but derive disposition from
this capture's target IDs instead of mixing its newer mutable marker fields.

Before capture, close the separate pending-repair gap on the same core store:
enumerate terminal unsealed run/session coordinates across the authorized domain,
including archived and pinned-main sources, through a minimal metadata query.
Under the existing projection lock, repair all selected terminal projections before
attempting their idempotent seals; the existing prefix guard rejects advancement
while terminal evidence is incomplete. Reauthorize every source/run pin. Propagate
failure before provider invocation; do not skip incomplete sources or use wall-clock
order. Preserve actual durable seal order. Per-session `RepairSubjectHistory(B)`
alone cannot discover archived A. Repair/capture remain separate phases; no SQL
coverage cursor, second body store, or moving-head fallback is introduced.

Until S2 Task2 connects lossless tool/steering/interrupted-delta evidence repair,
Task1 returns `ErrSubjectEvidenceIncomplete` for those runs and retains their
originals without allocating sequence. Text-only interactions seal only after
terminal/status/canonical-ID checks, admitted user identity, deterministic
assistant projection and Journal completion-hash validation, and a digest of all
exact message/attachment/file-snapshot fields. Idempotent seal and original reads
revalidate the evidence/digest. Outcome remains the actual terminal event.
Direct shell uses a separate non-atomic admission today: Task1 rejects an eligible
shell-first/main source before allocation or execution with explicit unavailable.
Task2 must integrate the same subject pins/protection in a shared admission
transaction, retain exact shell tool evidence, and preserve `NoContext` exclusion.
This temporary fail-closed boundary is not an approved final LIFE tool restriction.


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
and unfinished interactions cannot disappear through hard deletion. Authorized
internal source retirement or ordinary-source list removal archives the underlying
record; no LIFE session delete/archive control is exposed. The singleton remains
permanently addressable. Low-level hard deletion rejects retained sources
even if called outside UI. Recovery and exact source reads include archived sessions
under the same grant. Ordinary-session legacy deletion remains unchanged for unprotected sources. Physical source or
Recap-history deletion is excluded; a backend tombstone never claims that deletion.

### C3: admission, compaction and activation

Runtime resolves policy and binding before any persona/Recap/history preparation.
Child/worker paths are denied by default including inherited hooks, tools and
instructions; an explicit scoped grant permits only its named source read, not
automatic resident Recap/persona. Ordinary-session Coding policy continues its existing pipeline; LIFE may still
execute coding tools under its own admitted policy. Other subject-owned sessions
retain existing Persona behavior; only delegated workers lack implicit persona access.

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
authorize reader; repair domain terminal projections then seal
capture complete disposition snapshot and its interaction ceiling; load owner head
assemble all uncovered complete interactions + current interaction from that capture
if covered targets changed or head certificate reset: reconcile before admission
if pressure: select oldest complete prefix; summarize(head.text, exact prefix)
validate free text, output budget and unchanged scope
commit owner(base revision, operation ID, contiguous range+digest)
  conflict -> reread head, rebuild remaining range, regenerate (at most one fresh retry)
  unknown -> retain exact pending request; Close/reopen same owner and recover
             durably lookup same operation; busy/unknown/unavailable -> pause
  failure before commit attempt -> retain old head and every source; no offload
  error/cancel after Finalize begins -> resolve operation, reload committed head
reload committed head; assemble its remaining exact suffix + current once
admit only after final capacity, permission and clear-epoch checks
```

For that flow, capture the C2 ceiling and disposition set together after repair;
all suffix pages use its immutable ceiling. If the head's certificate is absent,
old or different, text is nonempty, and any target sequence is at/below coverage,
reconcile before the ordinary provider call regardless of pressure. Do not guess
from keywords that an affected covered claim is absent. Explicit compact follows
this rule even with zero new completed interactions. Empty set needs no rewrite;
empty head with no new range remains empty without model-driven historical replay.

The same native `summarization.New` adapter handles correction and normal folds.
`GenModelInput` supplies exact head, typed disposition metadata and only the exact
affected originals needed to identify withdrawn claims. Covered originals are
correction references, never newly folded experience or grounds to advance coverage.
Keep historical occurrence/actual terminal outcome; mark the claim no longer current
in the affected source view without inventing an opposite or replacement. Copied-view
supersession must not remove a still-valid parent-view assertion from Recap merely
because both refer to one canonical original. Only new admitted evidence supplies
a replacement. A nonempty contiguous fold may incorporate correction and commit
`summary`; a correction with no new range commits `reconcile` in custom Finalize.
No independent summarizer, fake user turn or fake tool result is allowed.

Recheck grants and recapture disposition digest before Finalize. A change discards
the uncommitted candidate and consumes the same bounded fresh retry budget as CAS
conflict. Owner CAS binds it to the original base revision. No inference runs under
owner locks; a later SQL mutation creates a newer snapshot, not a cross-database
transaction. Recapture before admission after any commit: reconcile the newer set
or pause, never send an unqualified stale head. The admission cut pins head, exact
suffix, disposition snapshot and epoch together. Uncovered superseded originals
carry delimited non-authoritative disposition annotations; count all correction and
annotation text/references in both actual request budgets. If exact correction cannot
fit, pause explicitly; never replay the full covered transcript or silently trim.

Keep the exact pending operation, canonical receipt envelope, captured markers and
correction references through uncertain commit/Callback/after-summary failures.
Resolve the same operation under C1 recovery before retrying; a confirmed receipt
and the current head are distinct if a later edit/clear intervened. Manual edit or
restore invalidates the certificate, so reread/regenerate once after CAS conflict.
Clear wins by epoch/revision and cannot cause reconstruction from covered references.
Repeated unchanged explicit compact is a no-op. After a crash, complete capture
rediscovers committed markers even on archived A without an observer queue/cursor.

S3 Task 1 cannot expose a working resident provider path while mandatory
reconciliation is deferred to Task 2. Until the shared adapter is integrated and
reviewed, a required reconciliation returns explicit unavailable before provider
invocation. Tasks 1 and 2 form one activation/acceptance cut for covered supersession.

One retry is a cost bound, not a forced success. Further conflict pauses explicitly.
Eino can fail in Callback or after-summary event emission after Finalize committed.
Track commit-attempt/receipt outside mutable Eino state and resolve the same
OperationID after any such error. A committed head remains committed; rebuild from
its cursor before retry/resume. Do not claim the old head survived a post-commit
error. The receipt is operation metadata, not another Recap authority.
Inference is outside owner locks. A confirmed durable commit (returned or resolved
by OperationID) is the only retirement boundary. No cross-database transaction is needed: evidence precedes
owner CAS, and context retirement is derived from the committed cursor on reload.
The index is not updated with a duplicate cursor after owner commit. S3 retains
the exact pending OperationID, request, source range and all evidence until the
owner returns a confirmed positive receipt, including after Callback/Finalize errors.
It cannot acknowledge from visible text, an ordinary read of advanced coverage,
revision zero, or bare missing-head/not-found observations. Unsupported negative
resolution pauses; revision zero never permits evidence retirement or discarding a
pending request. Initial base-zero publication/exact retry may yield the first
positive receipt. Recovery uses the same owner lifecycle, never a second Store to
bypass uncertainty or a busy lock.

Ordinary edit/restore/other-session consolidation affects the next run admission.
Source rewind/edit after the admitted cut also takes effect at the next admission;
do not splice its new marker set into an already-admitted pinned head/suffix.
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

History must render the owner cause `reconcile` distinctly and preserve its exact
text and unchanged coverage/epoch. Edit/restore/clear reset the host certificate;
the next S3 admission performs required reconciliation. This adds no public mutate
action or caller-supplied receipt/cause/schema field: management actions retain
their fixed edit/clear/restore causes, and only the trusted runtime emits reconcile.

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
| Entry and scope | Memory UI manages the actual subject Recap. No LIFE enable/convert control or Coding/LIFE selector. The pinned main-conversation sidebar entry is owned by #66. |

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

S6 uses Chinese experience A then continuation B in the same visible pinned
conversation after multiple folds/restart. A/B may be distinct underlying authorized
source/session segments in storage-level tests, not UI-created LIFE sessions: corrected dates/decisions, a promise, unfinished work and verified tool result
must survive without invented facts. Collect exact provider request bodies in a
controlled test environment plus request digest/Recap revision/source range. Evaluate
quality, omissions and model token/call costs; no simulated model proves real quality.

Test concurrent summaries, edit, clear, cancellation, fail-before-commit, commit with
lost response, reopen, Windows replacement/lock, index lag, archive removal and exact
source navigation; test ordinary Coding-policy isolation, LIFE coding-tool use,
worker and unauthorized-domain negative cases. UI and
headless must operate on the same head. DIVA/Mentle remains pending until its actual
selected installed composition passes, even if BML passes. One candidate commit and
exact dependency/artifact identity must own all claimed gates; a fix invalidates
affected evidence and requires a rerun on the new candidate.

D1 is withdrawn, not awaiting approval: the owner-confirmed singleton LIFE shape
replaces it. The remaining external integration dependency is #66's trusted main-
conversation binding/policy hook; #58 does not implement its full sidebar UI. Overall
package approval and engineering/runtime evidence remain pending. The current
environment lacks Go/just/PowerShell. No Story is Ready for development this turn.

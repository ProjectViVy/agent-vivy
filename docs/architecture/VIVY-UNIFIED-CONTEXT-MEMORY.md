# Unified context pipeline: Recap-only ACTMEM and editable memory UI

Status: owner-decided requirements, documentation alignment only (2026-10-11).
Implementation, tuning and product acceptance remain open. Owning proposals:
[VIVY #58](https://github.com/ProjectViVy/agent-vivy/issues/58) and
[session and worker boundaries #57](https://github.com/ProjectViVy/agent-vivy/issues/57).
This contract refines the [memory profile](VIVY-MEMORY-PROFILE.md) and
[Host contracts](VIVY-MEMORY-HOST-CONTRACTS.md); it does not replace the backend abstraction.

## Scope and owners

This is a foundational VIVY capability in the existing unified context pipeline.
LIFE is the first default-enabled pilot. Coding-agent compositions and coding tasks
are excluded from phase one. Use the existing Service.Run, Journal, policy,
ContextHost and runtime admission paths; do not add a LIFE-only pipeline, second
runtime, compaction engine, memory authority or database.

BML, Mentle and other selected providers own concrete storage, indexing, search,
CRUD, retrieval and returned content. VIVY owns authorized query scope, admission,
placement and model-input budget. Keep one capability-aware abstract interface;
unsupported capabilities fail explicitly. BML is the first validation backend.
The owner will validate Mentle through DIVA's Garden/Mentle composition. This
reconciliation authorizes no backend swap, dual write, data migration or deployment.

## Subject scope and concurrent revision

All persistent sessions of the same VIVY subject share **one subject-level Recap**,
not independent per-session summaries. Session identity still scopes raw history,
continuation positions and permissions; sharing a subject does not bypass tenant,
workspace or authorized-source checks. The trusted host binds the subject.

Delegated role-only workers retain only their assigned task context and explicitly
authorized materials/tools. They receive no Laputa persona or unrestricted shared
Recap/history access by default. Inspect inherited instructions, memory hooks and
permissions as well as parent-chat inclusion. VIVY decides whether worker results
become its experience. A single foreground focus is not a global execution mutex.

All writers, including humans and consolidation from different subject sessions,
operate against the same committed Recap revision. An update names its base revision
and covered source segment/continuation coordinates. Commit atomically checks that
revision and records the new revision and coverage. A stale writer conflicts; it
must reload and deliberately recompute/reconcile, never overwrite a human edit,
apply the same segment twice or advance a handoff that was not committed. Concurrent
sessions must preserve both sets of commitments. Exact scheduling, serialization,
conflict UX and storage representation remain engineering decisions.

## Active-memory lifecycle

ACTMEM is one free-form, AI-organized **Recap**. Pulse/Recap/Work is the earlier
implemented shape, not the selected target. The dedicated memory UI must expose the
entire actual Recap, its revision and provenance, and support inspect, edit and delete;
reviewed interaction flows/wireframes are a required design deliverable.
A hidden parallel summary or partial view is insufficient. Headless operations use
the same governed API, with the UI as a consumer.

Persist original raw evidence continuously through its existing legitimate owners,
rather than waiting until a context threshold. Retain it after context retirement. Consolidate
previous committed Recap plus a retiring complete raw interaction segment into the
next Recap. Only after that revision and its continuation position commit may the
old raw segment unload from model context. On model/save/conflict/cancellation
failure keep evidence and the previous handoff; do not advance or discard input.
Restart restores committed revision/coverage without repeating or skipping a fold.
Raw evidence retention is a requirement of the new policy, not a claim that every
current backend already captures full transcripts. The default BML observer writes
a successful-completion projection, not the entire conversation.

A finished interaction whose tool effects can never be resolved (an operation left
claimed by a cancel, crash or unretainable result, or tool arguments that are not
valid UTF-8) must not stop the subject's history. Core Storage seals it as an
`excluded` interaction in the next sequence slot: none of its originals become
Recap input, coverage advances past it, and the unresolved operation keeps its
recorded state. Storage re-derives the condition in the sealing transaction, so a
healthy or merely unprojected run can never be excluded. This is exclusion from
Recap, not proof that no external effect happened.

Recent raw-window and Recap budgets, trigger thresholds, retained tail, hysteresis,
topic boundaries and token placement are experimental. No example percentage or
2k–4k token budget is an agreed default or acceptance threshold. Oversized output
needs an explicit failure/review outcome rather than silent truncation. Record model
inputs to prove admission, rather than accepting a saved ACTMEM document alone.

## Explicit management and deletion

Require agent tools to inspect/read, update and delete ACTMEM through authorized,
revision-aware operations. Normally hide/defer these tools and activate them
explicitly when needed. Exact discovery/activation mechanics remain an engineering
choice; do not require always-on schemas or autonomous maintenance. Current model
ACTMEM read tools are absent despite human control actions; this is planned work,
not delivered by the existing memory_* backend CRUD tools.

UI ACTMEM deletion explicitly clears the current active Recap as a new shared
revision and invalidates derived active-memory projections for subsequent admission.
Its scope is active memory for that subject, not raw Journal, retained Recap history,
archived conversations, backend records/indexes or persona authority. In-flight and
resumable admitted snapshots need an explicit safe-boundary/conflict policy; do not
claim their bytes were retrospectively wiped. Stale consolidation cannot resurrect
the deleted revision by committing against its old base. A future authorized fold
or recall may use retained evidence: forgetting active text is not permanent erasure
or a promise that the content can never be learned again.

Agents may use the selected backend's authorized delete tools for explicitly
identified backend resources. Each operation must report its target and confirmed
outcome; unsupported/failed/partial outcomes are visible. Never infer cascading
cross-store erasure from ACTMEM deletion or a backend tombstone. Further source
retention, revision-history erasure and prevention of reintroduction need separately
specified, authorized operations, not silent extra deletion.

## Frozen layer and authority

When a human marks material “must not be forgotten”, guide them to Laputa's existing
frozen layer / Frozen Core and legitimate human-controlled workflow for eligible
content. Ordinary Recap edits are mutable continuity and do not pin content forever.
Frozen Core is a bounded session-frozen projection of governed authority revisions,
not an immutable copy of every remembered fact. Approved changes take effect at the
existing lifecycle boundary; an admitted session/run keeps its pinned projection.
The newer cognitive contract includes Mission alongside governed persona slots;
older six-file ADR descriptions are historical baseline terminology. Do not invent
a new authority, slot or automatic pinning mechanism. Summaries/agents cannot write
Mission, Dream or frozen persona slots automatically. WORLD remains tool-only and
is not authorized for automatic bootstrap/recall/ContextView injection.

## Compatibility and verification work still open

Existing Work/reconcile/capture consumers need reviewed compatibility planning
before selecting the new policy. Keep their runtime and historical tests intact;
this change authorizes no code deletion or migration. The old absolute ACTMEM
exclusion applies to the earlier policy, and is superseded for the selected #58
pipeline; WORLD exclusion and #57 permission boundaries remain mandatory.

- [ ] Validate BML and the owner-led DIVA/Mentle path through the same capability-aware interface.
- [ ] Record bounded Recap + recent raw model inputs over repeated Chinese LIFE rotations; verify decisions, corrections and unfinished commitments without invented facts.
- [ ] Exercise two subject sessions plus concurrent human edit/delete, stale revision conflicts, crash before/after commit, failed save and retained evidence/continuation position.
- [ ] Verify worker denial and explicitly granted access; coding phase-one exclusion and WORLD non-injection.
- [ ] Exercise full UI inspect/edit/delete and headless equivalent, tool activation and honest targeted backend deletion outcomes.
- [ ] Verify source/time provenance, obsolete facts and raw recovery, summary quality and call/token costs; define oversized output handling and compatibility consumers.
- [ ] Complete formal installed-product, Windows/real-model, unknown/partial recovery and workspace matrices. Existing developer note recall/correction/deletion and crash-cut proofs remain valid but do not close these gates or this new policy.

Before implementation, perform the mandatory pinned Eino/EinoExt capability check
within the existing import quarantine. This reconciliation adds no code and selects
no new orchestration primitive; it does not claim that Eino alone supplies subject
permissions, Recap CAS, evidence retention or the editable UI.

## Evidence and remaining decisions

Refreshed VIVY main: `ce33bc8776b9f52ab13bed5f2cdaf8ebad01f666`.
[Laputa PR #5](https://github.com/ProjectViVy/laputa/pull/5),
[VIVY PR #46](https://github.com/ProjectViVy/agent-vivy/pull/46) and
[PR #48](https://github.com/ProjectViVy/agent-vivy/pull/48) are merged.
[Exact-baseline CI run 38095646661](https://github.com/ProjectViVy/agent-vivy/actions/runs/38095646661)
passed backend ci, ui ci, full UI browser smoke and just ci, including the full
supported DIVA memory selection. These verify the existing baseline, not #58.
BML first-initialization cancellation is fixed by PR #48: bounded WithoutCancel in
bml/store.go and deterministic store_initialization_test.go. Older red runs and
pass/fail logs stay unchanged.

Unresolved: experimental budgets/placement; subject-to-backend scope mapping across
workspaces; source-segment coverage ordering and conflict UX; activation mechanics;
active deletion versus already admitted/resumable snapshots and retained revision
history; compatibility for Work/reconcile/capture; exact UI flows and installed
validation matrices. None authorizes a second authority or backend replacement.

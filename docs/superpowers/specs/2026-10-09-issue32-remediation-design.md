# Issue #32 remediation work-package design

Status: reviewable design, 2026-10-09 (Asia/Shanghai). Product implementation
has not started. The owner requested work-package design and a detailed,
persisted plan for every phase. This request authorizes the documentation
package; implementation starts after its review. Phase state and finding
coverage are owned by the [plan index](../plans/issue32-remediation/index.md).

## Purpose and source authority

Repair the concrete authorization, data-integrity, lifecycle, workflow and
release-gating defects in [VIVY #32](https://github.com/ProjectViVy/agent-vivy/issues/32).
Successful remediation preserves one Service/Journal/policy path, makes the
negative paths observable, and binds final acceptance to the delivered bytes.
The tracker has P0 scheduling priority; finding severities remain 5 P1,
21 P2 and 2 P3. This is not an assertion of a production incident.

The original audit anchors are VIVY
`b4db00bdaa47dba58e03ba41d8906b9cf10503d4` and DIVA
`d81381f8f992f9b201144619673bbfcd7b07265c`. This design was rechecked against:

| Source | Reviewed identity |
| --- | --- |
| VIVY main | `017ec8cc37970b291e04c619990aed00d5403116` |
| DIVA main | `518a33ef09858ee1bb190579dd7529aceaa15dd6` |
| VIVY Laputa source lock | `ff3936f44ff8cf08c12af2cf698c194cfe474fd3` |
| DIVA VIVY source lock | `3073d1e126bbe2d9579879c79869f363214938b1` |
| DIVA Laputa source lock | `6f2eed2d71c82261333e50883e98b404070a6801` |
| INOFY | `v0.0.0-20260930141905-71e2c9bbe47d` |
| Eino / OpenAI EinoExt | `v0.9.13` / `v0.1.13` |
| VIVY Go directive / DIVA locked toolchain | `1.26.4` / `1.26.4` |
| DIVA Wails | `v3.0.0-beta.27` |

Local preanalysis probes ran at VIVY `83e46efc6c6d56a6a07ba7dbf60fae9617e6268e`;
the relevant runtime/logging/app/host source matches reviewed main exactly.
Their four defects and six leaf scenarios reproduce C1, R1, R3 and R4. They
are disposable defect probes, not fixed-behavior regressions or release evidence.

R2 is **superseded**, not implemented as originally requested. The owner's
[issue #40](https://github.com/ProjectViVy/agent-vivy/issues/40) decision and
merged [PR #42](https://github.com/ProjectViVy/agent-vivy/pull/42) remove built-in
automatic redaction. Record that disposition explicitly. Do not reintroduce
recursive redaction through R3 resource clipping.

## Chosen approach and global constraints

Use contract-sized repairs in eight phases, with independently reviewable
tasks and one integration lane. Per-finding patches offer smaller diffs but
leave shared state and lifecycle decisions easy to contradict. An overall
runtime rewrite costs more and has no demonstrated need. Contract grouping
keeps the existing seams while fixing the whole affected path.

- Preserve one Service/Journal/policy path; no second runtime, scheduler,
  Journal, policy owner, model loop or persistence service.
- Retain `internal/embedded` and `sdk/host/v1`; they implement the active Go host.
- Only `internal/runtime` and `internal/provider` import Eino. Keep Eino at
  `v0.9.13`; preserve reviewed VIVY dependency pins and DIVA's Wails pin.
  DIVA consumer pins advance only to the verified final VIVY source and its
  required reviewed Laputa lock, not to arbitrary upstream versions.
- Core Storage owns DDL. No schema migration is expected for the selected
  repairs. Any demonstrated migration need requires paired, append-only
  SQLite/PostgreSQL migrations and upgrade/reopen/failure conformance.
- Module changes follow Module -> typed Port -> Provider/Consumer -> Recipe
  -> generated Assembly -> Generation evidence. Never hand-edit generated wiring.
- Backend capability is the sole source for displayed executable readiness.
- Preserve faithful task/error/log content within explicit resource bounds;
  no built-in automatic redaction or generic argument veto.
- Never inspect or operate on tenant `data/vivy.db`, `data/demo/` or
  `data/workspaces/`. Fault tests use fresh temporary stores and synthetic errors.
- Plans and durable records are English. Conversation remains Chinese.
- Shared root has one write lane. Parallel product edits require separate
  worktrees; overlapping files are sequenced in one lane.
- Product `just ci`, relevant storage conformance, native gates and actual
  selected-product smoke are completion requirements for implemented changes.
  This documentation-only delivery needs document checks, not product CI.
- Existing human Git attribution is retained. No AI attribution, direct main
  push, release publication or transition deletion is part of this design delivery.

## Eino capability check

Inspected pinned module source: `components/model/interface.go` exposes
Generate/Stream errors; `internal/callbacks/interface.go` returns context from
OnEnd, not an error; `schema.StreamReader.Copy` and
`compose.genericOnEndWithStreamOutput` create sibling stream readers. Callbacks
cannot put mandatory Journal failure on the producer's terminal path.
The existing `observeGenerate`/`observeStream` wrapper is the required adapter
for R4. Preserve its bounded pipe and governed child/model route.

The W9 proof uses the real `compose.NewWorkflow` API. Its removal is removal
of unused executable proof code, not replacement of an active workflow engine.
Cognitive state CAS, window ownership, authority and restart reconciliation
belong to VIVY; Eino does not own the host's SnapshotStore or operation identity.
Recheck these APIs against the final unchanged pins before implementation.

## P0 baseline and traceability

Freeze the planning facts, finding disposition and lane ownership before code.
The index lists all 28 findings exactly once: 27 planned repairs, R2 superseded.
Use one row in `docs/TODO.md` to link the package, not 27 duplicate status boards.
Use `docs/DEFER.MD` for R2's disposition. Each implemented task adds evidence to
its phase row and the existing authoritative iteration records; unchecked
steps and successful defect probes never mean a finding is fixed.

At execution start refresh both remote heads and source locks. A changed source
requires affected call-chain rechecks and updated baseline records; do not
silently carry this plan's line numbers forward. Lack of PostgreSQL, native
Windows or voice capability is an unexercised gate, not a pass.

## P1 desktop authorization and release gates

H1 uses one main-window capability check before VivyCall, DesktopDispatch and
MediaToken. Missing, foreign and revoked identities fail before host dispatch;
valid main-window calls remain usable. Native window capability complements
Wails asset/navigation origin restrictions and does not create a browser grant.

H2 makes active Go host, speech, binding, Go module, source-lock, packaging,
script and workflow changes trigger CI. Aggregate `just ci` includes canonical
Go-host checks, generated-binding drift, frontend boundary and packaging/seal
checks. Linux and Windows exercise the actual consumer modfile. Transition
Rust checks remain until the existing W6 conditions permit their replacement.

Fresh-checkout prerequisite: the reviewed DIVA tree tracks package-lock.json
but not agent-diva-gui/pnpm-lock.yaml, although its build wrapper unconditionally
reads the latter and uses frozen pnpm install. Generate/track that required lock
from existing package-lock resolutions with packageManager-pinned pnpm10.33.2,
preserve dependency versions, and prove frozen installation before canonical
behavioral tests. This belongs to H2's executable gate, not a separate feature.

The sole source-pin owner remains DIVA `build/vivy-sources.lock.json`; explicit
native target declarations live there. Derive target input locks deterministically
into scratch/artifact directories, retain both canonical and derived bytes and
validate their relationship. Never maintain separate handwritten platform pin
copies or compare Windows's actual build to a Linux-only tool declaration.

H3 introduces a strict candidate-evidence contract: required scenario inventory,
exact full source/pin identities, clean source trees, consumer module/sum/lock
hashes, tools/platform, Generation and frontend identity, and delivered artifact
checksums. Missing or duplicate cases, unknown cases, pending/failed results,
stale sources, altered assets and artifact mismatch all reject promotion.
Historical v1 fixtures remain historical; they cannot authorize a v2 candidate.

Build final signed/package bytes once from a clean frozen source. Store their
acceptance evidence outside that source snapshot, since committing it into the
candidate changes the source identity. Promote the same bytes after every
required platform passes and the tag peels to the accepted source. No rebuild
or per-platform public upload before the aggregate gate. Engineering completion
of H3 means rejection behavior is proven; actual W5 product acceptance is P7.

## P2 cognitive state and recovery

### State ownership (C1)

Every state write carries the SnapshotStore version from the original load.
`saveCognitiveState(ctx, st, expectedVersion)` never queries a new version.
Serialize pure load/mutate/put operations with a Service-owned state mutex,
separate from lifecycle `cogMu`; serialize manual/automatic admission with a
separate attempt mutex. Never hold the state mutex while resolving authority,
starting/cancelling a workflow, waiting for shutdown or applying effects.

Pure mutations reapply against a fresh state and preserve concurrent captures.
Admission uses expected-version commits; a conflict before launch reevaluates,
while a conflict after possible launch enters reconciliation of the saved intent.
Never retry an entire effectful admission as a state-CAS retry.

### Durable intent (C2)

Add an optional intent to the existing JSON snapshot, containing original
parent Run ID, operation key, strategy ID, immutable canonical input and attempt.
The input includes the exact SourceID, Window.After/Through and binding/policy
pins. Persist intent and PendingThrough before StartCognitiveWorkflow. Look up
the original `(parentRunID, operationKey)` before replacing a terminal supervisor.
An intent that has not admitted a run can only launch under its persisted parent;
if that parent is no longer admissible, fence it until a deliberate new decision.

| Reconciled outcome | Required transition |
| --- | --- |
| Active / accepted | adopt the exact Run; never launch another |
| Completed | settle the persisted window, advance Watermark to its Through |
| Failed with proven retry-safe effects | preserve the window; use the next bounded attempt |
| Cancelled | retain cancellation fence; no automatic retry |
| Unknown, missing evidence, changed authority or ambiguous lookup | retain unknown-outcome fence; no automatic replay |

Inputs arriving during a run remain above its Through for the next window.
Retry safety requires existing workflow/node/effect evidence and complete
inference-child model settlement/finish evidence. A failed evidence lookup or
missing mandatory finish record is unknown, even if the Run row says failed.
Legacy snapshots with ActiveRunID use the existing durable run/revision evidence.
Add snapshot StateSchema=2; absent StateSchema means legacy, and unknown future
values fail closed. An empty new store starts schema2 without a fence. During
the one-time legacy upgrade, an outstanding input window with no intent and no
proof of a retry-safe outcome is fenced unknown_outcome. Idle legacy state with
no outstanding input can upgrade normally. Legacy snapshots cannot reconstruct
an unrecorded old crash window by invention: ambiguous historical effects are
not replayed or counted as completed. Snapshot JSON additions require reopen
and legacy-decoding tests; they do not require a second table.

### Binding, policy and control (C3-C6)

Resolve the next run's current Mission binding, validate that resolved revision
against current authority, and persist it. A later authority change rejects
execution under the old run pin; do not repin an admitted run. Scope,
destination and strategy identity remain fixed by composition.

Change the core Bundle resolver to accept the immutable policy value read by
the runtime: `ResolveBinding(ctx, policy)`. Hash that value for the run pin.
Bundle Policy supplies only an immutable first-load seed. Remove mutable bundle
policy and post-write `setPolicy`; reopening derives the same pin from durable
state without maintaining a synchronized second owner.

Expose `CognitiveControlState(ctx)` from one snapshot version, populating
PendingThrough, Phase, BlockReason, policy and active Run together. Phase values
are `idle`, `admitting`, `running`, `completed`, `failed`, `cancelled`, `blocked`.
The existing core ControlState fields are projected verbatim through dispatch
and DIVA. Cancellation accepts only the current cognitive strategy Run under
the bound supervisor/scope. A foreground or stale Run ID causes zero cancellation.

## P3 workflow product contract

Browser/RPC draft creation sends `create: true` with no ETag and maps directly
to `WorkflowDefinitionETagAbsent`; editing carries a concrete current ETag.
Reject omitted/empty edit tokens and ambiguous create-plus-token requests.
The current pre-read conversion already protects some
absent writes and foreign authors. Repair the remaining same-author creator
collision without weakening authorization. Concurrent PostgreSQL inserts map
to the same conflict contract as SQLite. Generate a fresh opaque UUID ETag on
each accepted write, retain the true UpdatedAt timestamp and exact legacy token
comparison, and invalidate a legacy token on the first repaired write.

Reject duplicate child tool names in schema and publication validation using
the same canonical rules as child execution. Do not silently normalize immutable
published definitions or rewrite their digests. Historical malformed definitions
remain readable and fail explicitly if started.

A UI start intent owns one operation ID and one complete immutable request,
including session/parent, definition/revision, input and saved draft ETag. Apply
a successful save reply immediately. Ambiguous start retry resends the identical
request without saving again; changed input is a new intent. If another editor
has changed the draft, the current backend can reject the old ETag: surface that
safe conflict, retain the ambiguous intent and never mint a replacement start.

Run cursors encode `(created_at, run_id)` and compare the same DESC/ASC ordering.
Reject obsolete timestamp-only tokens with an explicit refresh requirement.
Definition cursors accept existing colon-containing IDs by parsing the final
revision delimiter and strictly rejecting malformed suffixes. Both storage
drivers share semantics; no existing identifier is invalidated.

UI continuation preserves filters, rejects late replies from superseded queries,
deduplicates rows and handles an empty-string exhausted cursor. Native completed/
failed/cancelled are terminal; engine status is a separate projection, including
recovery_required. No resume control is exposed while supports_resume is false.
Tests run from authoritative module source and are included exactly once;
generated UI staging is not hand-edited. Source hashes and generation evidence
are regenerated with repository tools after the source changes.

## P4 host lifecycle and provider capabilities

H4 passes the owned logger into App before composition via a typed AppOption.
Global logger compatibility, where required, is scoped to the actual host owner
and cannot restore an older logger over a newer owner. Closing a daily sink
permanently closes it; late writes cannot reopen files or write into another
profile. C7 registers cognitive bundle cleanup immediately after opening it and
transfers ownership only on successful App construction. Cleanup errors are
joined with the original initialization error without duplicate close.

H5 begins one 5-second shutdown budget before pump join. Pump, speech and host
teardown consume the remaining deadline; an earlier caller deadline wins.
Repeated/concurrent shutdown observes one coordinator/outcome. Join errors,
including speech and pump failures, rather than replacing them with a later nil.
Deadline expiry returns promptly while unfinished teardown retains runtime
ownership until actually complete. Never claim completed/reopenable resources
when only the caller's wait ended.

H7 preserves the existing process-global singleton scope. Wails beta.27 performs
election in `application.New`, before Run, so election precedes host.Open.
Queue/coalesce reopen during startup and deliver it after the main window is
bound. Register services after successful host startup. Failed startup clears
queued handoff and closes partial host ownership; the command exits to release
the OS election lock. Pre-Run App.Quit is a no-op and is not a cleanup strategy.
Use subprocess restart/handoff tests for actual election behavior.

H6 shares one pending native-listener installation Promise. Concurrent subscribers
get one listener; close-before-resolution detaches it once. Installation failure
is handled, clears the shared attempt and permits a later explicit subscription
retry without unhandled rejections or duplicate callbacks.

C8 consumes backend profiles/endpoint executable truth. Deferred and unknown
providers cannot execute/select/test/refresh models. Supported but unconfigured
providers remain configurable; existing deferred configurations remain readable
and deletable. Preserve backend fail-closed enforcement.

## P5 diagnostics and mandatory model settlement

R1/R3 share a cursor/reader repair. Use file identity from the opened descriptor,
offset, shrink checks and a bounded anchor fingerprint; modification time alone
cannot invalidate normal append. A versioned cursor records whether it is
discarding the remainder of an oversized physical line. Replacement, shrink or
anchor mismatch marks gap; malformed cursors remain invalid. Legacy cursor
tokens may reset once with an explicit gap, never silently miscontinue.

Use a limited physical reader; all inspected file bytes, including anchor bytes,
consume the request scan budget. Keep at most a bounded record prefix in memory,
discard oversize fragments incrementally, and never parse a line's middle as a
new record. A nonoversized unfinished line remains at its starting offset until
completed. An oversized record can expose one explicitly truncated bounded
prefix while its remainder is discarded across requests. Final JSON size,
including envelope fields, escaping and Truncated, is at most 8192 bytes. The
total request scan bound is 4096000 bytes and record count is at most 500.

R4 calls End exactly once and joins its error with provider/chunk errors.
Generate returns a settlement error rather than success; Stream sends that error
before terminal EOF. Closing/cancelling a stream never leaves a blocked terminal
sender. Introduce an error-returning persistence helper inside the existing
Service path so the actual Journal cause survives; keep the existing boolean
wrapper for unrelated callers. Settlement persistence failure is a non-provider,
non-retryable outcome with evidence gap; model retry middleware must not replay
the call. Retain detached bounded terminal persistence and quota exemptions.

## P6 obsolete proof and historical safety

Move executable old orchestration proof, runner and test-only registrations into
`_test.go` source. Keep a minimal legacy-target rejection check in the live
approval path before decision consumption; never fall through to generic agent
resume. Verify old pending approvals, schema-1 rows and opaque checkpoint/history
reads with zero model/tool execution. Retain runtime schema registrations only
if an identified production decoder actually needs them. No historical row,
checkpoint, approval or descriptor rewrite is authorized by this cleanup.

## P7 final integration and release

Merge compatible task branches in one lane. Finalize internal/module hashes and
source-bound SDK conformance only after all VIVY source edits, then pack and
inspect the selected Recipe. Update DIVA's VIVY lock and consumer module evidence
to that verified VIVY commit; derive/check consumer Laputa against its reviewed
`laputa-source.lock.json` rather than assuming DIVA's older pin is compatible.
Use VIVY's reviewed `ff3936f44ff8cf08c12af2cf698c194cfe474fd3` when that closure
requires it, without selecting arbitrary upstream. Canonical DIVA source inputs
do not pin their own containing commit: that would be circular. Record the full
resolved host commit/tree in the generated build report and external acceptance
evidence, and require the final release tag to peel to that exact commit. Run both product aggregate
gates, both storage drivers and real selected UI/native paths on final pins.

Freeze candidate source/tool/lock/Generation/frontend identities, build the
final artifacts once and execute the strengthened P1 acceptance contract against
them. Historical 15-pass/2-pending evidence cannot be inherited across source
changes. `W5-T5-VOICE-REAL` and `W5-WINDOWS-MATRIX` remain pending until actually
exercised on the final candidate. Product promotion remains the owner's separate
action and uses the same accepted bytes.

W6 transition retirement is conditional on the existing approved cutover
contract, replacement CI, consumer inventory and paired annotated
`archive/tauri-cabi-20261004` tags. Required historical peel targets are DIVA
`5444795a2d9db31e158c2cf009d64697e6289e50` and VIVY
`fc559e6b03ce4e65c0099b9745855dccc4fb067e`. Their presence was not established in
preanalysis. Do not automatically delete transitional C ABI/Tauri while gates
remain incomplete. If retirement changes final bytes, rebuild and repeat final
candidate acceptance before promotion.

## Dependency and acceptance rules

P0 precedes all code. P1 and P2 can start as separate repository lanes. P2.1
precedes P2.2; P2.3/P2.4 use the repaired state boundary. P3.1 precedes P3.2;
P3.3 precedes P3.4. P4 shares App/lifecycle files and is sequenced internally.
P5 and P6 touch runtime/Service and must be sequenced with P2 or reconciled in
isolated branches before final evidence generation. P7 follows P1-P6.

Before a phase runs full CI or packs its changed source, refresh its scoped
internal/Module identity and source-bound conformance using P7.0 Steps 3-4.
Otherwise even a correct isolated phase will test stale checked-in evidence.
These phase-local attestations prove only that phase source; P7 regenerates the
final integrated evidence after the last source edit, before consumer repinning
and candidate production.

A finding is engineering-verified only with its negative regression and relevant
real adapter path. A phase additionally requires its stated exit gate and log.
Issue closure additionally requires every finding fixed or explicitly
owner-disposed and final installed-product acceptance. Missing tools or native
targets are recorded as pending, never replaced by mocks or a checklist tick.

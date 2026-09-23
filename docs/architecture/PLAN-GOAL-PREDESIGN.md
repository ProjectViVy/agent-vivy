# Plan / Goal Work Control — Detailed Pre-design

Revision: PG-D1, 2026-09-21. Status: proposed engineering design, not implementation-ready.
Product direction: [Issue #47 architecture addendum](https://github.com/ProjectViVy/agent-vivy/issues/47#issuecomment-5754532762).
Baseline: a8d361b0244a1c40be513622bbdaebb5c9d40014.
Delivery index: [Plan package](../plans/plan-goal/README.md).

The issue remains the product-decision record. This document specifies engineering details beneath it; it does not repeat or supersede that record. Conflicts require a recorded correction before implementation. The user separately authorized publishing these documents directly to main and referencing them in Issue #47; this package does not authorize product implementation or release.

## 1. Requirements and decision

| ID | Required behavior |
| --- | --- |
| R1 | Soft Plan guidance independent of hard policy; preserve legacy hard-Plan resumes |
| R2 | Human-reviewed exact plan submission; ordinary execution or explicit Goal handoff |
| R3 | One current session Goal; revisioned lifecycle and durable evidence |
| R4 | Ordinary Service runs, bounded admitted rounds, human admission priority |
| R5 | Pause/cancel/edit/abnormal stop cannot revive stale work |
| R6 | Restore/fork/rewind do not recreate execution authority or erase spent rounds |
| R7 | Scoped model tools; no self-approval, permission widening or human-pause reversal |
| R8 | Backend-authoritative GUI and future-run discovery; reuse Todo |
| R9 | Atomicity, migration, deterministic conformance and separate live effectiveness evidence |

Choose a cohesive session work-control service inside the runtime boundary, pure domain reducers, and a narrow session journal extension on the existing backend. Reuse the Eino runner and current run startup pipeline. Snapshot-only storage is cheaper initially but cannot satisfy audit and transaction requirements; a permanently active synthetic Goal run damages existing terminal semantics. No new dependency, public Port, scheduler or engine is proposed. One coordinator per process handles active sessions; do not spawn idle per-session goroutines or repeatedly scan the entire run history.

## 2. Verified source and unverified seams

Verified existing paths:
- internal/runtime/service.go: runWithOptions custom runPersistence callback, detached drive, consume, emitTerminal, RunHook, Recover.
- internal/runtime/runmode.go: Plan currently coerces PolicyProfilePlan.
- internal/runtime/tooladapter.go and shell.go: hard non-Readonly Plan checks.
- internal/runtime/engine.go: pinned Eino Runner.Run / Query / ResumeWithParams.
- internal/runtime/prompt.go: static and per-run composition. It still contains string constants at this baseline; do not assume every prompt has already moved to Markdown.
- internal/domain/question.go: text Answer and ResumeTarget; no version-bound structured plan decision.
- internal/storage/contracts.go: per-run Journal and atomic HistoryMutationStore.
- internal/storage/sqlite/history_mutations.go: transactional edit/fork precedent.
- internal/storage/sqlite/sqlite.go: one DB connection, versioned migration list currently through 23.
- internal/storage/postgres/schema.go and history_mutations.go: second first-party backend.
- ui/src/lib/api.ts, store.ts, rpc.ts, run-subscription.ts: existing control path.
- ui/src/components/chat/ChatInput.tsx, ChatView.tsx, TodoProgressStrip.tsx: chat controls.
- ui/AGENTS.md: store.ts owns UI state; generated UI must not be edited.

Go is unavailable in this environment. The selected Eino revision's runtime behavior for mid-run collaboration changes and structured review interruption has not been executed. PG-0 produces the evidence needed to release downstream implementation. Existing capability documentation is background evidence, not a substitute for testing new behavior.

## 3. Ownership and lock ordering

Proposed new runtime files: work_control.go (operations), work_admission.go (serialization), goal_driver.go (continuation), plan_review.go (review adapter).
Proposed pure types: internal/domain/work_control.go.
Proposed storage contract: internal/storage/work_control.go.

Dependencies: RPC and tools -> runtime operations -> domain + storage; driver -> runtime admission -> existing Service; UI -> RPC.
No other layer may directly append work-control events.

A session admission mutex serializes control mutations and startup reservations, not the lifetime of an engine turn. Global Service.projectionMu remains the existing startup/deletion barrier. Lock order: session admission gate, then existing startup/projection lock, then backend transaction. Never take a session gate inside a synchronous publish/RunHook callback. Driver wake-up records work and returns immediately.

Cancellation/draining must release all startup/session locks before waiting. Use a transition token (process-local generation plus durable state version) to detect conflicting resume/edit/delete requests after drain. A failed drain keeps Goal disarmed and Plan ineffective with a visible error; do not show a false completed transition.

All entry points that can start primary work must register at admission: normal RPC, channel, cron, session edit, shell and Goal. Apply the new serialization only to startup coordination; children retain their existing parent ownership. An unrelated cron/automated request is not treated as a human request. No silent dropping or global single-run lock.

## 4. Proposed shared types

The following are design signatures, not compiled declarations. No SDK exposure is implied.

```go
type WorkVersion int64
type GoalRef struct { ID string; Revision int64 }
type WorkExpectation struct { Version WorkVersion }
type EvidenceRef struct {
    RunID RunID
    Seq EventSeq // 0 means run-level reference
}
type GoalState struct {
    Ref GoalRef
    Objective string
    Phase string // active, paused, blocked, completed
    MaxRounds int
    RoundsStarted int
    ReasonCode string
    ReasonText string
    PlanSubmissionID string
}
type PlanState struct {
    Active bool
    SubmissionID string
    ReviewStatus string // none, pending, accepted, rejected, cancelled, expired
    OriginRunID RunID
    OriginToolCallID string
    ResumeTarget string // opaque Eino target, never caller supplied
    BlockedToolCallIDs []string // same-batch siblings not covered by the reviewed Plan
    DecisionAction string
}
type WorkState struct {
    Version WorkVersion
    Goal *GoalState
    Plan PlanState
}
type WorkView struct {
    State WorkState
    Activation string // armed, disarmed
    Transition string // none, stopping_for_plan
    CurrentRunID RunID
}
type PlanSubmission struct {
    ID string
    Markdown string
    OriginRunID RunID
    OriginToolCallID string
}
type PlanDecision struct {
    SubmissionID string
    Action string // revise, execute_once, start_goal
    Feedback string
    Objective string // required for start_goal
    MaxRounds int // required for start_goal
}
```

WorkVersion is a session journal sequence, Goal revision changes only when its definition/lifecycle changes. Round admission changes WorkVersion and the count, not Goal revision. Reports must match both GoalRef and the owning admitted run. A UI may refresh and retry an unrelated version conflict; it must never silently resubmit an objective edit against a changed GoalRef.

Actor data comes from verified RPC session identity or live root-run context. Do not put a caller-selectable actor/isHuman flag in tool JSON.
Control request IDs are caller-stable opaque strings; reuse with different payload returns conflict.

## 5. Storage and transaction contract

Proposed operations, all on first-party backend implementations:

```go
ReadWork(ctx context.Context, sessionID domain.SessionID) (domain.WorkState, error)
ReplayWork(ctx context.Context, sessionID domain.SessionID, after domain.WorkVersion, limit int) ([]domain.WorkEvent, error)
CommitWork(ctx context.Context, mutation WorkMutation) (WorkCommitResult, error)
CommitGoalRun(ctx context.Context, admission GoalRunAdmission) (WorkCommitResult, error)
```

WorkEvent: session ID, seq, event kind, payload version, request ID, timestamp and validated payload.
WorkMutation: session ID, expected WorkVersion, request ID, payload fingerprint, event kind and payload.
GoalRunAdmission: WorkMutation + GoalRef + next round + complete ordinary Message/Run/run.started records constructed by Service.
WorkCommitResult: committed WorkState, assigned WorkEvents, optional existing/admitted RunID and run.started event.
These definitions are shared here; Story workers must not invent incompatible variants.

New SQL table proposal: session_work_events(session_id, seq, kind, payload_version, request_id, request_hash, created_at, payload).
Primary key (session_id, seq), unique (session_id, request_id), foreign key session deletion policy matching first-party storage. No independently writable Goal snapshot table. Initial implementation folds session work events; hot admission may use a version-checked process projection only after measurement justifies it.

Goal round uniqueness is enforced by serialized session transaction + expected seq + sequential round validation. Replay is strict: invalid current-format records stop work access rather than being silently skipped. Reads are paged; configured RPC frame and event payload limits apply. Oversized objectives/plans fail before persistence, using one documented bound resolved in PG-0 rather than a second hard-coded competing limit.

CommitGoalRun:
```text
begin transaction; lock owning session
look up request_id; identical retry -> original result; different hash -> conflict
read/fold work state; require exact expected version/ref
require active Goal, next round=count+1, count<cap
require no conflicting nonterminal primary run
insert user message and ordinary active run
append run.started using existing versioned payload path
append goal.round_admitted including run ID/ref/round
commit
return records; only then publish and start engine
```

Armed is process-local and checked under the admission gate immediately before this transaction. A crash after commit before engine start leaves a charged accepted round: existing recovery closes that run without re-executing it. Never refund/replay automatically. A crash before commit creates neither run nor charge.

Session mutations, exact plan-review outcome and Goal handoff must commit consistently. If QuestionStore is used for review settlement, the backend transaction updates its row and work events together; sequential AnswerQuestion then CommitWork is forbidden.

The first ordinary user run may reference a new session before a session row exists. `CommitPrimaryRun` creates that row in the same transaction as the user message, active run and `run.started`, using the established workspace-write/ask defaults. Existing session settings are left untouched. This matches `sessionSandbox`'s runtime defaults and avoids a split first-run admission.

## 6. State transitions and tool authority

| Request | Preconditions | Durable effect | Live effect |
| --- | --- | --- | --- |
| Create Goal | no unfinished current Goal; Plan inactive; human authorization | active, rev=1, rounds=0 | arm |
| Edit Goal | human; exact ref | objective/rev updated; preserve spent rounds | invalidate candidate |
| Pause | human; exact ref; resumable Goal | paused and reason | disarm, cancel owned run |
| Resume | human; Plan inactive; capacity remains | active and next rev when transition needed | arm once |
| Clear | human; exact ref | tombstone | disarm, cancel/drain owned run |
| Complete report | live admitted current-ref run | completed + evidence | disarm |
| Block report | live admitted current-ref run | blocked + reason/evidence | disarm |
| Human enter Plan | valid current session | pause Goal first; Plan active only after drain | invalidate, stop, then enter |
| Model enter Plan | live root run; no armed Goal | Plan selection | apply at safe boundary |
| Submit plan | effective Plan; live root run | immutable submission + pending review | suspend/wait |
| Decide plan | human; exact pending submission | revise or end Plan; optionally create/arm Goal | never bypass permissions |

Model tools: enter_plan_mode, submit_plan, get_goal, create_goal, report_goal. Tool catalog metadata must not mark a state-mutating tool as Readonly merely to bypass a policy check. Restricted policies may deny it; show that honestly.

create_goal without an already validated Goal authorization opens a confirmation, not an immediately armed Goal. File/tool text cannot authorize it. Human-only edit/resume/clear are deliberately absent from model tool schemas. Model root provenance alone does not prove semantic authorization.
Tool reports cannot extend caps. Children return findings to their parent, not mutate session controls.

When complete/block is reported, remaining tool calls in that model batch must not execute product work after settlement. PG-0 must prove the smallest supported Eino mechanism to end/defer the batch and produce a final user-facing report. Do not claim a prompt alone guarantees this.

## 7. Admission and human input

One process-local candidate per session: GoalRef, expected version, next round, activation generation. No persistent prompt queue.
Human requests register a process-local startup intent before waiting on the per-session admission gate. A human request wins over an uncommitted Goal candidate; the final check and atomic Goal commit share the intent gate, so a later registration cannot undo an already committed candidate.

`turn/start` uses synchronous admission-waiter semantics, not a durable request ticket: the call returns only after a real run ID is committed or an error is known. While waiting on the short startup gate, the intent can invalidate an uncommitted Goal candidate. If a primary run is already committed, the request returns the existing conflict response and the caller may retry; it is not silently queued behind the run. Cancellation before commit creates no message or run. After commit, the existing detached-run rule applies: disconnecting does not cancel the run.

```text
on wake(session):
  coalesce signal; acquire session gate
  if stopping/deleting, Plan active, disarmed, busy or pending decision: return
  if human request pending: dispatch human path, invalidate auto candidate; return
  read current Goal; if cap spent: commit blocked(round-limit); disarm; return
  validate candidate against latest ref/version/activation generation
  ask existing Service startup to build governed run records
  atomically CommitGoalRun
  release gate; execute via ordinary Service drive
on terminal:
  schedule wake only after run state cleanup and required projections settle
```

Human messages after a Goal is armed may clarify current work without changing the objective; an objective change requires the edit operation. Entry into formal Plan requires explicit user transition. Approval/question pending counts as busy. Provider failure, unknown stop, queue failure, exhausted budget or cancellation disarms and records the specific cause. No outer retry around abnormal run termination.

Existing run-tree ledgers reset per ordinary run. MaxRounds is the outer bound; this is not an aggregate cost budget. Numeric Goal defaults remain a PG-0 configuration decision based on existing config constraints, not borrowed uncritically from DSH's 256.

## 8. Plan review, permissions and compatibility

New run.started payload must carry collaboration contract version separately from immutable execution policy snapshot. Historical missing version => legacy hard-Plan interpretation. New soft Plan does not coerce PolicyProfilePlan. Independent read-only policy remains unchanged even after Plan exit.

Plan guidance is reconciled from persisted Plan state by the runtime middleware before each model request. It is inserted once while Plan is active and removed after exit; this is advisory and does not change execution policy.

The Eino v0.9.13 adapter uses supported `tool.StatefulInterrupt` and `Runner.ResumeWithParams`. After a durable `plan.submitted`, the model tool interrupts; Service verifies the checkpoint, persists `plan.review_suspended` with the opaque exact Eino resume target and origin tool-call ID, and keeps the run active. The decision commits first, then resumes only that target. A retry of the same Work request returns the original result and does not start another resume. Restart restores only a pending review with a readable checkpoint and matching session/run/submission/call identity.

The submitted model batch is fenced before the interrupt reaches Service because Eino may still visit sibling calls. Service keeps the original tool-batch IDs after those calls return; after review it blocks those unreviewed siblings but permits new model calls. `ExecuteSequentially` plus the behavioral probe is the evidence; the config flag alone is not.

Review options: revise, execute_once, start_goal. The full plan is stored in the immutable `plan.submitted` work event; review linkage and resume data are separate versioned work events. Cancelling the originating run marks that review cancelled and leaves Plan active; no execution occurs. Repeated identical human decision returns the original result. Stale submission returns conflict. An old goal paused for planning remains paused unless the user explicitly resumes it or clears/replaces it; start_goal cannot silently overwrite an unfinished Goal.

Migration does not rewrite historical events or checkpoint bytes. Schema downgrade is unsupported: rollback uses a pre-upgrade backup, not deleting unknown events. Keep new activation disabled while migration or replay is uncertain.

## 9. History boundary: message/work ordering

Every persisted message carries `messages.work_seq`, the greatest session work-event sequence committed when the message transaction acquires the session lock. Each edit/rewind/fork marker carries the selected cutoff message's `work_seq` in `session_truncations.work_seq`. `WorkSeq` is independent of wall-clock timestamps and is stable across replay.

For an ordinary message or message projection, storage reads the current work-event maximum and writes the message plus anchor under the same session lock. An atomic Goal-run admission message records the pre-admission version; the matching `goal.round_admitted` event receives the next sequence in that transaction. This means the message is the cause/context for the admitted round, while the durable admission event remains separately auditable.

Rewind changes the visible context only. It records the cutoff sequence, preserves every work event and admitted-round count, and never wakes a Goal. Fork copies the visible message prefix with fresh message IDs and original `RunID` values as provenance, but writes `work_seq = 0` on copied rows and creates no child work events. The child therefore inherits no Goal identity, pending plan review, approval, admission count, or activation authority. Parent work history remains unchanged and replayable.

Same-timestamp rows are ordered by the stored sequence anchor, not guessed from timestamps. Historical rows predating migration 027 have anchor zero; migration does not fabricate an ordering for those records. The source session remains the audit record for work events that are not copied into a fork.

## 10. RPC and GUI draft contract

Proposed JSON-RPC methods:
- session/work/get {session_id} -> WorkView
- session/work/subscribe {session_id, after_seq} -> subscription
- goal/create|edit|pause|resume|clear: session_id, request_id, expect_version, operation payload; mutating existing Goal includes GoalRef.
- plan/enter|leave: same mutation envelope; leave never implies execute.
- plan/submit is model-facing operation; GUI reads immutable submissions through session/work/get plus referenced detail.
- plan/decide: mutation envelope + PlanDecision.
- plan/get {session_id, submission_id} -> immutable PlanSubmission + review disposition.
Existing turn/start and run/cancel remain the execution/cancellation routes.

Wire casing follows existing snake_case conventions. Errors map onto existing RPC codes with stable data.reason: stale_version, stale_goal, session_busy, goal_armed, plan_active, review_stale, round_limit, unauthorized_actor, payload_too_large, recovery_required. Do not turn domain conflicts into generic internal errors.

Subscription must replay through a captured watermark then deliver later committed seq without a gap; duplicates are deduplicated by session+seq. Activation events carry process epoch and current work version; reconnect fetches WorkView, never trusts an old armed projection.

Proposed UI: WorkControlBar.tsx with Plan control and Goal summary, PlanReview.tsx for document/decisions. State remains in store.ts; transport remains rpc.ts. Preserve TodoProgressStrip and existing task panel. Explicitly render active/disarmed, stopping_for_plan, pending review and blockers. No demo-api imports or generated-file edits.

## 11. Verification and economy

No runtime benchmark claims. Admission adds one bounded session transaction; no per-token work-event writes. Store only changes/reports/admissions, not duplicate transcript streams.
Plan Markdown is capped at 256 KiB, a Goal objective at 8 KiB, and Goal rounds at 1000. These fixed product limits live in `internal/domain/work_control.go` and are shared by model tools, runtime operations and RPC validation. Feedback remains capped by the existing RPC limit.

Mandatory implementation gate: just ci, plus configured Postgres conformance (unset DSN is SKIPPED), split dev browser flow and one real coding walkthrough. The execution log records which gates have run.
Design-only validation: local relative links, existing file references, dependency DAG, requirement coverage, git diff --check.
No code, schema, dependency, rules or generated assembly changes are made by this pre-design.
## 12. PG-D2 execution decisions (2026-09-22; probe update 2026-09-23)

This section records the accepted foundation decisions for implementation on the issue-47 branch. Executed probe evidence is linked from the PG-0 delivery index; full CI and the PostgreSQL/browser/live gates remain required.

### Durable work ordering

- Use a session-scoped `WorkSeq`/`WorkVersion` as the cross-record ordering key. Keep existing per-run `EventSeq` for run-local Journal replay.
- Persist work-control events in one `session_work_events` stream keyed by `(session_id, work_seq)`. The stream stores event kind, schema version, request ID, request hash, timestamp, and bounded payload.
- `(session_id, request_id)` is unique. Repeating the same request and payload hash returns the original committed result; the same request with a different hash is a conflict.
- Sequence allocation, Goal-round admission, ordinary message/run startup records, and `goal.round_admitted` evidence commit in one transaction. Publication and engine drive happen only after commit.
- `messages.work_seq` stores the committed session work version at message persistence; `session_truncations.work_seq` stores the cutoff message's anchor. A Goal admission message anchors the prior version and its atomic `goal.round_admitted` event gets the next version.

### History and authority

- Rewind changes the visible context only and never refunds admitted Goal rounds. Historical work evidence remains replayable.
- Fork copies visible context as new historical rows but transfers no Goal identity, pending review, approval, or host ticket authority. A copied source `RunID` is provenance only.
- A forked message receives `work_seq = 0` in the child because no source work event is copied. The parent marker records the source cutoff's sequence; the child marker's sequence is zero.
- The existing Service, Journal, projection barrier, and policy engine remain the sole lifecycle owners. No second Journal, synthetic permanent Goal run, or independent execution loop is permitted.

### Eino boundary

- The probe verifies guidance before the next model request; a pending Plan leaves its primary run active; exact decision resumes the stored Eino target; repeat decision does not create another model call; and a same-batch effectful sibling never reaches product code.
- Plan identity stores the origin run, tool-call ID, exact resume target and same-batch sibling IDs in the session work stream. The service fails closed if the checkpoint or any identity does not match. Leaving Plan or cancelling the source run closes the review without executing it.
- The pinned ToolsNode executes calls in model order. Runtime combines it with a pre-interrupt Plan fence, persisted sibling fence, and the terminal Goal fence; behavioral tests cover both Plan and Goal cases.
- Downstream stories still own the user-facing Plan/Goal experience and controlled Goal handoff. The separate host-queued human request ticket/lifetime contract remains unresolved and blocks story release.

### Human admission

- Mutating work requests carry a host-authenticated stable `request_id`, `session_id`, expected `WorkVersion`, exact Goal/Plan reference where applicable, and a payload hash.
- Current `turn/start` uses a process-local admission waiter rather than a durable ticket: register intent before waiting for the session gate, check request cancellation before persistence, and return only the actual committed run ID. An already committed primary returns the existing conflict; callers explicitly retry. A per-session intent gate linearizes pending human registration against atomic Goal admission. Any future ticket is correlation/idempotency state, never authority.

These decisions supersede the earlier unresolved placeholders in sections 5 and 9 while retaining the requirement for executable probe and backend evidence.

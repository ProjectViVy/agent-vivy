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
Human requests register at the host before competing for startup. Pending human work wins over an unadmitted candidate; once a run is committed, a later human request cannot undo it.

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

Plan guidance source (proposed): internal/runtime/prompts/plan.md, embedded and assembled by the existing runtime composer. Runtime facts (submission/ref/round) are separate bounded dynamic input. Add no duplicate personality prompt, memory layer or external asset loader.

Two adapter choices are permitted only after PG-0 evidence: supported in-turn guidance refresh at Eino's next model boundary, or a controlled interruption/resume through the current engine. No model reimplementation. If neither preserves safe review ordering, PG-2/PG-4 remain Blocked.

Review options: revise, execute_once, start_goal. The full plan is stored once in an immutable submission event; views reference its ID. Review expiry or cancellation leaves Plan active, no execution. Repeated identical human decision returns original result. Stale submission returns conflict. An old goal paused for planning remains paused unless the user explicitly resumes it or clears/replaces it; start_goal cannot silently overwrite an unfinished Goal.

Migration does not rewrite historical events or checkpoint bytes. Schema downgrade is unsupported: rollback uses a pre-upgrade backup, not deleting unknown events. Keep new activation disabled while migration or replay is uncertain.

## 9. History boundary: explicitly unresolved implementation blocker

The issue requires consistent fork/rewind history and no reset of spent rounds. Existing history APIs address message IDs, whereas work-control actions can occur between messages. Timestamp comparison is insufficient, especially with same-millisecond events.

PG-0 must specify and test a durable ordering anchor between message cutoffs and work-event seq:
- Rewind may alter visible historical context but cannot refund Goal rounds already consumed.
- Archived execution evidence remains accessible and must be distinguished from the current context.
- Fork inherits a prefix as historical context with no activation or transferable review authorization.
- Define whether copied Goal identity is source-qualified or newly allocated; cross-session report references must never grant mutation rights.

Do not implement history copying by guessed timestamps, silently omit Goal from forks, or relax the issue contract. This blocker is owned by PG-0 and must be resolved in this document before PG-1 is released. No downstream implementer is asked to improvise it.

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
Plan payload limits, notification buffering and shutdown deadlines reuse established bounds where suitable; PG-0 resolves exact sources.

Mandatory implementation gate: just ci, plus configured Postgres conformance (unset DSN is SKIPPED), split dev browser flow and one real coding walkthrough. Current environment lacks Go and has not run these checks.
Design-only validation: local relative links, existing file references, dependency DAG, requirement coverage, git diff --check.
No code, schema, dependency, rules or generated assembly changes are made by this pre-design.

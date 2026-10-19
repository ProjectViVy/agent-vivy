# S01: Extract same-Run steering orchestration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. One active write lane: `internal/runtime` is shared by every task in this Story.

**Goal:** Move same-Run steering's queue orchestration out of `internal/runtime` into the selected `plugins/exp/steering` contribution through the core `SteeringHost`, so the selected species has exactly one policy/adapter path while core keeps generic follow-up, cancel, approval, durable storage and Runner/checkpoint primitives.

**Architecture:** A per-run, root-owned Host captures the current leg (`Service`, run/session, event mapper, nudge/ledger dependencies) and exposes only the §11.3 primitives (`Snapshot`, `Enqueue`, `SetMode`, `RequestCancelAfterModel`, `ContinueBoundary`). The adapter validates policy choices and calls those primitives; `Service.Steer`, `SetQueueMode`, the consume boundary branches and terminal cleanup call the controller when bound. The Host starts the checkpoint-resume consume leg on its own goroutine because the boundary-cancelled iterator is still open; the RunID, checkpoint ID, Journal and terminal ownership do not change.

**Tech Stack:** Go 1.26.4; pinned `github.com/cloudwego/eino v0.9.13` (`adk.WithCancel`, `CancelAfterChatModel`, `ResumeParams`, `ChatModelAgentResumeData.HistoryModifier`); existing Journal/queue storage. Eino stays inside `internal/runtime`; `sdk/port/loopexp` and `plugins/exp/steering` remain Eino-free.

**Spec:** [Complete deletion design §§10.1a C11, 10.3, 11.3, 11.5](../../../plans/2026-10-19-complete-deletion-plan.md). [S00](S00-seam-binding.md) produces the contracts and binding. [Index](index.md) owns status.

**Story:** S01 · **Epic:** E-STEERING · **Requirements:** SR2, SR3, SR5.

**Immediate predecessors:** S00 accepted. External **G2** accepted (source/generation/binding form). G1 accepted.

**Readiness gate:** S00's binding, adapter factory and `HasSteeringFactory` accessor are accepted and unchanged; the §11.3 Host/controller signatures are frozen; the selected default composition builds. If S00 changed the Host shape, revise this Story before writing code.

## Global Constraints

- One `Service.Run`, one Eino `Runner`, one Journal, one policy path. No second queue, scheduler, goroutine-owned runner or storage.
- Steering keeps the same `domain.RunID`, `checkpointIDFor(runID)`, native cancel safe point, drain and checkpoint resume. It never starts a sibling Run.
- The Host binds the current leg and accepts no caller-selected RunID, checkpoint, model, credentials or Journal handle.
- Generic follow-up, queue storage/rebuild, user cancel, approval/question recovery, terminal settle, budgets and the follow-up admission path remain in core and keep their behavior.
- Unselected species has no controller and no fallback to the baseline core steering path: `Service.Steer` returns `NOT_COMPILED`; generic follow-up still works.
- No hash/digest is added to the queue, Journal, events or resume path. Preserve `turn.queued`, `turn.dequeued`, `turn.steered` and their payload shape.
- Do not edit the EXP package's public contract without S00/index revision. No commit/push, no production data/schema changes.

## Review Focus

- Boundary duplication: both `errRunInterrupted` and the generic error branch still calling old core steering, or both controller and legacy code handling one cancel.
- Resume inside the cancelled consume frame deadlocking on the checkpoint; the Host must keep the baseline "own goroutine" rule.
- Exactly-once drain: an item resumed twice, dropped on refusal, or demoted to follow-up after it was already delivered.
- Policy authority creep: the policy returning a RunID, checkpoint, permission, model or record selection; the adapter must reject anything outside the enum/count contract.
- Loss of same-Run proof: a new RunID, a duplicate `turn.steered` for one drained item, or a follow-up Run admitted by the steering leg.
- Core regression: user cancel, approval resume, follow-up admission and queue rebuild changing behavior or payloads.

## Baseline A mapping

| Baseline fact | S01 action |
| --- | --- |
| `internal/runtime/queue.go:168-226` `Steer` owns route, enqueue, arming and demote-on-refusal. | Route/track/count/render come from the policy; enqueue, arming, refusal reclassification and Journal markers stay in Host primitives. `Service.Steer` becomes the call site + `ErrQueueUnavailable`/`NOT_COMPILED`. |
| `internal/runtime/queue.go:366-405` `takeSteerTurn` pops items, journals one `turn.steered` per item and appends one transcript message per item. | Move drain/consume-once into Host; keep one marker and one transcript row per item, and one joined resume message. |
| `internal/runtime/queue.go:512-541` `steerPendingCancel`/`demoteSteerItems`. | Host owns the armed-flag consumption and demotion; terminal cancel of a user abort is not a steer boundary. |
| `internal/runtime/queue.go:597-648` `resumeSteeredRun` builds `ChatModelAgentResumeData.HistoryModifier` and walks `InterruptCtx` targets. | Host `ContinueBoundary` owns this; the adapter supplies only the rendered text via the `HistoryModifier` contract. Keep `CancelAfterChatModel` and the "append before trailing assistant(tool_calls)" rule. |
| `internal/runtime/queue.go:666-691` `resumeSteeredAsync` goroutine + nudge context reset + terminal `"steer resume unavailable"` demote. | Host `ContinueBoundary` keeps the goroutine and failure behavior; do not call `consume` synchronously from `OnBoundary`. |
| `internal/runtime/service.go:2963-2977,4658-4676` arms `steerCancels` for initial and approval-resume legs. | Host registers the cancel handle at leg bind/arm time; `Service` keeps only the controller and Host reference. Approval/initial flows are not otherwise changed. |
| `internal/runtime/service.go:3568-3597` two boundary branches call `steerPendingCancel` then `resumeSteeredAsync`. | Both branches call `controller.OnBoundary`; a `handled=true` returns without terminal/abort; `handled=false` falls through to the existing interrupt/error handling. |
| `internal/runtime/service.go:4949-5005` terminal cleanup + follow-up drain/flush. | Host cleanup hook releases per-run maps/goroutine refs; generic drain/flush stays in core. |
| `internal/runtime/queue_test.go:281-350,524-...` same-Run/no-sibling tests. | Keep generic queue tests; move steering behavior assertions to Host tests and policy tests. No assertion deleted without a replacement covering the same invariant. |

## Files and ownership

- Create: `internal/runtime/steering_host.go`, `internal/runtime/steering_host_test.go`; extend `internal/runtime/extensions/steering/adapter.go` and its tests.
- Modify: `internal/runtime/service.go` (fields 311-314/525-526, arm 2963-2977, consume branches 3568-3597, terminal 4949-5005), `internal/runtime/queue.go` (Steer/FollowUp boundary, take/demote/resume helpers), `internal/runtime/queue_test.go`, `internal/app/assembly_steering.go`/shared app integration point (inject controller into `Service`); `internal/app/app.go` only if the construction call lives there.
- Do not touch: `internal/domain/*` event/queue types, `internal/rpc/control.go` (S02 owns RPC behavior), `ui/` (S02), generated Assembly (S00/S03), `sdk/port/loopexp` contract (S00).
- Keep `internal/runtime/queue.go` as the durable generic queue owner; do not create `queue_steer.go`.

### Consumed and produced contracts

- Consumes: S00's `loopexp.SteeringFactory/SteeringPolicy`, `runtime.SteeringFactory/SteeringController/SteeringHost/HistoryModifier`, generated factory.
- Produces: `steeringHost` implementing the five Host methods over core primitives; `Service` controller injection; `Service.SteeringAvailable() bool` and `Service.ErrSteeringNotCompiled` (proposed sentinel) used by S02; unchanged `QueueState`/event types.

## Tasks

### Task 1: Add the Host primitives without changing behavior

- [ ] Define the per-run Host in `internal/runtime/steering_host.go`: fields for `*Service`, `*eventMapper`, `domain.SessionID`, selected tool names, `domain.RunMode`, `*BudgetLedger`, `beforeComplete`, `runExecutionOptions`, and the leg context builder. It is constructed at run start and at approval resume when a controller is bound; it is never stored globally by session. A session-bound Host instance (no run leg) is used only by `SetMode`, whose state is the durable per-session `q.steerMode` owned by core.
- [ ] `Service` holds the injected `runtime.SteeringFactory` and a per-run `map[domain.RunID]SteeringController` populated when the leg is armed and deleted at terminal cleanup. `Service.Steer` resolves the controller by the active run ID; a nil factory means `NOT_COMPILED`, while a live run with no controller (run predates the seam or arm race) keeps the Baseline A demotion to follow-up.
- [ ] Implement `Snapshot(ctx, candidate)`: return copies of the steer lane and mode, plus eligibility (`Suspended` from `s.pending[runID]`, `HasBoundaryCancel` from the registered cancel handle, `PreferencesCompatible=true` for the current RunMode, attachments/file-context/continuity false unless the current admitted turn carries them). No mutation.
- [ ] Implement `Enqueue(ctx, item, track)`: validate track, append to the correct lane, assign `EnqueuedOn=runID`, journal the exact `payloadTurnQueued` marker, return a copy. This is the only enqueue entry the controller gets; it cannot select a different session or RunID.
- [ ] Implement `SetMode(ctx, mode)`: validate `all|one-at-a-time`, set only the steer lane mode, mirror the current `SetQueueMode` validation/error strings. Follow-up mode stays on the core method.
- [ ] Implement `RequestCancelAfterModel(ctx, queueID)`: find the already-enqueued steer item, arm `steerArmed[runID]`, call the registered `adk.AgentCancelFunc` with `adk.WithAgentCancelMode(adk.CancelAfterChatModel)`, and on refusal remove the same ID, journal `turn.dequeued{reason:"aborted"}`, move the same ID to the follow-up lane and journal `turn.queued` with track `follow_up`. Never call `Enqueue` again. Persist errors propagate; do not swallow them.
- [ ] Implement `ContinueBoundary(ctx, ids, modifier)`: goroutine-owned resume exactly as baseline. Resolve the checkpoint `checkpointIDFor(runID)`, build `ResumeParams.Targets` from `m.interrupt.Raw` (agent targets -> `&adk.ChatModelAgentResumeData{HistoryModifier: modifier}`; fallback `"agent:"+engine.agentName`), call `engine.Resume`, and on nil demote the same IDs to the follow-up lane before emitting terminal `run.cancelled{reason:"steer resume unavailable"}`. On success create a fresh nudge state/emitter and run `consume` with the captured leg dependencies.
- [ ] Keep `steerCancels`/`steerArmed` access under the Host, not `Service`; expose a cleanup method the terminal path calls. If a map must stay in `Service` for the existing approval-resume arm, document why and ensure exactly one owner.

### Task 2: Rewire Service.Steer and SetQueueMode

- [ ] `Service.Steer`: no active run -> `ErrQueueUnavailable` unchanged. Factory nil -> `ErrSteeringNotCompiled`. Otherwise resolve the per-run controller: if absent (a live run that predates the cancel seam or lost the arm race), keep the Baseline A path and call `enqueueFollowUp`; if present, call `controller.Steer(ctx, domain.QueuedTurn{SessionID, Text, EnqueuedOn: activeRun})`, which runs `Snapshot` -> `Route` -> `Enqueue` -> `RequestCancelAfterModel`. No queue routing logic remains in `Service.Steer`.
- [ ] `Service.FollowUp`: keep the existing core path unchanged; do not route follow-up through the controller.
- [ ] `Service.SetQueueMode(track)`: `follow_up` stays core; `steer` requires the factory and returns `ErrSteeringNotCompiled` when absent, otherwise constructs a controller over the session-bound Host and calls `SetMode`; the mode value is written to core `q.steerMode`, so it remains durable without an active run and survives unselected recovery reads. Keep invalid-mode errors and accepted values identical.
- [ ] `Service.QueueState`: selected -> return the copy plus mode; unselected -> return follow-up lane and an empty feature lane (S02 adds the recovery projection). Do not expose the old core steer lane as if compiled. Add `Service.SteeringAvailable() bool` returning whether the factory is bound; it is the single capability truth S02 consumes.
- [ ] Remove the now-dead core `Steer` routing/enqueue/arm block. Do not leave a fallback branch that reconstructs it.

### Task 3: Rewire the consume boundary and drain

- [ ] At both `errRunInterrupted` and the generic error branch, replace `s.steerPendingCancel(...)`/`s.resumeSteeredAsync(...)` with `handled, err := controller.OnBoundary(ctx)`. `handled=true` -> return/continue without terminal abort; propagate `err` through the existing terminal path; `handled=false` -> fall through to the current interrupt/error handling unchanged.
- [ ] `controller.OnBoundary` calls `Host.Snapshot` for the pending steer lane, `policy.Take(mode, len(pending))`, select the first N IDs (never reorder or drop), call `policy.Render(texts)`, build the `HistoryModifier`, and call `Host.ContinueBoundary`. It consumes the armed flag once; a second call without a new cancel returns `handled=false`.
- [ ] Drain exactly the selected IDs from the steer lane, journal one `turn.steered` marker per item with the exact Baseline A payload (`queue_id`, `text`), and append one user transcript row per item through `deps.Messages` (same role/content shape as `takeSteerTurn`). The resumed leg injects one joined user message and must not re-enqueue or re-drain the items.
- [ ] Keep the two-branch structure: a checkpoint interrupt and a generic error can both carry the armed cancel; `OnBoundary` must be idempotent per armed cancel. A user abort (`turn/interrupt`) must not be classified as a steer boundary.
- [ ] On terminal settle, call the Host cleanup once; keep the existing `drainFollowUps` (completed/failed) and `flushQueue` (cancelled) behavior. When the controller is absent, `popFollowUps`/`materializeQueueTail` must not demote steer items into follow-up (`queue.go:458-510`): leave them untouched for S02's recovery projection, so an unselected species cannot silently apply a persisted steer turn.

### Task 4: Wire the selected controller and prove parity

- [ ] In `internal/app/assembly_steering.go` (or the shared loop-exp integration point created by S00), construct `runtime.SteeringFactory` from the generated binding only when present; pass it to `runtime.NewService`/the existing service construction path with a narrow option. Unselected passes nil. A selected build with a nil/failed factory fails construction; no silent nil.
- [ ] Split tests: keep generic queue/rebuild/follow-up/cancel tests in `internal/runtime/queue_test.go`; move `TestSteerInjectsAtTurnBoundary` equivalents to `internal/runtime/steering_host_test.go` with a real `Service` + fake engine/controller, asserting: one RunID, one `turn.steered` per drained item (the single-item case has exactly one), one joined injected user message ordered before a trailing assistant(tool_calls), no second Run, cancel mode `CancelAfterChatModel`.
- [ ] Add refusal/demote tests: cancel returns false -> same queue ID in follow-up, two Journal markers, no duplicate text; resume nil -> items demoted before terminal, no data loss; resume error -> same.
- [ ] Add mode/count tests through the adapter: `all` drains N, `one-at-a-time` drains 1, zero pending -> handled false, invalid mode/count -> error without mutation.
- [ ] Add unselected tests: controller nil -> `ErrSteeringNotCompiled` for steer/mode; follow-up still queues and admits; terminal cancel/approval resume unchanged.

## Verification commands and expected result

Future gates, not executed by this planning turn:

```text
go test ./internal/runtime -run 'TestSteer|TestQueue|TestFollowUp|TestBoundary|TestResume' -count=1
go test ./internal/runtime/extensions/steering ./internal/app -run 'TestSteering|TestAssembly|TestEinoImportsQuarantined' -count=1
go test ./internal/rpc -run 'TestTurnSteer|TestQueue' -count=1
just ci
```

Expected: steering behavior tests pass against the selected controller; same-Run/no-second-run/one-marker-per-item assertions hold; unselected returns NOT_COMPILED while generic follow-up/cancel/approval pass; no queue state or event payload drift; no second runner/queue; `just ci` green. Focused checks are not a waiver for the final gate or the real-path smoke in S02.

## Acceptance, rollback and handoff

S01 is accepted when selected steering is driven only by policy + Host, core contains no steering route/take/render/resume decision, the same RunID/checkpoint/native-cancel semantics are proven, generic queue/follow-up/cancel/approval behavior is unchanged, and unselected species has no fallback path. S02 may not start until this is accepted.

Rollback: revert S01's commit; S00's seam remains but is not behaviorally wired. Do not leave both old and new handlers active, and do not ship a selected species between S00 and S01. Handoff must include the Host method inventory, the exact baseline functions removed, the same-Run/no-sibling evidence, the unselected error behavior, and any shared-contract deviation.

# Parallel Tool Execution — Design

Date: 2026-10-08
Status: proposed
Ref: CODING-TOOL-BATCH-PARALLELISM (validation report, `docs/logs/2026-10-08-parallel-tools-probe/`)

## Problem

`compose.ToolsNodeConfig.ExecuteSequentially` stays `true` in `internal/runtime/engine.go`
because the report proved that flag-only produces **zero** real concurrency:
`Service.WorkToolCall` (`internal/runtime/work_control_tools.go`) wraps the whole
`dispatchUngated` call — admission **and** invocation — in a per-run mutex
(`s.workGates[runID]`). Measured bottleneck: 4-tool batch 555ms → 105ms and
8-tool batch 1126ms → 139ms under a diagnostic bypass.

Deleting the gate is not acceptable. It guards the check-to-invoke boundary
against terminal Goal reports (`report_goal` → `fenceWorkRun`) and unreviewed
Plan sibling effects (`submit_plan` → suspend → `PlanBlockedToolCalls`).

## Design: split admission and invocation behind an RW gate

Replace the per-run `sync.Mutex` with `sync.RWMutex` semantics:

- **Non-work calls take the read side for their whole dispatch**
  (admission + invoke). All non-work siblings now run truly in parallel.
- **Model-work calls (`enter_plan_mode`, `submit_plan`, `get_goal`,
  `create_goal`, `report_goal`) take the write side for their whole
  dispatch** — admission and the commit both run exclusively.

Preserved boundaries:

| Boundary | Mechanism |
| --- | --- |
| check-to-invoke vs terminal commit | fence check under R is atomic w.r.t. the W-held commit; a work commit can never interleave with a sibling's check+invoke |
| post-admission siblings | Go `sync.RWMutex` writer preference: once a work call queues for W, later readers wait, then see the fence and are refused |
| pre-commit siblings | W waits for all in-flight readers to drain — every sibling effect completes *before* the work call's admission+commit (disjointness = sequential-equivalent, never weaker) |
| durable operation identity | unchanged: coordinator Admit/claim/flight CAS paths untouched |
| approval/resume | unchanged: effectful calls still interrupt; Eino `CompositeInterrupt` + `RerunTools` serializes multi-interrupt batches into sequential suspends |
| cancellation | unchanged: run ctx cancels in-flight invokes; locks are released via `defer` |
| result order to model | unchanged: Eino restores call-index order |

Work calls in one batch are rare and the writer only waits for in-flight
siblings — the same wait sequential execution already imposed — so the RW gate
is faithful at near-zero cost.

## Second required change: eino bump ≥ v0.9.20

`EINO-TOOLSNODE-ERR-RACE` (`docs/TODO.md`): v0.9.13's
`ret[index].UserInputMultiContent, err = tr.ToMessageInputParts()` races across
parallel enhanced converters in `Stream()`. Fixed upstream (`parts, convertErr :=`)
in v0.9.20+. Bump `github.com/cloudwego/eino` to the latest 0.9.x and verify the
full suite under `-race`. Fallback if API drift blocks the bump: vendor the
one-line fix via `third_party/` (pattern exists for renameio) — bump preferred.

## Third required change: journal-derived resume batch

Exposed by `TestTwoEffectfulCallsInOneBatchSuspendSequentially`: after the
second sequential approval resume of one batch, the run stayed `active`
forever. The resumed model boundary (`nudgeWrappedModel.prepare` →
`nudgeState.Take`) waits for a sealed batch covering **every** tool-result id
in the model input tail — replayed results included. The resume leg's fresh
`nudgeState` only sees `tool.finished` for calls that actually re-executed on
that leg; sibling results replayed inside eino emit no events, so the batch
can never cover the tail ids and `Take` blocks forever (verified by goroutine
dump: consume parked in `AsyncIterator.Next`, model call parked in `Take`).

The designed remedy already exists — `restoreResumeNudgeBatch` registers the
full batch and `SatisfyDurable`s siblings whose finishes are durable — but
only the plan-review resume supplied `resumeBatchIDs`; approval and question
resumes (and restart-rebuilt pendings) passed `nil`. The mapper's `toolBatch`
cannot help: it is empty on resume legs (no `tool.requested` events re-emit).
Fix: `resumeRun` derives `resumeBatchIDs` from the durable journal when the
caller passes none — the batch is the tool-call set of the model turn
containing the suspended call, which survives restarts and every sequential
suspend. Single-call batches behave exactly as before; refusal/failure
siblings satisfy the barrier through their journaled `tool.finished`
(refusals always land a tool result).

## Fourth required change: model-turn grouping + request-order barrier

Review found two order regressions the first cut missed.

**Turn-boundary batch recovery.** The original derivation grouped
*contiguous* `tool.requested` events — but admission-time events
(`policy.evaluated`, `tool.operation`, approval lifecycle) interleave between
a batch's request records under parallel dispatch, so one turn's set split
into several single-call "batches" and resume restored a barrier that could
never seal (run stuck `active`). `resumeBatchIDsFromJournal` now bounds by
the model turn: `model.request` seals a turn; the answer is the
`tool.requested` set since the most recent `model.request` that contains
the call. Covers interleaved events, finished/pending mixes, consecutive
suspends inside one batch, and restart rebuilds uniformly.

**Request-order barrier.** `RWMutex` acquisition order is scheduling order,
not model-request order: a write tool positioned after `submit_plan` could
take the read side before the writer queued and execute past the fence
(`TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch` observed
`effect.calls == 1`). Fix: per-run `toolBatchOrder` tracks each model turn's
calls in request order (consume registers them where `state.Register` runs;
journal order is the authoritative order on resume legs) plus a done signal
per call. An ordinary call waits for every earlier model-work call in its
batch before entering the gate — its fence check then observes the committed
result. A model-work call waits for every earlier sibling, so a later commit
never precedes an earlier call's effect. Calls before a work call keep
running in parallel with it; waits always point later→earlier so they cannot
cycle; `tool.finished`, resume replays, and the dispatch defer all release
waiters so replayed/rebuilt/suspended siblings never hang a batch.

## Shared-state audit (dispatch concurrency)

Verified safe under parallel dispatch:

- journal/`appendRunEvent` — `s.projectionMu`
- `governanceSink` payload build → `m.build` (read-only for governance types),
  `ledger.Reserve*` — per-scope mutex, documented atomic
- operation coordinator — CAS + `operationMu` + `operationFlights`
- `nudgeState`, `MountedTools` — own mutexes
- `ToolHookChain` — immutable hook list, per-call arg copies, sink under
  projectionMu; no internal state
- policy eval, `untrustedToolResultHeader`, `compactToolResult` — pure / ctx reads
- `workGates`, `workFenced`, `workBlockedCalls`, `pending`, `runSessions`,
  `goalRuns` — `s.mu` (already used for all of these)

Open check during implementation: `CommandOperations` (bash/file tools) shared
workspace/process state.

## Changes

1. `internal/runtime/work_control_tools.go` — `workGates` → `sync.RWMutex`;
   `WorkToolCall(ctx, exclusive bool, call)`; writer preference documented.
2. `internal/runtime/tooladapter.go` — `dispatch` passes
   `exclusive = isModelWorkTool(spec.Name)` (or spec-derived equivalent).
3. `internal/runtime/engine.go` — `ExecuteSequentially: false`; update the
   comment to describe the gate split.
4. `go.mod`/`go.sum` — eino v0.9.13 → ≥v0.9.20 (latest 0.9.x); compile-check
   every eino API Vivy uses.
5. Multi-interrupt handling — serialized suspends verified against raw eino
   (minimal two-interrupt repro completes) and end-to-end. `resumeRun` derives
   `resumeBatchIDs` from the journal when the caller passes none
   (`resumeBatchIDsFromJournal`, model-turn bounded); `extractInterrupt`/
   `handleInterrupt` unchanged.
6. `internal/runtime/tool_batch_order.go` — per-run request-order tracker;
   `WorkToolCall` waits for earlier batch siblings per the work-call barrier
   rules; consume and resume-restore register batches and release settled
   calls.
7. `internal/runtime/nudge_acceptance_test.go` — request-order assertion
   narrowed to presence + the nudge naming contract (journal finish order is
   legitimately completion order under parallelism).
8. `docs/TODO.md`/`docs/COMPLETE.MD` — close `EINO-TOOLSNODE-ERR-RACE`;
   `CODING-TOOL-BATCH-PARALLELISM` was a report id, not a board row.

## Tests

- channel-barrier probe: N sibling tools enter invoke before release
  (concurrency proof, mirrors report methodology).
- terminal order: sibling dispatched after `report_goal`/`submit_plan`
  W-pending → fenced/refused; sibling in-flight → completes before commit.
- plan batch: `submit_plan` + siblings → siblings either complete pre-commit or
  refused post-review; `PlanBlockedToolCalls` recorded.
- two effectful calls in one batch → two sequential approval suspends.
- durable identity: resume replays stored operation result.
- journal derivation under interleaved events, mixed settled/pending,
  turn boundaries, consecutive suspends (unit tests on crafted journals).
- order tracker: registration wait, done barrier, cancel release (unit).
- earlier sibling of `submit_plan` still executes exactly once, across the
  plan-review resume.
- `go test -race ./internal/...` clean (this also covers the eino fix).

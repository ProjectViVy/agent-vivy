# N02: Live nudge adapter, parity and recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Make the selected `vivy/exp-nudge` package drive real coaching through one small Eino adapter and one per-leg runtime host, with settings/UI/event parity and the existing durability/recovery semantics preserved.

**Architecture:** `internal/runtime/extensions/nudge/` implements the N00 slot: a `Factory(loopexp.NudgeFactory) runtime.NudgeFactory` whose extension returns an immutable `adk.ChatModelAgentMiddleware` (embedded `*adk.BaseChatModelAgentMiddleware`, `WrapModel` around `Generate`/`Stream`) and binds a per-leg `runtime.NudgeLegHost` into the context. The root runtime owns the leg host and the producer feed: the durable tool boundary records a bounded producer-owned outcome per call; `WrapModel` asks the host for the trailing batch outcomes in native order, evaluates once, schedules `tool.nudge` through the Service before injecting the transient reminder. Root runtime imports neither the adapter package nor the EXP module; the generated Assembly imports both and passes the typed factory through `internal/app/`. Unselected builds execute none of this.

**Tech Stack:** Go 1.26.4; pinned Eino v0.9.13 (`adk.ChatModelAgentMiddleware`, `adk.BaseChatModelAgentMiddleware`, `WrapModel`, `schema.Message`); existing SQLite/Postgres Journal and durable tool operations; TypeScript/React unchanged.

**Spec:** [Complete deletion design](../../../plans/2026-10-19-complete-deletion-plan.md) C11, §10.3, §11.1–11.5; [NUDGE-DESIGN](../../../../docs/architecture/NUDGE-DESIGN.md) §4–§7; [index](index.md). Contract from [N00](N00-seam-binding.md); policy from [N01](N01-policy-extraction.md).

**Story:** N02 · **Epic:** E-NUDGE · **Requirements:** R1,R2,R3,R5,R6.

**Immediate predecessors:** N00, N01.

**Readiness gate:** Accepted N00 (slot, catalog, generator, app unwrap) and N01 (module + Recipe); accepted D02/D03 (producer-owned typed operation outcomes and the single continuation path) and D04 (handoff removal shape). Current request authorizes planning only.

## Global Constraints

- One `Service.Run` / Eino Runner / Journal / policy path; no second runner, coordinator, queue, scheduler or watcher.
- No blocking barrier on progress: `WrapModel` synchronously reads outcomes that the producer already recorded after the durable commit; a missing outcome is an invariant error, never a wait or a parked completion.
- `tool.nudge` is persisted through the Service's existing `persistAndPublish` **before** the reminder is injected. An append failure aborts that model call with the original cause; no unrecorded injection.
- Provider retries re-present the same input: one evaluation per settled batch, one scheduled event, same notice text.
- A fresh detector and leg host per leg (initial run, approval/question resume, each steering resume); no window state or pending notice survives a leg. Resume restores sibling batch **coverage** from the journal and excludes replayed siblings from the fresh window.
- Unselected builds: no middleware, no adapter/EXP import, no state, no event producer, no control.
- Only `internal/runtime/` (including `internal/runtime/extensions/nudge/`) and `internal/provider/` import Eino; `plugins/exp/nudge/` and `internal/app/` do not.
- No nudge config key, RPC action or UI control is added. `NOT_COMPILED` applies if any future control exists; this Story proves none does.
- No schema/migration change; no new dependency; no production journal access.

## Review Focus

- Request-before-send ordering: outcome read, schedule, then `inner.Generate/Stream`; the reminder is a transient tagged `schema.UserMessage` appended to a copy of the input, never persisted or checkpointed.
- Legacy Eino hooks are not used for persistence: `BeforeModelRewriteState` would replay synthetic reminders after resume/compaction and is rejected.
- The adapter holds no Journal/model/storage handle; everything arrives from context per leg.
- The reminder stays inside the 1024-byte ceiling and the remaining context budget (`ErrContextBudgetExceeded`).
- The leg host validates coverage of the trailing call IDs and returns a bounded, immutable sample set; it must not read raw arguments beyond the canonical form already recorded by the tool boundary.
- Refusal/corrective classification remains the tool boundary's owner; nudge presence never changes whether a failure is converted to a model-visible result.
- Approval/question resume and same-Run steering resume keep their existing lifecycle; nudge absence at those legs is handled explicitly, not by panic.
- Terminal/loop mapping: the core `errLoopDetected` guard keeps the existing `run.failed{cause_category:loop_detected}` path once; the package has no Stop decision.

## Baseline A mapping (202aac38, 2026-10-07)

| Concern | Baseline A |
| --- | --- |
| Middleware | `internal/runtime/nudge_middleware.go`: `newNudgeMiddleware(maxContextBytes)`; `nudgeWrappedModel{inner, maxContextBytes}`; `prepare` = `trailingToolCallIDs` -> `state.Take(ctx, ids)` -> `renderNudge` -> 1024/context checks -> conditional `emit` -> copy input + tagged user message; both `Generate` and `Stream`. |
| Handoff to remove | `internal/runtime/service.go:3501–3560` (`Register`/`takeCompletion`/`Complete`/`Seal`/`terminalErr`), `restoreResumeNudgeBatch` (`4682`), `nudgeEmitter` (`3264–3283`), mapper parking (`mapper.go` `openCalls`/`completions`/`storeCompletion`/`takeCompletion`); D04 removes the handoff, this Story must not reintroduce it. |
| Producer feed | `internal/runtime/tooladapter.go:897–949` (classification gate, `markCommandFailure`), `internal/runtime/tool_failure.go` (`classifyToolFailure`, vocabulary); D02/D03 move the typed outcome onto the durable operation row and the tool result boundary. |
| Leg creation | `service.go:2953–2956`, `4648–4656`; `queue.go:682–689` (steering); `engine.go:372–376` (unconditional handler, appended last/innermost). |
| Engine wiring | `internal/runtime/engine.go`: `EngineConfig` (`35–121`), `handlers` list (`~250–376`), `adk.ChatModelAgentConfig.Handlers` (`390–409`); `internal/modules/loop` passes `EngineConfig` through `loopDriver.Build`; app builds it in `buildEngineConfig` (`internal/app/compaction.go:102–130`, shared by startup and settings reload) and rebuilds through `ServiceDeps.RebuildEngine` (`internal/app/app.go:862–868`). |
| Event/UI | `payloadToolNudge` (`payloads.go:271–280`), `EventToolNudge` (`internal/domain/event.go:28,109`), strict `schemas/events/payloads/tool.nudge.json`, `run-event.schema.json:40`; `ui/` has no `tool.nudge` handler — generic event/Inspect rendering only. No nudge config/action exists. |
| Tests to re-home | `nudge_middleware_test.go` (boundary behavior), `nudge_state_test.go` (leg state), `nudge_contract_test.go` (test-local mirror of the removed core contract, call-ID/ordering/retry/cancel/compaction checks), `nudge_acceptance_test.go` (real Service/SQLite/governed tools, cases A–J). |

## Files and ownership

- Create:
  - `internal/runtime/extensions/nudge/adapter.go` — `Factory(loopexp.NudgeFactory) runtime.NudgeFactory`; extension struct holding one detector; `Middleware() adk.ChatModelAgentMiddleware`; `BindLeg(ctx, runtime.NudgeLegHost) context.Context`; wrapped model implementing `Generate`/`Stream` via one `prepare`; trailing-`schema.Tool` extraction; per-batch memo so provider retries reuse the evaluated notice without a second schedule; 1024-byte and context-budget checks; tagged `schema.UserMessage` (`kind=runtime_nudge`, `tool_call_id`, `tool_name`, `template_version`).
  - `internal/runtime/extensions/nudge/adapter_test.go` — fake detector + fake host: schedule-before-inject ordering, retry dedupe, append-failure abort, budget/ceiling, omitted notice path, missing-coverage invariant error, copy-not-mutate input.
  - `internal/runtime/nudge_leg.go` — per-leg host implementation: bounded outcome projection keyed by call ID, `Outcomes(ids) ([]loopexp.ToolSample, error)` validating coverage/order, `Schedule(notice) error` delegating to `Service` persistence, release on leg end; producer recording hook `recordLegOutcome(callID, sample)`.
  - `internal/runtime/nudge_leg_test.go` — coverage/order/bounds/release, no double schedule, refusal `not_executed` forwarded.
  - `internal/runtime/nudge_activation_test.go` (production-path integration with the real package): selected build end-to-end via `NewEngine`/`NewService` with a scripted model, real SQLite Journal and governed tools (reuse `nudge_acceptance_test.go` harness helpers): thresholds 3/5, corrective and refusal templates, hard stop at 6 with `loop_detected`, schedule-before-injection order, provider-retry single event, approval-resume sibling coverage with no stale notice, steering-resume fresh window, cancel while the call is in flight.
- Modify:
  - `internal/runtime/extensions.go` — only if N00 left nil-safe helpers or wiring hooks incomplete for this Story; no contract change without returning to N00.
  - `internal/runtime/engine.go` — `EngineConfig.NudgeFactory runtime.NudgeFactory`; when non-nil, append `Factory().Middleware()` last (innermost), in place of (not in addition to) the deleted unconditional middleware; when nil, register nothing.
  - `internal/runtime/engine_nudge_test.go` — handler ordering (prompt/AGENTS/compaction handlers still outer; nudge innermost) and nil-factory behavior.
  - `internal/runtime/service.go` — replace per-leg state construction with `nudgeFactory.BindLeg` + host allocation at `~2953`, `~4648`; resume coverage restore through the journal; remove `nudgeEmitter`, `restoreResumeNudgeBatch`, `Register/Complete/Seal` calls and `terminalErr` reads once N03 deletes their owners; keep the existing core `errLoopDetected` terminal mapping (not package-driven).
  - `internal/runtime/queue.go` — steering leg binds its own host/detector (replacing `newNudgeState`), same lifecycle.
  - `internal/runtime/tooladapter.go` + `internal/runtime/tool_failure.go` — the typed outcome conversion runs at the tool boundary regardless of nudge selection; when a nudge leg is bound, the boundary also records the bounded `ToolSample` after the durable commit (no side-channel gate that disables correction).
  - `internal/app/assembly_loop_extensions.go` + `internal/app/app.go` — pass the typed factory into the engine config at both construction sites (`buildEngineConfig` caller and `RebuildEngine`), one factory instance per App; no adapter/EXP import.
  - `internal/app/assembly_loop_extensions_test.go` — selected/absent wiring, rebuild path parity.
  - `docs/architecture/NUDGE-DESIGN.md` — boundary/ownership clauses (full revision in N03).
- Do not touch: `plugins/exp/nudge/` semantics (N01), `schemas/`, UI feature code (must remain generic), generated files by hand.

## Tasks

- [ ] 1. Implement the leg host (`internal/runtime/nudge_leg.go`) with the N00 `NudgeLegHost` shape: `Outcomes([]string) ([]loopexp.ToolSample, error)` returns samples in the requested order and fails with a named invariant error on any missing/duplicate/unknown ID; `Schedule(loopexp.NudgeNotice) error` builds the existing `payloadToolNudge` and calls `persistAndPublish` once, returning the append error unchanged. Bound to the leg, released on terminal/interrupt/drain.
- [ ] 2. Feed the host from the producer boundary: after the durable commit of a completed/refused/failed invocation, record `ToolSample{CallID, ToolName, Arguments (canonical argsJSON already used by the loop policy), Result (bounded), Error (bounded diagnostic), Status, Reason, Effects}`. Refused pre-claim calls record `not_executed`. Operation-backed calls record at operation completion; work calls at their owner. No sample for calls that never dispatched.
- [ ] 3. Implement the adapter (`internal/runtime/extensions/nudge/adapter.go`): factory/extension/middleware per the contract; `prepare` extracts the contiguous trailing `schema.Tool` call IDs in request order, calls `host.Outcomes`, evaluates once per batch (memo keyed by the batch's first/last IDs), renders `decision.Notice.Text`, enforces 1024 bytes and remaining context budget, schedules only when not yet scheduled for that batch, appends the tagged transient message to a copied slice. The adapter never returns the loop-detected error; the core guard owns it (same sentinel path as baseline) so Service keeps its terminal classification.
- [ ] 4. Wire activation: `EngineConfig.NudgeFactory`; engine appends the middleware last only when present; `Service` binds a fresh host+detector per leg at run, approval/question resume and steering resume; `apply` the same code path in all three legs (no third variant). Remove the unconditional registration and the old state plumbing (git-level deletion completes in N03; do not leave both paths active).
- [ ] 5. Wire the App: one `runtime.NudgeFactory` value derived from the generated Assembly via `internal/app/assembly_loop_extensions.go`; set on the startup `EngineConfig` and on the settings-reload `RebuildEngine` config. Unselected assembly: nil factory, no handler, no error.
- [ ] 6. Durability/recovery:
  - schedule-before-injection test (event index < injected message index, and injected only after the event append succeeds);
  - append-failure test (model call fails with the append cause; no injection; run terminal honest);
  - provider-retry test (one event, identical text, no second schedule);
  - resume coverage test (approval/question resume for a multi-call batch whose siblings were journaled: coverage satisfied from `tool.finished`, replayed siblings excluded from the window, no stale notice);
  - steering-resume test (fresh detector; steering message injection unchanged);
  - cancel/terminal test (a waiting/failed leg releases the host; no panic; no duplicate schedule);
  - fresh-window test (duplicate tuple before resume does not count after resume).
- [ ] 7. Parity:
  - `tool.nudge` payload byte-compatible with `schemas/events/payloads/tool.nudge.json` (five required fields, `repeat_count` 3/5, `template_version` `nudge-v1`, no extra properties);
  - `tool.finished` typed outcome/reason/effects projection unchanged (D02/D03 owner);
  - generic UI/Inspect reads the event with no feature-specific handler; unselected build reads historical events passively (UI test or projection assertion);
  - assert no new config key, RPC action ID, control-action ID or UI toggle ships; keep the N03 negative guard green.
- [ ] 8. Purity/architecture checks: root `internal/runtime` package does not import `internal/runtime/extensions/nudge` or `plugins/exp/nudge` (only the generated Assembly does); `internal/app` does not import Eino or the EXP module; `TestEinoImportsQuarantined`/`TestPluginWindowCannotImportInternal` pass.
- [ ] 9. Focused checks (toolchain absent here — record honestly):
  - `go test ./internal/runtime/... -run 'Nudge|Loop|Adapter'`
  - `go test ./internal/app/... -run 'Assembly|Nudge'`
  - `go test ./plugins/exp/nudge/...`
  - `go test ./internal/runtime -race -run 'Nudge'` (race gate; record the pre-existing `EINO-TOOLSNODE-ERR-RACE` limitation if it still blocks)
  - `git diff --check`

## Consumed and produced contracts

Consumed: N00 slot types/closed Port/generator/app unwrap; N01 detector/templates/constants; D02/D03 producer-owned outcomes (`domain.ToolOperation` typed fields); D04 handoff-removal shape; existing `persistAndPublish`, `payloadToolNudge`, `EventToolNudge`, `errLoopDetected` terminal mapping.

Produced for N03: the live adapter/host path, activation in all three legs, parity tests, the durability behaviors above, and the exact list of core symbols/rows now unused so N03 can delete them without guessing.

## Verification commands and expected result

| Command | Expected |
| --- | --- |
| `go test ./internal/runtime/... -run 'Nudge'` | Adapter, leg host, ordering, retry, resume/steer durability and parity pass on the selected path. |
| `go test ./internal/app/... -run 'Assembly|Nudge'` | Selected wiring and nil/absent wiring pass; no Eino/EXP import. |
| `go test ./plugins/exp/nudge/...` | Policy unaffected by adapter integration. |
| `go run ./sdk/internal/cmd/generate-default ...` | Generated default unchanged while unselected; selected fixture build emits one factory field. |
| `go test ./internal/runtime -race -run 'Nudge'` | Race-clean for nudge paths, or the pre-existing upstream race is recorded with its exact failure. |

## Acceptance, rollback and handoff

Acceptance: a selected build produces the exact baseline coaching behavior (thresholds, templates, `tool.nudge` v1, hard stop, retry dedupe, schedule-before-injection) through the package, including approval and steering legs; an unselected build has no handler/state/producer and still reads historical events; no second execution path exists; correction of failed tool calls works regardless of selection.

Rollback: revert the Story; the deleted core behavior is still present until N03, so reverting N02 restores the old path exactly. No data change: `tool.nudge` and `tool.finished` schemas are unchanged, so existing journals remain valid.

Handoff: exact commit, adapter/host files and activation call sites, the durability/parity evidence, the unused-symbol inventory for N03, the Recipe used for the selected build, and every check that could not run with the reason. Return to N00 if the slot shape must change or to N01 if detector semantics must change; do not patch around either.

# N03: Remove the frozen core nudge implementation and prove the epic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Delete the superseded core nudge/loop-window implementation, move and carve its tests, and produce the evidence that selection is physical omission, coaching is still real, durability/recovery are preserved and no residue remains.

**Architecture:** After N02, the package and adapter own all coaching behavior and the count-6 stop. This Story removes the frozen core coaching bodies, the loop-window/loop-detected implementation that moved into the package, the consumer-owned handoff remnants and the unconditional engine registration, leaving one selected path and one unselected absence. The stop crosses the seam as `NudgeDecision.Stop` and is mapped once to the existing terminal; no core window remains. Tests are re-homed rather than weakened; production integration tests stay with the runtime. Documentation records the new ownership. No behavior change is allowed in this Story except deletion of now-unreachable code.

**Tech Stack:** Go 1.26.4; pinned Eino v0.9.13 inside runtime/provider only; existing SQLite/Postgres and event schemas; Recipe/generated-Assembly tooling; PowerShell `just` CI in the repository-supported environment.

**Spec:** [Complete deletion design](../2026-10-19-complete-deletion-plan.md) C11, §10.3, §11.1–11.5, §11.11; [NUDGE-DESIGN](../../architecture/NUDGE-DESIGN.md) §4–§7; [index](index.md). Outputs consumed from [N00](N00-seam-binding.md), [N01](N01-policy-extraction.md), [N02](N02-adapter-settings-recovery.md).

**Story:** N03 · **Epic:** E-NUDGE · **Requirements:** R1,R2,R3,R5,R6.

**Immediate predecessors:** N01, N02 (and thereby N00, D04, D02, D03, D06).

**Readiness gate:** Accepted N01/N02 with selected-path parity and durability evidence; the N02 unused-symbol inventory; implementation handoff. Current request authorizes planning only.

## Global Constraints

- Delete completely: implementation, construction, callers, state, configuration read, generated default wiring and feature-only tests. No disabled path, no-op wrapper, alias, renamed hash or retained "just in case" branch.
- Keep real behavior: thresholds, templates, `tool.nudge` v1, hard stop, schedule-before-injection, retry dedupe, resume coverage and fresh-per-leg semantics must remain exactly as N02 evidenced them.
- Keep the single `Service.Run` / Eino / Journal / policy and durable-operation path; no new runner/coordinator/registry.
- No schema or migration change. Historical `tool.nudge` rows remain valid and renderable in every build.
- Do not edit issue-91 Story docs or other epics; do not edit the peer-owned baseline re-anchor research file; do not reintroduce hashes.
- Only `internal/runtime/` and `internal/provider/` import Eino; `plugins/` never imports `agent-vivy/internal/`.
- Record exactly which checks ran; a skipped environment/real-path check is not passing evidence.

## Review Focus

- Deletion completeness: every symbol in the N02 inventory is gone, with no replacement copy left in core; `git grep` dispositions cover FNV, `loopWindow`, `nudgeState`, `nudgeBatch`, `nudgeNotice`, `renderNudge`, `takeCompletion`, `storeCompletion`, `restoreResumeNudgeBatch`, `nudgeEmitter`, `newNudgeMiddleware`, `withNudgeState`, `setNudgeState`.
- Correction is not gated on nudge: `classifyToolFailure`/soft conversion at the tool boundary behaves identically in selected and unselected builds.
- The hard stop is single-homed in the package: `Stop` maps once to `errLoopDetected` and `run.failed{cause_category:loop_detected}`; no second core window or iteration guard remains.
- Tests re-homed without weakening: moved window/template/notice cases keep their assertion values; production acceptance tests remain in `internal/runtime` and run against the selected generation.
- Physical omission is demonstrated, not asserted: inspect generated imports, `HasNudgeFactory`, `generation.Manifest.Modules`, dependency lists and artifact symbols for none/each/all variants.
- Regeneration is idempotent and generated files are not hand-edited.
- Docs and instructions no longer require a core implementation or describe hashes/handoff as operative.

## Baseline A mapping (202aac38, 2026-10-07)

| Delete / move | Baseline A path |
| --- | --- |
| Window + FNV + terminal sentinel | `internal/runtime/loopguard.go` (whole file after D04/N01 replacement; no copy may remain in core) |
| Coaching state | `internal/runtime/nudge_state.go` (393 lines: state, batch, notice, completed call, marks, window, Take/Seal/Register/Complete/SatisfyDurable/Abort, context key) |
| Boundary middleware | `internal/runtime/nudge_middleware.go` (`newNudgeMiddleware`, wrapped model, `trailingToolCallIDs`, `nudgeReminderMaxBytes`, emitter type/context) |
| Templates | `internal/runtime/prompt.go:100–118` (`nudgeFailureTemplate`, `nudgeRefusalTemplate`, `renderNudge`) |
| Consumer handoff remainder | `internal/runtime/service.go:2953–2956,3264–3283,3501–3560,4648–4656,4682+`; `internal/runtime/mapper.go` nudge pointer/completions store-take and `argsJSONFor` if now unused; `internal/runtime/queue.go:682–689` old state creation; `internal/runtime/engine.go:372–376` unconditional registration |
| Failure side channel | `internal/runtime/tool_failure.go` `markInvocationFailure` (deleted); `classifyToolFailure`/`refusalFailure`/vocabulary **stay** (tool-boundary ownership); `internal/runtime/tooladapter.go` nudge-presence gate removed, `markCommandFailure` kept |
| Tests to move | `internal/runtime/loopguard_test.go`, `loopdetect_test.go` -> package; `nudge_state_test.go` state-only cases -> adapter/leg tests, window cases -> package; `nudge_middleware_test.go` boundary cases -> adapter tests; `nudge_contract_test.go` test-local mirror -> deleted (its still-relevant call-ID/order/retry/cancel/compaction assertions move into N02's adapter/activation tests) |
| Tests to keep | `internal/runtime/nudge_acceptance_test.go` as the selected production-path acceptance (rename only if it clarifies scope), `tool_failure_test.go`, `enhanced_tooladapter_test.go`, `internal/mcphost/toolworld_test.go` |
| Docs | `docs/architecture/NUDGE-DESIGN.md` (ownership/selection, no FNV, no handoff, adapter boundary); `docs/archive/plans/nudge/README.md` (mark implemented design as historical, point at this epic); `docs/COMPLETE.MD` is a historical log — do not rewrite it |
| Events | `schemas/events/payloads/tool.nudge.json`, `run-event.schema.json`, `internal/domain/event.go` **unchanged**; `payloadToolNudge` unchanged |
| Generation | `internal/generated/assembly/zz_default.go` regenerated only through the generator; unselected default must not import the package |

## Files and ownership

- Delete: `internal/runtime/nudge_state.go`, `internal/runtime/nudge_middleware.go`, `internal/runtime/loopguard.go`, `internal/runtime/nudge_contract_test.go` (after moving assertions), plus the specific hunks/symbols listed above in `service.go`, `mapper.go`, `queue.go`, `engine.go`, `prompt.go`, `tooladapter.go`, `tool_failure.go`.
- Move: window/threshold/stop tests to `plugins/exp/nudge/`; state/boundary tests to `internal/runtime/extensions/nudge/` and `internal/runtime/`.
- Modify: `docs/architecture/NUDGE-DESIGN.md`; `docs/archive/plans/nudge/README.md`; `internal/runtime/nudge_acceptance_test.go` (selected-path harness binding); `internal/runtime/extensions/nudge/` test files; `internal/generated/assembly/zz_default.go` only via regeneration.
- Create: `docs/logs/2026-10-19-epic-nudge/` with `summary.md`, `verification.md`, `acceptance.md`, optional `rollback.md` (repository rule for a deliverable change).
- Do not touch: issue-91 docs, other epics, `docs/research/2026-10-19-baseline-a-reanchor.md`, `data/`, schemas, migration files.

## Tasks

- [ ] 1. Freeze the baseline evidence before deleting: capture selected and unselected build behavior (event payloads, reminder text, counts 3/5, hard stop, resume/steering outcomes) from N02's tests, and write the symbol inventory with owners (deleted / moved / retained-with-justification).
- [ ] 2. Delete the core coaching bodies and handoff remnants as one coherent change: state, middleware, window, templates, emitter, resume-batch restore, mapper parking, unconditional registration. Compile must fail only for missing references you are removing; do not leave `//nolint` or unused-but-exported shims.
- [ ] 3. Keep correction independent of selection: ensure the tool boundary still classifies allowlisted invocation failures, returns the bounded diagnostic as a model-visible result, records the typed durable outcome and maps refusals with `not_executed`; test selected and unselected builds with the same assertions.
- [ ] 4. Map the hard stop once: package `Stop` -> existing loop-detected sentinel -> `run.failed{cause_category:loop_detected}`; guarantee no duplicate guard (Eino `MaxIterations` remains the separate turn cap). Test the sixth identical call in the selected build and the continued-run behavior in the unselected build (no package guard, turn cap only).
- [ ] 5. Re-home tests without weakening: move window/threshold/tie-break/stop cases to the package; move boundary/retry/ordering/context-budget/tag tests to the adapter; keep production acceptance under `internal/runtime` using the real Service/Journal/tools and the selected Recipe. Delete the obsolete test-local mirror.
- [ ] 6. Update documentation: `NUDGE-DESIGN.md` records package ownership, selection (`vivy/exp-nudge`, T1, `core/nudge-factory@v1`), no FNV, no consumer-owned handoff, the Eino `WrapModel` boundary and the deletion of the core body; `docs/archive/plans/nudge/README.md` labels ND-0..ND-4 historical and points at this epic. No new normative claim about behavior that was not verified.
- [ ] 7. Build the selection matrix and inspect artifacts:
  - none: `recipes/minimal.vivy.yml` (and `lite`, `masks-*`) — generated Assembly has no nudge import/field; `HasNudgeFactory()==false`; `Manifest.Modules` lacks `vivy/exp-nudge`; no `tool.nudge` producer; historical event rendering still works.
  - each: `recipes/exp-nudge.vivy.yml` alone.
  - all: the owner-confirmed default-family selection (`default`, `headless`, `diva`, `vivy-code`, `scx`) with the other experiments as decided by their epics.
  - Regenerate each with `go run ./sdk/internal/cmd/generate-default ...` (default form) and the Recipe build path; run the generator twice and require a byte-identical second result.
- [ ] 8. Prove physical omission beyond the generated file: `go list -deps` for the unselected binary shows no `plugins/exp/nudge` or `internal/runtime/extensions/nudge`; selected shows exactly one; no `runtime_nudge`/`tool.nudge` producer symbol in the unselected build; all builds still parse and render historical `tool.nudge` events.
- [ ] 9. Add the permanent guards: a test that no nudge config key/RPC action/control-action ID/UI toggle exists (so `NOT_COMPILED` cannot be bypassed by a dead setting); a residue test/`git grep` disposition list (FNV, `loopWindow`, removed symbols) that fails if any returns; keep `TestEinoImportsQuarantined`, `TestPluginWindowCannotImportInternal` and a root-runtime "no EXP/adapter import" assertion.
- [ ] 10. Run the required gates in the repository-supported environment and record exact outcomes: `just ci` (build, test, vet, fmt-check, ui-ci, headless-compile, plugin-ci); a split-pair real-path smoke at `http://127.0.0.1:3015` only if the selected/unselected behavior is user-visible and the environment allows it (no provider key required path: scripted/fake provider as used by existing tests); Postgres gate if the change touches no storage (it should not — state this and do not claim it). Record every skipped check with its reason. `git diff --check` is mandatory even if the toolchain is absent.
- [ ] 11. Write `docs/logs/2026-10-19-epic-nudge/`: `summary.md` (what moved, what was deleted, what was not done), `verification.md` (commands/results/reasons), `acceptance.md` (how a human observes selected coaching vs unselected omission, including Inspect and the historical-event case), `rollback.md` (Git revert of the epic series; no data conversion; old journals remain valid).

## Consumed and produced contracts

Consumed: N02's unused-symbol inventory, durability/parity evidence and selected/unselected test harness; N01's moved behavior tests; N00's generation/absence contract; D02/D03's durable outcome ownership; D04's handoff-removal shape.

Produced: the deletion evidence, selection matrix, residue dispositions, documentation ownership update, permanent guards and the iteration log. No downstream Story consumes code from N03.

## Verification commands and expected result

| Command | Expected |
| --- | --- |
| `go test ./internal/runtime/... ./internal/app/... ./plugins/exp/nudge/...` | Selected-path acceptance passes; no references to deleted core symbols. |
| `go build ./...` (or `go vet ./...`) | Compiles with no nudge remnants in core. |
| `go run ./sdk/internal/cmd/generate-default ...` twice | Second run leaves the tree unchanged; unselected form has no nudge import/field. |
| `go list -deps` per Recipe build | none/each/all match the matrix; physical omission proven. |
| `git grep -n 'fnv\|loopWindow\|nudgeState\|newNudgeMiddleware\|restoreResumeNudgeBatch\|takeCompletion\|storeCompletion'` | Only dispositions (package/adapter/history docs) remain; no core implementation. |
| `just ci` | Green, or every failing/skipped part recorded with the real reason (environment/toolchain included). |
| `git diff --check` | Clean. |

## Acceptance, rollback and handoff

Acceptance: the selected generation still delivers the exact baseline coaching behavior and `tool.nudge` contract; unselected generations contain no implementation, produce no nudge events, keep generic correction and historical-event readability, and expose no dead setting; the core has exactly one fewer mechanism with its test/value coverage intact; hashes and the consumer-owned handoff are gone; documentation states the retained-experiment ownership truthfully.

Rollback: revert the epic commit series in reverse order (N03, N02, N01, N00). No schema, migration, event-contract or persisted-state change is involved; journals written by a selected build remain readable by an unselected build (the event is passive). Use Git only; never run an old executable on a converted store or discard new data silently.

Handoff: exact commit/build, the full changed/deleted path list, selected and unselected evidence, `just ci` and real-path outcomes (or exact reasons they did not run), residue dispositions, the Recipe matrix table, and unresolved limitations. If the deletion requires a contract change, return to the owning predecessor rather than shipping a divergent seam.

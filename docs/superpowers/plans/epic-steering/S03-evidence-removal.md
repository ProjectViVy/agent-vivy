# S03: Steering evidence, residue removal and handoff Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Final Story: run it only after S00–S02 are accepted, and keep the residue audit read-only until the removal list is proven.

**Goal:** Prove the selected and unselected species from built artifacts, remove only a steering-owned custom hash binding (if one exists) and proven duplicate steering setup, and hand off a residue-free implementation with an exact rollback boundary.

**Architecture:** The acceptance evidence is generated Assembly plus packed artifacts plus named runtime/RPC/UI tests. Removal is evidence-led: the Baseline A steering path has no Vivy-owned hash, so S03 first records the hash owners it must not touch (`checkpoint.go`, continuity admission, approval arguments), then removes only steering-specific bindings or duplication that S01/S02 left behind and that an equivalence test covers. No new abstraction is introduced to justify a deletion.

**Tech Stack:** Go 1.26.4; pinned Eino v0.9.13; `vivy-sdk` pack/inspect; existing just/Go/UI/TUI gates; Git-only rollback. No schema, provider, database or migration work.

**Spec:** [Complete deletion design §§10.3, 11.5, 11.7](../../../plans/2026-10-19-complete-deletion-plan.md); [S00](S00-seam-binding.md), [S01](S01-policy-extraction.md), [S02](S02-adapter-settings-recovery.md); [Index](index.md).

**Story:** S03 · **Epic:** E-STEERING · **Requirements:** SR1–SR6 (final recheck; SR5/SR6 primary).

**Immediate predecessors:** S02 accepted. External G2 (source/generation), G3 (C3/C6 hash-free approval/checkpoint, only if S03 is to claim shared-boundary removal), G4 (all-three Recipe/composition; otherwise that row is blocked). Baseline A `202aac38`.

**Readiness gate:** S00–S02 accepted; the final candidate commit/build is identified; no open contract deviation; selected and unselected artifacts can be built with G5. If S03 is asked to remove a hash owned by `checkpoint.go`/approval/continuity without G3, stop: that is another owner's contract, not a steering residue.

## Global Constraints

- Remove only steering-owned hash binding and proven duplicate steering setup. Never delete shared checkpoint, continuity-admission, policy or approval hashes, and never replace them with a different digest.
- One `Service.Run`/Eino/Journal/policy path; no second runner, queue, registry, discovery, init hook or no-op/disabled fallback may remain.
- Generic follow-up, cancel, approval/question recovery, queue rebuild/storage, budgets and terminal settle stay in core with unchanged behavior and payloads.
- Unselected species remains physically omitting the package/adapter and reporting `NOT_COMPILED`; selected keeps same-Run semantics and settings/UI/event parity.
- Evidence is tied to a concrete commit/build; do not claim a skipped test, a focused check, a planning doc or a textual grep as end-to-end acceptance.
- The all-three row is blocked until G4 exists. A blocked row is recorded as blocked, never as passing.
- Do not edit issue-91 docs, sibling experiment epics, or the peer `2026-10-19-baseline-a-reanchor.md`; no commit/push and no production data in this planning turn.

## Review Focus

- False deletion: removing a shared hash/field and calling it steering residue; or removing duplicate setup without an equivalence test.
- Residual path: old `Steer`/`takeSteerTurn`/`resumeSteered*` still reachable, a second profile, or a fallback that behaves like steering when the controller is nil.
- Omission failure: packed minimal artifact imports the package/adapter or allocates steer state; packed selected artifact behaves like a no-op or omits a control/event.
- Evidence inflation: "all-three" claimed without G4, or an `inspect-artifact` output not tied to the packed output path.
- Recovery regression: unselected clear/cancel losing persisted steer text, or auto-applying it.
- Cross-contract break: event payloads, queue-state keys, Face/TUI types or RPC errors changed incompatibly in the final pass.

## Baseline A mapping

| Baseline fact | S03 handling |
| --- | --- |
| `internal/runtime/queue.go:3-13` imports `crypto/rand`, `encoding/hex`, `encoding/json`, `adk`, `schema`; no hash package. | Record: no steering-owned hash producer at Baseline A. The random queue ID is not a hash binding. |
| `internal/runtime/queue.go:45-51,366-405` `sessionQueue`/`takeSteerTurn` have no digest field; `payloadTurnSteered` is `{queue_id,text}`. | After S01, verify the Host drain keeps this shape; do not add a digest for recovery or dedup. |
| `internal/runtime/checkpoint.go:41,113-115,126-129` checkpoint envelope checksum; `internal/runtime/service.go:976-1010,1425-1436` continuity request hash; `:3794-3800` approval arguments hash; `internal/runtime/payloads.go:19,54,132,148,302,307,322,331` policy/context hashes. | These are shared C3/C6 owners (D03/D08), not steering. Record as out of scope; remove only if G3 lands and the owning Story authorizes it, never as part of S03. |
| `internal/runtime/service.go:2963` `var steerCancel adk.AgentCancelFunc` immediately reassigned by the same-scope `:=` at `:2964`. | Delete the redundant declaration as a proven duplicate setup with no behavior change. |
| `internal/runtime/queue.go:666-691` steered-leg nudge state/emitter construction; `service.go:2955-2956` initial leg and `:4646-4657` approval-resume leg do the same. | Only remove if S01's Host reuses one leg-context builder and an equivalence test covers initial, approval and steering legs; otherwise record the shared leg setup as outside this epic. |
| `internal/runtime/queue.go:168-226` refusal demote block; `:512-541` demote/consume helpers; `:407-510` follow-up drain/materialize also move steer items. | After S01, audit each steer-lane move: one owner per trigger, equivalence test for any deletion. Keep the settle-time follow-up rule. |
| `internal/runtime/service.go:3568-3597` two `steerPendingCancel` call sites (checkpoint interrupt and generic error). | Keep both; they cover different error shapes. Not a duplicate. |
| `sdk/internal/cmd/generate-default/main.go:49-70` duplicate external table (S00 replaced with one shared table). | Verify no second table returned; regeneration is deterministic. |
| `recipes/` has no steering Recipe at Baseline A. | S00 adds `exp-steering.vivy.yml`; S03 packs it and minimal; all-three consumes G4. |
| `internal/generated/assembly/zz_default.go:49-128`. | Regenerate and diff; never hand-edit. |

## Files and ownership

- Modify only after audit: `internal/runtime/queue.go`, `internal/runtime/service.go`, `internal/runtime/steering_host.go`, `internal/runtime/extensions/steering/adapter.go`, `internal/app/assembly_steering.go` (dead wiring only), `internal/generated/assembly/zz_default.go` (regenerated), `recipes/exp-steering.vivy.yml` (only if S00's content needs a correction proven by the artifact).
- Create: `docs/logs/2026-10-19-epic-steering/{summary,verification,acceptance}.md` at implementation time (not by this planning turn); optional `notes.md`/`rollback.md`. Use the repository iteration-log convention.
- Do not edit: `internal/runtime/checkpoint.go`, `internal/domain/*`, `internal/rpc/control.go` (unless a proven S02 defect; then revise S02), `ui/`, `sdk/ui`, `sdk/tui`, `sdk/port/loopexp`, `internal/moduleport`, `sdk/internal/assembly` (unless a proven S00 defect), issue-91 docs, sibling epics, the peer re-anchor file.
- Evidence artifacts (pack outputs, temporary DBs) live outside the repository or in an ignored `dist/` path; remove them before handoff. Never commit `data/`.

### Consumed and produced contracts

- Consumes: accepted S00 binding, S01 Host/controller, S02 capability/recovery matrix; G3 hash-free owner contracts; G4 all-three composition.
- Produces: removal list with equivalence evidence; deterministic generation diff; pack/inspect evidence for none/each (all-three blocked or passing); residue audit; final handoff with rollback boundary.

## Tasks

### Task 0: Pin the candidate and freeze the audit scope

- [ ] Record the exact commit, branch, Go/Eino versions and generated-file digest-free identity (commit + file diff, not a new hash) in the implementation log. Do not start from an uncommitted tree.
- [ ] Re-read the S00–S02 handoffs and list open deviations. A deviation is fixed in its owning Story, not patched in S03 unless the fix is a one-line evidence correction and the owning Story is revised.

### Task 1: Hash audit and removal

- [ ] Search the steering surface for any digest/binding: `plugins/exp/steering/**`, `sdk/port/loopexp/**`, `internal/runtime/extensions/steering/**`, `internal/runtime/steering_host.go`, the generated steering binding, any digest field on queued turns or recovery payloads, and `recipes/exp-steering.vivy.yml`. Expected: none.
- [ ] Record the shared hash owners from the Baseline A table and explicitly mark them out of scope. If G3 has landed and its owner removed them, record that as imported evidence, not as S03's deletion. The concurrent [Baseline A re-anchor audit](../../../research/2026-10-19-baseline-a-reanchor.md) §2.6 and §4.4 independently records that no steering hash exists in `queue.go`, `turn_queue.go` or the service steering paths; cite it as corroboration, not as runtime proof.
- [ ] If a steering-owned hash binding is found (for example a descriptor `sha256` left from before G2, or a queue/recovery digest introduced by S01), remove it and its test in the same change; prove the artifact still packs and the selected behavior is unchanged. If the binding is shared, stop and return it to the owner.
- [ ] If a `sha256`/`checksum`/`digest`/`fingerprint` field remains in the steering descriptor or generated binding, S03 is not accepted even if tests pass.

### Task 2: Proven duplicate-setup removal

- [ ] Delete the redundant `var steerCancel` at `internal/runtime/service.go:2963`; run the focused resume/boundary test set to prove no behavior change.
- [ ] Audit the steered-leg context setup (`queue.go:682-688` at Baseline A) against the Host after S01. If the Host now builds the leg context once, remove only the old copy and add/adjust an equivalence test covering initial run, approval resume and steering resume. If the Host still relies on the separate copy, record it as required setup and do not delete it.
- [ ] Audit each steer-lane move (enqueue refusal, pending-cancel drain, settle-time follow-up, rebuild/materialize). Keep one owner per trigger; delete only a redundant move that an equivalence test proves unobservable. Keep the two error-branch `steerPendingCancel` call sites.
- [ ] Audit `internal/app/app.go` for dead selected/unselected wiring left by S01/S02; remove only code with no reachable path, not the capability gate.
- [ ] No new helper/abstraction is added solely to enable a deletion. If a deletion needs a new seam, defer it and record why.

### Task 3: Deterministic regeneration

- [ ] Run the repository's generation command (`go generate ./internal/generated/assembly` or the exact `sdk/internal/cmd/generate-default` invocation from S00), then run it a second time and confirm the file is byte-identical and matches the committed generated file.
- [ ] Confirm the generated default still selects only the default Recipe's modules; no steering import/field appears there unless the default Recipe was intentionally changed (it must not be in this epic).
- [ ] Confirm the single build-owned source table is still the only place the package path appears; no second list was reintroduced.

### Task 4: Pack and inspect the selection matrix

- [ ] Build the packer (`go build -o vivy-sdk.exe ./sdk` or the repository's SDK build target), pack the unselected composition from `recipes/minimal.vivy.yml` to a fresh output path, and run `vivy-sdk inspect-artifact <output>`. Record: module list, typed-binding/capability projection, and the generated imports. Expected: no `vivy/exp-steering`, no adapter import, `NOT_COMPILED` projection, generic queue/follow-up present.
- [ ] Pack `recipes/exp-steering.vivy.yml` from baseline minimal dependencies plus the experiment. Record: package and adapter present exactly once, factory binding present, full steering capability, no duplicate constructor or second queue/runner.
- [ ] Pack the all-three composition from `recipes/exp-loop-features.vivy.yml` only when G4 exists. If G4 is absent, record the row as blocked with the exact missing artifact and do not substitute a locally invented Recipe.
- [ ] Inspect each artifact's generated source path and module manifest; do not infer omission from the Recipe text alone. A pack that fails must be reported with the failing stage, not bypassed.
- [ ] Run the unselected artifact's real-path smoke: start it against a deterministic provider fixture (no live network), create a session, start a run, send a follow-up, cancel it, and confirm no steer control/capability. For the selected artifact, repeat and exercise Enter-steer at a turn boundary with a fixture model/tool sequence; record one RunID and the per-item markers.
- [ ] Remove pack outputs, temporary databases and SDK binaries after recording evidence.

### Task 5: Residue and contract audit

- [ ] Run `rg -n "Steer|steer" internal/runtime internal/app internal/rpc` and classify every hit: generic queue/Host/event/RPC names, or removed policy logic. Core must contain no `Route`/`Take`/`Render` policy implementation.
- [ ] Run `rg -n "steerArmed|steerCancels|steerPendingCancel|resumeSteered|takeSteerTurn|demoteSteer" internal/runtime`; every remaining symbol is in the Host (or a documented generic helper), with one owner and no duplicate path.
- [ ] Run the import-quarantine tests and `rg -n "cloudwego/eino" --glob '*.go'`; only `internal/runtime`/`internal/provider` (including their subpackages) and generated/lifecycle files that legitimately import them are allowed. `plugins/exp/steering`, `sdk/port/loopexp` and `internal/runtime/extensions/steering` must not import Eino directly unless the final contract explicitly permits the adapter; record the decision.
- [ ] Run `rg -n "sha256|checksum|digest|fingerprint" plugins/exp/steering sdk/port/loopexp internal/runtime/extensions/steering recipes/exp-steering.vivy.yml`; expected empty except a documented shared contract outside the steering surface.
- [ ] Confirm no `queue_steer.go`, no `init()` registration, no directory scan, no plugin discovery, no `NOT_COMPILED` no-op, and no baseline fallback branch.
- [ ] Confirm event payloads and queue-state keys are unchanged (`payloadTurnQueued/Dequeued/Steered`, the eight baseline keys plus only the S02 additive fields); confirm no new config key/schema migration.
- [ ] `git diff --check`; check local Markdown links added by this epic; ensure the iteration log records commands and outcomes, including skipped/blocked checks and why.

### Task 6: Final acceptance matrix

- [ ] Same-Run: selected steering resumes `checkpointIDFor(runID)`, the RunID is unchanged, no sibling Run is created, one joined injected user message is visible, one `turn.steered` per drained item and one transcript row per item, and the cancel mode is `CancelAfterChatModel`.
- [ ] Exactly-once/drain: `all` drains all, `one-at-a-time` drains one, refusal reclassifies the same ID, resume failure demotes then terminates without loss; no double append.
- [ ] Generic core: follow-up admission, user cancel, approval/question resume, queue rebuild and terminal settle behave as at Baseline A in both species.
- [ ] Omission: unselected artifact has no package/adapter import, no state, no control, explicit `NOT_COMPILED` for steer/mode, recovery-required for persisted steer turns, restorable text.
- [ ] Selected parity: UI/TUI controls, queue state, modes, events and recovery behave as specified; no hardcoded capability list.
- [ ] Eino capability check is current: the adapter still uses the named native APIs, no duplicate Runner/queue, and the recorded gap/migration boundary still holds. Reject any unexplained parallel implementation.

## Verification commands and expected result

Future gates, not executed by this planning turn:

```text
go generate ./internal/generated/assembly
git diff --exit-code internal/generated/assembly/zz_default.go
go build -o vivy-sdk.exe ./sdk
./vivy-sdk.exe pack --recipe recipes/minimal.vivy.yml --output dist/steering-none
./vivy-sdk.exe pack --recipe recipes/exp-steering.vivy.yml --output dist/steering-each
./vivy-sdk.exe inspect-artifact dist/steering-none
./vivy-sdk.exe inspect-artifact dist/steering-each
go test ./internal/runtime ./internal/rpc ./internal/app -run 'TestSteering|TestQueue|TestTurnSteer|TestCapabilities|TestEinoImportsQuarantined' -count=1
go test ./sdk/tui/... -count=1
cd ui && pnpm vitest run src/components/chat/ChatInput.queue.test.tsx src/lib/store.test.ts src/lib/ui-sdk-face-compat.test.ts
just ci
```

Expected: second generation is byte-identical; both packs succeed; `steering-none` has no experiment import/binding and reports NOT_COMPILED; `steering-each` has one binding and full capability; the all-three row is either passing with G4 or explicitly blocked; same-Run/exactly-once/recovery tests pass; residue greps are clean; `just ci` green; real-path smoke recorded. A focused pass is iteration evidence, not a waiver for the final CI gate or the real-path smoke.

## Acceptance, rollback and handoff

S03 is accepted when the candidate build proves none/each (and all-three only with G4), the steering path has no Vivy-owned hash, only the proven duplicate setup was removed with equivalence evidence, the unselected species is physically omitting and explicit, the selected species retains same-Run steering and full parity, and the residue audit is clean with shared owners untouched.

Rollback: revert S03's commit only. If a duplicate removal regresses, restore that one deletion and keep the evidence; do not resurrect a second steering path. Epic rollback reverts S03 then S02 then S01 then S00; the unselected baseline is unaffected because the package is absent by default. No production data/schema rollback is needed.

Handoff must include: candidate commit/build, exact changed files, the removal list with why each item was steering-owned/proven duplicate, deterministic generation evidence, both pack outputs and inspect evidence (all-three status), the same-Run and recovery matrix results, residue/import audit, UI/TUI test results, `just ci` result, real-path smoke evidence, remaining limitations, and the rollback boundary. Do not claim completion from a planning diff.

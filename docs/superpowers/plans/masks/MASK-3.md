# MASK-3 immutable admission and prompt integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development only when that method is selected. Steps use checkbox syntax for tracking.

**Goal:** Every primary run captures durable authoritative instructions once and resumes them unchanged.
**Architecture:** One Core Storage admission transaction and the existing Eino Runner; native BeforeAgent assigns per-execution instruction and WrapModel guards final message bytes.
**Tech Stack:** Go, Eino v0.9.13, existing checkpoints/context meter, embedded Markdown.
**Spec:** [design](../../specs/2026-09-21-mask-subsystem-design.md), baseline `5253f77`/`a0f892c`, [MASK-C1](contracts.md).
**Epic / requirements:** MASK-43 / M1, M2, M3, M6, M7, M8. State/dependencies: [index](index.md).

## Execution status

Implementation is complete on `feat/issue43-mask-system`; local Go verification
is complete. In addition to `df4f369`, `de1da98`, and `5305151`, the branch
contains `18e54db` (sealed App admission gate), `8270327` (canonical snapshot
bytes), `a6ca51` (legacy instruction reservation), `e3544be` (run-linked edit
marker migration and symmetric retry comparison), and the runtime resume fix in
that commit. Static checks, JSON validation, `gofmt`, and the full Go test suite
pass with the workspace-local Go toolchain. `just ci`, browser smoke, and
live-model release checks remain pending because their tooling/credentials are
not installed.

## Global Constraints

- Persist snapshot/message/run/start/edit marker before any model call or visibility.
- Snapshot includes full rendered instruction, not only mutable mask ID.
- Prompt-v1 resume is same-Generation only; no current-catalog fallback.
- Byte limit is UTF-8 message content, not exact provider tokens/tool-schema budget.
- Native Eino APIs stay quarantined; no second runner or provider implementation.
- No persona backend, memory database, model overrides or child mask inheritance.

## Review Focus

- Edit admission failure must not hide old conversation history (Task 2).
- An absent snapshot cannot downgrade a new run to legacy behavior (Task 3).
- Resumed tools must not execute before snapshot compatibility checks (Task 3).
- A tool-followup/compaction call must keep exactly one admitted persona/mask (Task 3).
- Middleware-injected AGENTS.md can exhaust a previously fitting budget (Task 3).

## Task 1: Markdown composer and immutable prompt values

**Files:** Modify `internal/runtime/prompt.go`, `prompt_test.go`;
NEW `internal/runtime/prompt_state.go`, `prompt_state_test.go`;
NEW `internal/runtime/prompts/runtime.md`, `persona-default.md`, `configuration.md`,
`code-mode.md`, `memory.md`, `project-instructions.md`, `dynamic-context.md`.
Module frame/bodies remain MASK-1 assets; no Runtime embed of optional Module tree.

**Consumes:** mask.Capture and Service.PromptAssets, storage.RunPromptSnapshot,
existing Face, existing composeRunPreamble/date/notebook behavior.
**Produces:** PromptInput/buildPromptSnapshot and owned context values from MASK-C1.

- [ ] Re-read any newly landed canonical Markdown assembler and reuse it if present.
  On inspected baseline extend prompt.go directly; no new template framework.
- [ ] Add meaningful input fixtures: empty mask => no wrapper; fallback persona once;
  supplied persona replaces only fallback; braces/quotes stay literal; FaceCode rule
  remains independent; absent memory/project instructions add no filler.

```text
snapshot = buildPromptSnapshot({run:R, generation:G, persona:default,
    capture:{selection:{mask_id:"",revision:0}, mask:nil}, face:web})
assert decoded Instruction contains default identity once
assert Instruction has no mask wrapper and no "no memory" filler
snapshot2 = buildPromptSnapshot(same input with supplied persona P and mask M)
assert P appears once, default absent, quoted M body present
assert snapshot2 digest equals SHA256(snapshot2 payload bytes)
```

- [ ] Run `go test ./internal/runtime -run 'MaskPrompt|Prompt' -count=1` for red evidence.
- [x] Move existing literal prose to Markdown without losing tool-approval and code
  rules. Use fixed host-owned placeholders only, json.Marshal for untrusted body/name,
  and deterministic ordered assembly. No prompt body enters template evaluation.
- [x] Capture persona/mask/framing provenance and canonical serialized payload.
  Empty Generation identity when snapshots are enabled is an error, not a made-up ID.
  Return immutable copies. Keep source metadata out of default user transcript.
- [ ] Add bounded private optional memory value tests without wiring a backend.
  Inspect native agentsmd framing before changing anything: augment only missing
  scope/permission semantics; no second wrapper around the same file contents.
- [ ] Rerun prompt fixtures and commit `refactor: compose authoritative prompt from markdown`.

## Task 2: Atomic ordinary/edit admission and capture validation

**Files:** NEW `internal/storage/sqlite/run_admission.go`, `run_admission_test.go`,
postgres equivalents; modify both `history_mutations.go` transaction helpers;
modify `internal/runtime/service.go`, `rewind_service.go`;
NEW `internal/runtime/mask_admission_test.go`; modify storage conformance tests.

**Consumes:** MASK-2 stores/locking and Task1 snapshot; MASK-C1 RunAdmissionStore.
**Produces:** both RunAdmissionStore implementations and one ordinary/edit write path.

- [ ] Write rollback fixture that injects failure at snapshot/start/message/edit write
  boundaries and asserts no new run/message/truncation is visible, model calls=0.
  Extend existing `TestEditSessionCommitsReplacementAndRunTogether` rather than
  discarding its historical atomicity guarantees.
- [ ] Write capture-vs-switch/update race with explicit barriers (no sleeps): capture
  revision1, update before commit, expect revision_conflict and zero admission writes.
  Commit first then edit catalog: admitted bytes remain old, next admission uses new.
- [ ] Run `go test ./internal/storage/sqlite ./internal/runtime -run 'MaskAdmission|EditSession' -count=1`
  for meaningful red behavior.
- [x] Implement the backend operation using existing insert helpers and one transaction:

```text
validate message/run/start bindings and snapshot SHA/schema
begin write transaction
lock session; validate captured selection revision when ExpectedMask != nil
lock custom definition if selected; validate revision/digest
if run ID already exists: compare complete admitted payload, return original or conflict
insert optional edit marker, message/attachments/file contexts, run
insert immutable prompt row; append start with assigned sequence; active status
commit; return assigned start event
```

- [x] Change runPersistence to carry RunAdmission (or remove it in favor of optional
  edit marker) so EditSession cannot bypass snapshots. Keep permission/workspace and
  existing session serialization. Release created workspace resources on precommit
  failure using existing cleanup; never publish a provisional run event.
- [x] First-party App uses RunAdmissionStore for all new primary runs. Capability
  absent means explicit unmasked Prompt and no ExpectedMask; no stored selection
  rewrite. Legacy embedders alone may retain their documented no-snapshot path.
- [ ] Assert fork/rewind semantics from MASK-2 still hold; a masked fork's next run
  has an independent captured selection. Child hint path remains unchanged.
- [ ] Run both backend conformance with disposable Postgres and runtime admission
  tests. Commit `feat: atomically admit immutable run prompt snapshots`.

## Task 3: Native instruction, final budget and recovery integration

**Files:** Modify `internal/runtime/engine.go`, `service.go`, `checkpoint.go`,
`checkpointadapter.go`, `compaction_middleware.go`, `compaction_service.go` only at
prompt propagation seams; NEW `internal/runtime/prompt_middleware.go`,
`mask_checkpoint_test.go`, `mask_budget_test.go`;
modify `checkpoint_test.go`, `agentsmd_test.go`, existing compaction fixtures.

**Consumes:** persisted snapshots, withRunPrompt/loadRunPrompt, native ADK hook signatures.
**Produces:** exact first/continuation/resumed model input, fail-closed snapshot checks.

- [ ] Inspect pinned upstream BeforeAgent, WrapModel and checkpoint APIs again if
  dependency pin changed. Record the exact version/source; API naming alone is no proof.
- [ ] Build a capturing fake domain model through the existing model adapter. It
  records outbound messages for first call, tool followup, interruption and resume.
  Test two concurrent sessions and catalog edits while one is suspended.

```text
admit R with mask body A; fake model requests approval-protected tool
persist checkpoint; edit catalog body to B; restart Service with same Generation
resume R; assert all R requests contain A exactly once, never B
assert a new run captures B; other session's mask never appears in R
```

- [ ] Run `go test ./internal/runtime -run 'MaskCheckpoint|MaskBudget|MaskIsolation' -count=1`
  for red evidence before adapter changes.
- [x] Implement BeforeAgent assignment (not append) from immutable context. Keep shared
  Engine instruction unchanged; restore snapshot context in both drive and resumeRun,
  which currently rebuilds a background context. Reload by run ID before any resumed
  tool continuation, not just before a model call.
- [x] Add snapshot metadata to Vivy's envelope Set/Get and validate schema/digest/run/
  composer/Generation. Old start without marker + old envelope follows legacy behavior;
  marker + missing/mismatched envelope/snapshot fails. Module-omitted masked resume
  fails and remains suspended. No opaque Eino checkpoint mutation.
- [x] Native WrapModel guards both Generate and Stream as the last/innermost handler.
  Use existing projectedContextBytes semantics on actual final messages. Existing
  MaxContextBytes<=0 keeps its documented unlimited behavior, rather than inventing
  a new limit. Reserve fixed instruction before selecting history when limit>0.
- [ ] Test final injected AGENTS/skills overage, fixed instruction overage, exact
  boundary, multibyte text, stream/nonstream parity, compaction and post-tool calls.
  Assert overage invokes model zero times and never truncates mask/persona.
- [ ] Confirm checkpoint messages don't duplicate or replace the captured Instruction;
  BeforeAgent running on Resume is not sufficient. Any failed upstream invariant
  blocks this Story until the same native seam is corrected.
- [ ] Run `go test ./internal/runtime -run 'Mask|Prompt|Checkpoint|AgentsMD|Compaction|EditSession' -count=1`,
  full affected storage suites, import quarantine and `just ci`. Commit
  `feat: preserve admitted mask prompts across native runtime resume`.

## Acceptance and handoff

Return AC-04 through AC-07 evidence and same-Generation restart capture. Include
failure-before-tool proof, not just model input screenshots. Module is still not
release-complete until UI/recipe/Inspect acceptance. Do not weaken incompatible
resume rules or silently drop an oversized mask to pass. Live behavioral evaluation
is distinct from deterministic fixture results and recorded under AC-10.

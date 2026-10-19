# N01: Extract the nudge coaching policy into plugins/exp/nudge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Create the species-selected experimental Module `vivy/exp-nudge` at `plugins/exp/nudge/` with the real, pure coaching policy — ten-entry exact-tuple window, thresholds 3/5, count-6 hard stop, fixed templates, 1024-byte reminder — and select it through `recipes/exp-nudge.vivy.yml`.

**Architecture:** A repository Module in the root Go module (no separate `go.mod`, like `plugins/hello-fs`) that imports only `agent-vivy/sdk/module` and `agent-vivy/sdk/port/loopexp`. It provides `core/nudge-factory@v1` as a build-owned T1 source and returns a pure `NudgeDetector`; it has no Eino, Journal, storage, policy, model, credential, network or `agent-vivy/internal/*` access, and performs no I/O. The behavior is moved from the D04-simplified core policy — not re-invented — so templates, ordering, and terminal semantics stay identical.

**Tech Stack:** Go 1.26.4; `sdk/module`, `sdk/port/loopexp`; no Eino (pure package); existing YAML Recipe/generation tooling.

**Spec:** [Complete deletion design](../../../plans/2026-10-19-complete-deletion-plan.md) C11, §10.3, §11.1–11.5; [NUDGE-DESIGN](../../../../docs/architecture/NUDGE-DESIGN.md) §4–§7 for the behavior contract; [index](index.md) owns status. Seam contract from [N00](N00-seam-binding.md).

**Story:** N01 · **Epic:** E-NUDGE · **Requirements:** R1,R2,R5,R6.

**Immediate predecessors:** N00.

**Readiness gate:** Accepted N00 output (pure contract, closed Port, generator slot, T1 source-catalog entry) and accepted D04 output whose core policy is the exact-tuple version this Story moves. Current request authorizes planning only.

## Global Constraints

- English docs and identifiers; no placeholder text, no stub provider, no "TODO policy".
- Package purity: no `github.com/cloudwego/eino*`, no `agent-vivy/internal/*`, no `os`/`net` I/O, no logging, no mutable global state.
- One coaching policy: the package replaces the core body in N03; do not leave two implementations.
- Preserve exact behavior: window length and tuple fields, thresholds, tie-break, template text, template version, 1024-byte and context-budget rules, and the count-6 stop condition that crosses the seam as `Stop`.
- Do not change `tool.nudge` schema, event vocabulary, daemon behavior or any storage.
- No hash: no FNV, SHA or any digest in the policy or descriptor identity beyond the existing generation/source mechanism.
- No new dependency; no Recipe other than the package's own selection file plus the explicit selections recorded in [index.md](index.md).

## Review Focus

- Tuple identity is the exact canonical `(toolName, arguments, result, error)` quartet; no concatenation ambiguity, no hashing, no normalization at evaluate time.
- Window memory is bounded to ten references; successful calls advance an empty tuple and never produce a notice; failures without a typed status never produce a notice.
- Notice selection: only counts 3 and 5; highest eligible count wins; tie breaks to the earliest request position; status selects the refusal vs corrective template; rendered length is checked against 1024 bytes.
- `Stop` is returned exactly when the identical tuple count exceeds five, so the existing `loop_detected` terminal still fires once.
- The module descriptor claims only the closed Port, no grants, `scope: generation`; it cannot affect selection, capability or trust.
- Tests moved from core stay behaviorally identical (no relaxed assertions to make extraction pass).

## Baseline A mapping (202aac38, 2026-10-07)

| Behavior | Baseline A source (to move) |
| --- | --- |
| Ten-step window, repetition limit 5, terminal | `internal/runtime/loopguard.go`: `loopWindow.record(toolName,argsJSON,result,errMsg) (int,error)`; `loopWindowSteps=10`; `loopRepeatLimit=5`; `errLoopDetected` when `count > 5`. D04 replaces the FNV `uint64` signature with an exact tuple; this Story consumes that shape. |
| Batch/notice semantics | `internal/runtime/nudge_state.go`: `nudgeBatch` (request order, remaining, calls, satisfied, implicit), `nudgeNotice`, `nudgeTemplateVersion`, `sealLocked` candidate selection, `Take` retry identity, `MarkFailure`/`Failure` typed marks. |
| Failure vocabulary | `internal/runtime/tool_failure.go`: `recoverable`/`refused`; reasons `invalid_arguments`, `not_found`, `policy_denied`, `user_denied`, `command_failed`, `remote_tool_error`; effects `not_executed`/`none`/`unknown`. The package must compare status strings supplied by the runtime, not classify errors. |
| Templates and reminder bound | `internal/runtime/prompt.go:100–118` (`nudgeFailureTemplate`, `nudgeRefusalTemplate`, `renderNudge`); `internal/runtime/nudge_middleware.go:21–22,103–108` (`nudgeReminderMaxBytes=1024`, context-budget branch). |
| Tests to move | `internal/runtime/loopguard_test.go`, `loopdetect_test.go` window cases; `nudge_state_test.go` window/notice/template cases (state-only parts stay for N02's adapter context); `nudge_middleware_test.go:510–522` template assertions. |
| Module/Recipe precedents | `plugins/hello-fs/{plugin.go,descriptor.go,vivy-module.yaml}` (root module, `module.Module` + `module.Instance`, descriptor/`Source`); `recipes/minimal.vivy.yml` (`apiVersion: vivy.generation/v1`, `modules:` list); T1 catalog entry from N00 (`sdk/internal/frontend_v1.go` `repoSourceDirs`). |
| No experiment precedent | `plugins/exp/` and `recipes/exp-*.vivy.yml` do not exist; this Story creates the first one. |

## Files and ownership

- Create:
  - `plugins/exp/nudge/vivy-module.yaml` — `apiVersion: vivy.module/v1`; `module: {id: vivy/exp-nudge, version: 0.1.0}`; `source: {ref: repo:plugins/exp/nudge}` (plus `sha256` only until D06 removes it; after D06 the ref alone is authoritative); `provides: [{port: core/nudge-factory@v1, id: vivy.nudge-factory}]`; `requires: []`; `requestedGrants: []`; `lifecycle: {scope: generation}`.
  - `plugins/exp/nudge/descriptor.go` — matching `module.Descriptor` (same fields as the YAML, `Source.Ref` identical).
  - `plugins/exp/nudge/module.go` — `New() module.Module`; `Construct` returns the module instance; `module.Instance` Start/Ready/Stop/Close are real no-ops for a pure policy (documented, not a provider placeholder).
  - `plugins/exp/nudge/policy.go` — `Factory() loopexp.NudgeFactory`; detector with the ten-entry exact-tuple window (`[10]sample`/slice of references), `Evaluate`, candidate selection, `Stop`; imports only `sdk/module` and `sdk/port/loopexp` (plus stdlib `strings`/`strconv` if unavoidable).
  - `plugins/exp/nudge/templates.go` — the two fixed templates and `render`; the rendered text and `TemplateVersionNudgeV1` come from the SDK contract constants.
  - `plugins/exp/nudge/policy_test.go`, `templates_test.go`, `descriptor_test.go` — moved/adapted behavior tests.
  - `recipes/exp-nudge.vivy.yml` — `apiVersion: vivy.generation/v1`, `profile: exp-nudge`, `modules:` at minimum `vivy/loop`, `vivy/model`, `vivy/tool-host`, `vivy/storage`, `vivy/checkpoint`, `vivy/credential`, `vivy/sandbox`, `vivy/exp-nudge` (mirrors `minimal.vivy.yml` plus the experiment, so the selected build is a real runnable species form).
- Modify:
  - `recipes/default.vivy.yml`, `headless.vivy.yml`, `diva.vivy.yml`, `vivy-code.vivy.yml`, `scx.vivy.yml` — add `vivy/exp-nudge` only after owner confirmation of the selection policy (index open item 1); `minimal.vivy.yml`, `lite.vivy.yml`, `masks-*.vivy.yml` stay unselected.
  - `sdk/internal/cmd/generate-default/main.go` — add the `expnudge.New().Descriptor()` external entry and import (only if the default Recipe selects the package; the generator must compile, so this lands with the Recipe change).
  - `docs/architecture/NUDGE-DESIGN.md` — a one-line ownership note pointing at this package; full revision is N03.
- Do not touch: `internal/runtime/*` behavior (N02/N03), generated `zz_default.go` by hand, `schemas/`.

## Tasks

- [ ] 1. Record the D04-accepted core policy file paths and copy the *behavior*, not the hashing: confirm the window stores exact tuples after D04; if it still hashes, stop and return to N00/D04 rather than duplicating the window.
- [ ] 2. Create the module identity: descriptor + `vivy-module.yaml` with the exact fields above; the module claims only `core/nudge-factory@v1`, no grants, generation scope. Test: descriptor/YAML agreement and Port/ID values.
- [ ] 3. Implement the detector:
  - `Evaluate(orderedFresh []loopexp.ToolSample) loopexp.NudgeDecision` receives exactly the fresh batch in native request order.
  - Append one sample identity per call; the window keeps the last ten identities; count exact matches by field-by-field equality of `ToolName`, `Arguments`, `Result`, `Error` (no string concatenation, no hash, no trimming).
  - A successful sample (empty `Status`) advances an empty identity and produces no notice.
  - Count == 3 or 5 with a non-empty typed `Status` yields a notice for that call, using the highest eligible count and earliest request position as tie-break; the notice carries `CallID`, `ToolName`, `Status`, `Reason`, `Count`, `TemplateVersionNudgeV1`, and the rendered `Text`.
  - `Stop = true` exactly when the matching count exceeds five; no notice is needed for the stop itself.
  - No retention of `ToolSample` beyond the window (store copied identity strings only).
- [ ] 4. Implement templates exactly as baseline (`nudgeFailureTemplate`/`nudgeRefusalTemplate` text, `%d` count substitution, refusal selection by `Status=="refused"`), expose the 1024-byte ceiling check result honestly (the adapter enforces the ceiling; the package tests that the rendered text is far below it and that the contract constant matches).
- [ ] 5. Prove purity and ownership with tests/CI assertions: `go list -deps ./plugins/exp/nudge` shows no Eino; `TestPluginWindowCannotImportInternal`/`TestEinoImportsQuarantined` still pass; no `hash/` import; no `os`, `net`, `plugin`, `reflect`-based registry.
- [ ] 6. Move the behavior tests from core (window/threshold/tie-break/refusal/corrective/retry-identity/stop) and adapt only their package reference and fixtures; keep assertion values identical. Add a distinct-field test proving `("a","b")` and `("ab","")` identities differ, the exact-tuple replacement for the FNV separator test.
- [ ] 7. Add `recipes/exp-nudge.vivy.yml` and the matching T1 catalog registration (`sdk/internal/cmd/generate-default` externals if default selects it). Validate Recipe schema/known-fields via the existing compiler in a focused test; do not edit generated output by hand.
- [ ] 8. Apply the owner-confirmed selection policy to the official Recipes; if confirmation is absent, ship only `recipes/exp-nudge.vivy.yml` and record the deferred default-family selection as an explicit blocker for N03 evidence (do not silently drop coaching from the default species).
- [ ] 9. Record focused commands/results (toolchain absent here — state that honestly):
  - `go test ./plugins/exp/nudge/...`
  - `go vet ./plugins/exp/nudge/...`
  - `go list -deps ./plugins/exp/nudge | Select-String cloudwego` (must be empty)
  - `go test ./internal/app/... -run 'Import'`
  - `git diff --check`

## Consumed and produced contracts

Consumed: N00's `loopexp` contract constants/types, closed Port and T1 catalog entry; D04's exact-tuple core policy bodies and baseline templates.

Produced for N02: `expnudge.New` (Module) and `expnudge.Factory` (`loopexp.NudgeFactory`) with the exact detector behavior, plus the selected Recipe file and T1 registration. N02's adapter may not change these semantics; a behavior change means N02 is wrong, not the package.

## Verification commands and expected result

| Command | Expected |
| --- | --- |
| `go test ./plugins/exp/nudge/...` | Moved behavior tests pass unchanged in value; purity tests pass. |
| `go list -deps ./plugins/exp/nudge` | No `github.com/cloudwego/eino*`, no `agent-vivy/internal/*`, no `hash/`. |
| `go test ./sdk/internal/... -run 'Source|Recipe'` | T1 record and Recipe compile validation pass. |
| `go run ./sdk/internal/cmd/generate-default ...` then `git diff --exit-code internal/generated/assembly/zz_default.go` | Unchanged unless the default Recipe was confirmed to select the package. |
| `git diff --check` | Clean. |

## Acceptance, rollback and handoff

Acceptance: `vivy/exp-nudge` is a pure, dependency-clean Module owning the real coaching policy; the selected Recipe compiles; unselected official Recipes remain unchanged; the policy behavior is byte-identical to the D04-simplified core behavior for window, thresholds, templates, `nudge-v1` and the count-6 stop.

Rollback: delete `plugins/exp/nudge/`, `recipes/exp-nudge.vivy.yml` and the T1/catalog entries; core policy is untouched in this Story, so behavior remains exactly as before. No data or schema impact.

Handoff: exact commit, package/test paths, the moved-behavior test mapping, Recipe and catalog diffs, selection-policy confirmation (or its recorded blocker), focused check outcomes, and any core test still owning a behavior now covered here (with its planned N03 disposition).

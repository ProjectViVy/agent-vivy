# N00: Bind the species-selected nudge package into the typed loop-exp seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Bind the nudge experiment into the typed loop-exp seam so that selection is a generated, T1-only, physically-omitted choice, with no runtime discovery and no second execution path.

**Architecture:** Consume the inert seam landed by [D04](../issue-91/D04.md): pure Eino-free SDK contract in `sdk/port/loopexp/nudge.go`, closed core catalog Port `core/nudge-factory@v1`, typed optional factory slot in `internal/runtime/extensions.go`, generated typed Assembly field and accessors, and typed unwrapping in `internal/app/`. This Story adds only the nudge-specific binding: the T1 build-owned source-catalog entry for `plugins/exp/nudge/`, generator validation for the nudge slot, the absence/capability contract, and test-only fixture proof. Root runtime imports neither the EXP module nor the adapter. No `plugins/exp/nudge/` implementation, constructor, Recipe selection or settings read ships here.

**Tech Stack:** Go 1.26.4; pinned Eino v0.9.13 inside `internal/runtime/` and `internal/provider/` only; existing SQLite/Postgres; TypeScript/React only if a generated capability projection changes.

**Spec:** [Complete deletion design](../../../plans/2026-10-19-complete-deletion-plan.md), especially §10.1a C11, §10.3, §11.1–11.5. [Index](index.md) owns status and evidence; [N01](N01-policy-extraction.md), [N02](N02-adapter-settings-recovery.md) consume this contract.

**Story:** N00 · **Epic:** E-NUDGE · **Requirements:** R1,R2,R5,R6.

**Immediate predecessors:** D04 (accepted seam and handoff removal), D02, D03, D06; implementation handoff.

**Readiness gate:** Accepted D04 output containing the inert loop-exp seam; accepted D02/D03 (durable typed operation outcomes and the single continuation contract) and D06 (source/Assembly generation shape) — or the equivalent facts proven in the implementing commit; owner confirmation of the Recipe selection policy in [index.md](index.md). Current request authorizes planning only. A plan file is not permission to implement or proof that prerequisites passed.

## Global Constraints

- Preserve one `Service.Run` / Eino Runner / Journal / policy and durable tool-operation path.
- Remove Vivy-owned hashes entirely; no disabled path, no-op verifier, alias or renamed hash.
- Closed catalog: only a build-owned T1 source may provide `core/nudge-factory@v1`; a public/T2 Module must fail compilation.
- Only `internal/runtime/` and `internal/provider/` import `github.com/cloudwego/eino*`; `plugins/` may not import `agent-vivy/internal/`.
- No placeholder provider, no-op policy or stub that satisfies generation while doing nothing; fixture modules exist only under `_testdata`/test packages.
- No runtime directory scan, no engine import of the EXP package, no unconditional wiring, no new registry.
- Historical `tool.nudge` journals stay readable; no schema or migration change in this Story.
- Current mandatory CI and affected real-path checks remain in force; no new dependency.

## Review Focus

- A T2/external source cannot obtain the closed Port by declaring it; the error names the owner rule.
- The generated Assembly exposes `false`/`nil` for an unselected build and emits no EXP/adapter import.
- The root runtime package has no import edge to `plugins/exp/nudge` or `internal/runtime/extensions/nudge`.
- The seam adds no capability source of truth: selection is `generation.Manifest.Modules` and the generated field, not a runtime switch, config key or UI list.
- No nudge setting/action/control is introduced; the absence contract stays verifiable.
- No reliance on D04-unknowable internals: the contract is restated here, so this plan is implementable from its own text.
- Hard-stop ownership: core-owned per D04 and [index.md](index.md) open item 2. Do not add a `Stop` channel or move the count-6 terminal; the package contract carries coaching decisions only. Never add a second core-owned coaching window and never drop the core stop silently.

## Baseline A mapping (202aac38, 2026-10-07)

| Element | Baseline A |
| --- | --- |
| Closed catalog | `internal/moduleport/ports.go`: `Catalog()` with `closed/conditional` helpers; `ValidateProvider(ref, moduleID, source)` requires `SourceInternal` **and** `moduleID == definition.Owner`; owner table entries such as `core/mask-service@v1 -> vivy/masks` (`ports.go:55–73`). No loop-exp Port exists. |
| Typed factory precedent | `internal/moduleport/masks.go` (`MaskDependencies`, `MaskFactory func(context.Context, MaskDependencies) (maskcontract.Service, error)`); `internal/cognitivecontract` factory for `core/cognitive-factory@v1`. |
| SDK contract precedent | `sdk/port/observer/observer.go` (pure DTOs, no runtime import); `sdk/port/loopexp/` does not exist at baseline A. |
| GoBinding | `sdk/internal/assembly/source.go:32–58` (`GoBinding{ImportPath, Package, Constructor, ProviderConstructor, ProviderCollection, MaskFactory, CognitiveFactory, ...}`); T1/T2 trust assigned by `SourceCatalog`, never by a Module. |
| Generator | `sdk/internal/assembly/runtime_generate.go`: factory validation `140–181`, `Has*`/`*FactoryValue()` accessors `567–586`, import emission `183–208`, struct/field emission `202–219`. |
| App unwrap | `internal/app/assembly_validate.go:250–262` (`validateMaskFactoryBinding`), `internal/app/masks.go:14–70`, `internal/app/assembly_cognitive.go:18–40`. |
| T1 source table | `sdk/internal/frontend_v1.go:210–239` `repoSourceDirs` (single build-owned table; `sourceRecords` at `1769–1826` assigns `TrustT1` and `Ref:"repo:"+dir`, and enforces descriptor `SHA256` until D06 removes it); default form at `sdk/internal/cmd/generate-default/main.go:53–62`. |
| Generated output | `internal/generated/assembly/zz_default.go` (generated, do not hand-edit); `generate.go` invokes the generator. |
| Quarantine gates | `internal/app/importlint_test.go:86,126`. |
| No experiment precedent | `plugins/exp/` and `recipes/exp-*.vivy.yml` do not exist; nothing to rename or preserve. |

## Files and ownership

D04 owns the generic inert seam. This Story verifies it and adds only the nudge-specific
selection binding. Classify every path before editing; never ship a second variant.

- Verify (D04-owned; edit only if the accepted output is missing the element):
  - `sdk/port/loopexp/nudge.go` — pure contract (final nudge-specific form; no `Stop` channel under the core-owned decision; keep the constants below).
  - `sdk/port/loopexp/nudge_test.go` — pure contract tests (bounds, canonical-string equality, decision semantics).
  - `internal/runtime/extensions.go` — typed slot types (`NudgeFactory`, `NudgeExtension`, `NudgeLegHost`, opaque observed-input handle) and nil-safe no-extension behavior.
  - `internal/moduleport/ports.go` + `internal/moduleport/ports_test.go` — `closed("core/nudge-factory@v1", "vivy/exp-nudge", CardinalityAtMostOne, RequirementConditional)` and the T1/owner/cardinality tests.
  - `sdk/internal/assembly/source.go` (`GoBinding.NudgeFactory`), `runtime_generate.go` (field/accessors/validation), `runtime_generate_test.go`, `testdata/nudgefixture/`.
  - `internal/app/assembly_loop_extensions.go` + test, `internal/app/assembly_validate.go` + test (typed `any` unwrap; no EXP or Eino import).
- Create/add (N00-owned):
  - `sdk/internal/frontend_v1.go` — the T1 build-owned source entry `{dir: "plugins/exp/nudge", importPath: "agent-vivy/plugins/exp/nudge", pkg: "expnudge"}` in `repoSourceDirs` (skipped while the directory is absent; becomes the T1 record once N01 lands the module).
  - `sdk/internal/frontend_v1_test.go` — T1 assignment and Pack-snapshot inclusion assertions.
  - The nudge-specific absence contract and negative-guard shape (task 7), including the selected/omitted fixture-generation cases.
- Read-only contracts: `internal/runtime/extensions/nudge/` (created in N02), `plugins/exp/nudge/` (created in N01), `recipes/exp-nudge.vivy.yml` (N01), `docs/architecture/NUDGE-DESIGN.md` (N03 edits).
- Do not touch: issue-91 Story docs, other epic packages, `internal/runtime/nudge_*.go` behavior (N03 deletes), `internal/runtime/loopguard.go` (N01/N03), generated `zz_default.go` by hand.

## Tasks

- [ ] 1. Re-verify the accepted D04 seam and record exact paths in the handoff: which of `sdk/port/loopexp/nudge.go`, `internal/runtime/extensions.go`, `internal/moduleport/ports.go`, `sdk/internal/assembly/{source,runtime_generate}.go`, `internal/app/` already carry the nudge slot. Do not duplicate an existing element; add only missing ones per tasks 2–8.
- [ ] 2. Finalize the pure SDK contract in `sdk/port/loopexp/nudge.go` (no `agent-vivy/internal`, no Eino):
  - `type ToolSample struct { CallID, ToolName, Arguments, Result, Error, Status, Reason, Effects string }` — bounded producer-owned values; `Arguments` is the canonical key-sorted JSON captured at request time; empty `Status` means success.
  - `type NudgeNotice struct { CallID, ToolName, Status, Reason string; Count int; TemplateVersion, Text string }`.
  - `type NudgeDecision struct { Notice *NudgeNotice }` — no `Stop` field: D04 and this package record core-owned hard stop (see [index.md](index.md) open item 2); the core guard keeps the count-6 terminal.
  - `type NudgeDetector interface { Evaluate(orderedFresh []ToolSample) NudgeDecision }` — one call per settled batch; must be pure, deterministic, allocation-bounded, and must not retain `ToolSample` beyond the window.
  - `type NudgeFactory func() NudgeDetector`.
  - Constants for `TemplateVersionNudgeV1 = "nudge-v1"`, the ten-entry window, thresholds 3/5 and the 1024-byte reminder ceiling so the adapter and package cannot drift.
  - Tests: canonical-string equality is byte equality (no hashing); success advances the window without a notice; no `Stop` channel exists; zero/one sample behavior; no growth beyond ten references.
- [ ] 3. Add the closed catalog definition and owner gate: `core/nudge-factory@v1`, `CardinalityAtMostOne`, `RequirementConditional`, owner `vivy/exp-nudge`. Tests: `Catalog().Lookup` returns it; a T1 module with another ID is rejected; a T2 module with the right ID is rejected; two providers are rejected.
- [ ] 4. Extend the generator: `GoBinding.NudgeFactory`; emit `Rt.NudgeFactory` only when a selected module provides the Port (add `agent-vivy/internal/runtime` and the adapter import to the generated import block only in that case); validate `NudgeFactory` binding requires the Port, a Port provider requires the binding, the value matches `goIdentifierPattern`, and duplicates fail. Tests with `testdata/nudgefixture` for: selected emits field/import/accessors; omitted emits neither and `HasNudgeFactory()==false`, `NudgeFactoryValue()==nil`.
- [ ] 5. Add the app-side typed unwrap in `internal/app/assembly_loop_extensions.go` and validation in `internal/app/assembly_validate.go`: the Assembly must expose the factory exactly when `Manifest.Modules` contains `vivy/exp-nudge`; an emitted factory without the module, or the module without an emitted factory, is a startup error naming both facts. No EXP import.
- [ ] 6. Register the T1 source: `repoSourceDirs` entry for `plugins/exp/nudge`; the default generator `externals` entry and the `recipes/exp-nudge.vivy.yml` reference land in N01 together with the module (a catalog entry for an absent directory is skipped by `sourceRecords`, so this ordering never breaks the build). Test that the entry is T1, `Ref` is `repo:plugins/exp/nudge`, and the Pack source snapshot includes it.
- [ ] 7. Encode the absence/capability contract as tests: for an omitted recipe/assembly there is no generated EXP or adapter import, no `NudgeFactory` value, no nudge config key, no RPC/control-action ID and no `tool.nudge` producer path; for a selected fixture the generated file carries exactly one field and one import pair. Add the negative guard shape that N03 extends to the real Recipes.
- [ ] 8. Run the focused checks (toolchain absent in this environment — record honestly in the verification log):
  - `go test ./sdk/port/loopexp/... ./internal/moduleport/... ./sdk/internal/assembly/... ./internal/app/...`
  - `go run ./sdk/internal/cmd/generate-default -repo . -output internal/generated/assembly/zz_default.go` then `git diff --exit-code internal/generated/assembly/zz_default.go` (must stay unchanged while no official Recipe selects the package)
  - `git diff --check`

## Consumed and produced contracts

Consumed: D04's `NudgeFactory`/`NudgeExtension`/`NudgeLegHost` slot shape, D06's source/Assembly generation shape, D02/D03's durable typed `domain.ToolOperation` outcome fields (`OutcomeStatus`, `Reason`, `Effects`, request arguments) used by N02's producer feed.

Produced for N01/N02: the final pure contract above; the closed Port and its T1 owner; the generator's typed field/accessors/validation; the app unwrap; the T1 source-catalog entry; the absence tests. N01 must not change these signatures without re-opening this Story.

## Verification commands and expected result

| Command | Expected |
| --- | --- |
| `go test ./sdk/port/loopexp/...` | Pure contract tests pass; no Eino/internal import in the package. |
| `go test ./internal/moduleport/...` | Port present; T1/owner/cardinality gates reject non-owners. |
| `go test ./sdk/internal/assembly/...` | Fixture generation selected/omitted, validation errors and accessors pass. |
| `go test ./internal/app/... -run 'Assembly|Import'` | App unwrap/validation and quarantine tests pass. |
| `go run ./sdk/internal/cmd/generate-default ... && git diff --exit-code` | No generated change while no official Recipe selects the package. |

## Acceptance, rollback and handoff

Acceptance: an unselected build contains no nudge package/adapter import and no factory value; a selected fixture build contains exactly one; a T2 source cannot provide `core/nudge-factory@v1`; the nudge slot is provably inert (no middleware registered, no state allocated, no event produced) until N02 wires it. Known limitation: the selected end-to-end path is not exercised until N01/N02.

Rollback: revert this Story's commit; it adds no runtime behavior, no schema change, no data conversion and no generated default change, so Git rollback is complete.

Handoff: exact commit/build, the final contract text, all touched paths, focused check outcomes (or the recorded toolchain absence), and any element D04 already provided (verified, not rewritten). Return to the predecessor contract if the slot shape or the artifact/source generation contract changes; update N01–N03 before work continues.

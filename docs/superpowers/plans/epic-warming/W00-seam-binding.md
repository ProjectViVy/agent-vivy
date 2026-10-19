# W00: Bind the warming package to the loop-exp seam and species selection

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Make the real warming package selectable: create `plugins/exp/warming` as a T1 module that provides `core/warming-factory@v1`, bind it through the generated Assembly and typed runtime slot, and prove none/each/all selection, T1-only trust and `NOT_COMPILED` for unselected builds. The seam is inert in this Story: the single core warming implementation keeps serving until W02 cuts over.

**Architecture:** Reuse the inert loop-exp seam landed by base-cut Story D05: an Eino-free SDK contract in `sdk/port/loopexp`, one closed core catalog Port, one typed optional factory slot in `internal/runtime/extensions.go`, build-owned Source Catalog provenance and generated Assembly bindings. This Story adds only the warming package side of that seam plus its Recipes, and refuses to invent a second registry, discovery path, service locator or dual construction path.

**Tech Stack:** Go 1.26.4, pinned Eino v0.9.13 inside runtime/provider only, existing SQLite/Postgres, TypeScript/React not affected here; each plugin is its own Go module (`require agent-vivy`, `replace agent-vivy => ../../..`).

**Spec:** [Complete deletion design §§10.1a, 10.3, 11.1–11.3](../../../plans/2026-10-19-complete-deletion-plan.md). Epic index: [index.md](index.md). Story package: [issue-91](../issue-91/index.md); seam predecessor [D05](../issue-91/D05.md).

**Story:** W00 · **Epic:** E-WARMING · **Requirements:** R2, R3, R5, R6.

**Immediate predecessors:** accepted base-cut D05 seam output (typed slot, SDK contract, catalog Port, generator support) and accepted D06 source/generation contract. Both are prerequisites, not this Story's work.

**Readiness gate:** Accepted base-cut D05 seam output: `sdk/port/loopexp/warming.go`, the `core/warming-factory@v1` catalog entry, the typed slot in `internal/runtime/extensions.go`, the `runtime_generate.go` named-factory support and `internal/app/assembly_loop_extensions.go` are expected from D05/D04 (the re-anchor audit §4.2–4.3 records that the whole seam is absent at Baseline A and must be created). D06 has removed source-tree digests from Module sources and defines the build-owned T1 catalog location used for Recipe-driven generation. This Story **consumes** that seam and must not re-author it; if an accepted-seam element is missing, W00 adds that single element following the recorded §11 contract and never a second variant. Current request authorizes planning only.

## Global Constraints

- One `Service.Run` / Eino / Journal / policy path; no second runner, scheduler, registry, discovery path or receipt framework.
- `core/warming-factory@v1` is provided only by the build-owned T1 module `vivy/exp-warming`; T2 sources cannot provide or replace it and Recipe/descriptor text cannot promote trust.
- `plugins/exp/warming` imports only the standard library and `agent-vivy/sdk/...`; never `agent-vivy/internal/...` and never Eino.
- Only `internal/runtime/` and `internal/provider/` import `github.com/cloudwego/eino*`; the generated Assembly is the only hand-off between selected code and the kernel.
- Do not hand-edit generated Assembly files; regenerate from the build-owned catalog and Recipe.
- Unselected Recipes (including `minimal` and the committed default) contain no plugin import, constructor, capability projection, settings read, RPC registration or UI control.
- No Vivy-owned hash, no disabled path, no alias/no-op verifier, no placeholder provider that satisfies generation while doing nothing.
- No production Journal access, no database reset, no commit/push by the implementing agent.
- The single core warming implementation keeps running; W00 does not change `internal/runtime/cachewarmer.go` behavior.

## Review Focus

- Provenance: can a T2 source, arbitrary root or Recipe text obtain T1 trust or provide the Port? It must fail closed at build time.
- Cardinality: 0..1 — two providers must fail; an absent one must not synthesize a default.
- Inertness: a selected build must not serve two warming implementations, and the none case must not gain any warming symbol.
- Generated-code truth: the selected binding is produced only from the build-owned catalog, never edited by hand, and its typed value is unwrapped once in `internal/app`.
- Error quality: a selected module with a missing/mistyped binding fails init; it never degrades to "warming silently off".
- Physical omission: the none artifact's import list is the test, not a runtime boolean.

## Baseline A mapping (202aac38, 2026-10-07)

Verified in this tree; the 10-18 plan's paths are remapped, not assumed.

| Target from §11 / D05 | Baseline A reality | Disposition |
| --- | --- | --- |
| `sdk/port/loopexp/warming.go` | does not exist | created by D05 seam or by this Story's warm-up task |
| `internal/moduleport/ports.go` closed catalog | exists; `core/cognitive-factory@v1` at line 61 is the 0..1/conditional structural analogue | add `core/warming-factory@v1` owner `vivy/exp-warming` by the same `closed(...)` helper |
| typed factory slot `internal/runtime/extensions.go` | does not exist | created by D05; W00 verifies the warming part, does not restate it |
| `internal/runtime/extensions/warming/` adapter | does not exist | W02 |
| `plugins/exp/warming/` | does not exist; existing plugins are individual Go modules with `module.go`, `vivy-module.yaml`, `go.mod` | create |
| build-owned T1 Source Catalog | `internal/modules/defaults/catalog.go` for internal modules; `sdk/internal/cmd/generate-default/main.go:59` hardcodes first-party plugin externals as T1 | D06 replaces this with the explicit Recipe/source catalog; W00 adds the single exp-warming record in the D06 location |
| `sdk/internal/assembly/source.go` `GoBinding` / `runtime_generate.go` named-factory pattern | exists: `MaskFactory`, `CognitiveFactory` with validation (`runtime_generate.go:144-178`), typed field (204-207), assignment (278-281) and `Has*`/`*Value` accessors (567-586) | mirror for `WarmingFactory` under D05/D06 ownership |
| `internal/app` typed unwrapping for a factory | `internal/app/assembly_cognitive.go` is the pattern (opaque `CognitiveFactoryValue() any`, init failure on mistype) | create `internal/app/assembly_loop_extensions.go` for warming |
| generated Assembly | `internal/generated/assembly/zz_default.go`, generated from `recipes/default.vivy.yml` by `sdk/internal/cmd/generate-default` | default artifact unchanged; selected fixture generated |
| `recipes/exp-warming.vivy.yml`, `recipes/exp-loop-features.vivy.yml` | do not exist; `recipes/minimal.vivy.yml` (3 lines) and `recipes/default.vivy.yml` exist | create the each-Recipe; contribute one line to the combined Recipe |
| capability states | `sdk/generation/manifest.go:95` (`NOT_COMPILED`) and `NetworkStates` in the generated Manifest (`channels`, `mcp` `Unconfigured`) | project a warming capability state from the same admitted Module set |
| `NOT_COMPILED` request behavior | masks use `inactive_reason: "not_compiled"` vocabulary (`internal/modules/masks/service.go:225`) | reuse the vocabulary for warming-only settings/RPC in unselected builds (W02 exposes it) |

No `cache.warmed` JSON schema exists at Baseline A (`schemas/events/payloads/` has no cache-warmed file); W03 owns that evidence decision.

## Files and ownership

- Create: `plugins/exp/warming/go.mod`, `plugins/exp/warming/module.go`, `plugins/exp/warming/module_test.go`, `plugins/exp/warming/imports_test.go`, `plugins/exp/warming/vivy-module.yaml`, `plugins/exp/warming/README.md`.
- Create: `recipes/exp-warming.vivy.yml`.
- Modify (post-D06 location): the build-owned Source Catalog entry for first-party T1 plugin sources; at Baseline A that is `sdk/internal/cmd/generate-default/main.go` and its imported plugin list.
- Modify (or verify as D05 output): `internal/moduleport/ports.go`, `internal/moduleport/ports_test.go`, `sdk/port/loopexp/warming.go`, `sdk/internal/assembly/source.go`, `sdk/internal/assembly/runtime_generate.go`, `sdk/internal/assembly/runtime_generate_test.go`, `internal/runtime/extensions.go`.
- Create: `internal/app/assembly_loop_extensions.go`, `internal/app/assembly_loop_extensions_test.go`.
- Regenerate: `internal/generated/assembly/zz_default.go` (must stay byte-identical for the unchanged default Recipe) and the selected testdata Assembly used by generation tests.
- Do not touch `internal/runtime/cachewarmer.go`, `internal/runtime/service.go`, `internal/config/*`, `internal/rpc/*` in this Story.

## Contracts consumed and produced

Consumed (D05, verbatim from §11.3 — do not fork):

```go
// sdk/port/loopexp/warming.go
type WarmingFactory func() WarmingPolicy
type WarmingInput struct {
    Mode string
    SupportsWarming bool
    CacheLifetimeSeconds, PrefixTokens int
    InputPerMTokens, CachedInputPerMTokens, MinSavingsUSD float64
}
type WarmingRequest struct { Prompt string; MaxOutputTokens int; TimeoutMillis int64 }
type WarmingDecision struct { Reason string; Request *WarmingRequest }
type WarmingPolicy interface { Decide(WarmingInput) WarmingDecision }

// internal/runtime/extensions.go
type WarmingFactory func(MaintenanceHost) ModelSettlement
type ModelSettlement interface { Settled(context.Context, ObservedModelInput) error }
type MaintenanceHost interface {
    ReusablePrefix(ObservedModelInput) (ReusablePrefix, error)
    PolicyInput(ReusablePrefix) loopexp.WarmingInput
    GenerateMaintenance(context.Context, ReusablePrefix, loopexp.WarmingRequest) (MaintenanceResult, error)
    Diagnostic(context.Context, MaintenanceDiagnostic)
}
```

Produced by W00:

- Module descriptor: `vivy/exp-warming` v0.1.0, `Provides: [{Port: "core/warming-factory@v1", ID: "vivy.warming-factory"}]`, `Lifecycle: {Scope: generation}`, no `Requires`, no `Optional`, no `Conflicts`, no `RequestedGrants`, no I18N.
- T1 Source Catalog record: module `vivy/exp-warming`, explicit source ref `repo:plugins/exp/warming`, import `agent-vivy/plugins/exp/warming`, package `expwarming`, constructor `New`, binding `WarmingFactory: "New"`.
- Generated Assembly for a selected Recipe: field `WarmingFactory runtime.WarmingFactory`, assignment `warmingrt.Factory(expwarming.New)`, imports `agent-vivy/internal/runtime`, `agent-vivy/internal/runtime/extensions/warming`, `agent-vivy/plugins/exp/warming`; accessors `HasWarmingFactory() bool` and `WarmingFactoryValue() any` returning `false`/`nil` when unselected.
- `internal/app` unwrapping: `generatedWarmingFactoryBinding` interface, nil when `vivy/exp-warming` is absent from the sealed Manifest, init error when selected but missing/mistyped, value handed to `runtime.ServiceDeps.WarmingFactory` (field added by W02; W00 may add the field as an unused typed slot only if D05 has not).

## Tasks

- [ ] 1. Preflight the D05 seam. Read `internal/runtime/extensions.go`, `sdk/port/loopexp/warming.go`, `internal/moduleport/ports.go`, `sdk/internal/assembly/{source,runtime_generate}.go` and `internal/app/assembly_loop_extensions.go`. Record in the Story's handoff which symbols already exist. If an accepted-seam element is missing, add that single element exactly as §11.3/§11.2 records; do not re-author the seam, do not rename D05 symbols and do not create a second variant.
- [ ] 2. Create `plugins/exp/warming/go.mod` mirroring `plugins/vivy-notebook/go.mod`: module `agent-vivy/plugins/exp/warming`, `go 1.26.4`, `require agent-vivy v0.0.0`, `replace agent-vivy => ../../..`.
- [ ] 3. Create `plugins/exp/warming/module.go`: package docs stating this is the explicit species-selected experimental warming package; constants `ModuleID = "vivy/exp-warming"`, `Port = "core/warming-factory@v1"`, `ProviderID = "vivy.warming-factory"`; `New() module.Module` owner, `NewProvider()` exported typed pair, `Descriptor()` as produced above, and `Construct/Start/Ready/Stop/Close` returning empty instances. No policy logic here (W01).
- [ ] 4. Create `plugins/exp/warming/vivy-module.yaml` with `apiVersion: vivy.module/v1`, the same identity/provides/lifecycle as the Go descriptor, and the source form required by D06 (no source-tree digest after D06; if executed pre-D06, use the existing `source.sha256` form and record that D06 removes it). Add `README.md` describing selection, behavior and the physical-omission contract.
- [ ] 5. Create `plugins/exp/warming/imports_test.go`: parse this module's own `.go` files with `go/parser` and fail on any `agent-vivy/internal/` or `github.com/cloudwego/eino` import. This is the plugin-window gate for the new module (`TestPluginWindowCannotImportInternal` already covers plugins as a tree; this keeps the failure local and named).
- [ ] 6. Register the module in the build-owned T1 Source Catalog (D06 location; Baseline A analogue `sdk/internal/cmd/generate-default/main.go` externals). Register the exact module ID, explicit source ref and import/constructor; no directory scan, no runtime discovery.
- [ ] 7. Extend `sdk/internal/assembly/source.go` `GoBinding` with `WarmingFactory string` if D05 has not. Extend `runtime_generate.go` mirroring the cognitive factory: (a) reject a `WarmingFactory` binding whose descriptor does not provide `core/warming-factory@v1`, (b) require the binding when the Port is provided, (c) reject duplicate providers, (d) validate the binding is a Go identifier, (e) emit the typed field, import aliases and assignment only when selected, (f) emit `HasWarmingFactory`/`WarmingFactoryValue` accessors that are false/nil when absent. Add table tests in `runtime_generate_test.go` for each rejection and for the none case.
- [ ] 8. Enforce T1-only at compile time: extend the assembly/compiler core-Port check so a T2 source providing `core/warming-factory@v1` is rejected with a diagnostic naming the Port and source class, and so a T1 record for `vivy/exp-warming` cannot be replaced by a different import or owner. Add compiler tests.
- [ ] 9. Create `internal/app/assembly_loop_extensions.go` with `generatedWarmingFactoryBinding` and `warmingFactoryForAssembly(assembly *genassembly.RuntimeAssembly) (runtime.WarmingFactory, error)` following `assembly_cognitive.go`: nil when the module is absent; error when the engine lacks the seam, the value is mistyped or nil; otherwise the typed factory. The package imports root `runtime`, never the EXP package. Wire it in `internal/app/app.go` next to the Service construction (a typed `ServiceDeps` field; W02 consumes it).
- [ ] 10. Create `recipes/exp-warming.vivy.yml`:

```yaml
apiVersion: vivy.generation/v1
profile: exp-warming
modules: [vivy/loop, vivy/model, vivy/tool-host, vivy/storage, vivy/checkpoint, vivy/credential, vivy/sandbox, vivy/exp-warming]
```

  If `recipes/exp-loop-features.vivy.yml` exists and is owned by the loop-experiments integration owner, append exactly `  - vivy/exp-warming` to its `modules` list. If it does not exist, do not create a placeholder; record in the handoff that the "all" case is blocked on the integration owner and includes the exact line above. Apply the owner's Q0 answer to the shipped family Recipes as exact module lines; do not add them without that answer.
- [ ] 11. Project the capability state: extend the generated Manifest with a warming capability entry derived from the same admitted Module set (selected → the configuration state, unselected → `NOT_COMPILED`), using the existing `generation.CapabilityState` vocabulary. Do not add a runtime boolean that claims feature presence.
- [ ] 12. Regenerate: `go generate ./internal/generated/assembly` (directive in `internal/generated/assembly/generate.go`). For the committed default Recipe, confirm `git diff --exit-code internal/generated/assembly/zz_default.go` when Q0 keeps the default family unselected; when Q0 selects the package in the default family, the diff must show exactly the warming module/binding additions and nothing else. Then generate the selected exp-warming Assembly into the generation test fixture used by `sdk/internal/assembly/*_test.go` and `internal/app/*_test.go`.
- [ ] 13. Write selection tests: none (`recipes/minimal.vivy.yml`) — generated source contains no `plugins/exp/warming` import and no `WarmingFactory` assignment; `HasWarmingFactory() == false`; each (`exp-warming`) — import, assignment, accessor true; all (when available) — all three bindings present and independent; default family — matches the recorded Q0 decision (module line plus generated binding, or their absence).
- [ ] 14. Update only the docs this Story makes true (plugin README, Recipe comments). Do not update the epic index's status claims to "implemented"; return evidence to the Lead instead.

## Verification commands and expected read

Implementation-time (not runnable in the planning environment; Go/`just` absent):

- `go generate ./internal/generated/assembly` → exit 0; default generated file unchanged.
- `go test ./internal/moduleport ./sdk/internal/assembly ./internal/app ./plugins/exp/warming` → exit 0.
- `go test ./internal/app -run 'TestEinoImportsQuarantined|TestPluginWindowCannotImportInternal'` → exit 0.
- `rg -n "plugins/exp/warming" internal/generated/assembly/zz_default.go` → no match for the none artifact; match in the selected fixture.
- `git diff --check` → exit 0.
- `just ci` (implementation time, full repository gate) → record the exact result.

## Acceptance, rollback and handoff

The package exists as `vivy/exp-warming` with the exact descriptor and T1 provenance; a Recipe that selects it generates a typed `WarmingFactory` binding while a Recipe that does not select it contains no warming symbol and its artifact is byte-identical; the committed default family matches the recorded Q0 decision; T2 provision and duplicate/wrong bindings fail closed with diagnostics; a warming-only request in an unselected build reports `NOT_COMPILED` with no constructor or config read; the core warming implementation still serves (no behavior change in this Story, no dual serving).

Return the exact commit/build, changed paths, generation commands and diffs, focused test output, the three-case selection evidence, residual matches with dispositions, and any seam symbol D05 had already landed. Roll back by reverting this Story's commit; do not disable a failing selection test. Handoff to W01: the package must compile and be selectable, but must not contain behavior the core still owns.

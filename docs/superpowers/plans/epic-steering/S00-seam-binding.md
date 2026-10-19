# S00: Bind the species-selected steering seam Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. Preserve the user's execution choice; use one active write lane and fresh independent reviewers for final acceptance.

**Goal:** Make `vivy/exp-steering` a T1 build-owned selectable Module that provides `core/steering-factory@v1`, bind its pure factory through generated Assembly into the root runtime's typed slot, and make the unselected composition physically omit the package and adapter.

**Architecture:** Closed core Port (owner `vivy/exp-steering`, `0..1`, conditional) + Eino-free `sdk/port/loopexp` factory + root-runtime `SteeringFactory`/`SteeringHost` slot + selected adapter. The generated Assembly imports the package and adapter only when the Recipe lists `vivy/exp-steering`; the app resolves the factory through an opaque generated accessor and fails closed on a selected-but-missing or mistyped binding. No registry, discovery, second runner or queue is added.

**Tech Stack:** Go 1.26.4; pinned `github.com/cloudwego/eino v0.9.13` (Eino imports stay in `internal/runtime` and `internal/provider` only); existing SQLite/Postgres; TypeScript/React only where the later UI Story touches queue controls; BML is an independent Go module and is untouched.

**Spec:** [Complete deletion design §§10.1a C11, 10.3, 11.1–11.3, 11.5](../../../plans/2026-10-19-complete-deletion-plan.md), read at Baseline A `202aac38`. [Index](index.md) owns status, DAG and prerequisite evidence. [D05](../issue-91/D05.md) records the superseded package split; this Story is the steering-only successor.

**Story:** S00 · **Epic:** E-STEERING · **Requirements:** SR1, SR5, SR6.

**Immediate predecessors:** External **G1** (accepted D00 scope/contract freeze), **G2** (accepted D06 hash-free source/generation and typed-binding generator) and **G2b** (accepted shared loop-exp seam envelope, or an explicit Lead decision that this Story creates the steering portion first under the append-only rule in the index). No in-epic predecessor. Baseline A is `202aac38`; the branch delta is docs only.

**Readiness gate:** Accepted G1/G2/G2b handoff and the §11.1–§11.3 contracts unchanged. G2 is a hard block, not a preference: at Baseline A `sdk/internal/assembly/source.go:76-89` verifies every repository source tree against the descriptor's `Source.SHA256`, so adding the package first would reintroduce a custom hash binding. G2b is a write-safety block: the seam files are shared with E-NUDGE/E-WARMING. Current request authorizes planning only.

## Global Constraints

- Preserve one `Service.Run` / Eino / Journal / policy and durable queue path.
- T1 build-owned source only may provide `core/steering-factory@v1`; T2, fixtures and Module-controlled text cannot.
- No Vivy-owned hash or source digest is added; after G2 the descriptor carries `source.ref` only.
- Unselected Recipes import neither `agent-vivy/plugins/exp/steering` nor its adapter and allocate no steer state.
- Only `internal/runtime/` and `internal/provider/` import Eino. The EXP package and SDK contract are Eino-free and cannot import `agent-vivy/internal/*`.
- No commit/push, no production data, no schema change and no second execution path.
- Follow the shared C11 and §11 architecture; do not extend experimental retention to any other mechanism.

## Review Focus

- T2 trust bypass: an external source, test fixture or descriptor claiming `vivy/exp-steering`/the Port.
- Binding drift: generated import/constructor no longer names the canonical package, or a hand-edited generated file.
- False omission: nil accessor still allocating state, registering an RPC action or reading settings.
- False availability: selected Module whose factory is missing or mistyped degrading to baseline steering instead of failing init.
- Hash creep: a `sha256`/`digest`/`checksum` field added to the descriptor, binding, generation evidence or recipe.

These risks are exercised by the tasks and tests below; S03 rechecks the integrated artifact. No test that merely restates a textual edit is required.

## Baseline A mapping

| Fact at `202aac38` | Consequence for S00 |
| --- | --- |
| `internal/moduleport/ports.go:55-72` is the closed catalog; no steering Port. | Add one `closed("core/steering-factory@v1", "vivy/exp-steering", CardinalityAtMostOne, RequirementConditional)`. |
| `internal/modules/defaults/catalog.go:86-104` is the T1 table for `internal/modules/*` only. | Do not register the EXP package there; it lives under `plugins/exp/steering`. |
| `sdk/internal/frontend_v1.go:220-239` (`repoSourceDirs`) assigns T1 to repository Module dirs; `:1769-1825` builds the catalog records. | Add the package dir/import/package name here; leave `ProviderConstructor` empty and set the new factory binding. |
| `sdk/internal/cmd/generate-default/main.go:49-70` hardcodes a second external-source list. | Replace the duplicate list with one shared build-owned source table consumed by both packer and default generator; a third copy is a defect. |
| `sdk/internal/assembly/source.go:32-58` `GoBinding` has `MaskFactory`, `CognitiveFactory`; `runtime_generate.go:143-178,277-282,566-587` is the precedent. | Add `SteeringFactory` and emit the field, constructor expression and accessors in the same pattern. |
| `sdk/internal/assembly/compiler.go:121-155` and `graph.go:12-29` validate core Port providers. | No new trust mechanism: the catalog owner plus T1 check is the enforcement point. |
| `plugins/hello-fs/` convention: `vivy-module.yaml`, `descriptor.go`, `New`, `NewProvider`; `sdk/internal/source_verify_v1.go:92-94` requires both exported constructors. | `New` returns `module.Module` (lifecycle); `NewProvider` returns `loopexp.SteeringFactory` and is the generated binding target. |
| The 10-18 plan writes `steeringrt.Factory(expsteering.New)`. | Remap to `steeringrt.Factory(expsteering.NewProvider)`; `New` must remain the lifecycle constructor in this tree. |
| `internal/generated/assembly/zz_default.go:49-128` | Regenerate; never hand-edit. |

## Files and ownership

- Create: `sdk/port/loopexp/steering.go`; `plugins/exp/steering/{vivy-module.yaml,descriptor.go,module.go,provider.go,policy.go,policy_test.go}`; `internal/runtime/extensions/steering/adapter.go`; `internal/app/assembly_steering.go`; `recipes/exp-steering.vivy.yml`.
- Shared, append-only: `internal/runtime/extensions.go` (add only the steering declarations; if the seam already exists, do not replace sibling contracts); `internal/app/assembly_loop_extensions.go` if the Lead's seam created it.
- Modify: `internal/moduleport/ports.go`, `sdk/internal/assembly/{source.go,runtime_generate.go,compiler.go,graph.go}`, `sdk/internal/frontend_v1.go`, `sdk/internal/cmd/generate-default/main.go`, `internal/generated/assembly/zz_default.go` (regenerated), `internal/app/app.go` (one construction call only).
- Tests: extend `sdk/internal/assembly/{compiler_test.go,core_ports_test.go,runtime_generate_test.go}`, `sdk/internal/frontend_v1_test.go`; add focused cases in `internal/app` for the opaque accessor. Keep the existing Eino quarantine tests (`internal/app/importlint_test.go`) passing.
- Do not touch `internal/runtime/queue.go` or `service.go` steering logic in S00; S01 owns the rewire. The new adapter must compile against the Host interface and be covered with a fake Host, not leave a placeholder.
- Do not rewrite `sdk/port/loopexp/nudge.go`/`warming.go`, their adapters, their Ports/bindings or their Recipe; append only the steering symbols to shared files.

### Consumed and produced contracts

- Consumes: §11.3 exact `loopexp.SteeringFactory`, `SteeringPolicy`, `SteeringHost`, `SteeringController`, `SteeringSnapshot`, `HistoryModifier` shapes; the existing generated-Assembly accessor pattern.
- Produces: `core/steering-factory@v1` (T1-only), `GoBinding.SteeringFactory`, `runtime.SteeringFactory` slot, `expsteering.NewProvider() loopexp.SteeringFactory`, `steeringrt.Factory`, `HasSteeringFactory() bool` / `SteeringFactoryValue() any`, and the `each` Recipe.

## Tasks

### Task 1: Freeze the exact contract and baseline facts

- [ ] Re-read the five Baseline A paths and §11.1–§11.3; record the exact head, file paths and line anchors in the implementation log. If G2 changed the source/binding form, record the accepted form before writing code.
- [ ] Write `sdk/port/loopexp/steering.go` with exactly: `SteeringFactory func() SteeringPolicy`; `SteeringEligibility`; `QueueTrack` (`steer`/`follow_up`); `QueueMode` (`all`/`one-at-a-time`); `SteeringPolicy{Route(SteeringEligibility) QueueTrack; Take(QueueMode,int) int; Render([]string) string}`. No runtime/domain/Eino import.
- [ ] Add the constant/validation helpers the adapter will use for enum checking; reject unknown tracks/modes/counts at the adapter boundary, not in the policy.

### Task 2: Register the closed T1 Port and the build-owned source entry

- [ ] Add `core/steering-factory@v1` to `internal/moduleport/ports.go` with owner `vivy/exp-steering`, cardinality `0..1`, requirement `conditional`.
- [ ] Add the package to the single build-owned repository source table used by both `sdk/internal/frontend_v1.go` and `sdk/internal/cmd/generate-default/main.go`; the entry is dir `plugins/exp/steering`, import `agent-vivy/plugins/exp/steering`, package `expsteering`, lifecycle constructor `New`, factory binding `NewProvider`.
- [ ] Set `GoBinding.SteeringFactory` in `sdk/internal/assembly/source.go`; in `runtime_generate.go` validate that a non-empty binding belongs to the canonical owner and that the selected owner provides the Port, mirroring `MaskFactory`/`CognitiveFactory`.
- [ ] Add compiler tests: a T1 non-owner provides the Port -> rejected; a T2 source with module id `vivy/exp-steering` -> rejected; a selected owner without the typed binding -> rejected; two providers -> rejected; zero providers -> allowed.
- [ ] Add a parity test that the packer source table and the default generator use the same entry, so the package cannot be added to one and forgotten in the other.

### Task 3: Create the EXP package and its real pure policy

- [ ] Create `plugins/exp/steering/vivy-module.yaml` with `apiVersion: vivy.module/v1`, `module: {id: vivy/exp-steering, version: 0.1.0}`, `source: {ref: repo:plugins/exp/steering}` (no `sha256` after G2), `provides: [{port: core/steering-factory@v1, id: vivy.exp-steering}]`, no public grants, `lifecycle: {scope: generation}`.
- [ ] `descriptor.go` returns the same data; `module.go` exports `New() module.Module` with no-op construct/Start/Ready/Stop/Close (the package has no runtime lifecycle state); `provider.go` exports `NewProvider() loopexp.SteeringFactory`.
- [ ] `policy.go` implements the Baseline A decisions as pure functions: route to `follow_up` when the run is suspended or has no boundary-cancel handle, otherwise `steer`; `Take(all,n)=n`, `Take(one-at-a-time,n)=min(1,n)`; `Render` joins with `"\n\n"`. These are real, tested rules copied from `internal/runtime/queue.go:181-185,366-395,578-595`; S01 wires them and deletes the core copies.
- [ ] `policy_test.go` covers: suspended -> follow_up; no cancel -> follow_up; eligible -> steer; all/one-at-a-time counts including zero/negative; empty and multi-text render; unknown track/mode rejection at the adapter helper. Keep the policy free of Run IDs, storage, Journal, model and credentials.

### Task 4: Add the root typed slot and adapter factory

- [ ] If the shared `internal/runtime/extensions.go` exists, append the §11.3 steering contracts only: `SteeringFactory func(SteeringHost) SteeringController`, `SteeringController{Steer, SetMode, OnBoundary}`, `SteeringSnapshot`, `HistoryModifier`, and the five-method `SteeringHost`. If it does not exist, create the minimal envelope and record in handoff that E-NUDGE/E-WARMING must append, not rewrite. No imports of the adapter child or the EXP package.
- [ ] Create `internal/runtime/extensions/steering/adapter.go` exposing `Factory(loopexp.SteeringFactory) runtime.SteeringFactory`. The returned controller validates policy output, translates `Steer` into `Host.Snapshot` + `Host.Enqueue` + `Host.RequestCancelAfterModel`, `SetMode` into `Host.SetMode`, and `OnBoundary` into `Host.ContinueBoundary`; it owns no Runner, iterator, queue or goroutine.
- [ ] Add an adapter test with a fake Host recording call order: eligible steer -> enqueue then cancel-after-model; ineligible -> enqueue follow_up and no cancel; cancel refusal -> same queue ID reclassified, no second enqueue; invalid policy output -> error with no Host mutation.

### Task 5: Generate and consume the typed binding

- [ ] Extend the shared seam additively: when a selected owner provides `core/steering-factory@v1`, emit the generated field `SteeringFactory <factory type>` and the constructor expression `alias.NewProvider` wrapped by `steeringrt.Factory`; always emit `HasSteeringFactory() bool` and `SteeringFactoryValue() any` returning `false`/`nil` when absent, so omission is a generated fact rather than a runtime check.
- [ ] Add the generated imports only when selected. In the absent case the generated file must contain no `plugins/exp/steering` or adapter import and no field of that type.
- [ ] Regenerate `internal/generated/assembly/zz_default.go` with `go generate ./internal/generated/assembly` (or the exact `sdk/internal/cmd/generate-default` command) and verify byte-identical output on a second run. Never hand-edit.
- [ ] Create `internal/app/assembly_steering.go` with the opaque unwrap (`SteeringFactoryValue() any` -> `runtime.SteeringFactory`); add one registration call at the shared app integration point (append to `internal/app/assembly_loop_extensions.go` if it exists, otherwise the existing Service construction call in `internal/app/app.go`). A selected-but-missing/mistyped binding fails app construction, while unselected yields nil. Do not add a fallback constructor.
- [ ] Add generated-Assembly tests: selected -> imports/field/constructor present and `HasSteeringFactory()==true`; minimal -> no imports/field, `HasSteeringFactory()==false`, `SteeringFactoryValue()==nil`.

### Task 6: Add the each Recipe and prove omission

- [ ] Create `recipes/exp-steering.vivy.yml` with `apiVersion: vivy.generation/v1`, `profile: exp-steering`, and exactly the baseline minimal dependency set (`vivy/loop`, `vivy/model`, `vivy/tool-host`, `vivy/storage`, `vivy/checkpoint`, `vivy/credential`, `vivy/sandbox`) plus `vivy/exp-steering`. No T2 `sources:` pin.
- [ ] Add an assembly test that compiles this Recipe and asserts the selected module list, Port edge and typed binding; compile `recipes/minimal.vivy.yml` and assert no steering import/binding/edge.
- [ ] Keep the Eino quarantine and plugin import-window tests passing; the EXP package must not import `agent-vivy/internal/*`, `agent-vivy/sdk/internal/*`, Eino or a sibling plugin.

## Verification commands and expected result

Future gates, not executed by this planning turn (G5 environment required):

```text
go test ./sdk/port/loopexp ./sdk/internal/assembly ./internal/modules/defaults -count=1
go generate ./internal/generated/assembly
git diff --exit-code internal/generated/assembly/zz_default.go
go test ./internal/app -run 'TestEinoImportsQuarantined|TestPluginWindowCannotImportInternal|TestSteering' -count=1
go test ./internal/runtime -run 'TestSteer|TestQueue' -count=1
just ci
```

Expected: port/trust/binding tests pass; second generation is byte-identical; selected default composition imports the package and exposes a non-nil factory; minimal exposes `false`/`nil` and contains no import; no hash field appears in the descriptor or generated output; `just ci` green. A focused test pass is iteration evidence, not a waiver for the final gate.

## Acceptance, rollback and handoff

S00 is accepted when the Port is T1-only, the selected Recipe compiles to a typed binding through `NewProvider`, the unselected composition has no import/field/state and reports `false`/`nil`, the source table has one owner, and the generated default file is deterministic. Steering behavior is not yet rewired in S00; no selected-species release may be cut before S01 removes the baseline core path.

Rollback is Git-only: revert S00's commit if the binding/trust form is rejected. Do not leave a half-generated Assembly or a hand-edited generated file. Return exact commit/build, changed paths, focused check outcomes, the regenerated file diff, and any accepted-contract change. If G2's source/binding form differs, revise this Story and the index before implementation continues; do not add a hash to make Baseline A admit the package.

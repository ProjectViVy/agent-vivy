# E-STEERING: Retained steering and later first-party plugin extraction

**Current status (2026-10-20): retain the stable implementation; re-plan before
extraction.** The owner explicitly chose to keep current behavior now and
require ordinary pluginization later under #93. Do not create `plugins/exp/`
or apply this draft's experimental Recipe policy.
[C11](../../../plans/2026-10-19-complete-deletion-plan.md#101a-c11-retained-behavior-and-required-first-party-plugin-extraction)
is the current scope authority. S00–S03 remain reference drafts; their
experimental identities, Recipes and old Eino APIs are not executable plans.
Re-anchor them to stable's typed middleware, preserving same-Run steering and
durable queue/recovery behavior. Keeping core code is not completion of
pluginization.

## Historical extraction draft (2026-10-19)

Epic package for the owner decision of 2026-10-19: nudge, warming and steering are
retained as **independent epics** and **explicit species-selected experimental
packages**. Steering is not deleted and not kept as an always-compiled core switch:
it is a T1 build-owned package that a Recipe selects through generated Assembly, and
an unselected Recipe physically omits.

**Current outcome:** planning only. This package settles the story split, exact
contributions, file ownership, acceptance gates and the Baseline A mapping for a
10-07 tree. No Go/just/CI run, no runtime deletion, no migration, no generation and
no commit is claimed. A plan file is not implementation authorization.

## Authority and baseline

- Shared design authority: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md),
  especially §10.1a C11, §10.3, §11.1–§11.5 and §11.7. Reuse it; do not create a
  competing architecture.
- Retained-feature decision: [issue-91 index](../issue-91/index.md) and
  [D05](../issue-91/D05.md). Those packages keep status precedence for the shared
  deletion cuts; this package owns only the steering experiment.
- Source evidence: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md)
  and [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md). The concurrent
  [Baseline A re-anchor audit](../../../research/2026-10-19-baseline-a-reanchor.md) §2.6 and §4.3–§4.4
  was read as a cross-check for the steering paths; it is a peer deliverable, not edited here.
- Baseline A: commit `202aac38` (2026-10-07, merge of
  `feat/vivy-code-parity`), branch `feat/agent-loop-deletion`. All product paths below
  were read at that commit. The delta from `202aac38` on this branch is docs only;
  the 10-18 plan's code references are validated or remapped in the table below.
- Pins: Go `1.26.4`, `github.com/cloudwego/eino v0.9.13`,
  `github.com/cloudwego/eino-ext` components as in `go.mod`. BML is an independent
  Go module and is not touched.

## Requirements and coverage

| ID | Observable outcome | Stories |
| --- | --- | --- |
| SR1 | Selection is explicit and physical: an unselected Recipe imports neither `plugins/exp/steering` nor its runtime adapter, allocates no active steering controller/path, registers no control, and reports `NOT_COMPILED`; persisted steer turns are read only into a recovery projection. | S00, S02, S03 |
| SR2 | A selected species keeps real same-Run steering: one RunID, boundary-safe native cancel, checkpoint resume, one joined injected user message, one `turn.steered` marker per drained item, no sibling Run and no effect replay. | S01, S02, S03 |
| SR3 | Generic follow-up, cancel, approval/question recovery, durable queue/journal storage and Runner/checkpoint primitives stay in core; steering does not become a second queue, scheduler or runner. | S01, S02, S03 |
| SR4 | Settings/UI/events parity for selected species; unselected species exposes an explicit unavailable/recovery-required state for already-persisted steer turns and never applies them silently. | S02, S03 |
| SR5 | No Vivy-owned hash is introduced. Only a steering-owned custom hash binding, if one is found, and proven duplicate steering setup are removed; shared hash-free checkpoint/approval contracts stay with their owners. | S00, S01, S03 |
| SR6 | Verification is build/pack/inspect plus named runtime/UI tests on the selected and unselected compositions; residue and import-quarantine audits are part of acceptance. | S00–S03 |

## Architecture decisions

1. **Typed contribution chain only.** `plugins/exp/steering` -> `sdk/port/loopexp`
   (pure factory contract) -> selected adapter `internal/runtime/extensions/steering`
   -> generated Assembly binding -> app injection. There is no registry, directory
   scan, runtime discovery or Module-controlled import.
2. **One closed T1 Port.** `core/steering-factory@v1`, owner `vivy/exp-steering`,
   cardinality `0..1`, requirement `conditional`, registered in
   `internal/moduleport/ports.go`. The compiler's existing T1/owner check
   (`validateInternalProvider`, `moduleport.ValidateProvider`) makes a T2 source
   structurally unable to provide it.
3. **Selection is the Recipe module list.** Unselected means no generated import,
   no constructor, nil factory accessor and `NOT_COMPILED` RPC behavior. There is no
   hidden always-compiled switch, no disabled path and no no-op implementation.
4. **The execution owner does not move.** `Service.Run`, the one Eino `Runner`, the
   Journal, budgets, terminal state and durable queue records stay in core. The
   package owns decisions (`Route`/`Take`/`Render`) and the adapter translates them
   into Host calls (`Enqueue`, `RequestCancelAfterModel`, `ContinueBoundary`).
5. **Recovery is a first-class state.** A persisted steer turn opened by a species
   without the package is reported as recovery-required and can be settled by generic
   cancel or cleared back to the user; it is never auto-applied, dropped or rewritten
   into a hidden legacy steer path.
6. **No hash and no duplicate setup.** The steering path has no Vivy-owned digest at
   Baseline A. Shared checkpoint/approval hash removal belongs to C3/C6 owners
   (D03/D08); this epic consumes their accepted contracts and removes only what the
   steering path itself duplicates.

## Story DAG

| Story | Deliverable | Immediate predecessors | Status |
| --- | --- | --- | --- |
| [S00](S00-seam-binding.md) | Closed Port, SDK factory, T1 source entry, GoBinding/generator, typed runtime slot, adapter factory, generated binding, `recipes/exp-steering.vivy.yml`, omission/NOT_COMPILED tests | External G1, G2, G2b; no in-epic predecessor | Planning |
| [S01](S01-policy-extraction.md) | Same-Run queue orchestration moved to `plugins/exp/steering` through a core Host; `Service.Steer`/boundary/resume/queue-state rewired; core keeps generic queue/follow-up/cancel/Runner | S00; external G2, G2b | Planning |
| [S02](S02-adapter-settings-recovery.md) | Capability-derived RPC/UI controls, settings/event parity, paused-steer recovery-required matrix, passive historical rendering | S01; external G2, G2b | Planning |
| [S03](S03-evidence-removal.md) | Steering-owned hash/duplicate audit and removal, deterministic regeneration, none/each/all-three evidence, residue/import audit and handoff | S02; external G2, G3, G4 | Planning |

**Waves (serial, one write lane):** `{S00} -> {S01} -> {S02} -> {S03}`.
The Story files overlap on `internal/runtime/service.go`, `queue.go`,
`internal/rpc/control.go`, the generated Assembly and the UI queue controls. They are
one lane deliberately; no parallel write lane is released by this package.

## Shared seam arbitration (cross-epic)

The three independent experiment epics share the generic loop-exp seam. These files are
**not steering-owned** and must not be rewritten independently by three branches:

| Shared artifact | Rule |
| --- | --- |
| `sdk/port/loopexp/` package directory | One package; each experiment adds its own file (`steering.go` here) and never edits a sibling file. |
| `internal/runtime/extensions.go` | One shared file containing the three factory/controller contracts; add only the steering declarations. If a lead-owned seam commit already exists, append; otherwise create the envelope and mark it append-only for E-NUDGE/E-WARMING. |
| `internal/moduleport/ports.go`, `sdk/internal/assembly/source.go`, `sdk/internal/assembly/runtime_generate.go`, `sdk/internal/assembly/graph.go`, `sdk/internal/assembly/compiler.go`, `sdk/internal/frontend_v1.go`, `sdk/internal/cmd/generate-default/main.go`, generated Assembly | One append-only seam commit adds the three Ports, bindings and accessors. E-STEERING adds `core/steering-factory@v1` and the steering binding/accessor; it must detect and extend the shared seam rather than re-create it. |
| `internal/app/assembly_loop_extensions.go` (proposed) and the Service construction call | Shared app integration point. Steering owns only `internal/app/assembly_steering.go` (unwrap) and one registration call; do not replace sibling wiring. |
| `recipes/exp-loop-features.vivy.yml` | G4 composition artifact; see below. |

If the epics run in parallel worktrees, the seam commit is a merge prerequisite: rebase
and append. Two independent rewrites of `runtime_generate.go`/`ports.go` are a conflict,
not a merge exercise. This package's S00 is written to consume an existing accepted seam
(D04/D05 or a lead-owned base Story) and only creates the steering portion when absent.

## Baseline A mapping and remaps

| 10-18 plan reference | Baseline A fact (202aac38) | Remap for this epic |
| --- | --- | --- |
| `internal/runtime/queue_steer.go` | File does not exist. | Steering lives in `internal/runtime/queue.go:3-13,168-226,366-405,512-541,597-691` and `internal/runtime/service.go:311-314,2963-2977,3568-3597,4658-4676,4949-5005`. S01 edits those ranges; no new `queue_steer.go`. |
| Proposed EXP package `plugins/exp/steering/` | No such package. Plugin convention at baseline: `vivy-module.yaml` + `New` + `NewProvider` (`plugins/hello-fs/`). | Create proposed package with `vivy-module.yaml`, `descriptor.go`, `module.go`, `provider.go`, `policy.go`, tests. |
| `sdk/port/loopexp/steering.go` | Does not exist; `sdk/port/` contains only public ports. | Create the exact §11.3 signatures there. |
| `internal/runtime/extensions.go` typed slot | Does not exist; factory precedent is mask/cognitive. | Create/append the shared proposed file; generated field type is `runtime.SteeringFactory`. |
| `internal/runtime/extensions/steering/` adapter | Does not exist. | Create proposed `adapter.go` with `Factory(loopexp.SteeringFactory) runtime.SteeringFactory`. |
| Generated binding `steeringrt.Factory(expsteering.New)` | `New` is the lifecycle Module constructor everywhere; `verifySource` requires exported `New` and `NewProvider`. | Bind `steeringrt.Factory(expsteering.NewProvider)`; `New` stays the lifecycle constructor. |
| GoBinding typed fields | `sdk/internal/assembly/source.go:32-58` has `MaskFactory`, `CognitiveFactory`. | Add `SteeringFactory`; emit like `runtime_generate.go:143-178,277-282,566-587`. |
| T1 source table | `sdk/internal/frontend_v1.go:220-239` (`repoSourceDirs`) plus a duplicated hardcoded list in `sdk/internal/cmd/generate-default/main.go:49-70`. | Add one shared build-owned source table entry and make both consumers use it; do not add a third copy. |
| Queue Host contract | The 10-18 Host names were proposals; baseline has no Host type. | Implement the §11.3 core-side Host in S01 over the existing queue/service functions. |
| `recipes/exp-steering.vivy.yml`, `recipes/exp-loop-features.vivy.yml` | Neither exists; existing Recipes list only modules. | S00 creates `exp-steering.vivy.yml`. `exp-loop-features.vivy.yml` is a cross-epic composition artifact (G4), not authored here. |
| `steering_mode` config | No steering config key exists. Modes are RPC `queue/mode` (`internal/rpc/control.go:1424,3829-3845`) and `domain.QueueMode*`. | No new config key. Selected species keeps RPC mode parity; unselected returns `NOT_COMPILED`. |

## Selection matrix

| Selection | Recipe | Expected composition |
| --- | --- | --- |
| none | `recipes/minimal.vivy.yml`, `recipes/lite.vivy.yml` (unchanged) | No `agent-vivy/plugins/exp/steering` or adapter import; `HasSteeringFactory()==false`, `SteeringFactoryValue()==nil`; `turn/steer` and steer-track `queue/mode` return `NOT_COMPILED`; generic follow-up/cancel/approval work; persisted steer turns are recovery-required. |
| each | `recipes/exp-steering.vivy.yml` (new; baseline minimal set plus `vivy/exp-steering`) | Package and adapter imported, factory bound, full steering behavior, settings/UI/events parity. |
| all-three | `recipes/exp-loop-features.vivy.yml` (G4, shared with E-NUDGE/E-WARMING) | All three experiment packages selected; this epic verifies steering alongside the sibling contributions without editing their packages. Blocked until G4 exists. |

## Eino capability check

The pinned `github.com/cloudwego/eino v0.9.13` was inspected read-only (module
source, `adk` package):

- Runner/checkpoint: `adk.NewRunner`, `adk.RunnerConfig{Agent, EnableStreaming, CheckPointStore}`, `(*adk.Runner).Run/Query`, `(*adk.Runner).Resume/ResumeWithParams`, `adk.ResumeParams{Targets map[string]any}`, `adk.WithCheckPointID`, `adk.CheckPointStore` (`Get`/`Set` plus optional `Delete`), `adk.ChatModelAgentResumeData{HistoryModifier}`, `adk.InterruptInfo`, `adk.InterruptCtx`.
- Cancellation: `adk.WithCancel() (AgentRunOption, AgentCancelFunc)`, `adk.AgentCancelFunc`, `adk.WithAgentCancelMode`, `adk.CancelAfterChatModel`, `adk.CancelAfterToolCalls`, `adk.CancelImmediate`, `adk.CancelHandle.Wait`.
- Baseline usage is already the supported mechanism: `internal/runtime/engine.go:424-427,546-561`, `internal/runtime/queue.go:642`, `internal/runtime/queue_test.go:3-6` (mechanism M2).

**Recorded gap that justifies the minimal custom seam:** Eino provides the native
safe-point cancel, checkpointed resume and history-modification mechanism, but it has
no durable turn queue, no steer/follow-up lane, no eligibility/mode policy and no
exactly-once drain contract. The product must decide when a queued instruction is
eligible, persist the decision, arm the cancel, drain the selected items once and
resume the same checkpoint. `WithHistoryModifier` as a run option is deprecated; the
supported seam is `ChatModelAgentResumeData.HistoryModifier` inside
`ResumeParams.Targets`. Eino's `CheckPointStore` also carries no Vivy run/prompt
identity; that binding is the existing C6 store, not a steering wrapper. The custom
code is therefore limited to the pure policy and the Host adapter; it must not wrap
or replace the Runner. Removal boundary: if Eino later ships an identity-bound
durable steering/turn primitive, delete the policy/adapter and consume it; the core
Host remains the generic queue/cancel primitive.

## Delivery prerequisites (external, recorded not assumed)

- **G1** accepted D00 scope/contract freeze from the issue-91 package.
- **G2** accepted D06 hash-free source/generation and typed-binding generator. At
  Baseline A every repository Module descriptor is checked against a source SHA256
  (`sdk/internal/assembly/source.go:76-89`) and `verifySource` demands `New` +
  `NewProvider`; adding the package before G2 would reintroduce a custom hash binding.
- **G2b** accepted shared loop-exp seam envelope (D04/D05 or a lead-owned base Story):
  one `sdk/port/loopexp` package, `internal/runtime/extensions.go`, the three factory
  Ports, the typed generated bindings and the app integration point. E-STEERING appends
  only the steering portion; without G2b it creates the minimal envelope under the
  append-only rule above and records that in handoff.
- **G3** accepted C3/C6 hash-free approval/checkpoint contracts (D03/D08) if S03 is
  to claim shared-boundary hash removal. Without G3, S03 must record the shared
  boundary as owned elsewhere and must not delete another owner's fields.
- **G4** shared all-three Recipe/composition artifact from the sibling experiment
  epics. Without G4, the `all-three` row is recorded blocked and is not passing evidence.
- **G5** target environment with Go `1.26.4`, just, pnpm and dependencies for the
  listed gates. This planning turn ran none of them.

## Global constraints

- English documents; concrete existing paths or explicitly marked proposed paths.
- One `Service.Run` / Eino / Journal / policy path; no second runner, queue, scheduler
  or source of truth. Only `internal/runtime/` and `internal/provider/` import Eino.
- T1 build-owned source only may provide `core/steering-factory@v1`; T2 cannot.
- Unselected Recipes physically omit package, adapter, state allocation and controls.
- No hash introduced; no disabled/no-op/alias/legacy path; no hidden auto-apply of
  persisted steer turns.
- Keep generic follow-up, cancel, approval/question recovery, durable storage and
  Runner primitives in core; steering keeps the same RunID and native
  cancel/drain/checkpoint-resume semantics.
- No production journal access, database reset, schema change, release rewrite,
  commit or push in this planning turn. Do not run go/just/CI while planning.

## Review focus

- Trust bypass: a T2 source, a fixture record or a Module-controlled binding supplying
  the steering Port, or a generated Assembly loading the package without selection.
- Second path: a nil controller silently falling back to baseline core steering, a
  second queue/runner, or a resume leg that creates a new Run.
- Loss/duplication: steer items applied twice, dropped, silently demoted, or restored
  without a Journal marker across restart/recovery.
- Omission leak: unselected build still allocating steer lane state, registering
  controls or reading settings; selected build behaving like a no-op.
- Hash reintroduction under another name (`digest`, `checksum`, `fingerprint`) in the
  descriptor, binding, queue payload or checkpoint path.
- UI truth: a control rendered from a hardcoded feature list instead of the compiled
  capability, or a recovery state hidden from the user.

## File ownership summary

| Story | Owns (proposed creates in bold) |
| --- | --- |
| S00 | **`sdk/port/loopexp/steering.go`**, **`plugins/exp/steering/`**, **`internal/runtime/extensions/steering/adapter.go`**, **`internal/app/assembly_steering.go`**, **`recipes/exp-steering.vivy.yml`**; shared append-only `internal/runtime/extensions.go` and `internal/app/assembly_loop_extensions.go` (if present); `internal/moduleport/ports.go`, `sdk/internal/assembly/{source,runtime_generate,compiler,graph}.go`, `sdk/internal/frontend_v1.go`, `sdk/internal/cmd/generate-default/main.go`, `internal/generated/assembly/zz_default.go`, one `internal/app/app.go` registration call. |
| S01 | **`internal/runtime/steering_host.go`**; `internal/runtime/{queue,service}.go`, `internal/runtime/extensions/steering/adapter.go`, `internal/runtime/queue_test.go`, **`internal/runtime/steering_host_test.go`**, steering registration in `internal/app/assembly_steering.go`/shared integration point. |
| S02 | `internal/runtime/service.go`, `internal/rpc/control.go`, `ui/src/components/chat/{ChatInput,ChatView}.tsx`, `ui/src/lib/{api,store}.ts`, `ui/src/i18n/{en,zh}.ts`, `internal/rpc/control_test.go`, `internal/app/codeface_rpc_test.go`. |
| S03 | **`docs/logs/2026-10-19-epic-steering/`** (implementation log, not created by planning), `internal/runtime/{queue,service}.go`, generated Assembly, `recipes/`, residue tests. No issue-91 or research re-anchor file edits. |

## Planning verification and change control

Checked for this package: required files exactly five; English; local links resolve;
every cited existing path exists at `202aac38`; the proposed paths are marked;
Story headings and predecessor declarations match D04; no placeholder tokens;
`git diff --check` clean. Not run: Go, just, CI, generation, pack/Inspect, UI, database,
provider or migration checks. No product behavior is claimed.

A shared-contract change (Port name/signature, Host method, factory binding form,
Recipe identity, recovery terminology) invalidates downstream readiness. Revise this
index and affected Stories together; do not create a parallel report. Selection of a
species, trust class, or public RPC/UI behavior returns to owner review. The final
implementation commit must state exactly what was verified; a planning commit does
not close the epic.

## Open questions and blocking facts

1. **G2 dependency is real.** At Baseline A a new repository Module cannot be admitted
   without a source SHA256, and a hash-free descriptor would fail `NewSourceCatalog`.
   S00 is blocked until G2 lands; do not work around it with a hash.
2. **Shared seam ownership across the three epics.** `internal/runtime/extensions.go`,
   the Port catalog, the generator and the app integration point are one append-only
   seam shared by E-NUDGE/E-WARMING/E-STEERING. The independent-epic decision does not
   make three parallel rewrites safe. The Lead must name the seam owner (D04/D05 or a
   base Story) and land it before the three S00s, or sequence the S00s so the first
   creates the envelope and the others append. This is a delivery blocker, not a
   documentation preference.
3. **All-three recipe ownership.** `recipes/exp-loop-features.vivy.yml` necessarily
   names packages from three independent epics. E-STEERING does not create it; G4
   must name its owner. Until then the `all-three` selection row is blocked, while
   `none` and `each` are in-epic.
4. **Shared hash removal ownership.** The only hashes on the steering recovery path
   belong to the checkpoint/approval envelopes (`internal/runtime/checkpoint.go:41,113-115`
   and the approval binding), owned by G3. S03 cannot claim their removal; it can only
   prove the steering path adds no hash and remove a steering-owned binding if one is
   found during implementation.
5. **Runtime cancellation check granularity.** Baseline arms only
   `CancelAfterChatModel` (`internal/runtime/queue.go:208`); the comment explains why
   `CancelAfterToolCalls` is wrong for this checkpoint shape. S01 must preserve that
   exact safe point; changing it would change the recovery contract and needs a
   separate design decision.

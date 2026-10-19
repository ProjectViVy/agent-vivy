# E-WARMING: species-selected experimental cache warming

Epic package for extracting real opt-in proactive prompt-cache warming out of the
single core runtime into the explicit experimental package `vivy/exp-warming`,
under the owner decision recorded in the issue-91 package (2026-10-19).

**Current outcome:** planning only. Four Story plans (W00–W03), one sequential
DAG, explicit contracts, Baseline A mapping and acceptance obligations. No Go
package, Recipe, generated Assembly, settings field, event or core deletion has
been implemented, and no product CI, benchmark or real-provider result is
claimed. The toolchain (Go, `just`) is absent in this environment; this package
was written by reading the Baseline A tree.

## Authority and baseline

- Owner decision and shared design: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md),
  especially C11 (§10.1a), §10.3 and §11.1–11.5. Reuse this authority; do not
  create a competing spec.
- Story conventions and prerequisite evidence: [issue-91 index](../issue-91/index.md).
  The base-cut Stories are seam-only; this epic owns the package, the selection,
  physical omission and the single-serving cutover.
- Base-cut seam contract consumed by W00: [D05](../issue-91/D05.md) (inert
  loop-exp seam for warming/steering; no `plugins/exp/*` in the base cut).
- Source evidence: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md)
  §5.4/§8 and [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md)
  §4.6. They are findings, not acceptance.
- Baseline A: commit `202aac38` (2026-10-07, pi-parity closeout). Code in this
  worktree is byte-identical to that commit; only docs were replayed on top
  (`git diff 202aac38 HEAD -- internal sdk plugins recipes ui` is empty).
  Baseline-A path mapping is also recorded in
  [the re-anchor audit](../../../research/2026-10-19-baseline-a-reanchor.md).
- Pinned versions: Go 1.26.4 (`go.mod`), Eino `v0.9.13`, EinoExt
  `components/model/claude v0.1.25` (repository pins; no upgrade is implied).
- Baseline facts below were verified by reading this tree on 2026-10-19. The
  10-18 plan's paths that do not exist here are remapped in each Story's
  **Baseline A mapping** section; they are not silently assumed.

## Owner-confirmed scope

Warming is retained as an explicit species-selected experimental package. It is
not deleted, not disabled and not silently replaced by ordinary error text or a
never-called warmer. In a Recipe that selects `vivy/exp-warming` the real
behavior remains: an eligible warming makes a real minimal maintenance request
on the current provider, the provider-reported prompt/cache-write tokens are
reported as the `cache.warmed` diagnostic, and configured-off (`off`) is
distinct from not compiled (`NOT_COMPILED`). In a Recipe that does not select
the package, the package implementation, constructor, capability projection,
settings reads, RPC/action registration, maintenance requests and UI controls are
physically absent.

## Requirements and coverage

| Requirement (issue-91 vocabulary) | Observable outcome | Owning Stories |
| --- | --- | --- |
| R1: no Vivy-owned hash implementations, state, protocols, fields or active requirements | warming path has no prefix digest/hash-only evidence; residue guard | W03 |
| R2: one production model → real tool → exact result → next model request → terminal path on existing Eino | the maintenance request rides the existing run cancellation and the existing single `Service.Run`/Eino/Journal path; no runner, scheduler, queue or receipt framework is added | W02, W03 |
| R3: preserve approved intent, permission/scope, durable ownership/outcomes, no blind replay | warming has no tool/approval authority; it cannot create, retry or claim tool effects | W00, W02 |
| R5: delete selected mechanisms through their complete closure; preserve retained packages as species-selected | core warming body deleted in the same change that serves from the package; unselected builds physically omit it | W02, W03 |
| R6: retain real behavior in selecting species, omit live surfaces in unselected ones | eligible warm is real and accounted; off ≠ not compiled; Settings/event parity | W02 |

## Story index

| Story | Title | Status | Immediate predecessors | Write scopes |
| --- | --- | --- | --- | --- |
| W00 | Bind the warming package to the loop-exp seam and species selection | planned | accepted #91 D05 seam; D06 source/generation contract | `plugins/exp/warming/`, `recipes/exp-warming.vivy.yml`, `internal/moduleport/`, `sdk/port/loopexp/`, `sdk/internal/assembly/`, `sdk/internal/cmd/`, `internal/runtime/extensions.go`, `internal/app/assembly_loop_extensions.go`, `internal/generated/assembly/` |
| W01 | Extract the pure warming policy | planned | W00 | `plugins/exp/warming/`, `sdk/port/loopexp/warming.go` |
| W02 | Bind the runtime adapter, settings, accounting, recovery and cut over serving | planned | W01, W00 | `internal/runtime/extensions/warming/`, `internal/runtime/cachewarmer.go`, `internal/runtime/service.go`, `internal/runtime/model_stream_observer.go`, `internal/runtime/payloads.go`, `internal/config/config.go`, `internal/config/config_test.go`, `internal/rpc/control.go`, `internal/app/app.go`, `config.example.yaml`, `ui/src/lib/api.ts` |
| W03 | Remove hash-only warming evidence and close the residue | planned | W02 | `internal/runtime/extensions/warming/`, `internal/runtime/cachewarmer.go`, `internal/runtime/payloads.go`, `schemas/events/payloads/` |

Derived DAG and waves (one active write lane, sequential):

```text
W00 (wave 1: selection) -> W01 (wave 2: policy) -> W02 (wave 3: serving) -> W03 (wave 4: evidence/residue)
```

The DAG is sequential on purpose: W00–W02 touch the same generated binding and
the same runtime seam, and W02 is the single change that both starts serving from
the package and deletes the core policy body. Parallel lanes would create a dual
path or two writers in one generated file.

## Selection matrix (C11 obligation)

| Case | Recipe | Expected generated Assembly | Expected runtime | Expected evidence |
| --- | --- | --- | --- | --- |
| none | `recipes/minimal.vivy.yml` (never selects it) and any other Recipe that does not select the package | no `vivy/exp-warming` module, no plugin import, no `WarmingFactory` field value; `HasWarmingFactory() == false`; unchanged artifacts byte-identical | no constructor, no settlement trigger, no maintenance call, no `cache.warmed` event | generated-file inspection; `go test ./internal/app` (import/assembly); no warming config read |
| each | `recipes/exp-warming.vivy.yml` (created by W00) | `vivy/exp-warming` selected, plugin import + `warmingrt.Factory(expwarming.New)`, `HasWarmingFactory() == true` | eligible settled main/child call warms once; off makes zero calls | selected-generation fixture test; runtime real-path test in W02 |
| all | `recipes/exp-loop-features.vivy.yml` (integration-owned; W00 contributes only `- vivy/exp-warming`) | all three EXP modules selected, one binding each | each package's behavior independent; warming does not read nudge/steering state | combined-generation test when the combined Recipe exists; W00 reports the exact single line it owns |

Whether the shipped default species Recipes (`default`, `diva`, `vivy-code`, …)
select `vivy/exp-warming` to preserve today's warming behavior or leave it
unselected is an owner decision (Q0), not an assumption of this package. The
`none` evidence always uses `recipes/minimal.vivy.yml`; the default-family
generated artifact is checked against the owner's choice once recorded.

## Epic decisions (Baseline A remaps)

These resolve clauses of the accepted design that do not exist at Baseline A.
They are deliberate, recorded decisions, not omissions.

- **A1 — accounting.** Baseline A warms on the raw engine chat model with the
  model-call observer stripped (`internal/runtime/cachewarmer.go:170-211`,
  `internal/runtime/model_stream_observer.go:89-96`), so a warm is *unobserved
  and unbudgeted at the ledger level*; its accounting record is the
  `cache.warmed` payload with `prompt_tokens` and `cache_write_tokens`. §11.5's
  "same provider, budget admission and mandatory observation/accounting
  lifecycle, with source `maintenance`" is **NOT_PRESENT** at Baseline A. This
  epic preserves the Baseline A accounting and does **not** add a maintenance
  ledger source: adding one would change billing totals and create a second
  accounting route, which needs separate owner authorization (Q1).
- **A2 — recovery/shutdown.** Baseline A keeps one in-flight guard, cancels the
  warm with the run context, silently drops a run-cancelled warm, bounds the
  call at 60 s, and journals on a detached bounded context. §11.5's
  "shutdown/cancel … waits under the current owner" is **NOT_PRESENT**; this
  epic does not add a shutdown wait because that changes Run terminal/shutdown
  timing (Q2).
- **A3 — hash-only prefix diagnostics.** `PrefixSHA256`, hash-only JSON prefix
  encoding and `prefix_digest_failed` do **not** exist at Baseline A (zero
  matches in `internal/`, `sdk/`, `ui/`, `schemas/`; the re-anchor audit §2.6
  and §4.3.5 record the same and conclude hash removal is not part of warming
  extraction at this baseline). W03 records that evidence and removes those
  symbols in the same change if the gated post-baseline batch track
  re-introduces them; it also adds a residue guard. No hash is reintroduced by
  this epic.
- **A4 — settings/UI.** Baseline A has YAML settings only
  (`runtime.cache_warming`, `runtime.cache_warming_min_savings`) and **no**
  warming UI, no `settings/get`/`settings/update` section and no payload JSON
  schema. C11's "unselected contributes no UI controls" therefore holds
  vacuously; this epic does not invent a presentation-host UI slot. W02 adds the
  selected-only capability/settings projection and the missing config
  parse/validate tests; an editable control is deferred (Q3).
- **A5 — call attribution.** `payloadCacheWarmed.CallID` is declared
  (`internal/runtime/payloads.go:194-203`) but never populated by the only
  producer (`cachewarmer.go:213-228`). W02 makes the declared field real from
  the maintenance settlement; no other payload property changes.
- **A6 — example config.** `config.example.yaml` documents no warming setting.
  W02 adds both keys with the selected-package note; it does not change the
  defaults (`streaming`, `0.05`).
- **A7 — tests.** There are no `internal/config` tests for warming today. W02
  adds them ("new config fields need parse/validate tests").
- **A8 — prefix source.** Baseline A builds the warm prefix from the engine's
  static instruction plus selected tool infos resolved against
  `eng.toolInfos` (`cachewarmer.go:92-99, 187-190`). §11.5 requires the Host to
  derive it from the observed invocation. W02 derives it from the observed
  `modelCallInput` captured at `Begin`; an equivalence test pins that the warm
  request messages/tools are identical to Baseline A on the normal chat path.
  No prefix cache is added.
- **A9 — policy shape.** `WarmingDecision` in §11.3 carries only
  `{Reason, Request}`. The idle-refresh delay (Baseline A:
  `CacheLifetimeSeconds * 0.8`) and the fixed request scalars are policy
  decisions, not adapter decisions, so W01 adds them to
  `sdk/port/loopexp/warming.go` as exported contract constants
  (`WarmingIdleHeadroom = 0.8`, `WarmingMaxOutputTokens = 16`,
  `WarmingTimeoutMillis = 60000`, `WarmingTokenFloor = 2048`,
  `WarmingPrompt = "Reply with one word: ok."`) and keeps the struct shape from
  §11.3 unchanged.

## Shared gates (all Stories)

- One `Service.Run` / Eino / Journal / policy path; one terminal transition; no
  second runner, scheduler, queue, receipt framework or cross-Run prefix cache.
- `core/warming-factory@v1` is T1-only: the build-owned Source Catalog assigns
  T1 to `repo:plugins/exp/warming` and to the exact module/import/constructor.
  T2 sources cannot provide it, impersonate `vivy/exp-warming` or replace the
  adapter.
- Import quarantine: only `internal/runtime/` and `internal/provider/` import
  `github.com/cloudwego/eino*` (`TestEinoImportsQuarantined`); plugins and
  public SDK windows never import `agent-vivy/internal/`
  (`TestPluginWindowCannotImportInternal`). The EXP package imports only the
  standard library and `agent-vivy/sdk/...`.
- Physical omission: unselected Recipes contain no package import, constructor,
  capability projection, settings read, event producer or UI control.
- Configured-off (`off`) is not compiled-out: a selected build with
  `runtime.cache_warming: off` constructs the package but makes zero maintenance
  requests and emits no `cache.warmed`; an unselected build reports
  `NOT_COMPILED` for a warming-only request.
- No Vivy-owned hash is added; no disabled path, alias, renamed hash or no-op
  verifier is used to satisfy a gate.
- No production Journal access, no database reset, no release-migration
  rewrite, no commit/push by the implementing agent, no feature flag that
  switches back to a core-owned policy.
- Current mandatory CI (`just ci`) and the affected real-path checks remain in
  force for implementation. A skipped environment/format check is not passing
  evidence.

## Eino capability check (required before code)

Versions inspected in this tree: Go 1.26.4, Eino `v0.9.13`, EinoExt
`components/model/claude v0.1.25`. Inspected APIs that bear on warming:

1. `github.com/cloudwego/eino/components/model`: `BaseModel.Generate`,
   `ToolCallingChatModel`/`WithTools`, `model.WithMaxTokens`, `model.WithTools`,
   `model.GetCommonOptions`. Baseline A uses these to issue the warm request
   (`cachewarmer.go:183-191`). They can *carry* a maintenance request; they
   provide no trigger, eligibility, idle timer, in-flight guard or accounting.
2. `github.com/cloudwego/eino/schema`: `SystemMessage`, `UserMessage`,
   `ToolInfo`, `Message.ResponseMeta`/`TokenUsage` — message and usage shapes
   used by the request and by the reported tokens (`cachewarmer.go:187-209`).
3. `github.com/cloudwego/eino-ext/components/model/claude`:
   `GetCacheCreationInputTokens` — the provider cache-write extra read by the
   accounting path (`cachewarmer.go:206`). Anthropic-family only; no fake
   warming for providers without `supports_warming`.
4. `github.com/cloudwego/eino/adk`: `ChatModelAgentMiddleware`,
   `BaseChatModelAgentMiddleware.WrapModel`, `ModelContext` — the model-level
   wrapping already used by prompt/compaction/tool-visibility middlewares
   (`engine.go:297-373`). It can inspect or replace a request inside the single
   Runner, but it has no post-settlement hook that issues an auxiliary provider
   call; doing it inside `WrapModel` would add a round trip inside every
   foreground call instead of refreshing a settled cache.
5. `github.com/cloudwego/eino/callbacks`: model callbacks are observability
   (`OnStart`/`OnEnd`/stream callbacks) and cannot initiate a maintenance
   request, own a 60 s timeout, or drive accounting.
6. `compose` graph/chain/workflow: no scheduler, idle timer or
   maintenance-invocation primitive; adding a compose graph would create a
   second execution path beside the single ADK Runner.

**Recorded gap:** pinned Eino has no prompt-cache-warming or
maintenance-invocation seam. The required behavior — after a settled
foreground/child call, decide and issue one minimal same-model request under the
run's cancellation with one in-flight guard, then report provider tokens — is a
runtime lifecycle decision, not an LLM orchestration primitive. Vivy therefore
keeps the existing minimal custom seam (warm request construction plus the
settlement trigger already in `internal/runtime`) and moves only the *decision
policy* behind the Eino-free `loopexp` contract; the adapter stays inside the
`internal/runtime` quarantine. **Migration/removal boundary:** if Eino later
provides a maintenance/cache-warming primitive or a post-settlement middleware
hook, delete the adapter's policy glue and bind that primitive through the same
`core/warming-factory@v1` slot; product, settings, event and UI contracts do
not change. `docs/eino-capability-verify.md` covers the checkpoint, streaming
and cancellation surfaces already relied on; it contains no warming API.

## Acceptance matrix

| Claim | Evidence required before it may be made |
| --- | --- |
| selected build warms | real-path runtime test with a scripted warmable model: one maintenance request after settle, `cache.warmed` with prompt/cache-write tokens and the settled `call_id`, no messages/context pollution |
| off ≠ not compiled | selected + `off`: zero maintenance calls, zero `cache.warmed`; unselected: no package/binding and `NOT_COMPILED` for a warming-only settings/RPC request |
| physical omission | generated import list and Manifest for none/each/all; no `agent-vivy/plugins/exp/warming` import in the none artifact |
| single path / no duplicate authority | search + diff review: no core policy body left, no fallback flag, no second scheduler/runner; `Service.Run`/Eino/Journal owners unchanged |
| no hash evidence | W03 scan evidence; zero prefix-digest identifiers; residue guard test |
| parity | config default/validation tests; event payload contract test; app import tests; `just ci` output (implementation time) |

## Open questions (owner decisions; none block planning)

- **Q0 — default-family selection.** Do the shipped species Recipes (`default`,
  `diva`, `vivy-code`, `headless`, …) keep warming by selecting
  `vivy/exp-warming`, or does warming become opt-in with only the explicit
  `exp-warming`/`exp-loop-features` Recipes selecting it? Preserving today's
  behavior argues for selection in the shipped families; C11's experimental
  framing argues for opt-in. W00 records the owner's answer as the exact Recipe
  lines and generates the corresponding artifacts; it does not decide it.
- **Q1 — maintenance budget/observation.** Do you want §11.5's
  `maintenance` model-call source with ledger admission and
  `model.request`/`model.usage`/`model.call.finished` rows? It changes billed
  totals and the run's event stream; this epic currently preserves Baseline A
  (unobserved, `cache.warmed` only).
- **Q2 — shutdown wait.** Do you want the warmer to wait for an in-flight paid
  call during shutdown/cancel (§11.5) instead of the Baseline A silent
  cancellation? It changes shutdown latency.
- **Q3 — editable warming UI.** Baseline A has none. An editable
  `settings/update` control needs a presentation-host settings slot that does
  not exist at Baseline A; approximate this epic adds only the selected-build
  capability/settings projection.
- **Q4 — combined Recipe ownership.** `recipes/exp-loop-features.vivy.yml` is
  shared by E-NUDGE, E-WARMING and E-STEERING. W00 contributes exactly one line
  and verifies the "all" case only once the other epics have contributed theirs.

## Planning verification

Document checks only: paths and symbols were read in this tree, the "no code
delta vs `202aac38`" claim was verified with `git diff --stat 202aac38 HEAD --
internal sdk plugins recipes ui`, and the absence claims were verified by
content search. No `go`, `just`, generation, CI, provider call or real-path test
was run: the toolchain is absent in this environment, and this is a planning
deliverable. Every implementation Story must return its own commands and
results; a planning file is not implementation evidence or permission to start.

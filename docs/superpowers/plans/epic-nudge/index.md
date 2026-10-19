# E-NUDGE: Species-selected experimental nudge/coaching epic

Baseline: branch `feat/agent-loop-deletion`, worktree at the rolled-back stable baseline
`202aac38` (2026-10-07 tree). Planning only: no runtime deletion, regeneration, product CI
or real-provider evidence is claimed by this package.

Owner decision (2026-10-19): nudge/coaching is **retained** and becomes an explicit
species-selected experimental package (`vivy/exp-nudge`, `plugins/exp/nudge/`) that
unselected Recipes physically omit. Only FNV signatures and the redundant
consumer-owned result handoff are removed; real coaching behavior stays. This package is
the executable plan for that decision. It does not reopen the retired-hash scope, the
other experiments (warming, steering) or any data/schema decision.

## Authority

- Shared design: [complete deletion plan](../../../plans/2026-10-19-complete-deletion-plan.md),
  especially §10.1a C11, §10.3, §11.1–11.5, §11.11.
- Story conventions and the inert seam this epic consumes:
  [issue-91 index](../issue-91/index.md), [D04](../issue-91/D04.md),
  [D05](../issue-91/D05.md).
- Source diagnosis: [core deletion insight](../../../research/2026-10-19-agent-loop-core-deletion-insight.md),
  [slimming review](../../../research/2026-10-19-agent-loop-slimming-review.md).
- Baseline-A re-anchor: [2026-10-19-baseline-a-reanchor.md](../../../research/2026-10-19-baseline-a-reanchor.md)
  (peer-owned; this package states its own verified mapping below so it is implementable
  without that file).
- Pins: Go `1.26.4` (`go.mod`), `github.com/cloudwego/eino v0.9.13`
  (inspected in the module cache, not re-vendored).

## Outcome

After this epic:

1. `vivy/exp-nudge` exists as a build-owned T1 Module at `plugins/exp/nudge/`, providing
   the closed Port `core/nudge-factory@v1`. It owns the ten-entry exact-tuple window,
   thresholds 3/5, the count-6 loop hard stop, fixed templates and the 1024-byte reminder
   bound; the stop crosses the seam as `NudgeDecision.Stop`.
2. Selected generations get real coaching through one typed factory slot and one small
   Eino adapter (`internal/runtime/extensions/nudge/`). Unselected generations contain no
   nudge import, constructor, window, middleware, state read, event producer or control.
3. The frozen core implementation (`nudge_state.go`, `nudge_middleware.go`,
   `loopguard.go`, `renderNudge`, handoff helpers, resume-batch restore, unconditional
   engine registration) is deleted, not disabled or wrapped.
4. `tool.nudge` keeps its exact v1 payload contract and stays readable in every build,
   including builds that omit the package.
5. No second runner, coordinator, queue, registry or execution path is introduced; the
   single `Service.Run` / Eino / Journal / durable-operation path is unchanged.

## Story DAG and waves

`N00 -> N01 -> N02 -> N03` (four waves). N01/N02 may overlap only with disjoint file
scopes; N02 integration evidence and all cross-cutting verification live in N03.

| Story | Outcome | Immediate predecessors | Status |
| --- | --- | --- | --- |
| [N00](N00-seam-binding.md) | Nudge-specific typed seam binding, T1 catalog entry, generated slot, selection/absence contract | D04 (accepted seam), D02, D03, D06, implementation handoff | Planned — blocked on D04/D02/D03/D06 acceptance |
| [N01](N01-policy-extraction.md) | `plugins/exp/nudge/` descriptor + pure coaching policy + `recipes/exp-nudge.vivy.yml` | N00 | Planned |
| [N02](N02-adapter-settings-recovery.md) | Eino adapter, runtime leg host, activation, durability/recovery, event/UI/settings parity | N00, N01 | Planned |
| [N03](N03-evidence-removal.md) | Delete frozen core implementation, move/carve tests, selection matrix and residue evidence | N01, N02 | Planned |

D04 (issue-91) lands the generic inert loop-exp seam and removes the FNV signatures plus
the consumer-owned handoff while coaching stays in core. This epic therefore **consumes**
D04's seam instead of re-authoring it: N00 adds only the nudge-specific selection binding,
T1 source-catalog entry, generator validation and absence contract; if accepted D04 output
is missing an element, N00 adds that single element following the recorded contract and
never a second variant. N01 moves the already-simplified core policy rather than
reimplementing behavior.

## Verified baseline A mapping (2026-10-07 tree)

Each row was checked in this worktree. Rows marked **remap** correct the 2026-10-19
issue-91 text for this baseline.

| Fact | Baseline A reality |
| --- | --- |
| Window/loop guard | **remap**: `internal/runtime/nudge_window.go` does **not** exist. The window is `internal/runtime/loopguard.go` (53 lines): `loopWindow{sigs []uint64}`, `hash/fnv` FNV-1a, `loopWindowSteps=10`, `loopRepeatLimit=5`, `errLoopDetected`, `record(toolName,argsJSON,result,errMsg string) (int,error)` rejecting `count > 5`. |
| Coaching state | `internal/runtime/nudge_state.go` (393 lines): `nudgeState`, `nudgeBatch`, `nudgeNotice`, `completedCall`, `nudgeTemplateVersion="nudge-v1"`, `window`, `marks`, `notice`, `noticeTaken`, `sealedIDs`, `changed`, `terminal`; `Register/MarkFailure/Failure/Complete/SatisfyDurable/Seal/Take/Abort`; context key binding. |
| Resume restore | **remap**: `restoreResumeNudgeBatch` is `internal/runtime/service.go:4682`, not `nudge_state.go`. It replays `tool.finished` for the interrupted batch's sibling IDs and calls `SatisfyDurable`. No window state is persisted anywhere; a fresh detector is created per leg (`service.go:2953`, `4648`, `queue.go:686`). |
| Boundary middleware | `internal/runtime/nudge_middleware.go` (148 lines): `newNudgeMiddleware(maxContextBytes int) adk.ChatModelAgentMiddleware`, embedding `*adk.BaseChatModelAgentMiddleware`, implementing `WrapModel`; `nudgeWrappedModel` wraps both `Generate` and `Stream`; `Take` before the inner call; `renderNudge`; 1024-byte cap; context-budget check; `tool.nudge` persisted only on `first`; transient tagged `schema.UserMessage` (`kind=runtime_nudge`). |
| Templates | `internal/runtime/prompt.go:105–118`: `nudgeFailureTemplate`, `nudgeRefusalTemplate`, `renderNudge(nudgeNotice) string`. |
| Unconditional install | `internal/runtime/engine.go:372–376`: `handlers = append(handlers, newNudgeMiddleware(cfg.MaxContextBytes))` always, last in the handler list. |
| Leg creation | `internal/runtime/service.go:2953–2956` (run), `4648–4656` (approval/question resume), `internal/runtime/queue.go:682–689` (steering resume). |
| Consumer handoff | `internal/runtime/service.go:3501–3560`: consume maps `tool.requested` -> `Register`, `tool.finished` -> `takeCompletion`/`Complete`, `Seal`, `terminalErr`; `internal/runtime/mapper.go` `openCalls`/`toolBatch`/`completions`/`storeCompletion`/`takeCompletion`/`setNudgeState`, typed-failure projection at `634–675`. |
| Typed outcome classification | `internal/runtime/tool_failure.go` (`toolFailure`, `classifyToolFailure`, `refusalFailure`, `markInvocationFailure`) and `internal/runtime/tooladapter.go:897–949` (soft conversion gated on `nudgeStateFromContext(ctx) != nil`; `markCommandFailure`). |
| Terminal mapping | `internal/runtime/service.go:4774–4783`: `errLoopDetected` -> `run.failed{cause_category:loop_detected}`. |
| Event payload | `internal/runtime/payloads.go:271–280` `payloadToolNudge{tool_call_id,tool_name,reason,repeat_count,template_version}`; `internal/domain/event.go:28,109` `EventToolNudge="tool.nudge"`; `schemas/events/payloads/tool.nudge.json` (v1, strict, enum `[3,5]`, `const "nudge-v1"`); `schemas/events/run-event.schema.json:40` includes it. |
| Design doc | `docs/architecture/NUDGE-DESIGN.md` (rev ND-D1, 2026-09-23) is operative; §3 Eino check, §4 contracts, §6 ordering, §7 event/templates. Plans `docs/plans/nudge/{README,ND-0..ND-4}.md` are historical design records. |
| Tests | `internal/runtime/nudge_state_test.go`, `nudge_middleware_test.go`, `nudge_acceptance_test.go` (product path: real Service/SQLite/governed tools, scripted model), `nudge_contract_test.go` (test-local mirror of the §4 state), `loopguard_test.go`, `loopdetect_test.go`, `tool_failure_test.go`, `enhanced_tooladapter_test.go`, `internal/mcphost/toolworld_test.go`. |
| No experiment precedent | **remap**: `plugins/exp/` and `recipes/exp-*.vivy.yml` do **not** exist at baseline A (the `plugins/exp/loop-detection/deferred.go` citation in the deletion plan belongs to the 10-18 tree). The T1 build-owned source table is `repoSourceDirs` in `sdk/internal/frontend_v1.go:220–239`. |
| Existing seam pattern to copy | `internal/moduleport/ports.go` (`Catalog`, `ValidateProvider`: T1 + exact owner only), `internal/moduleport/masks.go` (`MaskFactory`), `sdk/internal/assembly/source.go` (`GoBinding.MaskFactory/CognitiveFactory`), `runtime_generate.go` (field emission `203–208`, factory validation `140–181`, accessors `567–586`), `internal/app/assembly_validate.go` (`validateMaskFactoryBinding`), `internal/app/masks.go` + `assembly_cognitive.go` (typed `any` unwrap), `internal/modules/defaults/catalog.go` (`Binding`, `Catalog(repoRoot)`), `sdk/internal/cmd/generate-default/main.go` (repo Module `externals` table). |
| Quarantine gates | `internal/app/importlint_test.go`: `TestEinoImportsQuarantined` (line 86; Eino only under `internal/runtime/` or `internal/provider/`), `TestPluginWindowCannotImportInternal` (line 126; `plugins/` may not import `agent-vivy/internal/`). |
| No nudge settings/UI | Baseline has **no** nudge config key, RPC action or feature UI control. `ui/` matches for "nudge" are test text; `tool.nudge` is rendered only by generic event/Inspect projection (`ui` has no `tool.nudge` handler). Selection truth is therefore `generation.Manifest.Modules`. |
| Generation entry | `internal/generated/assembly/generate.go` invokes the SDK generator; the default file is produced by `go run ./sdk/internal/cmd/generate-default -repo . -output internal/generated/assembly/zz_default.go`. No nudge entry exists in `zz_default.go`. |

## Eino capability check (pinned v0.9.13, ADK middleware API)

Inspected at `github.com/cloudwego/eino@v0.9.13/adk` in the local module cache:

| API | Evidence | Use in this epic |
| --- | --- | --- |
| `adk.ChatModelAgentMiddleware` | `handler.go:259` (`= TypedChatModelAgentMiddleware[*schema.Message]`) | Typed slot contract; the adapter implements it. |
| `adk.BaseChatModelAgentMiddleware` | `handler.go:277`, default method set `handler.go:295–313` | Embedded for no-op defaults, as baseline `nudgeMiddleware` does. |
| `TypedChatModelAgentMiddleware.WrapModel(ctx, model.BaseModel[M], *TypedModelContext[M]) (model.BaseModel[M], error)` | `handler.go:254` | The only lifecycle hook used; wraps both Generate and Stream through the returned model. |
| `ModelContext` / `TypedModelContext[M]` | `handler.go:57–81` (`Tools` deprecated, `ModelRetryConfig`, `ModelFailoverConfig`) | Context passed to `WrapModel`; no tool mutation. |
| `ChatModelAgentConfig.Handlers []TypedChatModelAgentMiddleware[M]` | `chatmodel.go:401`; `adk.NewChatModelAgent` `chatmodel.go:503` | Registration point for the selected adapter, appended last (innermost) as today (`engine.go:376`). |
| `adk.NewRunner`, `ResumeParams` | `runner.go:89`, `runner.go:81` | Existing single Runner/resume path; unchanged. |
| `compose.GetToolCallID`, `compose.IsInterruptRerunError` | used by `tooladapter.go:881`, `tool_failure.go:63` | Call-ID correlation and interrupt exclusion stay at the tool boundary. |
| `schema.Message.Extra`, `schema.UserMessage` | used by `nudge_middleware.go:120–126` | Transient reminder tagging; not persisted, not checkpointed. |

Rejected alternatives (recorded, not re-opened): `BeforeModelRewriteState` persists message
edits in ADK state, which would replay synthetic reminders after resume/compaction — the
baseline deliberately uses transient `WrapModel`. `UnknownToolsHandler` remains unused.
`compose.ToolMiddleware` is not needed: classification stays in the existing tool adapter.
No new runner/scheduler/queue is justified; `Runner.Run`/`ResumeWithParams` already own
iteration and recovery.

## Selection and absence contract

- Closed catalog Port `core/nudge-factory@v1`: `CardinalityAtMostOne`, T1, sole owner
  `vivy/exp-nudge`. `moduleport.ValidateProvider` (T1 + exact owner) and the compiler's
  `validateInternalProvider` make a T2 source unable to provide it.
- Selection is Recipe/generated-Assembly only (`recipes/exp-nudge.vivy.yml`). No runtime
  directory scan, no engine import of `plugins/exp/nudge/`, no unconditional wiring.
- Unselected = physical omission: no generated import of the EXP module or adapter, no
  `NudgeFactory` field value, `HasNudgeFactory()==false`, no middleware, no window/state
  allocation, no leg bind, no `tool.nudge` producer, no config read, no control. The
  inert SDK types and the closed catalog definition may still exist.
- Historical `tool.nudge` events in old journals stay readable and renderable
  (generic Inspect) in **all** builds; passive readers never instantiate the package.
- `NOT_COMPILED`: no nudge setting/action/control exists at baseline and this epic must
  not add one, so no unselected build can accept a live nudge setting and silently do
  nothing. If a later Story adds a nudge control, that control must return the existing
  `not_compiled`/`NOT_COMPILED` vocabulary (`internal/modules/masks/service.go:225`,
  `sdk/generation/manifest.go:95`) instead of accepting and ignoring input. N03 adds a
  negative guard test that no such control/setting key exists.
- Recipe policy decision (needs owner confirmation at N00 readiness): the baseline
  installs coaching unconditionally, so every shipped Recipe currently has it. The
  recommendation is to select `vivy/exp-nudge` explicitly in `default.vivy.yml`,
  `headless.vivy.yml`, `diva.vivy.yml`, `vivy-code.vivy.yml`, `scx.vivy.yml` and to omit
  it in `minimal.vivy.yml`, `lite.vivy.yml`, `masks-*.vivy.yml`, so unselected omission
  is exercised by supported Recipes without silently retiring behavior from the shipped
  species.

## Durability and recovery contract (must survive extraction)

1. `tool.nudge` is appended through the existing `persistAndPublish` path **before** the
   reminder is injected; a failed append aborts that attempted model call with its
   original cause. This is not best effort.
2. Provider retries see the identical notice and must not schedule a second event.
3. A fresh detector is allocated per leg (initial run, approval/question resume, each
   steering resume) — no persisted window and no stale notice. Resume restores sibling
   batch **coverage** from the journal (`tool.finished` for `resumeBatchIDs`): replayed
   siblings satisfy coverage and are excluded from the fresh window.
4. Native `ToolsNode` ordering supplies the batch order; `WrapModel` reads the trailing
   `schema.Tool` messages in that order and asks the leg host for producer-owned
   outcomes. Missing expected outcomes are an invariant error — never a wait, a
   Register/Seal barrier or a parked completion.
5. Effect ownership, approval binding, budgets, final message and terminal state stay
   synchronous and are not affected by nudge presence. A refused pre-claim call has a
   typed `not_executed` outcome and no invented execution start.
6. The stop remains single-homed through one terminal mapping: the package's `Stop`
   decision maps once to `errLoopDetected` and `run.failed{cause_category:loop_detected}`;
   no second iteration guard and no core window remain.

## Acceptance matrix

| Risk | Required evidence | Owner |
| --- | --- | --- |
| Coaching still real when selected | Thresholds 3/5, ten-step window, refusal/corrective templates, 1024-byte cap, context-budget skip, hard stop at 6, provider-retry dedupe — through production `Service`/`Engine` | N01, N02, N03 |
| Physical omission | None/each/all builds: generated imports, `HasNudgeFactory`, manifest Modules, symbol/import scan, absence of `tool.nudge` producer and of any control | N00, N02, N03 |
| Durability/recovery | Schedule-before-injection, append-failure abort, resume sibling coverage, fresh per-leg window, approval and same-Run steering legs, cancel wakes waiters | N02, N03 |
| Event/UI parity | `tool.nudge` v1 schema unchanged; generic Inspect still renders it; no feature UI/control added; unselected build reads historical events passively | N02, N03 |
| Residue | FNV, `loopWindow`, handoff helpers, consumer state and unconditional registration absent with dispositions | N03 |
| Quarantine/architecture | `TestEinoImportsQuarantined`, `TestPluginWindowCannotImportInternal`, no EXP import in root runtime, one Service/Eino/Journal path | N00, N02, N03 |
| Generation | Second generator run is byte-identical; affected official Recipes built and inspected | N03 |

## Open items (recorded, not blocking planning)

1. Recipe selection policy above (default family selects the package) needs owner
   confirmation; it changes whether the shipped species keeps coaching by default.
2. **Loop hard stop ownership — owner decided 2026-10-19: package-owned.** The count-6
   stop and the ten-step window move together into `vivy/exp-nudge`; N00 adds the `Stop`
   decision to the pure contract, N01 returns it, N02 maps it once to the existing
   `run.failed{cause_category:loop_detected}` terminal, and N03 deletes the core
   `internal/runtime/loopguard.go` body. An unselected species then has no per-tuple loop
   detection and relies on the existing `MaxToolTurns`/Eino `MaxIterations` cap. This is an
   explicit owner-approved product consequence of making the loop experiment
   species-selected, and the acceptance evidence must show the selected build stopping at
   six identical calls and the unselected build running to its turn cap without a nudge
   package. No second core-owned window is added at any point; the swap from D04's interim
   core guard to N03's package guard is one change with no dual owner.
3. Cross-epic edits to `internal/runtime/service.go`, `queue.go` and
   `internal/app/app.go` overlap E-WARMING/E-STEERING and the core-continuation lane. One
   integrating owner per file; this package assumes the Lead serializes them.
4. `just ci`/Postgres/real-token checks cannot run in this planning environment; N00–N03
   record focused commands now and the final gate at the implementing commit.
5. Peer baseline re-anchor (`docs/research/2026-10-19-baseline-a-reanchor.md`) is consistent
   with the mapping above: it also records `nudge_window.go` as absent (redirect to
   `loopguard.go`), no `plugins/exp/` precedent, no nudge config/settings/UI, and D04 as the
   seam owner. This package follows that split.

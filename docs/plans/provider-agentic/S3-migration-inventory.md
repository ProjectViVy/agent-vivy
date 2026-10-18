# S3 — Migration Inventory, Residue-Deletion Set, and Risk Table

**Goal:** a decision-complete per-file inventory for the real migration, a
residue-deletion set with a checked consumer per item, and a risk table with
detection — so the migration is priced rather than discovered.

**Status:** `DONE 2026-10-17`. §1–§6 are the method as reviewed; the
completed inventory, re-cited at `main` `e7dcf85`, is §7.

**Depends on:** S2. **Blocks:** S4.

---

## 1. Measured scale

Counted in this repository on 2026-10-17, non-test unless stated. These numbers
define the migration's size and are the reason it is a program rather than a
patch.

| Measure | Count | Where |
|---|---|---|
| `schema.Message` — non-test, `internal/` | **145 occurrences / 24 files** | `internal/runtime` 139 / 22; `internal/provider` 6 / 2 |
| `schema.AgenticMessage` — non-test, `internal/` | **0 occurrences / 0 files** | nothing on the new generation exists yet |
| `ToolCallingChatModel` — non-test, `internal/` | **30 occurrences / 9 files** | the seam and its consumers |
| `WithTools(` — `internal/runtime` + `internal/provider` | **33 occurrences / 19 files** | the tool-binding change |
| `schema.Message` — tests, `internal/` | **312 occurrences / 42 files** | test-side rewrite |
| `schema.Message` — tests, repo-wide | **932 occurrences / 126 files** | upper bound of the test surface |

## 2. Inventory groups

Each group must be completed with per-site disposition (change / delete /
unchanged) and, for every change, the message-generation decision.

### 2.1 Model seam

- `internal/provider/ref.go:27-40` — `Ref`; `:33`
  `Model(ctx, spec) (model.ToolCallingChatModel, error)` becomes the agentic
  type. `ModelSpec` (`:17-21`) is unchanged: `ID`, `APIKey`, `BaseURL` are
  generation-independent.
- `internal/provider/ref.go:46-56` `KeyMissingError` — unchanged.

### 2.2 Bindings

- `internal/provider/openai.go:42` — the `openai-completions` binding.
- `internal/provider/claude.go:50` — the `anthropic-messages` binding.
- A new `openai-responses` binding, structurally identical to the other two.
- `internal/provider/adapters.go` — the table becomes bindings (see §3).

### 2.3 Resolving model, thinking options, and titler

- `internal/provider/resolving.go:57` `NewResolvingChatModel`, `:61`
  `type resolvingChatModel`, `:86` `WithTools` on the outer model, `:146`
  `type resolvingChatModelWithTools`, `:371` `WithTools` on the bound model,
  `:375` `bound`.
- The thinking-option mapping: `resolving.go:327-330` (`thinkingOptions`), called
  from both generations of the wrapper (`:75`, `:83`, `:360`, `:368`).
  Each protocol's reasoning control must be re-expressed on the agentic
  generation — `agenticopenai`'s `Reasoning` (`responses_model.go:96`) and
  `WithResponsesReasoning` (`option.go:105`) for Responses, and
  `agenticclaude`'s `Thinking` (`model.go:98`) for Anthropic. **Regression risk:
  the existing seven-level thinking policy must be re-asserted per protocol.**
- The model cache and its key (`resolving.go:61-70`) — confirm it still keys
  on endpoint identity and never stores a key (D-010).
- `internal/provider/titler.go:31,34` (`ChainTitler`, `NewChainTitler`),
  `:115` (`NewOverrideModel`), `:122-123` (`TitleCandidates`). **Decision D7
  belongs here**: titling is text-only, and the question is whether the titler
  stays on the older generation or moves with everything else. Answer it with
  evidence — the admissible answers are "move it" (one generation, more
  mechanical work) or "keep it on the old generation with a justified
  boundary" (two generations, which D1 and D10 constrain). Do not leave it
  implicit.

### 2.4 Domain firewall and runtime run path

- `internal/runtime/modeladapter.go:24` `WrapModel`, `:87` `WithTools` (returns
  the adapter unchanged), `:91` `fromEinoMessages` — today a **spike-era,
  text-only** bridge whose own comment says `ContentBlock` is not transported.
  It must be deleted or rewritten; leaving it as-is would be a silent
  text-only filter on the new generation. S2 does not import this bridge;
  only the product-path acceptance in §2.7 can catch that regression.
- The 22 non-test files carrying `schema.Message`: `contextadapter.go`,
  `model_stream_observer.go`, `context.go`, `compaction_middleware.go`,
  `mapper.go`, `prompt_middleware.go`, `service.go`, `nudge_middleware.go`,
  `engine.go`, and the remainder the stage enumerates.
- `internal/runtime/engine.go:416` (`adk.NewChatModelAgent`), `:427`
  (`adk.NewRunner`), `:297-298` (middleware slice) — become the typed
  constructors.
- The seven Vivy-owned middlewares listed in `S1-component-contract.md` §3 —
  each re-implemented against
  `adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage]`, with the Eino
  middlewares switching to their `NewTyped` entry points.

### 2.5 Streaming, Journal, compaction

- Streaming chunk types: `*schema.Message` → `*schema.ContentBlockChunk` with
  `StreamingMeta` indices. The run-path observer and the event projection are
  affected.
- Journal projection: **semantic**, not a rename. Model text and reasoning are
  projected into persisted events; the agentic content-block shape must be
  projected deliberately, and the projection is the code that decides whether
  reasoning is stored at all.
- Compaction and summarization (`compaction_middleware.go:136,258`,
  `mapper.go:16`): token accounting and the reduced message shape change
  together. This is the highest-risk semantic area in the migration.

### 2.6 RPC wire and UI

- `internal/rpc/control.go:5561-5574` (`catalogEndpointResult`), `:5571`
  (`Executable bool`), `:5573` (`State string`), `:5587`
  (`provider.Capabilities()`), `:5612` (`Executable: state ==
  modelhost.CapabilitySupported`), `:5613` (`State`), and the selection gate
  `profileSelectionError` at `:5521-5537`, whose `switch` at `:5529-5534`
  rejects any state outside `{COMPILED, UNCONFIGURED, READY}` with "provider
  Profile %q is %s and cannot execute". That gate is what makes a deferred
  adapter unselectable today, but it also rejects real `UNAVAILABLE` profiles.
  Remove only the obsolete deferral branch; retain actual availability gates.
- `ui/src/lib/api.ts:569` (`ProviderCapabilityState`), `:572`
  (`ProviderAdapterState`), `:578` (`state: ProviderCapabilityState`), `:601`
  (`executable: boolean`), `:603` (`state: ProviderAdapterState`).
- `ui/src/components/settings/provider-catalog.ts:26` (`EXECUTABLE_STATES`),
  `:117-118` (`declaredCapability`), `:125-136` (`projectProviderRow`), `:190`
  (`providerSelection` returns `undefined` when not executable).
- `ui/src/components/settings/ModelSettingsCard.tsx:78,89` (`capabilityBadge`),
  `:123-128` (badge render), `:103` (`disabled:pointer-events-none`), `:615`
  (`disabled={locked || !entry.executable}`), `:618` (badge passed through).
- Positive change to record, and **correct the older citation while doing so**:
  the UI reasoning surface is complete and does not live in
  `MessageBubble.tsx` — that file has no reasoning code at all. The chain is
  `ui/src/components/chat/ReasoningRow.tsx:23` (the disclosure component,
  `data-reasoning-row`), rendered from `ui/src/components/chat/ChatView.tsx:12,199`
  for a `kind: 'reasoning'` row, which `ui/src/lib/run-rows.ts:47,224` folds from
  a `model.reasoning_delta` event, which `ui/src/lib/store.ts:412` accumulates
  into `streamingReasoning`. In other words the frontend half already exists and
  is tested (`ReasoningRow.test.tsx`, `ChatView.test.tsx:65`).
  **The gap is on the backend**: today no adapter produces reasoning content, so
  the row only ever appears when something else emits that event. This is a
  user-visible improvement of the migration — and it depends on the Journal
  projection in §2.5 actually persisting reasoning, so the two must land
  together or the row still receives nothing.

### 2.7 Product acceptance, separate from the component spike

S3 must assign these checks to migration phases; S4's spike decision schedules
implementation, not release. Product acceptance requires the actual Vivy path,
not a re-run of the isolated S2 harness:

- Preserve existing conversation history, image/file context and tool calls,
  including approval/interrupt/resume. Exercise the affected domain projections,
  not only new Eino types. Preserve policy/Grant boundaries and secret isolation.
- Verify reasoning through provider -> observer/mapper -> durable Journal -> UI,
  including reopening a session. Define the handling of signatures and opaque/
  redacted blocks across tool results, history reconstruction and compaction;
  provider replay material is not automatically user-visible reasoning text.
- Verify exact usage counters, main/summary-route attribution and no duplicate
  charges/events; verify context budgets and compaction behavior under the new
  representation. Test cancellation, stream closure and parallel tool races.
- Inventory durable checkpoints explicitly. `VersionedCheckpointStore.Get`
  in `internal/runtime/checkpoint.go` rejects mismatched Eino versions, and
  `engine.go` wires it into the runner. The v0.9.20 -> v0.9.21 bump prevents
  direct old-checkpoint resume; a reverted old binary likewise rejects new
  checkpoints. Decide an explicit drain/finish or safe restart disposition for
  pending runs in both directions, with user-visible outcomes. Preserve
  fail-closed checks; add no custom checkpoint converter. Test upgrade and
  downgrade using synthetic persisted state, never production journals.
- Compare full product binaries and relevant stream-path latency, memory and
  allocation behavior against the recorded baseline with matching toolchain,
  platform, flags and workload. Record measured values versus estimates and
  known constraints. The owner accepts material cost/regression tradeoffs;
  merely measuring them is not acceptance. Do not invent numeric budgets or
  build a new benchmark framework just for the plan.
- Run required `just ci` and a real-path UI smoke on split Vite `:3015`,
  verifying selection, tool execution and reasoning/reopen behavior. Record
  any unverified provider compatibility separately; a local fake is not evidence
  that all 45 vendors work. Any required live-provider check needs separately
  authorized credentials/cost and is not part of this zero-network spike.

## 3. Residue-deletion set

"The requirement is no residual data." Each item below must be dispositioned
with its consumer checked. Deleting a redundant copy is a deliverable, reviewed
like code (`AGENTS.md`).

| Site | Item | Disposition |
|---|---|---|
| `internal/provider/adapters.go:21` | `AdapterOpenAIResponses` as a *deferred* row | becomes an executable binding |
| `internal/provider/adapters.go:29,31` | `ErrAdapterUnknown`, `ErrAdapterDeferred` | the deferred error has no subject once every row is constructible; confirm `ErrAdapterUnknown` still covers the same failure closed |
| `internal/provider/adapters.go:42-45` | `Adapter{Family, State}` | `State` is deleted; a row is a binding |
| `internal/provider/adapters.go:51-53` | `adapterTable` with a `CapabilityDeferredIndefinite` row at `:53` | three executable rows |
| `internal/provider/adapters.go:101-103` | `endpointCapabilities` empty entry for `openai-responses` at `:103` | becomes the real capability set |
| `internal/provider/catalog.go:189-200` | `checkAdapter` — "unknown families and deferred families both fail closed"; the deferred branch is `:197` | the deferred branch is deleted; fail-closed on unknown is retained |
| `internal/provider/catalog.go:202-211` | `newAdapterRef` switch, `default: ErrAdapterUnknown` at `:211` | gains the Responses case |
| `internal/modelhost/profile.go:18-19` | `CapabilitySupported`, `CapabilityDeferredIndefinite` | the deferred member is deleted from the provider path |
| `internal/modelhost/profile.go:28,33,112-140,122,154-164` | `Capabilities`, `ProfileState`, `Statuses` (deferred branch `:122`), `ResolveExecutable` (`:154`, `CapabilitySupported` check `:162`) | reduce to the states that still have subjects |
| `internal/provider/data/vendors.yaml:610-617,622` | the `openai-responses` endpoint declaration and its provenance note | the endpoint becomes an ordinary executable endpoint |
| `internal/provider/data/provider.schema.json:41-42` | adapter enum description ("never executable") | corrected |
| `internal/provider/doc.go:8-9` | the three adapters incl. a deferred one | corrected |
| `internal/provider/reconcile.go:17` | comment: a deferred adapter may declare no endpoint | deleted with the state |
| ~10 test files | assertions on deferral | rewritten to assert executability |
| `docs/TODO.md:115` (`DEEPSEEK-REASONING-CONTENT`) | reasoning dropped by the pinned ChatModel-generation adapter | **superseded for the migrated path**: once reasoning arrives as `schema.Reasoning` blocks, the gap it describes no longer exists on that path. The row must be closed or re-scoped explicitly — not left as a live open item describing behaviour that changed |

### Explicitly preserved (do not touch)

| Site | Why |
|---|---|
| Runtime `COMPILED` / `UNCONFIGURED` / `READY` / `UNAVAILABLE` states and selection gates | Construction/configuration failures still exist; a sealed binding is not runtime readiness |
| Endpoint/model feature declarations and `AdapterSupportsCapability` validation | A constructible protocol does not imply every model or endpoint supports thinking, tools or arbitrary options; preserve authoritative derived checks |
| `sdk/port/support.go:12-16` | The generic 5-state port support vocabulary (`RESERVED`/`SPECIFIED`/`CANDIDATE`/`SUPPORTED`/`DEFERRED-INDEFINITE`). It applies to public ports, not to provider adapters — a different axis. Deleting it would be scope contamination in the other direction |
| `sdk/port/providerprofile/providerprofile.go:34-41` | The declarative public `Profile`. It has no capability field today and **must not gain one**: the profile stays declarative, and capability is derived (D5) |
| `sdk/internal/testdata/default-generation.expected.json:168-172` | The adapter set stays three, so the generated expectation is unchanged. Editing it would be evidence drift |
| `internal/modules/defaults/providers.go#ProviderProfiles` and `TestDefaultProviderProfilesMatchExistingRuntimeFamilies` | Cited as Port evidence anchors by `sdk/internal/assembly/evidence.go:77-80` and `sdk/internal/conformance/reproduction_test.go:476`; renaming them invalidates conformance evidence |

## 4. Risk table

| # | Risk | Severity | Detection | Response |
|---|---|---|---|---|
| R1 | `AgenticModel` is upstream **Beta**; a patch release may break it | med | S2 + `just ci` after the bump | Exact pins and semantic regression gates; quarantine bounds packages, not the 24-file migration surface |
| R2 | `libs/acl/openai` arrives as a pseudo-version (`agenticopenai@v0.2.4/go.mod:9`) | low | S0 pin-legality step | Precedent exists (`go.mod:7,18,19,23`); record it, do not re-litigate |
| R3 | Tool binding changes from a method to a request-time option, 33 sites | med | compile + full test run | Mechanical, but no site may be missed; S2's A2 is the behavioural check |
| R4 | Streaming chunk type change | high | S2 A1–A3; observer tests | Enumerate every stream consumer before editing |
| R5 | Journal projection and compaction are **semantic** changes | high | Journal/compaction tests + browser smoke | Highest-risk area; treat as its own phase with its own acceptance |
| R6 | The seven-level thinking policy regresses on some protocol | high | Per-protocol outbound-body assertions (S2 rule 2) | The policy is a shipped product feature; re-assert per protocol |
| R7 | `DEEPSEEK-REASONING-CONTENT`'s gap may be *replaced* by a different loss | med | S2 A3 across all three protocols | Record the residual; do not close the TODO row until reasoning is verified end to end |
| R8 | 45 vendors with custom base URLs + DeepSeek's `thinking` extra field | med | S1 C1/C9; S2 rule 2 | `agenticopenai` appears to preserve extra request fields (`chat_model.go:114-133`, `:228`; `option.go:68`); confirm by run |
| R9 | A second OpenAI client in the binary | low–med | S0 Q3, measured | Decide with the number, not with the 2026-09-18 argument |
| R10 | `EINO-TOOLSNODE-ERR-RACE` (`docs/DEFER.MD:118`) — parallel tool goroutines share a function-scope `err` | med | A `-race` run of a parallel tool batch after the bump | Assert the state of the row after the version bump; do not assume it was fixed |
| R11 | Scale: 145/24 non-test + 312/42 test occurrences | — | Inventory completion | This is why the migration is phased rather than a patch |
| R12 | A partially-migrated intermediate state | high | Compile | See §5 |
| R13 | Persisted runs cannot resume across the Eino-version boundary, including rollback | high | Synthetic upgrade/downgrade checkpoint tests and pending-run disposition | Preserve fail-closed version checks; §2.7 and S4 §5.8 |

## 5. The two migration shapes

**Shape A — all three adapters and the seam at once.** One branch, one
generation at every point in time. Recommended.

**Shape B — adapters first, runtime later.** The intuitive "smaller steps"
option. It requires the seam to return a model usable by the *old* agent while
being constructed on the *new* generation — which requires a message bridge, and
`S1-component-contract.md` §4.4 records that no supported bridge exists. **S3
must state outright whether a two-generation interval is representable at all**
given `ref.Model`'s single return type, and if it is not, say so plainly instead
of listing Shape B as a fallback (D10). If Shape B is representable only with a
Vivy-owned bridge, it is not an option: that bridge is the custom substitute D9
forbids.

## 6. Exit evidence

- Every inventory group in §2 completed with per-site disposition.
- Every residue-deletion item in §3 with its consumer checked and its
  disposition confirmed, including the preserved list.
- The risk table with each row's detection plan confirmed as runnable.
- The Shape A / Shape B conclusion from §5, stated as a decision, with the
  bridge search output behind it.
- A phase decomposition proposal (P1…Pn) with each phase's gate, suitable for
  `S4-decision.md` to adopt or amend.

## 7. Result (2026-10-17)

All sites re-cited at upstream `main` `e7dcf85`. §2–§3 line numbers above were
taken on the issue-58 branch and drift by up to ~25 lines in `control.go`; the
tables below supersede them.

### 7.1 Measured scale (§1 recounted)

`rg -c` counts matching **lines**.

| Measure | `main` | Plan (§1) | Note |
|---|---|---|---|
| `schema\.Message\b` non-test, `internal/` | **119 / 23 files** (runtime 113/21, provider 6/2) | 145 / 24 | The plan counted `schema\.Message` without a word boundary; on either baseline that gives 145/24 and includes 28 multimodal part refs (`MessageInputPart` ×25, `MessagePartCommon` ×2, `MessageInputImage` ×1), which must move too |
| `schema.AgenticMessage` non-test | 0 | 0 | — |
| `ToolCallingChatModel` non-test | 30 / 9 | 30 / 9 | — |
| `WithTools(` runtime + provider | 33 / 19 (**8 / 5 non-test**) | 33 / 19 | Only 8 production sites change |
| `schema\.Message\b` tests, `internal/` | 306 / 42 | 312 / 42 | — |
| `schema\.Message\b` tests, repo-wide | **306 / 42** | 932 / 126 | Nothing outside `internal/` uses it; the plan's figure likely included ignored trees |
| Eino imports outside `internal/{runtime,provider}` | 0 | — | Quarantine holds; `storage`, `domain`, `rpc` carry no Eino type |

Non-test files with `schema.Message`: `internal/provider/{resolving,titler}.go`;
`internal/runtime/{always_skills,cachewarmer,child_activation,compaction_middleware,compaction_service,context,contextadapter,engine,mapper,model_stream_observer,modeladapter,nudge_middleware,overflow,prompt_middleware,prompt_state,queue,report_effects,request,scriptedmodel,service,work_plan_guidance}.go`.

### 7.2 Per-group disposition

| Group | Site (`main`) | Disposition |
|---|---|---|
| 2.1 Seam | `internal/provider/ref.go:33` `Model(...) (model.ToolCallingChatModel, error)` | **change** → `model.AgenticModel`; `ModelSpec` (`:17`) unchanged |
| 2.2 Bindings | `openai.go:55` (`einoopenai.NewChatModel`), `claude.go:66-79` (`einoclaude.Config`, `AutoCacheControl` at `:77`) | **change** → `agenticopenai.NewChatModel`, `agenticclaude.New` (`CacheControl`) |
| 2.2 | new `openai-responses` binding | **add** → `agenticopenai.NewResponsesModel` |
| 2.3 Resolving | `resolving.go:57,61,86,146,371` | **change**: the agentic generation has no `WithTools` method, so `resolvingChatModelWithTools` and both `WithTools` methods are **deleted**; tools arrive as request options |
| 2.3 Thinking | `resolving.go:304` (`einoclaude.WithThinking`), `:309` (`WithRequestTimeout`), `:313` (`thinking` extra field), `:316` (`WithReasoningEffort`), called from `thinkingOptions` `:330` | **change** per S1 §9.4: Claude budget → `agenticclaude.WithExtraFields`, timeout → `agenticclaude.WithRequestTimeout`; DeepSeek → `agenticopenai.WithExtraFields` (+ `reasoning_effort` as an extra field); Responses → `WithResponsesReasoning`. Re-assert the seven-level policy per protocol on the wire (R6) |
| 2.3 Titler (D7) | `titler.go:31,34,46,64-72,115,122-123` | **change — move it.** Candidates come from the same seam (`NewOverrideModel` `:115`) and the main model (`:122`); keeping them on the old generation needs a second seam (violates D1/D10). Work: build an agentic user message, read the text block |
| 2.4 Domain firewall | `internal/runtime/modeladapter.go:24` `WrapModel`, `:91` `fromEinoMessages` | **test-only today**: no production caller (`rg 'WrapModel\('` hits only `internal/app/worker_test.go`, `internal/rpc/*_test.go`). Rewrite as the agentic test seam or fold into `scriptedmodel.go`; its text-only filter must not survive |
| 2.4 Agent wiring | `engine.go:297` (handler slice), `:301-303` (toolsearch, `UseModelToolSearch: false`), `:416` `adk.NewChatModelAgent`, `:427` `adk.NewRunner` | **change** → typed constructors; Eino middlewares → `NewTyped` (S1 §3) |
| 2.4 Vivy middlewares (8) | `compaction_middleware.go:50-54`, `nudge_middleware.go:42-54`, `prompt_middleware.go:27-33,83`, `toolmount_middleware.go:18-24,53-60,102-109`, `work_plan_guidance.go:16-20`, **`always_skills.go:79-80`** | **change** → `TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]`; `prompt`/`nudge` override `WrapModel` (`BaseModel[*schema.Message]`) and are message-shape changes, not renames |
| 2.4 Child runs | `child_activation.go:294-311` builds `[]*schema.Message` | **change** |
| 2.5 Stream/Journal | `mapper.go:265-266,375-376,481-495` (`reasoningText` reads `ReasoningContent` + reasoning parts), `model_stream_observer.go:327` (`einoclaude.GetCacheCreationInputTokens`), `cachewarmer.go:166` (same) | **change**: reasoning from `ContentBlockTypeReasoning`, cache-write from `PromptTokenDetails.CacheWriteTokens`; Journal stores domain events only (no Eino types in `internal/storage`), so the **Journal schema does not change** — the projection does |
| 2.5 Compaction | `compaction_middleware.go:136,258`, `compaction_service.go`, `overflow.go` | **change** (highest semantic risk, R5) |
| 2.5 Multimodal input | 28 `MessageInputPart`/`MessagePartCommon`/`MessageInputImage` refs | **change** → `UserInputText/Image/File` blocks |
| 2.6 RPC | `internal/rpc/control.go:5539` (`catalogEndpointResult`), `:5549` `Executable`, `:5551` `State`, `:5565` `Capabilities()`, `:5590` `Executable: state == CapabilitySupported` | **delete** the adapter `Executable`/`State` wire fields with the state (consumer: UI below) |
| 2.6 RPC gate | `control.go:5499-5512` `profileSelectionError` | **unchanged.** Correction: the switch has no deferral branch; the deferred state is produced by `modelhost/profile.go:122` and lands in `default` with `UNAVAILABLE`. Deleting `:122` is enough; the gate keeps rejecting real `UNAVAILABLE` |
| 2.6 UI | `ui/src/lib/api.ts:569` (`'DEFERRED-INDEFINITE'` member), `:572` `ProviderAdapterState`, `:601` `executable`, `:603` `state`; `provider-catalog.ts:26,113,117,125-134`; `ModelSettingsCard.tsx:78,89,123-128,403,615,618` | **change**: drop the adapter-state type and `declaredCapability`; keep `EXECUTABLE_STATES` for runtime profile states and the `!entry.executable` lock only where it derives from runtime readiness |
| 2.6 UI reasoning | `ReasoningRow.tsx`, `run-rows.ts`, `store.ts` | **unchanged.** Correction (S2 §9.3.6): reasoning already reaches this row for DeepSeek and Claude on the current stack; the migration *preserves* it and adds Responses, it does not newly enable it |

### 7.3 Residue-deletion set — consumers checked (`rg -l` over `*.go`)

| Site (`main`) | Item | Non-test consumers | Disposition |
|---|---|---|---|
| `provider/adapters.go:21` | `AdapterOpenAIResponses` | `adapters.go`, `modules/defaults/catalog.go:126` (Port evidence record) | **kept as a name**, row becomes executable; `catalog.go:126` unchanged |
| `adapters.go:31` | `ErrAdapterDeferred` | `catalog.go:197` | **delete** with the branch |
| `adapters.go:29` | `ErrAdapterUnknown` | `catalog.go` (`checkAdapter`, `newAdapterRef` default) | **keep** — still the fail-closed unknown-family error |
| `adapters.go:42-44` | `Adapter.State` | `adapters.go`, `profile.go:27` uses `adapterTable` length only | **delete** field |
| `adapters.go:53` | deferred row | — | three executable rows |
| `adapters.go:101-103` | `endpointCapabilities` empty `openai-responses` entry | `AdapterSupportsCapability` (`vendor.go`) | **fill** with the Responses capability set |
| `catalog.go:191-197` | `checkAdapter` deferred branch | — | **delete** branch; keep unknown |
| `catalog.go:204` | `newAdapterRef` | — | **add** Responses case; keep `default: ErrAdapterUnknown` |
| `modelhost/profile.go:19` | `CapabilityDeferredIndefinite` | `modelhost/profile.go:65,122`, `provider/adapters.go` | **delete** from the provider path |
| `modelhost/profile.go:15,18,28,65,112,154,168` | `CapabilityState`, `CapabilitySupported`, `Capabilities`, validation, `Statuses`, `ResolveExecutable`, `Capability` | `rpc/control.go:5565,5590`, `app/model.go`, `provider/{catalog,resolving}.go` | With one member left, `CapabilityState` has no subject: **delete** `CapabilityState`/`Capabilities`/`Capability()` and the `SUPPORTED` check; **keep** `Statuses`/`ResolveExecutable` (they carry `COMPILED`/`UNCONFIGURED`/`READY`/`UNAVAILABLE`) |
| `provider/data/vendors.yaml:610,622` | Responses endpoint + note | embedded data | **keep** endpoint (now executable); rewrite note |
| `provider/data/provider.schema.json:41-42` | enum description | schema | **correct** description |
| `provider/doc.go:8-9` | doc | — | **correct** |
| `provider/reconcile.go:16-18` | comment on deferred adapters | — | **delete** comment |
| 12 test files (`*_test.go` matching the deferral symbols) + 4 UI tests (`custom-providers`, `ModelSettingsCard`, `provider-catalog`, `saved-models`) | deferral assertions | — | **rewrite** to assert executability |
| `docs/DEFER.MD:104` `DEEPSEEK-REASONING-CONTENT` | — | — | **re-verify on the product path, then close as stale**; S2 §9.3.6 shows the component already surfaces the field. Do not tie its closure to this migration |

Preserved list (§3) confirmed untouched by every disposition above:
`sdk/port/support.go`, `sdk/port/providerprofile/providerprofile.go` (no field
added), `sdk/internal/testdata/default-generation.expected.json` (adapter set
stays three), `modules/defaults/providers.go#ProviderProfiles`, the runtime
profile states and `AdapterSupportsCapability`.

### 7.4 Risk table — additions and confirmations

R1–R13 stand; each detection is runnable (`just ci`, the S2 harness pattern,
`go test -race`, synthetic checkpoint tests). Updates:

| # | Update |
|---|---|
| R2 | Confirmed legal (S0 §7.7) |
| R7 | **Re-scoped**: the reasoning gap it guards is already closed on the current stack (S2 §9.3.6); the residual risk is losing it in the new projection |
| R9 | Measured: +5.19 MB component probe; product delta unknown until migrated (S0 §7.5) |
| R10 | Closed by evidence on v0.9.20+/v0.9.21 (S2 §9.2); still re-run `-race` on the product path |
| R11 | Rescaled: 119/23 exact + 28 multimodal refs non-test; 306/42 tests |
| **R14** | Completions drops `WithDeferredTools` silently. Detection: keep `UseModelToolSearch: false` for that family; add a guard test if native search is ever enabled. Medium |
| **R15** | Streaming Responses drops `encrypted_content`; replay depends on `store` (S4 B3). Detection: outbound-body assertion in the product test. Medium |
| **R16** | Claude `TotalTokens` max-merge (S4 B5). Detection: usage-projection test with output-only `message_delta`. Low |
| **R17** | Claude cache semantics change (`AutoCacheControl` breakpoints → top-level `cache_control`). Detection: cache-read counters on a two-turn product run with an authorized key. Medium |
| **R18** | Truncated stream reported as success — **pre-existing on every adapter**, not introduced by the migration (S2 §9.3.1). Tracked on the board, outside this program |
| **R19** | Shipped `components/model/claude v0.1.25/v0.1.26` stream race (S2 §9.3.5) — removed by the migration, present until then. Tracked on the board |
| **R20** | If P1 lands first, P2 changes the gob-encoded checkpoint payload (`TypedAgentEvent[*schema.AgenticMessage]`) **without** an Eino version change, so `VersionedCheckpointStore` (`checkpoint.go:107`, keyed on `EinoEngineVersion`) would not reject old checkpoints. High. Response: P2 must make the envelope fail closed on the generation change too (the same drain/restart policy as P1), and prove it with a synthetic pre-P2 checkpoint |
| **R21** | Chat-completions user `content` becomes an array of parts (S2 §9.3.7). Any OpenAI-compatible vendor that accepts only string content breaks. High for the 45-vendor catalog. Detection: authorized live check per compatible vendor family before P2 lands (S4 B7) |

### 7.5 Shape decision (§5, D10)

**Shape A, and only Shape A.** A two-generation interval is **not
representable** without a bridge: `Ref.Model` has one return type
(`ref.go:33`), and the same seam feeds the main agent (`engine.go:416`), the
compaction/summary models (`compaction_middleware.go:136`, typed
`BaseModel[*schema.Message]` for both) and the titler (`titler.go:115,122`).
The bridge search (S1 §9.2) finds no supported conversion. Shape B is not an
option.

### 7.6 Phase decomposition (proposal for S4)

| Phase | Scope | Gate |
|---|---|---|
| **P0** · *DONE 2026-10-18 except B3–B7 decisions* | Owner decisions B1–B3 and the B7 live-check approval (S4 §8.3); written pending-run policy for the Eino version boundary | Owner signature on S4; policy doc under `docs/` |
| **P1** · *DONE 2026-10-18* | Eino core `v0.9.13 → v0.9.21` **on the current generation**, alone (lifts `EINO-CORE-UPGRADE`; also fixes `EINO-TOOLSNODE-ERR-RACE`). S0 §7.6 shows it compiles and keeps provider/runtime tests green | `just ci`; synthetic upgrade/downgrade checkpoint tests (fail-closed preserved, drain/restart outcome as P0 decided); `-race` parallel tool batch |
| **P2** | Shape A on one branch: seam, three bindings, resolving/thinking projection, titler, 8 Vivy middlewares + typed Eino middlewares, mapper/observer/cachewarmer, compaction, child runs, multimodal parts, test rewrite | `just ci`; per-protocol outbound-body tests (thinking policy, tools, cache); usage projection test (R16); `-race`; checkpoint boundary fails closed on the generation change (R20) |
| **P3** | Residue deletion (§7.3) + RPC/UI (§7.2 2.6) | `just ci`; UI smoke on split Vite `:3015`: Responses row selectable, no deferral badge, `UNAVAILABLE` still locks |
| **P4** | Product acceptance (§2.7): full-binary and stream-path cost vs baseline, reasoning through Journal and reopen on all three, cache-read check on Claude, optional authorized live-provider check | Owner accepts measured cost; `docs/logs/` record |

P2 and P3 may share one PR (P3 is fail-closed if it lags), but P1 should land
on its own: it carries the checkpoint-boundary risk in isolation and has value
even if P2 never starts.

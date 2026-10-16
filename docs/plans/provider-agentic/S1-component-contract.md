# S1 — Component Contract (the Eino capability check for the new generation)

**Goal:** produce a per-component contract with a cited API for every capability
the migration needs, and an explicit list of what the pinned surface **cannot**
do — before any code is written. This is the mandatory Eino capability check
required by root `AGENTS.md` for any plan that touches model orchestration,
streaming, context management, or tool binding.

**Status:** `UNSCHEDULED`. This document states the method, the questions, and
the raw material already verified; the verdicts in §5 are what the stage
produces.

**Depends on:** S0. **Blocks:** S2.

All evidence is read from the **module cache at the target versions**
(`eino@v0.9.21`, `agenticopenai@v0.2.4`, `agenticclaude@v0.1.7`), not from online
documentation. Every claim cites a path and a line.

---

## 1. The rule this stage enforces

> If the pinned surface satisfies a requirement, adapt it. If it does not, the
> capability is `DEFERRED-INDEFINITE` and **no custom substitute is built**
> (`AGENTS.md`; `README.md` decision D9).

A custom protocol converter would be exactly such a substitute. This stage
exists to establish whether one is needed — the expected answer is that it is
not, and the stage's job is to prove or refute that per capability rather than
assert it.

## 2. The generational structure of Eino's agent layer

This is the structural fact that makes the migration possible, and it must be
stated precisely because the 2026-09-18 record got the surrounding question
right but never asked this one.

The untyped ADK names are **aliases** of the typed ones, specialised to
`*schema.Message`:

```go
// eino@v0.9.21
adk/chatmodel.go:417   type ChatModelAgentConfig = TypedChatModelAgentConfig[*schema.Message]
adk/handler.go:259     type ChatModelAgentMiddleware = TypedChatModelAgentMiddleware[*schema.Message]
adk/interface.go:467   type Agent = TypedAgent[*schema.Message]
adk/runner.go:76       type RunnerConfig = TypedRunnerConfig[*schema.Message]
adk/interface.go:43    type MessageType interface { ... }
```

Consequences, in order of importance:

1. `adk/chatmodel.go:280` is `Model model.BaseModel[M]` inside
   `TypedChatModelAgentConfig[M]` (`:260`), so
   `NewTypedChatModelAgent[*schema.AgenticMessage]` (`:508`) accepts an
   `AgenticModel` with **no conversion**. The historical objection — "no
   `Message`↔`AgenticMessage` bridge exists, so a bridge would have to be
   written" — was correct about the model components and inapplicable here.
2. `NewTypedRunner[M MessageType]` (`adk/runner.go:94`) does the same for the
   runner.
3. The *same* aliasing means the migration is not "the old agent plus a new
   model": every Vivy-owned middleware must be re-implemented against
   `TypedChatModelAgentMiddleware[*schema.AgenticMessage]`, because the untyped
   interface is a different instantiation, not a superset.

## 3. What Vivy actually uses (measured)

**Eino middlewares in use** — `agentsmd`, `skill`, `summarization`, `reduction`,
`plantask`, `filesystem`, `dynamictool/toolsearch`. Sites:
`internal/runtime/agentsmd.go:9`, `always_skills.go:11`, `skilladapter.go:6`,
`skills_backend.go:17`, `compaction_middleware.go:11-12`, `mapper.go:16`,
`todo_backend.go:13-14`, `engine.go:12-13`, `bundle_rules.go:10`,
`project_agentsmd.go:12`.

**Every one of those seven has a `NewTyped` variant** in `eino@v0.9.21`:

| Middleware | `NewTyped` |
|---|---|
| `agentsmd` | `agentsmd.go:72` |
| `filesystem` | `filesystem.go:353` |
| `plantask` | `plantask.go:39` |
| `skill` | `skill.go:199` |
| `summarization` | `summarization.go:259` (plus `finalizer_builder.go:83` `NewTypedFinalizer`) |
| `reduction` | `reduction.go:314` |
| `dynamictool/toolsearch` | `toolsearch.go:55` |
| `patchtoolcalls` | `patchtoolcalls.go:91` (not currently used by Vivy; listed for completeness) |

**Conclusion available for confirmation in S1:** there is **no foreign
blocker**. The middleware migration is mechanical (`NewChatModelAgentMiddleware`
→ `NewTyped[*schema.AgenticMessage]`), and every Eino-provided middleware Vivy
depends on has a supported typed entry point.

**Vivy-owned middlewares that must be re-implemented** — seven, all returning
`adk.ChatModelAgentMiddleware` today:

| Site | Symbol |
|---|---|
| `internal/runtime/compaction_middleware.go:53` | `newContextMonitor` |
| `internal/runtime/nudge_middleware.go:42` | `newNudgeMiddleware` |
| `internal/runtime/prompt_middleware.go:31` | `newPromptMiddleware` |
| `internal/runtime/toolmount_middleware.go:22` | `newSuppressedToolVisibilityMiddleware` |
| `internal/runtime/toolmount_middleware.go:58` | `newActivatedToolVisibilityMiddleware` |
| `internal/runtime/toolmount_middleware.go:107` | `newMountedToolVisibilityMiddleware` |
| `internal/runtime/work_plan_guidance.go:19` | `newWorkPlanGuidanceMiddleware` |

Plus the agent wiring itself: `internal/runtime/engine.go:416`
(`adk.NewChatModelAgent`) and `:427` (`adk.NewRunner`), and the handler slices
typed as `[]adk.ChatModelAgentMiddleware` (`engine.go:297-298`,
`compaction_middleware.go:136,258`).

The re-implementation is mechanical in shape, not merely in theory: the no-op
base these handlers embed is also an alias, so it has a typed counterpart.

```go
// eino@v0.9.21
adk/handler.go:261   type TypedBaseChatModelAgentMiddleware[M MessageType] struct{}
adk/handler.go:277   type BaseChatModelAgentMiddleware = TypedBaseChatModelAgentMiddleware[*schema.Message]
```

Vivy already does exactly this in one place —
`compaction_middleware.go:50` embeds `*adk.BaseChatModelAgentMiddleware` — so
that embedding becomes `*adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]`
and only the methods that touch messages change. **S1 confirms this for all
seven** rather than assuming it; a handler that embeds the base but overrides a
message-carrying method is not a rename, and `WrapModel`
(`adk/handler.go:295`) changes the model type it receives.

## 4. Contract rows the stage must fill

One row per component. Fill every cell with a path + line or with **unknown**,
and state for each row whether it blocks the migration.

### 4.1 `agenticopenai.NewChatModel` (the `openai-completions` binding)

| Property | Evidence so far |
|---|---|
| Constructor | `chat_model.go:141 func NewChatModel(ctx context.Context, config *ChatConfig) (*ChatModel, error)` |
| Config type | `chat_model.go:34 type ChatConfig struct` |
| Base URL | `chat_model.go:63 BaseURL`; `:146-148` falls back to `consts.go:23 defaultBaseURL` |
| Interface | `chat_model.go:30 var _ model.AgenticModel = (*ChatModel)(nil)` |
| Tools | `model.GetCommonOptions` — verify the call site in this component; `WithTools` (`eino/components/model/option.go:116`) and `WithDeferredTools` (`:143`) |
| **Extra request fields** | `chat_model.go:114-133 ExtraFields map[string]any`; applied at `:228` via `openai.WithExtraFields`; also settable per request via `option.go:68 func WithExtraFields(fields map[string]any) model.Option` |

The `ExtraFields` row is the one that decides the open vendor-compatibility
question in `DESIGN-CONCEPT.md` §6.7: the DeepSeek path depends on an
OpenAI-compatible request carrying a `thinking` extra field, and this component
appears to preserve that expressiveness on the agentic generation. **S1 confirms
by run, not by reading alone** — a field that exists but is not plumbed into the
serialised request body is not a capability.

### 4.2 `agenticopenai.NewResponsesModel` (the `openai-responses` binding)

| Property | Evidence so far |
|---|---|
| Constructor | `responses_model.go:165 func NewResponsesModel(_ context.Context, config *ResponsesConfig) (*ResponsesModel, error)` |
| Config type | `responses_model.go:40 type ResponsesConfig struct` |
| Base URL | `responses_model.go:47 BaseURL`; `:187-191` fallback + `option.WithBaseURL` |
| Interface | `responses_model.go:38 var _ model.AgenticModel = (*ResponsesModel)(nil)` |
| Tools | `responses_model.go:491 model.GetCommonOptions(&model.Options{...})`; `:383 toFunctionTools`; `:818-820` `DeferredTools` |
| Reasoning — config | `responses_model.go:96 Reasoning *responses.ReasoningParam` |
| Reasoning — per request | `option.go:105 func WithResponsesReasoning(reasoning *responses.ReasoningParam) model.Option` |
| Reasoning — inbound | `responses_convertor.go:2128 reasoningToContentBlocks`; `responses_event_convertor.go:1162 makeReasoningIndexKey`, with `Index:` sites at `:418,597,940,962` and `StreamingMeta` sites at `:820,851` |
| Extra fields | `responses_model.go:155 ExtraFields`, `:222`; `option.go:68 WithExtraFields` |

### 4.3 `agenticclaude.New` (the `anthropic-messages` binding)

| Property | Evidence so far |
|---|---|
| Constructor | `model.go:216 func New(ctx context.Context, cfg *Config) (*Model, error)` |
| Config type | `model.go:45 type Config struct` |
| Base URL | `model.go:66 BaseURL`; `:288-301` — `ANTHROPIC_BASE_URL` is honoured and `cfg.BaseURL` overrides it |
| Interface | `model.go:43 var _ model.AgenticModel = (*Model)(nil)` |
| Required field | `model.go:84 MaxTokens int`, validated at `:210`; per request via common options (`:452-453`) |
| Tools | `model.go:550 model.GetCommonOptions`; `convertor.go:523 toFunctionTools`; `:575 toDeferredFunctionTools`; `DeferredTools` at `model.go:581-582` |
| Thinking — config | `model.go:98 Thinking *anthropic.ThinkingConfigParamUnion`, stored `:184`, applied `:465` |
| Thinking — outbound conversion | `convertor.go:286 anthropic.NewThinkingBlock(reasoning.Signature, reasoning.Text)` — signature round-trip |
| Thinking — inbound | `convertor.go:624,637` (`ThinkingBlock`, `RedactedThinkingBlock`); `event_convertor.go:86-87` (`toStreamingThinkingStartBlock`), `:125-126` (`ThinkingDelta` → `schema.NewContentBlockChunk(&schema.Reasoning{Text: item.Thinking}, meta)`), `:274-277` |
| Alternate transports | `model.go:57 ByBedrock *BedrockConfig`, handled `:261-273` (optional; the trigger is `cfg.ByBedrock != nil`) |

Note that `agenticclaude` requires a `MaxTokens>0`; this repository already
carries a Claude token default (`internal/provider/claude.go:18`).

### 4.4 The generational gap (must be re-verified, then stated)

```go
// Search: ^func (Conv|To|From|Convert)[A-Za-z]*Agentic[A-Za-z]*\( over eino@v0.9.21
components/model/agentic_callback_extra.go:61,79     ConvAgenticCallbackInput / Output
components/prompt/agentic_callback_extra.go:45,59    ConvAgenticCallbackInput / Output
components/prompt/agentic_chat_template.go:34        FromAgenticMessages
```

All four are callback or prompt-template conversions. **None converts
`*schema.Message` to `*schema.AgenticMessage`.** S1 re-runs this search and
records the result verbatim, because it is the single claim that turns
"migration" into "all-or-nothing migration": there is no supported way to mix
generations inside one agent, so a partial migration cannot be expressed as a
runtime shortcut.

## 5. Verdicts S1 must return

One verdict per component, from a closed vocabulary, each with its blocking
status:

| Verdict | Meaning |
|---|---|
| `ADOPT` | The component satisfies every required capability with a cited API |
| `ADOPT-WITH-ADAPTER` | Satisfies them, but requires a thin Vivy seam at the existing boundary (e.g. a config projection), stated explicitly |
| `DEFERRED` | A required capability is absent. This is a legitimate S4 input, not a stage failure |

A `DEFERRED` verdict must name the missing capability, the search that proved it
absent, and the requirement it blocks. It must **not** be followed by a proposed
Vivy-owned implementation (decision D9).

## 6. Capability checklist S1 must answer explicitly

| # | Capability | Why it is required |
|---|---|---|
| C1 | Construct a model against an arbitrary base URL | 45 vendors and their endpoint variants; DeepSeek's OpenAI-compatible gateway |
| C2 | Bind tools at request time via `model.WithTools` | The ADK's binding path on the agentic generation; 33 `WithTools(` sites in runtime + provider |
| C3 | Bind deferred tools (`WithDeferredTools`) | `dynamictool/toolsearch` and tool mounting; Vivy has three tool-visibility middlewares |
| C4 | Stream text deltas as content blocks | The run path's incremental output and the Journal projection |
| C5 | Stream tool-call deltas with stable indices | Tool-call assembly on the new chunk type |
| C6 | Surface reasoning as `schema.Reasoning` blocks, inbound, with signature | The "reasoning is visible" half of this program. The frontend half already exists — `ui/src/components/chat/ReasoningRow.tsx:23` renders a `kind: 'reasoning'` row folded from a `model.reasoning_delta` event (`ui/src/lib/run-rows.ts:224`, `ui/src/lib/store.ts:412`) — but no adapter produces that content today, so the row never appears |
| C7 | Send reasoning control outbound | The existing seven-level thinking policy must not regress |
| C8 | Report token usage | Journal accounting and the context meter |
| C9 | Pass extra request fields | The DeepSeek `thinking` object and any vendor dialect that is not a first-class option |
| C10 | Express a Claude-style thinking budget | The `anthropic-messages` binding must not lose its current capability |
| C11 | Run inside the ADK typed agent and runner | The whole migration's premise |

## 7. Exit evidence

- The four contract tables in §4, filled in, every cell cited or marked
  `unknown`.
- One verdict per component from §5, with blocking status.
- The §6 checklist answered C1–C11, each with the cited API or an explicit
  absence.
- The re-run output of the §4.4 search, verbatim.
- The complete list of Eino middlewares Vivy uses with their `NewTyped` entry
  points, and the Vivy-owned middleware count with the interface they must move
  to.
- A single paragraph stating whether a Vivy-owned protocol converter or message
  bridge is required. The expected answer is **no**; if the answer is **yes**,
  the migration stops here and the finding goes to S4 as `NO-GO`.

## 8. Explicit non-goals

- Do not modify this repository's `go.mod`/`go.sum` (D8).
- Do not write production code, and do not construct a model against a live
  endpoint. S1 establishes the contract; S2 exercises it locally.
- Do not propose a Vivy-owned substitute for an absent capability.

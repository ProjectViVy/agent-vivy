# Eino Capability Check — Three Provider Adapters

**Status:** authored 2026-09-18. Evidence-only document; it changes no code.

This is the mandatory Eino capability check for the provider registry redesign
(root `AGENTS.md`: *"For Provider/model/OAuth/orchestration/RAG/MCP adapter
capability, inspect and cite the pinned Eino/EinoExt API. Adapt it when present;
otherwise mark the capability `DEFERRED-INDEFINITE`. Do not build a custom
substitute."*).

All evidence below was read from the **module cache at the pinned versions**, not
from online documentation.

## 1. Pins under test

From `go.mod` at `ff8a47d`:

```
github.com/cloudwego/eino                          v0.9.13
github.com/cloudwego/eino-ext/components/model/claude   v0.1.25
github.com/cloudwego/eino-ext/components/model/openai   v0.1.13
github.com/cloudwego/eino-ext/components/tool/mcp       v0.0.9
github.com/cloudwego/eino-ext/libs/acl/openai           v0.1.17  // indirect
github.com/eino-contrib/jsonschema                       v1.0.3  // indirect
```

## 2. `openai-completions` — SUPPORTED

Component: `github.com/cloudwego/eino-ext/components/model/openai v0.1.13`.

Directory contents of the pinned module:

```
chatmodel.go  chatmodel_test.go  go.mod  go.sum  option.go  README.md  README_zh.md  types.go
```

`chatmodel.go` exposes `openai.NewChatModel` over the Chat Completions API.
Grepping the whole package for `esponses` yields only two incidental hits, both
in prose comments about "API responses" (`chatmodel.go:89`, `chatmodel.go:187`);
there is no Responses API code path.

This is the adapter Vivy already uses
(`internal/provider/openai.go:83-87`).

**Verdict: ADAPT (already adapted).**

## 3. `anthropic-messages` — SUPPORTED

Component: `github.com/cloudwego/eino-ext/components/model/claude v0.1.25`.

Exposes `claude.NewChatModel` over the Anthropic Messages API, plus
`claude.WithThinking` / `claude.Thinking` and `claude.CacheControl`, both already
consumed by `internal/provider/claude.go` (`claudeDefaultMaxTokens` at
`claude.go:18`, `AutoCacheControl` at `claude.go:63-65`, thinking at
`resolving.go:144`).

**Verdict: ADAPT (already adapted).**

## 4. `openai-responses` — DEFERRED-INDEFINITE

### 4.1 The pinned component is at its newest version and has no Responses API

```
$ go list -m -versions github.com/cloudwego/eino-ext/components/model/openai
github.com/cloudwego/eino-ext/components/model/openai v0.1.1 v0.1.2 ... v0.1.13
```

`v0.1.13` is the newest tagged release. **Upgrading the pin does not help.**

A separate component path was probed and does not exist:

```
$ go list -m github.com/cloudwego/eino-ext/components/model/responses@latest
go: module github.com/cloudwego/eino-ext/components/model/responses: no matching versions for query "latest"
```

### 4.2 The capability exists, in a different component on a different interface generation

`github.com/cloudwego/eino-ext/components/model/agenticopenai v0.2.2` ships a
full Responses API implementation:

```
chat_model.go                          consts.go
responses_model.go                     responses_convertor.go
responses_event_convertor.go           responses_extension.go
responses_content_block_extra.go       responses_message_extra.go
register.go                            option.go  utils.go
```

Key citations:

- `consts.go:21` — `responsesImplType = "AgenticOpenAI/Responses"`
- `consts.go:23` — `defaultBaseURL = "https://api.openai.com/v1"`
- `responses_model.go:35` — `import "github.com/openai/openai-go/v3/responses"`
- `responses_model.go:165` — `func NewResponsesModel(_ context.Context, config *ResponsesConfig) (*ResponsesModel, error)`
- `chat_model.go:141` — `func NewChatModel(ctx context.Context, config *ChatConfig) (*ChatModel, error)`
- its `go.mod` requires `github.com/cloudwego/eino v0.9.5` — **older** than the
  repository pin v0.9.13, same minor version, so the core version is not the
  obstacle

**The obstacle is the interface generation:**

```go
// agenticopenai
chat_model.go:30        var _ model.AgenticModel = (*ChatModel)(nil)
responses_model.go:38   var _ model.AgenticModel = (*ResponsesModel)(nil)
// both operate on *schema.AgenticMessage
```

```go
// Vivy's whole model seam
internal/provider/ref.go:33   Model(ctx, spec) (model.ToolCallingChatModel, error)
internal/provider/resolving.go:56,64   Generate/Stream over []*schema.Message
```

### 4.3 Both generations exist in the pinned core

```go
// eino@v0.9.13 components/model/interface.go
:71   type BaseChatModel = BaseModel[*schema.Message]
:99   type ToolCallingChatModel interface { ... }
:109  type AgenticModel = BaseModel[*schema.AgenticMessage]

// eino@v0.9.13 schema/agentic_message.go
:71   type AgenticMessage struct { ... }
```

So the pinned core already carries the newer generation; the gap is confined to
which component speaks which interface.

### 4.4 Eino provides no bridge between the two message generations

Grepping `eino@v0.9.13/schema/*.go` and `eino@v0.9.13/components/model/*.go` for
`^func (Conv|To|From|Convert)[A-Za-z]*Agentic` returns **zero** matches. There is
`schema.ConcatAgenticMessages` / `ConcatAgenticMessagesArray`
(`agentic_message.go:896,901`), but nothing that converts `*schema.Message` to
`*schema.AgenticMessage`.

Consequence: a boundary bridge inside `internal/provider` would have to be
written from scratch, covering multimodal content blocks, tool calls, tool
results, streaming chunk types (`ContentBlockChunk`, `StreamingMeta`), token
usage, and reasoning. That is a custom mechanism, not a thin adapter, and
`AGENTS.md` requires a custom substitute **not** to be built for a missing
capability.

**Verdict: `DEFERRED-INDEFINITE`.** `openai-responses` is declared in the adapter
table and in the vendor data, projected to the UI as disabled, and backed by no
implementation placeholder.

## 5. Committed migration path (owner decision, 2026-09-18)

The owner's decision is: take route 3 (defer) **and explicitly commit to moving
onto `AgenticModel`**. That commitment is recorded here as the only lift
condition for the deferral.

### 5.1 Lift conditions (either one)

1. EinoExt publishes Responses API support on `model.ToolCallingChatModel`
   (`components/model/openai` or an equivalent new component), **or**
2. Vivy migrates its model seam to `model.AgenticModel` /
   `*schema.AgenticMessage`.

### 5.2 Blast radius of condition 2

Recorded so the migration can be estimated rather than rediscovered:

| Area | Effect |
|---|---|
| Model seam | `provider.Ref.Model` return type, `ModelSpec` plumbing |
| Resolving model | `resolvingChatModel`/`resolvingChatModelWithTools` message type, thinking options |
| Agent loop | `internal/runtime` engine, tool-call parsing/streaming over `schema.Message` |
| ADK | `ChatModelAgent` and any middleware bound to the older generation |
| Streaming | chunk types differ (`ContentBlockChunk` / `StreamingMeta`) |
| Journal projection | event payloads that carry model text/reasoning |
| Model metadata | unchanged (data-level, not message-level) |

`AgenticModel` is documented by CloudWeGo as **Beta**
([AgenticModel User Guide](https://www.cloudwego.io/docs/eino/core_modules/components/agentic_chat_model_guide/)).

### 5.3 Dependency cost

Adopting `agenticopenai` adds `github.com/openai/openai-go/v3` (the official
OpenAI Go SDK) alongside the current client stack
(`eino-ext/libs/acl/openai v0.1.17` + the `meguminnnnnnnnn/go-openai` fork). That
produces **two OpenAI clients in one binary**. This is the same class of reason
that previously rejected `eino-ext/components/model/deepseek` (see
`docs/TODO.md` `DEEPSEEK-REASONING-CONTENT`); it must be weighed when the
migration is scheduled.

## 6. Out of scope: the `reasoning_content` response gap

`docs/TODO.md` `DEEPSEEK-REASONING-CONTENT` records that the pinned
`eino-ext/components/model/openai v0.1.13` parses `reasoning_content` and then
drops it, so DeepSeek reasoning text never reaches `schema.Message`, the stream,
or the Journal. Outbound `thinking` / `reasoning_effort` control is unaffected.

**This document does not schedule or design a fix.** It records three boundaries:

1. The gap is **request-independent**: it survives any provider-registry change.
2. The fix belongs upstream or at the `internal/provider` seam; a Vivy-owned
   decoder is forbidden by `AGENTS.md`.
3. The UI is already prepared to render it — `ui/src/components/chat/MessageBubble.tsx:167`
   has a reasoning disclosure block that currently never receives data.

## 7. Summary table

| Adapter | Wire API | Pinned component | Interface generation | State |
|---|---|---|---|---|
| `openai-completions` | `POST {base}/chat/completions` | `components/model/openai v0.1.13` | `model.ToolCallingChatModel` | `SUPPORTED` |
| `openai-responses` | `POST {base}/responses` | `components/model/agenticopenai v0.2.2` | `model.AgenticModel` | `DEFERRED-INDEFINITE` |
| `anthropic-messages` | `POST {base}/messages` | `components/model/claude v0.1.25` | `model.ToolCallingChatModel` | `SUPPORTED` |

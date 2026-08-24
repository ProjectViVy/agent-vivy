# Eino ChatModel contracts & options (v0.9.13)

Source: `components/model/interface.go`, `components/model/option.go` in the local module cache.

## Interfaces

```go
type BaseModel[M messageType] interface { // messageType: *schema.Message | *schema.AgenticMessage
    Generate(ctx context.Context, input []M, opts ...Option) (M, error)
    Stream(ctx context.Context, input []M, opts ...Option) (*schema.StreamReader[M], error)
}

type BaseChatModel = BaseModel[*schema.Message] // the chat contract

// Deprecated: mutates in place, races under concurrency.
type ChatModel interface {
    BaseChatModel
    BindTools(tools []*schema.ToolInfo) error
}

// Preferred: returns a NEW instance with tools attached; safe to share the base.
type ToolCallingChatModel interface {
    BaseChatModel
    WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}
```

Key points:

- Input is a slice of `*schema.Message` = the whole conversation, oldest first, newest last.
- `Generate` blocks until a complete response; returns `*schema.Message` (may carry `ToolCalls`).
- `Stream` returns a `*schema.StreamReader[*schema.Message]` — **single-shot**, caller must `Close()`:

```go
reader, err := cm.Stream(ctx, msgs, opts...)
if err != nil { ... }
defer reader.Close()
for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) { break }
    if err != nil { ... }
    // handle chunk
}
```

- `AgenticModel = BaseModel[*schema.AgenticMessage]` exists for agentic runs; no `WithTools` — tools are passed via the `model.WithTools` option instead.

## Options (call-time, immutable, composable)

| Option | Type | Notes |
|---|---|---|
| `WithTemperature(v float32)` | randomness | |
| `WithModel(name string)` | model name override | |
| `WithTopP(v float32)` | diversity | |
| `WithMaxTokens(n int)` | cap; finish reason "length" when hit | |
| `WithStop(stop []string)` | stop words | |
| `WithTools(tools []*schema.ToolInfo)` | tools for this call | pass `[]` (not nil) to clear |
| `WithToolChoice(tc schema.ToolChoice, allowed ...string)` | constrain tool calls | chat models only |
| `WithAgenticToolChoice(tc *schema.AgenticToolChoice)` | agentic models only | |
| `WithDeferredTools(...)` / `WithToolSearchTool(...)` | server-side tool search | rarely used in Vivy |

Implementor helpers (when writing a custom ChatModel):

- `model.GetCommonOptions(base, opts...)` — merge standard options.
- `model.GetImplSpecificOptions(base, opts...)` — merge impl-specific options.
- `model.WrapImplSpecificOptFn(func(*MyOpts))` — expose a custom option.

## eino-ext OpenAI provider (v0.1.13)

```go
import einoopenai "github.com/cloudwego/eino-ext/components/model/openai"

cm, err := einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
    APIKey:  key,      // required at construction time
    BaseURL: baseURL,  // OpenAI-compatible gateway
    Model:   modelID,
})
```

`*einoopenai.ChatModel` implements `model.ToolCallingChatModel` (`Generate`/`Stream`/`WithTools`).

Impl-specific options (`option.go`): `WithExtraFields`, `WithExtraHeader`, `WithReasoningEffort(level)`, `WithMaxCompletionTokens(n)`, `WithRequestPayloadModifier`, `WithResponseMessageModifier`, `WithResponseChunkMessageModifier`.

Vivy constructs this in `internal/provider/openai.go`; do not construct it elsewhere.

## Streaming concat helpers (schema package)

When merging stream chunks, `schema` registers concat functions used by `compose`:

- `ConcatMessages(chunks []*Message) (*Message, error)`
- `ConcatMessageArray(mas [][]*Message) ([]*Message, error)`
- `ConcatToolResults(chunks []*ToolResult) (*ToolResult, error)`

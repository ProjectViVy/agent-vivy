# Eino callbacks: observability (v0.9.13)

Source: `callbacks/interface.go`, `callbacks/handler_builder.go` in the local
module cache. Callbacks observe every component invocation — prompts sent,
outputs received, errors, token usage — without touching pipeline code.

## Handler interface & timings

```go
type Handler interface {
    OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
    OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context
    OnError(ctx context.Context, info *RunInfo, err error) context.Context
    OnStartWithStreamInput(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context
    OnEndWithStreamOutput(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context
    // Needed(ctx, info, timing) bool — declare which timings you handle; the
    // framework skips stream-copy/goroutine overhead for the rest.
}
```

- `RunInfo{Name, Type, Component}` identifies the caller: node name
  (`compose.WithNodeName`), implementation type (e.g. "OpenAI"), and component
  category (`components.ComponentOfChatModel`, etc.). Filter on it.
- Concrete input/output types are component-defined. Cast safely with the
  per-package `ConvCallbackInput` / `ConvCallbackOutput` helpers:

```go
mInput := model.ConvCallbackInput(in)
if mInput == nil { return ctx } // not a model invocation
log.Printf("prompt messages: %v", mInput.Messages)
```

- Do NOT mutate `CallbackInput`/`CallbackOutput` values: all nodes and handlers
  share the same pointer — mutation causes data races in concurrent graphs.
- Stream handler copies MUST be closed after reading, or the original stream
  never frees (goroutine/memory leak).
- There is NO guaranteed ordering between different handlers.

## Builder

```go
h := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        // ... return ctx (may carry per-run state via context.WithValue)
    }).
    OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
        // ...
    }).
    OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
        // ...
    }).
    Build()
```

## Registration

- **Global** (process-wide, runs first, for tracing/metrics): 
  `callbacks.AppendGlobalHandlers(h1, h2, ...)` — call once at init, NOT
  thread-safe with concurrent runs. (`InitCallbackHandlers` is deprecated.)
- **Per invocation**: `compose.WithCallbacks(handlers...)` as a `compose.Option`
  to `Invoke`/`Stream`/etc., or `adk.WithAgentRunCallbacks(...)` for ADK runs.

## What Vivy uses

- `internal/runtime/observability_test.go` and `internal/runtime/service.go`
  show callback wiring against `adk` runs.
- `internal/provider/mockref.go` / `internal/runtime/scriptedmodel.go` — model
  mocks; add callbacks there to capture prompt/usage in tests.

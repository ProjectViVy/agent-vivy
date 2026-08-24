---
name: vivy-eino
description: Complete Eino framework usage guide for Vivy development — components (ChatModel, Prompt, Retriever, Embedding, Document, Indexer, Tool), orchestration (Chain / Graph / Workflow / Parallel / Branch), Runnable modes (Invoke/Stream/Collect/Transform), callbacks/observability, and the ADK agent layer (ChatModelAgent, Runner, events, middlewares). Use whenever code touches github.com/cloudwego/eino* (inside internal/runtime or internal/provider), or when the user asks how to build/design an LLM pipeline in Vivy, how to chain models+tools+retrieval, how to stream, how to add agents, or references the Eino quick start (快速开始 / chapter_01 / ChatModel 与 Message). Triggers: eino / chain / graph / workflow / compose / adk / agent / retriever / rag / embedding / prompt template / tools node / callbacks / stream / Invoke / Stream / Collect / Transform / 编排 / 流程图 / 智能体 / 工具调用.
---

# Vivy × Eino — framework usage guide (v0.9.13)

Official docs: <https://www.cloudwego.io/zh/docs/eino/> — **do not fetch over the network**; the sandbox blocks outbound TLS (`SEC_E_NO_CREDENTIALS`). Canonical reference is the local module source, which matches the repo pin in `go.mod`:

```
%USERPROFILE%\go\pkg\mod\github.com\cloudwego\eino@v0.9.13\
    components\model\        # ChatModel
    components\prompt\       # chat templates
    components\tool\         # tool interfaces + utils (InferTool)
    components\retriever\    # RAG retrieval
    components\embedding\    # embeddings
    components\document\     # loaders / transformers
    components\indexer\      # indexing
    compose\                 # Chain / Graph / Workflow / Parallel / Branch / Runnable
    flow\agent\              # ReAct agent options
    callbacks\               # observability handlers
    adk\                     # agent dev kit: ChatModelAgent, Runner, events
    schema\                  # Message, Document, ToolInfo, StreamReader
%USERPROFILE%\go\pkg\mod\github.com\cloudwego\eino-ext\components\model\openai@v0.1.13\
```

## Eino in one paragraph

Eino is Go's LLM-application framework. Four layers:

1. **`schema`** — shared data types: `schema.Message` (chat), `schema.Document` (RAG), `schema.ToolInfo` (tools), `schema.StreamReader` (streams).
2. **`components`** — reusable primitives behind small interfaces: `model.ChatModel` (Generate/Stream), `prompt.ChatTemplate` (Format), `tool` (Info/InvokableRun), `retriever.Retriever` (Retrieve), `embedding.Embedder` (EmbedStrings), `document.Loader/Transformer`, `indexer.Indexer` (Index).
3. **`compose`** — orchestration: compose components/lambdas into a `Chain` (linear), `Graph` (DAG with branches/parallel), or `Workflow` (dependency+field-mapping style), then `Compile()` to a `Runnable[I,O]` with four run modes: `Invoke` (one in → one out), `Stream` (one in → stream out), `Collect` (stream in → one out), `Transform` (stream in → stream out).
4. **`adk`** — agent layer: `adk.NewChatModelAgent` wraps a model+tools into a ReAct loop agent; `adk.NewRunner` executes it and emits `*adk.AsyncIterator[*adk.AgentEvent]`; middlewares (handlers) intercept the loop; `adk` also provides prebuilt agents (deep, planexecute, supervisor) and checkpoint/resume.

Vivy today: `internal/provider` builds models (`einoopenai.NewChatModel`), `internal/runtime` adapts domain types and runs an `adk` ChatModelAgent + Runner (engine.go), with tool middlewares and filesystem/skill/plantask backends.

## Choose your tool

| Need | Use |
|---|---|
| One model call, one reply | `model.Generate` / `model.Stream` directly (see `references/chatmodel.md`) |
| Fixed linear pipeline (prompt→model→parse) | `compose.NewChain` (`references/compose.md`) |
| Branches, parallel branches, joins, arbitrary DAG | `compose.NewGraph` + `AddEdge` / `AddBranch`; chain-style parallel via `AppendParallel` |
| Declarative dependencies + field mapping between nodes | `compose.NewWorkflow` (`WorkflowNode.AddInput/AddDependency`) |
| ReAct tool-calling agent loop, multi-agent, checkpoint/resume | `adk` (`references/adk.md`) |
| RAG: load → split → embed → index → retrieve → answer | components + compose (`references/rag.md`) |
| Observability / token accounting / tracing | `callbacks` handlers (`references/callbacks.md`) |

## Typical workflow

1. **Model**: construct via provider (in Vivy: `internal/provider`), get a `model.ToolCallingChatModel`.
2. **Messages**: build `[]*schema.Message` with `schema.UserMessage` / `SystemMessage` / `ToolMessage`; use `prompt.FromMessages(schema.FString, ...)` + `schema.MessagesPlaceholder` for templating (`references/message.md`).
3. **Tools**: define with `tool/utils.InferTool[T,D](name, desc, fn)` or manual `schema.ToolInfo`; group via `compose.NewToolNode(ctx, &compose.ToolsNodeConfig{Tools: ...})`.
4. **Orchestrate**: `NewChain` or `NewGraph`; `AppendChatTemplate(...).AppendChatModel(...).AppendToolsNode(...)`; or `AddChatModelNode` / `AddToolsNode` / `AddEdge`; compile with `r, err := g.Compile(ctx)`.
5. **Run**: `r.Invoke(ctx, in)` or `r.Stream(ctx, in)`; consume `*schema.StreamReader` with `Recv()` until `io.EOF`, `defer Close()`.
6. **Observe**: `callbacks.NewHandlerBuilder().OnStartFn(...).Build()`; register via `callbacks.AppendGlobalHandlers` or `compose.WithCallbacks`.
7. **Verify**: `just ci` (kernel path, runs the eino import quarantine).

## Vivy hard rules (read before coding)

1. **Import quarantine (D-007, `internal/app/importlint_test.go`)**: only `internal/runtime/` and `internal/provider/` may import `github.com/cloudwego/eino*`. Any other package fails `just ci`. App/domain layers talk through `internal/domain` types, never Eino types.
2. **Provider boundary**: model construction only in `internal/provider` (`einoopenai.NewChatModel` with `ChatModelConfig{APIKey, BaseURL, Model}`); API key read from `os.Getenv(r.bundle.EnvKey)` per request, never cached (D-010); `VIVY_API_BASE` overrides base URL.
3. **Domain firewall**: `internal/runtime/modeladapter.go` is the only bridge `domain.ChatModel` ↔ `model.ToolCallingChatModel`; `modelbroker.go` maps worker messages and never retains history.
4. **Immutable tools**: prefer `ToolCallingChatModel.WithTools(...)` (new instance) over deprecated mutating `ChatModel.BindTools` — races under concurrency.
5. **Close streams**: every `*schema.StreamReader` (model, tool, Runnable, callback copies) must be closed; readers are single-shot.
6. **No network fetch of docs**: use the local module cache above; treat it as the spec.

## Reference files

- `references/chatmodel.md` — model contracts, all options, StreamReader usage.
- `references/message.md` — `schema.Message`, roles, constructors, templates, ToolInfo.
- `references/compose.md` — Chain / Graph / Workflow / Parallel / Branch / Runnable modes / Lambda / ToolsNode, with code snippets.
- `references/adk.md` — ChatModelAgent, Runner, AgentEvent loop, middlewares, checkpoint/resume, prebuilt agents.
- `references/rag.md` — Retriever / Embedding / Document / Indexer and a RAG pipeline sketch.
- `references/callbacks.md` — handler builder, timings, global vs per-call registration.
- `references/vivy-integration.md` — how Vivy wires Eino today (provider, adapters, engine), plus verification.

## Forbidden

- Importing `github.com/cloudwego/eino*` outside `internal/runtime` / `internal/provider`.
- Fetching CloudWeGo docs over the network (sandbox TLS is blocked; use the module cache).
- `BindTools` on a shared instance; `schema.Message.MultiContent` (deprecated); unclosed/read-twice stream readers.
- Bypassing the provider/adapter seams to construct models or wrap domain types elsewhere.

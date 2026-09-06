# Eino ADK: agents, Runner, events, middlewares (v0.9.13)

Source: `adk/` in the local module cache. ADK = Agent Development Kit: run
a ChatModel-backed agent (ReAct loop: model → tool calls → model) and
orchestrate multi-agent setups with checkpointing/resume.

## Agent and Runner

```go
import "github.com/cloudwego/eino/adk"

// 1. Build the agent: model + tools → ReAct loop agent.
agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
    Model:        chatModel,   // model.BaseChatModel (Vivy: model.ToolCallingChatModel works)
    ToolsConfig:  adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
    Instruction:  "You are a helpful assistant.",     // system prompt
    MaxIterations: 10,                                 // ReAct loop cap
    Handlers:     []adk.ChatModelAgentMiddleware{...}, // intercept the loop
    // optional: ModelRetryConfig, ModelFailoverConfig, CheckPointStore
})

// 2. Run it.
runner := adk.NewRunner(ctx, adk.RunnerConfig{
    Agent:           agent,
    EnableStreaming: true,           // false: events carry final messages
    CheckPointStore: store,          // optional, enables Resume
})
it := runner.Query(ctx, "what is the weather in Beijing?")

for {
    evt, err := it.Recv()
    if errors.Is(err, io.EOF) { break }
    if err != nil { ... }
    // evt: *adk.AgentEvent
}
```

`adk.Runner` methods: `Run(ctx, []*schema.Message, ...AgentRunOption)`,
`Query(ctx, string, ...)` (convenience), `Resume(ctx, checkpointID, ...)`,
`ResumeWithParams(ctx, checkpointID, *adk.ResumeParams, ...)`.

## AgentEvent — what the loop emits

```go
type AgentEvent struct { // = TypedAgentEvent[*schema.Message]
    AgentName string
    RunPath   []RunStep
    Output    *TypedAgentOutput[*schema.Message] // final or streamed messages
    Action    *AgentAction                        // tool calls
    Err       error
}
```

- With streaming enabled, `evt.Output.MessageStream` is a
  `*schema.StreamReader[*schema.Message]` — consume it and call
  `SetAutomaticClose()` or `Close()`.
- `AgentAction` carries `Name` (agent name) and `ToolCalls` for tool-use events.
- `AsyncIterator` also exposes `Recv()` until `io.EOF`.

## AgentRunOption (run-time)

`adk.WithCheckPointID(id)`, `adk.WithAgentRunCallbacks(...)`,
`adk.WithMaxRunSteps(n)`, `adk.WithStatePreHandler/WithStatePostHandler`,
`adk.WithRunID(...)`, and compose options wrapped via `agent.WithComposeOptions`.

## Middlewares — intercept the ReAct loop

`adk.ChatModelAgentConfig.Handlers []adk.ChatModelAgentMiddleware`:

- `adk.BeforeAgent(fn)` — before the agent turn starts; modify
  `ChatModelAgentContext.Tools` or messages.
- `adk.BeforeModel(fn)` — before each model call (rewrite prompt/messages).
- `adk.AfterModel(fn)` — after the model returns (inspect/rewrite reply).
- `adk.BeforeModelRewriteState(fn)` — rewrite `state.ToolInfos` for dynamic
  tool filtering; recommended for tool selection.
- `adk.WrapModel(fn)` — wrap the model call (discouraged for tool changes:
  not persisted, breaks prompt cache).
- `adk.NewEventSenderToolWrapper(...)` — custom event sending.

Global agent middleware: `adk.AgentMiddleware` (Before/After around the whole
agent run) — Vivy's tool visibility seam is the mount-only
`internal/runtime/toolmount_middleware.go`.

## Checkpoint & resume

- `RunnerConfig.CheckPointStore` (interface in `adk`/`core`) persists run
  state; `adk.WithCheckPointID(id)` names the run.
- Interrupts come from tools (`tool.Interrupt`) or agent components; resume
  with `runner.Resume(ctx, id)` (implicit "resume all") or
  `runner.ResumeWithParams(ctx, id, &adk.ResumeParams{Targets: map[string]any{"addr": data}})`.
- Vivy: `internal/runtime/checkpointadapter.go` bridges Vivy checkpoints to
  ADK; `internal/runtime/engine.go` exposes `Query`/`RunHistory`/`Resume`.

## Prebuilt agents (adk/prebuilt)

- `adk/prebuilt/deep` — DeepAgent (hierarchical tool delegation).
- `adk/prebuilt/planexecute` — Plan & Execute agent.
- `adk/prebuilt/supervisor` — Supervisor/worker multi-agent.
- `adk.NewParallelAgent(ctx, &adk.ParallelAgentConfig{...})` — fan-out agents.

Sub-agents: `OnSubAgents` interface (`OnSetSubAgents`/`OnSetAsSubAgent`/`OnDisallowTransferToParent`) supports agent transfer, but the ADK docs and source comments recommend `NewAgentTool` or DeepAgent for most multi-agent scenarios.

## Middleware backends used by Vivy

- `github.com/cloudwego/eino/adk/middlewares/skill` — skills backend (`internal/runtime/skills_backend.go`).
- `github.com/cloudwego/eino/adk/middlewares/filesystem` + `plantask` — file and todo backends (`internal/runtime/todo_backend.go`).
- `github.com/cloudwego/eino/adk/filesystem` — filesystem backend for state.
- `github.com/cloudwego/eino/adk/middlewares/dynamictool`, `patchtoolcalls`, `reduction`, `summarization`, `agentsmd` — optional advanced middlewares.

### Official dynamic tool search (v0.9.13)

The pinned Eino core package
`github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch` provides
the progressive dynamic-tool surface. Construct it with
`toolsearch.New(ctx, &toolsearch.Config{DynamicTools: dynamicTools,
UseModelToolSearch: false})`. `DynamicTools` is a non-empty slice of
`components/tool.BaseTool`; the constructor rejects an empty slice and
duplicate names. With `UseModelToolSearch: false`, Eino adds the raw
`tool_search` meta-tool, hides the deferred tools before the first model call,
and rehydrates selected tools after a matching search result. Vivy installs
this middleware only when the active allowlist contains deferred tools; the
fixed-visible core remains in the static `ToolsNode` and hidden Skill-mount
tools are projected by a final Vivy mount middleware.

Vivy's business-tool adapters still enforce its allowlist, policy, approval,
hooks, output limits, and second authorization check. The official raw
`tool_search` meta-tool is deliberately not wrapped in a Vivy adapter. The
middleware order in the top-level engine is skill, compaction,
always-skills, AGENTS.md, official tool search, then the hidden mount
projection; this keeps transient instruction injection ahead of search and
lets a mount restore a tool missing from persisted Eino `ToolInfos` state.

## ReAct vs agentic models

- `adk.ChatModelAgent` with `*schema.Message` runs the full ReAct loop
  (model → tool calls → model), compatible with `ToolCallingChatModel`.
- With `*schema.AgenticMessage` (`adk.TypedChatModelAgent[*schema.AgenticMessage]`),
  the model handles tool calling internally (single-shot chain); cancel
  monitoring/retry on the model stream are not yet wired for agentic models.

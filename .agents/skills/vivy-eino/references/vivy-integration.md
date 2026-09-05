# How Vivy wires Eino today (v0.9.13)

Read these files before touching the model/agent layer. All paths are repo-relative.

## Import quarantine (D-007)

`internal/app/importlint_test.go` enforces: any `github.com/cloudwego/eino*` import
is only allowed in files under `internal/runtime/` and `internal/provider/`.
A leak anywhere else (cmd, internal/app, sdk, plugins) fails `just ci`:

```go
allowed := strings.HasPrefix(rel, "internal/runtime/") || strings.HasPrefix(rel, "internal/provider/")
if strings.HasPrefix(p, "github.com/cloudwego/eino") && !allowed { /* violation */ }
```

Consequence: new agent/model features belong in `internal/runtime` or
`internal/provider`; other layers communicate through the domain contract
(`internal/domain`, e.g. `domain.ChatModel`), never through Eino types.

## Provider layer (`internal/provider/`)

- `openai.go`: `openaiRef.Model(ctx, modelID)` builds the model per request:
  - reads API key from `os.Getenv(r.bundle.EnvKey)` (D-010: never cached/persisted);
  - `resolveAPIBase`: `VIVY_API_BASE` env var wins, else `bundle.DefaultAPIBase`;
  - returns `einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{APIKey, BaseURL, Model})` as `model.ToolCallingChatModel`.
- `ref.go` / `openai.go`: the `Ref` interface — `Model(ctx, modelID) (model.ToolCallingChatModel, error)`.
- `provider_test.go` / `internal/testsupport/echo.go`: provider and deterministic test-double coverage.

## Runtime adapters (`internal/runtime/`)

- `modeladapter.go`: `WrapModel(domain.ChatModel) model.ToolCallingChatModel` — the D-007 firewall bridge.
  - `Generate` implemented via `Stream` + concat loop (`io.EOF` ends); single `schema.Assistant` message returned.
  - `Stream` pumps the domain stream into `schema.Pipe[*schema.Message](8)`; honors ctx cancellation; `w.Send(...)` return value stops the pump when reader closed.
  - `WithTools` returns the adapter unchanged (V0 domain contract has no tool calls).
  - `fromEinoMessages`: maps `schema.Message` → `domain.Message` (role collapse: system→assistant in V0).
- `modelbroker.go`: supervised child workers.
  - `WorkerChatModel` = `Generate(ctx, []*schema.Message, ...model.Option)` + `WithTools([]*schema.ToolInfo)`.
  - `Complete`: budget reserve (`BudgetLedger.ReserveModelCall`), `WithTools(workerToolInfos(...))`, `Generate`, then maps `ResponseMeta.FinishReason` → `StopReason`, `ResponseMeta.Usage` → `WorkerModelUsage` (incl. `ReasoningTokens`).
  - `workerSchemaMessages`: WorkerModelMessage → `schema.Message` (ToolCalls JSON-marshaled into `schema.FunctionCall.Arguments`).
- `engine.go`: the top-level agent engine.
  - Builds `adk.ChatModelAgentConfig{ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: wrapped}}}` and `adk.NewChatModelAgent`.
  - Runs via `adk.NewRunner(ctx, adk.RunnerConfig{...})`; `Query`/`RunHistory`/`Resume` return `*adk.AsyncIterator[*adk.AgentEvent]`.
  - `RunHistory(ctx, msgs []*schema.Message, ...)` — history enters as Eino messages.
- `checkpoint.go` / `checkpointadapter.go`: Eino ADK checkpointing (`adk.WithCheckPointID`, `adk.CheckPoint` interfaces, resume params).
- `tooladapter.go` / `enhanced_tooladapter.go` / `toolmount_middleware.go`: wrap Vivy business tools for Eino (`einotool` `github.com/cloudwego/eino/components/tool`) and project hidden Skill-mount tools back into each model-call surface. Deferred active tools are connected to Eino core's `github.com/cloudwego/eino/adk/middlewares/dynamictool/toolsearch` middleware; the raw official `tool_search` meta-tool is not wrapped by Vivy.
- `engine.go`: partitions the exact fixed-visible core (`ask_user`, `list_dir`, `read_file`, `search_files`, `skills_list`, `skill_view`, `write_file`, `patch`, `multiedit`, `execute`, `bash`) from the remaining active allowlist. Deferred active tools are passed once to `toolsearch.New(..., UseModelToolSearch: false)` and are not duplicated in the static `ToolsNode`; no deferred tools means no official middleware. Hidden tools remain governed Vivy adapters and are mounted through the final projection.
- `internal/config` / `internal/app/settings`: normalize the retired legacy `tool_search` name once at config/settings input and write boundaries, preserving order and turning a legacy-only list into explicit empty chat-only tools. The registry remains a strict resolver and does not carry compatibility logic.
- `skills_backend.go`: `github.com/cloudwego/eino/adk/middlewares/skill`.
- `todo_backend.go`: `github.com/cloudwego/eino/adk/middlewares/filesystem` + `plantask`.
- `filesystem_backend.go`: `github.com/cloudwego/eino/adk/filesystem`.

## ADK packages in use

```go
import (
    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/adk/filesystem"                    // filesystem backend
    "github.com/cloudwego/eino/adk/middlewares/skill"             // skills backend
    "github.com/cloudwego/eino/adk/middlewares/plantask"          // todo backend
    "github.com/cloudwego/eino/components/model"
    einotool "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
    einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
)
```

## Verification

- Kernel path: `just ci` (runs `go vet`, lint incl. import quarantine, tests).
- Provider package: `go test ./internal/provider/...`.
- Runtime package: `go test ./internal/runtime/...`.
- Spike: `spike\einoverify\main.go` verifies the online eino v0.9.13 module surface; run with `go run ./spike/einoverify`.

## Reference material (read-only)

- `docs/eino-capability-verify.md` — capability verification write-up.
- `docs/architecture/VIVY-STUDIO.md` — product rules; Vivy Studio is the first-party daily IDE, while other authorized developer tools may work directly in this repo. Daily `vivy.exe` is not an IDE.

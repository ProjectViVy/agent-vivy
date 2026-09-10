# Plugin v1 pinned Eino capability check

Date: 2026-09-10

This note records the exact upstream capabilities available to PLG-P4 from the versions pinned by the repository. It is a build-time architecture input, not a claim about newer upstream releases.

## Pinned versions

- `github.com/cloudwego/eino v0.9.13`
- `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`
- `github.com/mark3labs/mcp-go v1.0.0`

Source of truth: repository `go.mod` on the PLG-P3/P4 implementation branch.

## Decisions

| Capability | Pinned API evidence | Decision | Vivy boundary |
|---|---|---|---|
| Skill loading | Eino v0.9.13 `adk/middlewares/skill`: `Backend.List`, `Backend.Get`, `Skill`, `FrontMatter` | **ADAPT** | `SkillHost` owns validation, identity, version/hash, budgets and provenance. `internal/runtime/skilladapter.go` implements Eino `skill.Backend` over the Host. |
| Skill middleware | Eino v0.9.13 `skill.NewMiddleware(ctx, *skill.Config)` | **ADAPT** | Only `internal/runtime` constructs the Eino middleware. Public Skill Sources never receive an Eino middleware or Agent handle. |
| Agents.md injection | Eino v0.9.13 `adk/middlewares/agentsmd.New` with a `Backend`, ordered files and byte budget | **ADAPT** | Useful for the existing first-party project-instruction compatibility path. Public Context Sources still return Vivy `Candidate` data and cannot inject messages directly. |
| Eino Tool ABI | Eino v0.9.13 `components/tool`: `BaseTool`, `InvokableTool`, streaming/enhanced variants | **ADAPT** | Existing ToolHost runtime adapter remains the only Eino Tool conversion boundary. |
| MCP tool conversion | EinoExt MCP v0.0.9 `mcp.GetTools(ctx, *Config)` over an initialized `mcp-go client.MCPClient` | **ADAPT** | MCPHost owns transport/session/lifecycle. `internal/runtime/mcpadapter.go` performs EinoExt conversion; resulting capabilities are projected into `std/tool-world@v1` and re-enter ToolHost. |
| MCP OAuth | No OAuth implementation or OAuth package exists in the pinned EinoExt MCP v0.0.9 tree | **DEFERRED-INDEFINITE** | Vivy does not add a parallel OAuth implementation. Existing static secret/header configuration remains available where already supported. |
| MCP resources | EinoExt v0.0.9 `GetTools` is tool-oriented and is not a Vivy Context composition API | **VIVY HOST BRIDGE** | MCPHost may expose bounded resource records, but they enter ContextHost only through an explicit resource bridge. |
| MCP prompts to Skills | No pinned API justifies automatic conversion | **FORBIDDEN** | MCP Prompts do not become Vivy Skills automatically. Import would require an explicit SkillHost validation path. |

## Exact upstream observations

### Eino v0.9.13 Skill

`adk/middlewares/skill/skill.go` defines:

```go
type Backend interface {
    List(ctx context.Context) ([]FrontMatter, error)
    Get(ctx context.Context, name string) (Skill, error)
}

func NewMiddleware(ctx context.Context, config *Config) (adk.ChatModelAgentMiddleware, error)
```

The upstream middleware can add instruction text and an Eino Tool. Vivy therefore MUST NOT expose this middleware directly to public Sources. `SkillHost` is the authority; the Eino backend is only a runtime adapter.

Pinned source:
`https://github.com/cloudwego/eino/blob/v0.9.13/adk/middlewares/skill/skill.go`

### Eino v0.9.13 Agents.md

`adk/middlewares/agentsmd/agentsmd.go` accepts an upstream `Backend`, ordered file paths and an aggregate byte budget. It injects transient **User** content during model rewrite and maintains a per-Run cache.

This is suitable for preserving the existing first-party Agents.md behavior, but it is not the public Context Source contract. ContextHost remains the sole public-source composition boundary.

Pinned source:
`https://github.com/cloudwego/eino/blob/v0.9.13/adk/middlewares/agentsmd/agentsmd.go`

### Eino v0.9.13 Tool

`components/tool/interface.go` defines `BaseTool`, `InvokableTool`, `StreamableTool` and enhanced result variants. This matches Vivy's existing runtime Tool adapter strategy.

Pinned source:
`https://github.com/cloudwego/eino/blob/v0.9.13/components/tool/interface.go`

### EinoExt MCP v0.0.9

`components/tool/mcp/mcp.go` defines:

```go
func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error)
```

It calls `ListTools`, converts MCP input schemas to Eino Tool schema, and returns wrappers whose `InvokableRun` calls `tools/call` on the supplied initialized `mcp-go` client.

Vivy will use this capability only inside `internal/runtime`. MCPHost remains responsible for configured instances, transport lifecycle, isolation, timeout/retry/circuit state and secret handling.

Pinned source:
`https://github.com/cloudwego/eino-ext/blob/components/tool/mcp/v0.0.9/components/tool/mcp/mcp.go`

## OAuth decision

The exact `components/tool/mcp/v0.0.9` repository tree contains no OAuth implementation. PLG-P4 therefore records OAuth as **DEFERRED-INDEFINITE**. This is intentional: adding a private OAuth stack would violate the architecture rule that missing pinned upstream capability is deferred rather than reimplemented in parallel.

This decision does not remove existing configured bearer/header or environment-secret behavior already supported by Vivy's current MCP backend. It only forbids presenting that static authentication mechanism as full MCP OAuth.

## Adapter quarantine

PLG-P4 maintains the import rule:

```text
public SDK / ContextHost / SkillHost / MCPHost
        -> Vivy-only types
internal/runtime/*adapter.go
        -> pinned Eino / EinoExt types
```

No public Port type contains Eino, EinoExt or `mcp-go` types. No T3 MCP server becomes a native Module.

# Plugin v1 pinned Eino capability check

Date: 2026-09-11

Status: **RECORDED** (PLG-P4 ownership continuation)

This note records the exact upstream capabilities available to PLG-P4 from the
versions pinned by the repository. It is a build-time architecture input, not a
claim about newer upstream releases. The ownership continuation recorded below
is the implementation boundary at the current head; OAuth remains explicitly
deferred.

## Pinned versions

- `github.com/cloudwego/eino v0.9.13`
- `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`
- `github.com/mark3labs/mcp-go v1.0.0`

Source of truth: `go.mod` and the local module cache for the PLG-P3/P4
implementation branch.

## Decisions

| Capability | Pinned API evidence | Decision | Current boundary | Intended target boundary |
|---|---|---|---|---|
| Skill loading | Eino v0.9.13 `adk/middlewares/skill`: `Backend.List`, `Backend.Get`, `Skill`, `FrontMatter` | **ADAPT** | `internal/runtime/skilladapter.go` implements Eino `skill.Backend` over `SkillHost`. | Keep SkillHost authoritative; expose only a read-only Eino backend from `internal/runtime`. |
| Skill middleware | Eino v0.9.13 `skill.NewMiddleware(ctx, *skill.Config)` | **ADAPT** | `internal/runtime/engine.go` constructs the middleware; `HostedSkillBackend` supplies its reads. | Keep middleware construction and Eino types quarantined in `internal/runtime`. |
| Agents.md injection | Eino v0.9.13 `adk/middlewares/agentsmd.New` with a `Backend`, ordered files, and byte budget | **ADAPT** | `internal/runtime/agentsmd.go` constructs the middleware over the existing first-party backend. | Preserve this first-party compatibility path; public Context Sources still return Vivy candidates, not Eino messages. |
| Eino Tool ABI | Eino v0.9.13 `components/tool`: `BaseTool`, `InvokableTool`, streaming/enhanced variants | **ADAPT** | `internal/runtime/tooladapter.go` adapts Vivy tools and keeps policy/approval before execution. | Continue using the runtime adapter; public Ports and Hosts remain Eino-free. |
| MCP tool discovery/schema conversion | EinoExt MCP v0.0.9 `mcp.GetTools(ctx, *Config)` over an initialized `mcp-go client.MCPClient` | **ADAPT** | `internal/runtime/mcpadapter.go` calls `GetTools` only after MCPHost opens and initializes its session, then projects bounded untrusted schemas into `mcphost.RemoteTool`. | Keep EinoExt conversion behind the MCPHost-approved session adapter and project through the standard ToolWorld and ToolHost path. |
| MCP transport/session governance | Pinned `mcp-go` clients and transports provide the low-level client/session APIs. | **ADAPT** | `MCPHost` owns activation, bounded retry/circuit decisions, session replacement, and close-once cleanup. `MCPBackend` retains only configuration/status records; `mcpadapter.go` builds one transport per Host session and reports status through the facade. | Keep MCPHost as the sole lifecycle owner; the runtime adapter performs only approved conversion and operation calls. |
| MCP OAuth | `mcp-go v1.0.0` exposes low-level OAuth primitives; see the OAuth section below. EinoExt `mcp.GetTools` has no OAuth lifecycle. | **DEFERRED-INDEFINITE** | Current `MCPBackend` supports only static bearer/header material from `AuthEnv`; no Vivy OAuth config, token store, callback, or authorization state is wired. | Reconsider only after a governed Host contract covers token storage, callback/state handling, redaction, reauthorization, and lifecycle. |
| MCP resources | EinoExt v0.0.9 `GetTools` is tool-oriented and is not a Vivy Context composition API. | **VIVY HOST BRIDGE** | `internal/mcphost/resourcesource.go` is an explicit, lazy bridge; generated production composition injects the runtime MCP ToolWorld before ToolHost staging and checks the compiled ContextHost before resource binding. | MCPHost emits bounded resources only when explicitly enabled; ContextHost remains the sole Context consumer. |
| MCP prompts to Skills | No pinned API justifies automatic conversion. | **FORBIDDEN** | The current backend exposes prompt operations for the existing MCP control surface; no prompt is converted into a Skill. | Any future import would need an explicit SkillHost validation/authority path, never implicit conversion. |

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

The upstream middleware can add instruction text and an Eino Tool. Vivy
therefore must not expose this middleware directly to public Sources. SkillHost
is the authority; the Eino backend is only a runtime adapter.

Pinned source: [`skill.go` at Eino v0.9.13](https://github.com/cloudwego/eino/blob/v0.9.13/adk/middlewares/skill/skill.go)

### Eino v0.9.13 Agents.md

`adk/middlewares/agentsmd/agentsmd.go` accepts an upstream `Backend`, ordered
file paths, and an aggregate byte budget. It injects transient **User** content
during model rewrite and maintains a per-Run cache.

This is suitable for preserving the existing first-party Agents.md behavior,
but it is not the public Context Source contract. ContextHost remains the sole
public-source composition boundary.

Pinned source: [`agentsmd.go` at Eino v0.9.13](https://github.com/cloudwego/eino/blob/v0.9.13/adk/middlewares/agentsmd/agentsmd.go)

### Eino v0.9.13 Tool

`components/tool/interface.go` defines `BaseTool`, `InvokableTool`,
`StreamableTool`, and enhanced result variants. This matches Vivy's existing
runtime Tool adapter strategy; a remote or public provider does not bypass the
ToolHost governance path.

Pinned source: [`interface.go` at Eino v0.9.13](https://github.com/cloudwego/eino/blob/v0.9.13/components/tool/interface.go)

### EinoExt MCP v0.0.9

`components/tool/mcp/mcp.go` defines:

```go
func GetTools(ctx context.Context, conf *Config) ([]tool.BaseTool, error)
```

`GetTools` calls `ListTools` on the supplied initialized
`client.MCPClient`, converts MCP input schemas to Eino Tool metadata, and
returns wrappers whose `InvokableRun` calls `tools/call` on that same client.
It does not create, initialize, retry, or close the transport.

At the current head, `internal/runtime/mcpadapter.go` calls `GetTools` for
discovery and schema projection only after the Host-owned session has been
initialized. The same Host-owned session performs governed calls, resources,
and prompts; the backend facade exposes only status/control projections.

Pinned source: [`mcp.go` at EinoExt MCP v0.0.9](https://github.com/cloudwego/eino-ext/blob/components/tool/mcp/v0.0.9/mcp.go)

## Current implementation versus intended P4 target

### Adapter placement

The current import quarantine is respected: Eino, EinoExt, and `mcp-go`
imports occur in `internal/runtime` (and the existing provider boundary), not
in SDK Ports or `internal/mcphost`. The ownership continuation now has this
shape:

- `internal/runtime/mcp_backend.go` stores normalized configuration and
  secret-free status records. It no longer stores a live mcp-go client or
  child process and no longer owns retry/replacement decisions.
- `internal/runtime/mcpadapter.go` creates one official mcp-go transport for
  each Host activation, performs the lazy handshake, invokes EinoExt
  `GetTools`, and closes that transport exactly once when MCPHost retires it.
- `internal/runtime/mcp_host_bridge.go` binds the one MCPHost to the standard
  dynamic ToolWorld. The app replaces the generated identity-only MCP
  provider with this runtime provider before ToolHost staging; the generated
  graph still seals the `mcp` identity, Host owner, and grants.

The intended P4 ownership target is implemented: MCPHost owns the configured
instance, activation, timeout/retry/circuit decisions, transport/session
cleanup, replacement, and standard ToolWorld projection; the runtime adapter
is the EinoExt conversion seam; and the explicit Resource bridge feeds
ContextHost only when that Host is compiled. Full phase acceptance and live
OAuth remain separate concerns recorded in the iteration logs.

### Transport and session ownership

The ownership split at the current head is:

1. The app constructs one shared `*runtime.MCPBackend` when the generated
   assembly includes the MCP world. The facade owns only the logical catalog,
   status records, and a pointer to the one MCPHost bridge.
2. `MCPHostSessionFactory.Open` creates exactly one transport object for the
   Host instance. The returned `mcpHostSession` owns that transport, including
   HTTP session or stdio process cancellation; its `Close` is idempotent.
3. `mcphost.Host` owns the active session reference, bounded retry/circuit
   policy, replacement retirement, and final close. Retired settings create a
   fresh session and close the old transport before the replacement is
   discoverable. Status writes carry the logical record identity, so retired
   sessions cannot contaminate a same-named replacement.
4. The compatibility factory used by older internal callers also creates a
   fresh Host-owned transport; it never returns a borrowed or no-op-close
   session. The lifecycle and replacement tests cover HTTP close-once behavior,
   stdio process death, and replacement cleanup.

## OAuth decision

The pinned `github.com/mark3labs/mcp-go v1.0.0` source does contain useful
low-level OAuth building blocks. `client/oauth.go` aliases
`transport.OAuthConfig`, `Token`, `TokenStore`, and `MemoryTokenStore`, and
provides `NewOAuthStreamableHttpClient`, `NewOAuthSSEClient`, PKCE/state helpers
(`GenerateCodeVerifier`, `GenerateCodeChallenge`, `GenerateState`), and
authorization-required error helpers. `client/transport/oauth.go` defines
`OAuthConfig`, the context-aware `TokenStore` interface (`GetToken` and
`SaveToken`), `NewOAuthHandler`, token refresh/metadata discovery, and the
transport OAuth options `WithHTTPOAuth` and `WithOAuth`.

Pinned sources: [`client/oauth.go` at mcp-go v1.0.0](https://github.com/mark3labs/mcp-go/blob/v1.0.0/client/oauth.go),
[`client/transport/oauth.go` at mcp-go v1.0.0](https://github.com/mark3labs/mcp-go/blob/v1.0.0/client/transport/oauth.go),
and [`streamable_http.go` at mcp-go v1.0.0](https://github.com/mark3labs/mcp-go/blob/v1.0.0/client/transport/streamable_http.go).

These primitives do **not** by themselves provide the governed Vivy/Eino
integration required by P4:

- EinoExt MCP v0.0.9 accepts an already initialized `client.MCPClient`; it
  does not define OAuth configuration, token persistence, redirect/callback
  handling, PKCE/state authority, or reauthorization status.
- The current `MCPBackend`/`MCPHost` configuration has `AuthEnv` for static
  bearer-header material, but no OAuth client fields, secret references,
  durable `TokenStore`, callback route, or authorization-required state.
- The Host boundary still needs to decide who owns token storage, scopes,
  metadata discovery, redaction, cancellation, refresh, retry/circuit
  behavior, and cleanup. Calling `mcp-go` OAuth constructors directly from
  the current backend or adapter would create a second, ungoverned
  transport/auth path and would not make EinoExt or ToolHost aware of that
  lifecycle.

Accordingly, the correct decision remains **DEFERRED-INDEFINITE**. This is a
deferral of the governed Vivy/Eino integration, not a claim that mcp-go lacks
OAuth code. Existing static bearer/header and environment-secret behavior stays
available where already supported, but it must not be presented as full MCP
OAuth. No placeholder OAuth implementation task is added.

## Adapter quarantine and exit evidence

```text
public SDK / ContextHost / SkillHost / MCPHost
        -> Vivy-only types
internal/runtime/*adapter.go
        -> pinned Eino / EinoExt / mcp-go types
```

No public Port type contains Eino, EinoExt, or `mcp-go` types. No T3 MCP
server becomes a native Module. The host-owned transport/session boundary,
standard ToolWorld/ToolHost wiring, explicit Resource bridge, and build-owned
conformance evidence are recorded in `docs/logs/2026-09-10-plugin-v1-p4/`.

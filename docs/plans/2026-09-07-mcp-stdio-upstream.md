# MCP Transport Upgrade Proposal: stdio Integration and Upstream OAuth (Eino/EinoExt Upstream First)

- Date: 2026-09-07
- Positioning: Proposal, decision record, and acceptance basis for slice 1. The stdio slice landed on 2026-09-08; OAuth remains a separate follow-up slice.
- User decisions (2026-09-07):
  1. Implement MCP capabilities with the existing upstream Eino/EinoExt components (and their mcp-go dependency); **do not build a custom protocol stack**.
  2. Address upstream gaps through the **PR path** and contribute upstream; do not fork or replace the pin.
- Impact surface: `internal/config`, `internal/app/settings`, `internal/app/app.go`, `internal/runtime/mcp_backend.go`, MCP RPC/browser management surfaces, and MCP state projection; no changes to Journal/Policy/HITL contracts (except OAuth state projection). The sidebar continues to reuse the existing `configured/initialized/error` states, while the existing MCP snapshot is extended additively with `transport` and `env_missing` fields so the TUI can display the stdio type and missing child key.

## 1. Eino Capability Check (Required, Evidence Cited)

| Capability | Upstream Source (Pinned Repository Version) | Conclusion |
|---|---|---|
| stdio transport | `github.com/mark3labs/mcp-go v1.0.0` (direct go.mod dependency): `transport.NewStdioWithOptions` constructs the transport, and `client.NewClient` constructs the official client; `transport.WithCommandFunc` (custom spawn hook), `WithCommandLogger`, `WithCommandStderrWriter`; bounded stderr ring buffer; Close waits for process exit | Adopt directly; do not use the eager-starting `client.NewStdioMCPClientWithOptions` |
| OAuth 2.1 (Streamable HTTP/SSE) | Same repository: `client.NewOAuthStreamableHttpClient` + `transport.OAuthConfig`; `OAuthHandler` includes dynamic client registration (`RegisterClient`), PKCE, refresh rotation (RFC 8707 resource parameter), the `TokenStore` interface, and the `OAuthAuthorizationRequiredError` sentinel error | Adopt directly (slice 2) |
| Tool discovery/schema projection | `GetTools(Config{Cli: client.MCPClient})` from `github.com/cloudwego/eino-ext/components/tool/mcp v0.0.9`—**transport-independent**; Vivy already consumes it at `internal/runtime/mcp_backend.go:279` | No changes |
| Forward path (observe; do not switch) | `eino-ext/components/tool/mcp/officialmcp v0.1.1` (based on the official `modelcontextprotocol/go-sdk` v1.6.1): `ClientSession` interface + transparent reconnect via `session.Session`, custom transport factories (#946 already merged), configurable `isError` semantics through `ResultPolicy`; still tools-only | Continue monitoring |

Custom-build comparison: without the upstream components, we would need to write the JSON-RPC-over-stdio framing protocol, subprocess lifecycle management, and OAuth 2.1 + DCR + PKCE + refresh rotation ourselves—this conflicts with architecture decisions 2/3 in AGENTS.md and is explicitly out of scope.

## 2. Current-State Inventory ("Do Not Redo" When Assigning Work)

- `MCPBackend` already uses the official `client.NewStreamableHttpClient` in place of a custom protocol stack (`docs/research/eino-boundary-audit-2026-09-05.md` §5.1, completed 2026-09-06); initialize, typed tools/resources/prompts, and the session lifecycle all go through the mcp-go client.
- The Eino tool "project, do not mount" principle is in place; the `mcp_list_tools`/`mcp_call` + `PrepareMCPCall` approval path is unchanged.
- Existing governed limits: 512KiB raw / 256KiB content / 32-page bounds (HTTP transport layer), 8s operation timeout (context-level and transport-independent), `ReplaceServers` configuration hot reload, and close fan-out.
- The TUI sidebar already distinguishes configured/initialized MCP state (TUI-SIDEBAR-N1-OPEN).
- Configuration surface: `config.MCPServer{Name,Endpoint,Command,Args,EnvFrom,Cwd,AuthEnv}` (`internal/config/config.go`) + structurally mirrored settings/runtime configuration; `env_from` is a CHILD→HOST environment-name reference and does not store environment values.

## 3. Slice 1: stdio Transport (First, No OAuth Dependency)

All changes are thin adapters; there is no custom protocol implementation:

1. `internal/config` and `internal/app/settings`: add `command`/`args`/`env_from`/`cwd` to `MCPServer`; choose exactly one of `endpoint` and `command`; authorize PATH-name or absolute-path commands, with a dangerous-basename denylist; **do not add an execute allowlist**; edit argv line-by-line/mapped line-by-line. Tests for the new fields cover parse/validate/round-trip.
2. `internal/runtime/mcp_backend.go`: mirror the new fields in `MCPServerConfig`; when `newServer` has `command`, use `transport.NewStdioWithOptions` + `client.NewClient`, otherwise retain Streamable HTTP. `Start` the transport only during `initialize` on the first operation, retaining EinoExt `GetTools`, `mcp_list_tools`/`mcp_call`, resources/prompts, and the governance path.
3. Spawn governance (through `WithCommandFunc`, owned by Vivy): inject the minimum environment (system variables + CHILD→HOST references from `env_from`, without passing through the entire parent environment), resolve cwd inside `runtime.workspace_root`, apply the command denylist, and emit structured spawn logs (server name/command basename, without secrets); use `ComSpec` as a fallback for Windows `.cmd/.bat`.
4. Response budgets: stdio reuses the decoded/projected bounds and operation context timeout; the stdio `ReadString('\n')` seam in mcp-go v1.0.0 has no raw-frame upper bound, so this slice does not claim a 512KiB raw guarantee; register the raw-frame limit as TODO.
5. Process death is fail-closed: the next operation recognizes transport/process-closed and reuses the existing `error` state; stdio does not restart automatically, and only a configuration replacement creates a new client.
6. settings/app/RPC/browser import-export/i18n are wired through; TUI/sidebar reuse the existing sidebar route and three states, additively passing `transport`/`env_missing`; no separate UI/surface protocol is added.
7. Do not subscribe to notification / continuous listening (preserve the current behavior); Windows subprocess-tree governance is also outside this slice's guarantees and is registered as TODO.

Acceptance: `just ci`; config/settings validation cases; stdio backend integration tests use the current Go test binary as a deterministic local subprocess (no network); RPC/UI import-export tests; hot-reload/close lifecycle covered by the existing backend seam. Unified gates and split-browser/real-path smoke results are recorded in `docs/logs/2026-09-08-mcp-stdio/verification.md`.

## 4. Slice 2: OAuth (Later, Independent Decision Point)

1. Add an oauth block to `MCPServerConfig` (client_id, client_secret_env, scopes); Vivy implements `TokenStore` (tokens go to a restricted file in the instance directory and never enter Journal, logs, or event payloads—D-010).
2. For servers configured with OAuth, have `newServer` switch to `client.NewOAuthStreamableHttpClient`; map `OAuthAuthorizationRequiredError` to the third projected state, needs-auth (the sidebar already distinguishes configured/initialized; add one state).
3. Interactive authorization control-plane RPC + state machine (the browser completes authorization, with the callback landing on a temporary 127.0.0.1 port).
4. Pre-start decision point: browser-callback localhost reachability for remote/multi-client forms (limitation ⑦)—whether to initially restrict support to the local face.

## 5. Upstream Limitations and Compensations (Source Verified 2026-09-07)

| # | Limitation | Compensation |
|---|---|---|
| ① | stdio passes through the entire parent environment by default (`os.Environ()`) | Minimal env in cmdFunc |
| ② | stdio has no cwd option | Set `cmd.Dir` in cmdFunc |
| ③ | No automatic restart: process death means transport death | fail-closed + state display |
| ④ | Launching Windows `.cmd/.bat` is not guaranteed | `cmd /c` fallback in cmdFunc (verify during implementation) |
| ⑤ | stdio has no raw-frame byte upper bound in production | This slice retains only decoded/projected bounds; the raw-frame upper bound requires upstream or a transport seam, and is registered as TODO |
| ⑥ | OAuth covers only Streamable HTTP/SSE | Per the specification (stdio has no OAuth); no compensation needed |
| ⑦ | OAuth interactive callbacks assume backend localhost is reachable | Slice 2 decision point |
| ⑧ | TokenStore is only an interface | Vivy implementation |
| ⑨ | Eino components are tools-only, `IsError`→Go error | Preserve the existing boundary (recorded in audit §5.1) |
| ⑩ | mcp-go v1.0.0 has no subprocess-tree exit callback; Close/WaitDelay cover only the current child | This slice records a Windows process-tree governance TODO and does not claim complete tree cleanup |

## 6. Upstream PR Path (Do Not Fork)

| Item | Repository / Process | Timing |
|---|---|---|
| stdio `WithWorkingDir` option; Windows batch-spawn handling; minimal env option | mark3labs/mcp-go: MIT, fork → branch → tests → PR to main, with no CLA/DCO gate; the 2026-09-07 collision check found no overlap (no similar issue/PR) | This slice first works around it with Vivy `WithCommandFunc`; whether to draft an upstream PR is an independent decision after sufficient raw-frame/process-tree evidence |
| Add resources/prompts to officialmcp | cloudwego/eino-ext: Apache-2.0, git-flow (**PR target is the `develop` branch**), AngularJS commit convention, gofmt + golangci-lint, feature work **issue first, then PR**, CLA required | When choosing the officialmcp route |
| Routine defect fixes | Upstream PR, with a local hook as the initial workaround | Propose one when encountered |

Fork trigger condition (none of the current 9 limitations meet it): the upstream refuses a fix and the hook cannot work around it at all.

## 7. Explicitly Out of Scope

- Do not fork or replace the pin for mcp-go or eino-ext.
- Do not switch to a parallel officialmcp/go-sdk dual-SDK stack (keep mcp-go v1.0.0 + eino-ext/tool/mcp v0.0.9 unchanged).
- Do not build a custom MCP protocol stack; do not subscribe to notifications.
- Do not treat stdio as a plugin-loading surface; it is an explicitly configured MCP dependency. Configuration authorizes only PATH-name/absolute-path commands and a safe denylist; do not add a separate execute allowlist.
- Do not describe the unimplemented raw-frame budget or Windows process-tree cleanup as delivered capabilities; both go into `docs/TODO.md` §0.1.
- Removal boundary unchanged: remove the corresponding mcp-go typed plumbing only after the Eino MCP components cover resources/prompts and preserve `isError` semantics (audit §5.1).

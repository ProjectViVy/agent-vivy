# MCP stdio slice 1

> Historical snapshot (2026-09-08), superseded by PLG-P4 closure on
> 2026-09-11. The existing `mcp_call` wording below is retained as history;
> current remote model calls use MCPHost → ToolWorld → ToolHost.

Date: 2026-09-08

## Delivered

- Configuration and the settings mirror support an HTTP/stdio choice; stdio
  uses `command`, per-item `args`, `env_from` (CHILD→HOST), and relative `cwd`.
- Configuration is authorization: commands accept a PATH name or absolute path,
  with a dangerous-basename denylist; no execute allowlist was added. Runtime
  clamps cwd to `runtime.workspace_root`, and the environment injects only
  minimal system variables and explicit references.
- runtime constructs through the upstream `transport.NewStdioWithOptions` +
  `client.NewClient`; it starts only on the first operation. EinoExt `GetTools`,
  existing mcp_list_tools/mcp_call, resources/prompts, and governance paths
  remain in place.
- Missing stdio environment, handshake failure, and process death all fail
  closed; the next operation recognizes death and reuses the existing `error`
  state, and stdio is not automatically restarted. Windows `.cmd/.bat` uses
  `ComSpec` as a fallback.
- The app overlay, control RPC, MCP browser settings surface, JSON
  import/export, and English/Chinese i18n are wired through. TUI/sidebar keeps
  the existing sidebar route and configured/initialized/error states, passing
  `transport`/`env_missing` additively; no separate UI/surface protocol was
  added.

## Explicitly not done

- The mcp-go stdio reader's raw-frame byte upper bound is not yet complete;
  currently only decoded/projected bounds are guaranteed.
- Windows child-process-tree governance (Job Object/process group) is not yet
  complete; current close/wait handling covers the current child managed by
  mcp-go.
- OAuth/TokenStore/needs-auth remains MCP-TRANSPORT-1 slice 2.

Related decisions and upstream evidence: `docs/plans/2026-09-07-mcp-stdio-upstream.md`.

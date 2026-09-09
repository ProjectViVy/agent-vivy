# MCP panel connected to the real backend (2026-08-30)

## Changes

Turn `/mcp` from the `vivy.demo.mcp` local mock into a real management surface. Add /
edit / enable / delete / import / export actions in the panel write to the user's
`settings.yaml` workspace and immediately enter the runtime catalog used by
`mcp_list_tools` / `mcp_call`.

### Core

- `internal/app/settings`: `mcp_servers` overlay. nil = use `config.yaml`
  `runtime.mcp_servers`; non-nil (including an empty list) replaces the whole list.
  Fields: `name` / `endpoint` / `auth_env` / `enabled`. `auth_env` accepts only an
  environment-variable name (D-010).
- `applySettingsOverlay`: merge enabled entries into `cfg.Runtime.MCPServers`.
- `EinoMCPBackend.ReplaceServers`: hot-replace the catalog and discard old sessions.
- Streamable HTTP SSE: parse the JSON-RPC result in `data:` lines of the
  `text/event-stream` response; the JSON path is unchanged.

### RPC

Added (capabilities registered):

- `settings/mcp`
- `settings/mcp/upsert`
- `settings/mcp/delete`
- `settings/mcp/probe`

Secrets are never returned; `auth_env_set` reports only whether the environment variable
exists. After saving, `OnSettingsChanged` → `ReplaceServers`.

### UI

- New view `ui/src/components/mcp/McpView.tsx`; the route removes `DemoBanner`.
- The form accepts only an HTTP address plus optional `auth_env`. STDIO is no longer
  offered (it was a fake operation in this iteration).
- Imports accept common MCP JSON; stdio entries are skipped with an explanation rather
  than being presented as connected.
- Delete `McpDemoView`, MCP CRUD from `demo-api`, and `DemoMcpServer`.

## Explicitly not done

- Remove demos from Notebook / Persona / Cron / Skills / Memory / Evolution.
- Chat edit / revert / fork, attachments / AutoDream / voice.
- MCP elicitation, stdio transport, or promoting remote tools to independent catalog
  entries.
- Introduce the `eino-ext` MCP package.

## Changed files

Backend: `internal/app/settings`, `internal/runtime/mcp_backend.go`,
`internal/app/app.go`, `internal/rpc/control.go`, `config.example.yaml`, and
corresponding tests.

Frontend: `ui/src/components/mcp/`, `ui/src/routes/_layout.mcp.tsx`,
`ui/src/lib/api.ts`, `ui/src/lib/demo-api.ts`, i18n, and e2e.

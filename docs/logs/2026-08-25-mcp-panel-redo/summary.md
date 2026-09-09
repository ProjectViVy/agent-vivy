# MCP panel redesign (oil-frontend conventions)

## Changes

Redesigned the `/mcp` panel according to oil-frontend conventions, replacing the
previously incomplete implementation.

### Removed

- `ui/src/components/demo/mcp.css` (353-line private pink design system:
  gradient background, dot pattern, floating heart decorations, and separate
  Tokens); fallback values are no longer expanded into a new design system.
- Decorative elements: three floating `Heart`s, the hero icon, and the gradient background.
- Fake operations and dead states: the “Refresh” button that only reread
  localStorage; the unreachable `degraded` state; the `status` field duplicating
  `enabled`; raw status text (the `connected` mono label); and the breadcrumb
  Back button duplicating sidebar navigation.
- Dead code: `getMcpConnectionStatus` and `McpConnectionStatusDto` (no callers).
- Four colored statistic cards (information duplicated by the filter and inline switches).

### Rebuilt

- Data model (`ui/src/lib/types.ts`, `ui/src/lib/demo-api.ts`):
  - `DemoMcpServer` adds `command` (stdio launch command) and `url` (HTTP
    service address), giving each object an identifiable connection target.
  - Added `updateDemoMcpServer` and `removeDemoMcpServer`; `addDemoMcpServer`
    now validates input (name required; HTTP must be an absolute http(s) URL;
    stdio must have a launch command; duplicate names rejected).
  - JSON import/export round-trips `command` / `url`.
- View (`ui/src/components/demo/McpDemoView.tsx`): follows the existing
  `CronTaskManagementView` pattern—`Card` list + inline `Switch`/edit/delete,
  `Dialog` add/edit form (transport-dependent command/address fields),
  `AlertDialog` delete confirmation, `Skeleton` loading state, empty and filtered
  empty states, and busy locking scoped to the current row. Everything uses the
  project’s shadcn components and design Tokens, with no page-private CSS.
- e2e (`ui/e2e/runtime.spec.ts`): assertions updated to the new contract (title
  “MCP Services”, switch checked state); no longer asserts the removed raw status code text.

### Explicitly not done

- No backend MCP management RPC was added: kernel MCP is still driven by
  `runtime.mcp_servers` in `config.yaml`; this panel remains within the demo/local
  mock boundary (`vivy.demo.*` localStorage), clearly marked by DemoBanner.
- Other demo pages and shared components were not changed.

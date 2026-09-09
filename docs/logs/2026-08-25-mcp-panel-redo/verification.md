# Verification

## Commands and results

- `pnpm typecheck` (ui/): passed.
- `pnpm test` (ui/, vitest): all 22 cases in 7 files passed, including the new
  `updates, removes and validates MCP servers` (failure paths for edit, delete,
  URL/command/duplicate-name validation) and assertions for `command`/`url`
  import/export round trips.
- `just ci` (fmt-check + vet + go test ./... + headless-compile + ui-ci): all passed.

## Browser smoke (http://127.0.0.1:3015/mcp, split Vite)

Stepped through the browser flow and checked:

1. Title “MCP Services”, demo banner, import/export/add buttons, and the service
   list card (“2 services · 1 enabled”).
2. Each row shows its connection target (`npx -y @modelcontextprotocol/server-filesystem .`,
   `http://127.0.0.1:9123/mcp`) and tool count.
3. Toggle the Browser Tools switch: row actions are disabled during the request
   (the busy scope is correct); afterward the switch is checked and the count is
   “2 enabled”.
4. Add Service dialog: after selecting HTTP, the linked field is “Service
   Address”; submitting empty shows “Please enter an HTTP service address” and
   preserves the entered content; submitting `http://127.0.0.1:9999/mcp` adds a
   new service sorted by name.
5. Delete: the confirmation dialog appears; after confirmation, the row disappears
   and the count drops.
6. Search with no results: “No MCP services match the filters” and “Clear
   Filters” appear; clearing restores the list.
7. The console has no error/warn. Screenshot: `ui/test-results/mcp-panel-redo.png`
   (test-results is a gitignored temporary artifact).

After the smoke test, demo data was restored to its initial state (Browser Tools disabled).

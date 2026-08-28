# Verification

## Commands

From `../agent-vivy-mcp-live` (worktree `feat/mcp-live`):

```text
just ci
```

UI unit tests covering the new import parser:

```text
cd ui; pnpm exec vitest run src/components/mcp/mcp-import.test.ts src/lib/demo-api.test.ts src/i18n/index.test.ts
```

Playwright:

```text
just ui-e2e
```

or at least `ui/e2e/mcp-settings.spec.ts` and the MCP section of `ui/e2e/runtime.spec.ts`.

## Expected

- Go tests: settings overlay round-trip (including explicit empty list), MCP RPC CRUD / invalid URL / read-only / probe error, `ReplaceServers`, SSE parse.
- UI: no `vivy.demo.mcp`; i18n zh/en leaf parity; import parser skips stdio.
- e2e: `/mcp` empty state, add HTTP server, reload persists, toggle, delete; no DemoBanner.

## Results

From `../agent-vivy-mcp-live` (worktree `feat/mcp-live`):

```text
just ci
```

- `fmt-check` / `vet` / `go test ./...` / `headless-compile` / `ui-ci` (typecheck + 162 vitest + `pnpm build`) **green**.
- First `vet` attempt on a fresh worktree failed with `ui/embed.go: pattern all:dist: no matching files found` (known `UI-CI-BOOTSTRAP`; filled `ui/dist` from the original tree, not committed).

```text
cd ui; pnpm e2e -- mcp-settings.spec.ts runtime.spec.ts
```

- `e2e/mcp-settings.spec.ts` **passed** (2.8s): empty state, add HTTP server, reload, toggle, delete, no `vivy.demo.*`.
- `e2e/runtime.spec.ts` **failed on a pre-existing assertion** `getByRole('button', { name: '画图' })` at line 9. ChatInput no longer has a drawing button on this branch; this is not an MCP regression. The MCP section of that file was not reached.

## Browser smoke

The Playwright MCP spec boots `go run ./cmd/vivy` and drives `/mcp` against the real control plane. That is the user-visible path for this change. Split Vite on `:3015` was not additionally walked in this iteration.

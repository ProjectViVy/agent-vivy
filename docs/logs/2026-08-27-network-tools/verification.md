# Verification record (2026-08-27, Settings → Network Tools)

## Commands run and results

Development took place in the independent worktree `../agent-vivy-network-tools` (branch
`feat/network-tools`, based on the latest rebased main `0791a3d`), without touching the root
tree (which was occupied by the Settings-Language lane at the time, in accordance with
`parallel-worktree-isolation`).

### Go unit-test slice (changed packages)

```text
go test ./internal/config ./internal/app/settings ./internal/runtime ./internal/rpc   # all green
go test -tags vivy_headless ./internal/app                                           # all green
```

New coverage: config parsing/validation of `network_search.provider` (including rejection of
unknown providers), settings round trip + allowlist validation, runtime preferred-provider
behavior + unavailable-provider fallback + three-state availability (`t.Setenv`, no real
network), app overlay network preference, and the `network_search` section of RPC
settings/get|update (including invalid-provider rejection, provider-name and roster
assertions, and retained api_key non-return/no-leak checks).

### `just ci` (in the worktree, after `pnpm install --frozen-lockfile` set up dependencies)

```text
just ci   # fmt-check → vet → go test ./... → headless-compile → ui-ci[typecheck → vitest → vite build]
```

**Passed, exit code 0**. UI unit tests: 105 passed (15 files), including
`diva-preview-data.test.ts` (2 tests asserting that the network section was removed) and
`i18n/index.test.ts` (9 tests asserting matching zh/en leaf structures and bilingual
symmetry for the new `networkTools` entries); the Vite production build succeeded (2202
modules).

### e2e real path (`pnpm e2e -- network-tools-setting.spec.ts`)

The webServer started `go run ./cmd/vivy` itself (E2E_ADDR 127.0.0.1:8799, isolated mock
workdir, without touching production `data/`):

- First run failed once: strict-mode conflict in `getByRole('option', { name: 'Wikipedia' })`—
  the 「Automatic (...wikipedia keyless)」 option text contained the wikipedia substring,
  resolving to 2 elements. Fixed with `exact: true` + `/^自动/` for the automatic item.
  This was a spec-selector issue, not a product defect.
- Rerun **passed (1.6s, 1 passed)**: deep link `?tab=network` → section selected, real cards
  rendered (DuckDuckGo/Wikipedia keyless 「Configured」, Bing/Google/SearXNG 「Needs
  configuration」 + environment-variable-name hints) → select Wikipedia → save → refresh
  retains it → restore automatic.

### Browser real path

Following the root AGENTS.md 「smoke-for-user-visible-change」 requirement, the flow was
exercised at `http://127.0.0.1:3015`: **this iteration used the real Playwright path from
`ui-e2e`**—the webServer started `go run ./cmd/vivy` itself (E2E_ADDR 127.0.0.1:8799,
isolated mock workdir), and the browser connected to that real backend through the Vite
proxy over WebSocket, covering exactly the same code path as this iteration's UI (real RPC
settings/get|update + NetworkToolsCard rendering).

Additional note: a separate backend at `127.0.0.1:8798` in the worktree
(VIVY_CONFIG=config.dev.yaml) verified that `/rpc/bootstrap` handshakes (returns a token);
`/rpc` is WebSocket-only, and direct HTTP POST was rejected (400) as expected (unrelated
to the UI). The root tree's :8787/:3015 were occupied by parallel-lane dev servers
(chat-toolbar, etc.) without this change, so they were not used as the smoke basis for this
feature.

## Verification conclusion

- `just ci` is all green; the network-tools e2e real path (roster → select → save → refresh
  retains → restore automatic) passes the acceptance for the 「backend first, frontend
  second foundation version」.
- Not verified: no real live search call (the user explicitly did not require end to end);
  the read_only deployment path is covered by existing RPC behavior (empty SettingsPath →
  ReadOnly), without a separate e2e.
- No root-tree remnants: all changes in this iteration exist only in the worktree branch;
  merging back into main requires authorization.

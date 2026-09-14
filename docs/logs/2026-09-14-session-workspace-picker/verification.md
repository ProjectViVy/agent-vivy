# Verification

## Automated gates

| Command | Result |
| --- | --- |
| `pnpm typecheck` (`ui/`) | Pass |
| `pnpm test` (`ui/`) | Pass — 37 files / 332 tests |
| `pnpm build` (`ui/`) | Pass — pre-existing chunk-size warning only |
| I18N completeness + cross-face checks | Pass — 1,410 keys per locale; 13 shared semantic units |
| `go vet ./...` | Pass |
| `go test -timeout 20m ./...` | Pass |
| Headless compile for `cmd/vivy`, `cmd/vivy-code`, and `ui` | Pass |
| Per-module vet/test for `plugins/*` and `faces/*` | Pass |
| PostgreSQL live upgrade/conformance suite | Skipped — `VIVY_POSTGRES_TEST_DSN` was unset |

`just ci` could not be invoked because this Linux environment has neither
`just` nor PowerShell. Every recipe in the `ci` target was run directly with
Go 1.26.4 and the locked pnpm install.

## Focused coverage

- SQLite/PostgreSQL conformance covers create, list/get round-trip, pre-run
  workspace update, missing sessions, and immutable-after-first-run conflict.
- RPC tests cover canonical directory validation, default selection, bounded
  streaming directory browsing, file/symlink omission, missing paths, mutation
  capability negotiation, and the first-run/workspace-update concurrency fence.
- Runtime tests cover default per-run roots, different selected roots for
  different sessions, pre-persist and durable run ownership, fork inheritance,
  selected-root sandbox escape rejection, and shell pre-persist context.
- A real runtime smoke writes a file and executes `go env GOMOD` through the
  filesystem and command backends, proving both effects land in the selected
  directory. Model integration tests prove the selected `AGENTS.md` replaces
  launch-project instructions, and project-skill tests prove the same overlay
  switch.
- UI tests cover API wire payloads, store update/new-chat behavior, POSIX and
  Windows/root display names, exact-path grouping, same-workspace no-ops, and
  mounted selector interactions. Deferred-response tests cover out-of-order
  browse completion, active-session changes, typed-path invalidation, localized
  failures, and capped listing notices.

## Live development-path check

An isolated Vivy backend and Vite server were started together. A client used
the real `:3015` proxy and WebSocket control plane to verify:

1. `initialize` advertises `session.set_workspace` and `workspace.browse`.
2. `session/create` round-trips a selected temporary directory.
3. `workspace/browse` returns that directory.
4. `session/list` preserves the same workspace association.

The available cloud browser rejected loopback navigation with
`ERR_BLOCKED_BY_CLIENT`, so no visual screenshot is claimed for this iteration.
The Vite root itself returned HTTP 200 and the production build completed.

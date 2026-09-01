# Verification: UI-CI-BOOTSTRAP

## Environment

- Throwaway worktree `../agent-vivy-ci-boot` (`git worktree add
  ../agent-vivy-ci-boot --detach` at commit 5b2e4ba = pre-fix HEAD),
  which has neither `ui/dist` nor `ui/node_modules` — the exact
  fresh-checkout condition from the board row's 2026-08-25 and
  2026-08-30 reproductions.

## Discrimination (before the fix)

With the original `ci` order, on the bare worktree:

```
$ just vet
& "C:/Program Files/Go/bin/go.exe" vet ./...
ui\embed.go:12:12: pattern all:dist: no matching files found
error: Recipe `vet` failed on line 21 with exit code 1
```

## Fix verification (after the fix)

Copied the fixed `justfile` + `.gitignore` into the same worktree and
ran the full gate, unpiped, exit-marker checked:

```
$ just ci            # log: /tmp/ci-uiboot.log
...
== plugin-ci: telegram
ok  	example.com/vivy/plugins/telegram	4.244s
CI-EXIT:0
```

Recipe order as executed: `fmt-check` (clean) → `ui-ci` (`pnpm install
--frozen-lockfile`, `typecheck`, `test`, `build` — `✓ built in 3.50s`,
materializing `ui/dist`) → `go vet ./...` (previously the failing step;
now passes) → `go test ./...` (every package ok: cmd/vivy 5.4s,
internal/app 22.6s, channelhost 22.4s, eval 34.5s, rpc 46.7s,
runtime, storage/sqlite 48.4s, sdk/internal 36.3s, ui 1.7s, …) →
`headless-compile` → `plugin-ci` (discord, feishu, lsp, qq, telegram
all ok) → `CI-EXIT:0`.

## Notes

- The throwaway worktree is scratch (its uncommitted justfile/.gitignore
  copies exist only for this run) and is removed after the gate; the
  canonical fix lives in the lane's working tree and this commit.
- Lane tree itself unchanged behaviorally: for an established tree with
  `ui/dist` present, the reorder only moves an idempotent pnpm/build
  earlier.
- `routeTree.gen.ts` churn excluded from the commit per lane convention.

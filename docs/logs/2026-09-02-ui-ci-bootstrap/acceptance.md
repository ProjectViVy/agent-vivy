# Acceptance: UI-CI-BOOTSTRAP

## How a human tells it worked

On any machine with Go, Node/pnpm, and `just` installed:

```
git clone <repo> fresh-checkout && cd fresh-checkout
just ci
```

Before the fix this dies early with
`ui\embed.go:12:12: pattern all:dist: no matching files found`.
After the fix the same command runs to completion: gofmt check, full
Vite build (which materializes `ui/dist`), then `go vet ./...`,
`go test ./...`, the headless compile, and plugin-ci all green.

Equivalent local check without a new clone:

```
git worktree add ../ci-probe --detach
cd ../ci-probe && just ci
```

## Regression guarantees

- Existing dev trees are unaffected: `ui-ci`'s `pnpm install
  --frozen-lockfile` and build are idempotent; they now simply run
  first.
- A stale or missing `ui/dist` can no longer poison the Go-side
  steps of `just ci` — the Vite build always precedes them.
- `git status` stays clean after builds (nothing new is committed
  under `ui/dist`; `.gitignore` still ignores all of it).

## What this does not cover

- Direct `just vet` / `just test` on a fresh tree without a prior
  build still needs `ui/dist`; the documented entry point for
  verification is `just ci`.

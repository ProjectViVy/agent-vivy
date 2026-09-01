# UI-CI-BOOTSTRAP: fresh-checkout `just ci` fails at go:embed

## What changed

- `justfile` — `ci` recipe reordered from
  `fmt-check vet test headless-compile plugin-ci ui-ci` to
  `fmt-check ui-ci vet test headless-compile plugin-ci`. The Vite build
  (`ui-ci`) now creates `ui/dist` before any Go compile step, so
  `ui/embed.go`'s `//go:embed all:dist` resolves on a fresh checkout.
  `fmt-check` stays first because it never compiles (gofmt over file
  lists only), keeping gofmt feedback instant.
- `.gitignore` — the comment claiming `ui/dist/.keep` "stays committed"
  was wrong (the file never landed) and is why this kept resurfacing.
  Rewritten to state the actual invariant: `ui/dist` is never
  committed; fresh checkouts get it because `just ci` runs `ui-ci`
  before the Go-side vet.

## Why not commit ui/dist/.keep

The board row recorded two fix options. Option (a) — committing the
`.keep` — was rejected on a hard fact: `ui/vite.config.ts` sets
`emptyOutDir: true`, so every `pnpm build` deletes the file and leaves
a permanent `D ui/dist/.keep` in every developer's `git status`.
Option (b) — recipe order — has no per-build residue and was already
the pre-recorded alternative.

## Reproduction (before the fix)

Throwaway worktree `../agent-vivy-ci-boot` checked out at the pre-fix
commit (5b2e4ba), i.e. no `ui/dist`, no `node_modules`:

```
$ just vet
ui\embed.go:12:12: pattern all:dist: no matching files found
error: Recipe `vet` failed on line 21 with exit code 1
```

Matches the 2026-08-25 and 2026-08-30 empty-worktree hits on the board
row.

## Explicitly not done

- No change to `ui/embed.go`, the headless build tag, or `ui-ci`'s
  recipe body — the fix is ordering only.
- `just vet` / `just test` run directly on a truly fresh tree (without
  `just ci`) still fail the same way; the documented gate remains
  `just ci`. Widening those recipes was judged out of scope for this
  row.
- The throwaway worktree used for verification is scratch and is
  removed after the gate; nothing from it lands.

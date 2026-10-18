---
name: vivy-kernel-ci
description: "Change Vivy kernel, Studio overlay, UI, or product-contract docs, then run just ci. Feature work starts the isolated split pair (just dev, backend :8788, Vite :3016) and opens http://127.0.0.1:3016. Use when editing internal/, cmd/, ui/, studio/, architecture docs, or the user says 改本体 / just ci / ST-6."
---

# Vivy kernel + just ci

This is lane C: Vivy can be developed from Vivy Studio or any other authorized
developer tool. Work directly in the `agent-vivy` repo root (or a worktree cut
from it), never `data/`. Do not hand work off to Studio solely to satisfy a
venue convention.

## Development loop

Start the split pair for kernel, control-plane, and browser UI work. Do not
use the embedded UI, Docker, or `just build-split` as the inner-loop server.

```text
just doctor                   # read-only environment/pin/port diagnosis
just dev                      # isolated split loop: backend :8788, Vite :3016
# or:
terminal 1: just run          # dev backend 127.0.0.1:8788
terminal 2: cd ui; VIVY_BACKEND_ADDR=http://127.0.0.1:8788 pnpm dev   # :3015
```

Open the printed Vite origin (`http://127.0.0.1:3016` for `just dev`). The
backend owns the JSON-RPC control plane; Vite serves the UI and proxies
`/rpc`. `just dev` uses `.workspace/dev/home`, not the daily `~/.vivy`.
The embedded UI is still built and checked by `just ci`. `just build-split`
is the packaged headless-backend + standalone-UI path, not the edit loop.

Parallel work uses `just lane-new <slug>` (nested
`.worktrees/<slug>/agent-vivy` + `.worktrees/<slug>/laputa`), never a shared
mutable `../laputa`. See `docs/dev/DEVELOPMENT-ENVIRONMENT.md`.

## Air gap

Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
Those are the tenant Journal. Studio's DSH home is `data/studio-home/`.

## Procedure

1. Edit one concern under `internal/`, `cmd/`, `ui/`, `studio/`, or a
   product-contract doc (`docs/architecture/`, PRD/ADR that this tree honors).
2. Run **`just ci`** from the repo root (`fmt-check` + `vet` + `test`).
3. For a deliverable (not a typo-only pass), write
   `docs/logs/YYYY-MM-DD-slug/{summary,verification,acceptance}.md`
   per root `AGENTS.md`. Unfixed findings go in `docs/TODO.md` §0.1.
4. If the change needs a new body, that is a later pack (`vivy-plugin-five`
   or `vivy-sdk pack`) — not this skill's success condition.

Success is `just ci` green plus the iteration log when the change is a
delivery. A hand-rolled `go test` is not the product path when `just ci`
exists.

## Forbidden

- Using daily `vivy.exe` as an IDE
- Developing against the embedded UI on `:8787` instead of the split Vite
  origin (`:3016` for `just dev`, `:3015` for the manual pair)
- Installing a plugin by editing `engine.go`
- Treating Vivy Studio as a mandatory execution venue for a tool that already
  has authorized access to this workspace

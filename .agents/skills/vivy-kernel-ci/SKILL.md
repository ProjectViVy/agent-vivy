---
name: vivy-kernel-ci
description: Change Vivy kernel, Studio overlay, UI, or product-contract docs, then run just ci. Feature work starts the split pair (just run + cd ui; pnpm dev) and opens http://127.0.0.1:3015. Use when editing internal/, cmd/, ui/, studio/, architecture docs, or the user says 改本体 / just ci / ST-6.
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
just dev                      # one-click split loop
# or:
terminal 1: just run          # control plane 127.0.0.1:8787
terminal 2: cd ui; pnpm dev   # Vite UI 127.0.0.1:3015, proxies /rpc
```

Open `http://127.0.0.1:3015`. The backend owns the JSON-RPC control plane;
Vite serves the UI and proxies `/rpc`. `just dev` uses `VIVY_USER_HOME=data/dev-home`
and does not fall back to a mock provider. The embedded UI is still built and
checked by `just ci`. `just build-split` is the packaged headless-backend +
standalone-UI path, not the edit loop.

## Air gap

Do not read or write `data/vivy.db`, `data/demo/`, `data/workspaces/`, or
the operator's `~/.vivy`. Those are tenant / user workspaces. Studio's DSH
home is `data/studio-home/`.

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
- Developing against the embedded UI on `:8787` instead of Vite on `:3015`
- Installing a plugin by editing `engine.go`
- Treating Vivy Studio as a mandatory execution venue for a tool that already
  has authorized access to this workspace

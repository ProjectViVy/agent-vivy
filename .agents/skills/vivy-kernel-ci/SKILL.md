---
name: vivy-kernel-ci
description: Change Vivy kernel, Studio overlay, UI, or product-contract docs, then run just ci. Use when editing internal/, cmd/, ui/, studio/, architecture docs, or the user says 改本体 / just ci / ST-6.
---

# Vivy kernel + just ci

This is lane C: Vivy can be developed from Vivy Studio or any other authorized
developer tool. Work directly in the `agent-vivy` repo root (or a worktree cut
from it), never `data/`. Do not hand work off to Studio solely to satisfy a
venue convention.

## Air gap

Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
Those are the tenant Journal. Studio's DSH home is `data/studio-home/`.

## Procedure

1. Edit one concern under `internal/`, `cmd/`, `ui/`, `studio/`, or a
   product-contract doc (`docs/architecture/`, PRD/ADR that this tree honors).
2. Run **`just ci`** from the repo root (`fmt-check` + `vet` + `test`).
3. If the change needs a new body, that is a later pack (`vivy-plugin-five`
   or `vivy-sdk pack`) — not this skill's success condition.

Success is `just ci` green. A hand-rolled `go test` is not the product path
when `just ci` exists.

## Forbidden

- Using daily `vivy.exe` as an IDE
- Installing a plugin by editing `engine.go`
- Treating Vivy Studio as a mandatory execution venue for a tool that already
  has authorized access to this workspace

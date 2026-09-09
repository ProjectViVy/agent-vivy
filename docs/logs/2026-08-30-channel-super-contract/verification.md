# Super Channel contract: verification record

Date: 2026-08-30

## Scope

Documentation only. No Go / UI / SDK behavior change.

## Commands

| Check | Result |
|---|---|
| `just ci` (worktree `../agent-vivy-channel-super-contract`, branch `feat/channel-super-contract`) | **green** (exit 0, ~102s) |

Empty worktree hit known `UI-CI-BOOTSTRAP` (`ui/embed.go` needs `ui/dist`). Pre-step: `cd ui; pnpm install --frozen-lockfile; pnpm build`. Generated `ui/src/routeTree.gen.ts` from that build was reverted and is not part of the deliverable.

`just ci` slices: `fmt-check` + `vet` + `go test ./...` + headless compile + `ui-ci` (typecheck, 21 vitest files / 175 tests, `pnpm build`).

Skipped: browser smoke (no user-visible executable change).

# Face Pack Proposal Verification

Date: 2026-08-29

## Scope

Documentation only. No Go / UI / SDK behavior change.

## Commands

| Check | Result |
|---|---|
| `just ci` (worktree `../agent-vivy-face-pack`, HEAD `faeb76e`) | **blocked by pre-existing HEAD compile errors**, not this change |

First attempt failed at `vet` with `ui/embed.go: pattern all:dist: no matching files found` (known `UI-CI-BOOTSTRAP`: empty worktree has no `ui/dist`). After `cd ui; pnpm install --frozen-lockfile; pnpm build`, `vet` reached Go packages and failed:

```text
internal/app/app.go:54:12: undefined: ModelResolver
internal/app/app.go:99:14: undefined: newModelResolver
internal/app/app.go:109:24: undefined: provider.NewResolvingChatModel
internal/app/app.go:550:21: backend.TakeOrganismLease undefined
```

Those symbols are not introduced by this docs lane. This worktree was cut from `faeb76e`; the matching implementations live as **uncommitted** files on the dirty root tree (`internal/app/model.go`, `internal/provider/resolving.go`, …). This deliverable does not mix that kernel work into the face-pack branch.

Skipped: browser smoke (no user-visible executable change). Generated `ui/src/routeTree.gen.ts` from the local `pnpm build` was reverted and is not part of the deliverable.

# summary — vivy-studio submodule migration

## What changed

Moved the Vivy Studio **shell and plugin trees** out of the `agent-vivy` host
repo into an independent repository and re-attached them as a git submodule.

| Piece | Location after |
|---|---|
| Shell + plugins (`dsh-vivy-studio`, `dsh-vivy-console`, hub fork, community snapshots) | [`ProjectViVy/vivy-studio`](https://github.com/ProjectViVy/vivy-studio) @ `main` |
| Host mount | git submodule path `studio/` (mode `160000`) |
| Lifecycle CLI | still `cmd/vivy-studio` + `internal/studiocore` in **agent-vivy** |
| First-boot install | `scripts/ensure-studio.ps1`, `just ensure-studio`, wired at top of `launch-vivy-studio.ps1` |

Independent repo initial import included dirty console edits and previously
untracked `dsh-plugin-subscriptions/` from the live tree so local Studio state
was not dropped.

Host branch: `feat/vivy-studio-submodule` (commit `27b0ba5`).
vivy-studio tip: `b7de607` (docs follow-up on hub ownership).

## Explicitly not done

- Nested submodules per community plugin (still one tree in vivy-studio).
- Moving install target of vivy-source plugins out of `studio/` into
  `data/studio-home/source-plugins/` (installs still dirty the submodule WT).
- Merging/pushing `feat/vivy-studio-submodule` to `origin/main` (needs explicit
  human authorization).
- Fixing pre-existing `internal/app` ModelResolver / TakeOrganismLease break on
  the worktree base HEAD (unrelated dirty lane on root `main`).

## Why

`studio/` had grown large third-party plugin trees (~23MB tracked, hub ~76MB on
disk). Host repo should own the species + lifecycle CLI; Studio shell should
version independently and auto-install on first agent/human Studio start.

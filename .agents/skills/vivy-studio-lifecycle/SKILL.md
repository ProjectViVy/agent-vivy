---
name: vivy-studio-lifecycle
description: Drive the Studio lifecycle from inside Vivy Studio: pack a generation with vivy-sdk, eval the candidate EXE that the Studio spawns itself, human-gated release, install into the daily location, and rollback. Use when the user mentions 发布 / 安装 / 回滚 / pack+eval / ST-5 / ST-7 / ST-8, or asks to cut and ship the next Vivy body.
---

# Vivy Studio lifecycle

Vivy application feature development starts the split pair (`just run` +
`cd ui; pnpm dev`, open `http://127.0.0.1:3015`) when browser validation is
needed. This skill is only the Studio distribution lifecycle: other
authorized developer tools may edit and verify the workspace directly, but
the daily `vivy.exe` is a tenant product; it never packs, evaluates,
releases, installs, or rolls back. Studio owns the distribution lifecycle
(`docs/architecture/VIVY-STUDIO.md` §4, ST-5/7/8).

## Air gap

Do not read or write `data/vivy.db`, `data/demo/`, or `data/workspaces/`.
Those are the tenant Journal. Studio's own home (ledger, evals, rollback
snapshots) is `data/studio-home/`. The candidate eval dir and the daily
install location are separate from both.

## Tool

`vivy-studio.exe` (build with `just studio`) owns the Studio ledger at
`data/studio-home/studio.db` and the lifecycle operations:

```text
vivy-studio workspace pin <path> [--kind kernel|first-party|plugin]
vivy-studio workspace list
vivy-studio pack --with <plugin>... [--out <dir>]
vivy-studio eval --candidate <gen> [--baseline <gen>] [--suite airgap.probe]
vivy-studio release --generation <gen> [--eval <evl>] --actor human --yes
vivy-studio reject --generation <gen>
vivy-studio install --release <rel> [--target <dir>]
vivy-studio rollback [--target <dir>]
vivy-studio inspect [--target <dir>]
vivy-studio list generations|evals|releases|installs|events
```

Global flags: `--worktree <dir>` (default cwd), `--sdk <path>` (default
PATH / worktree), `--target <dir>` (default `$env:VIVY_INSTALL_DIR`).

## Procedure

1. Build the tool once: `just studio` (produces `vivy-studio.exe`).
2. Pin the worktree (one-time): `vivy-studio workspace pin <repo-root> --kind kernel`.
3. **Pack** — Studio runs the sdk itself:
   `vivy-studio pack --with hello-fs` → Generation `gen_...` (phase `built`).
4. **Eval** — the Studio spawns the candidate EXE itself with an isolated
   data dir (zero live-species participation):
   `vivy-studio eval --candidate gen_...` → EvalRun (phase `evaluated`).
5. **Release** — a human must publish. `--actor human --yes` is the gate;
   any other actor or a missing `--yes` is refused:
   `vivy-studio release --generation gen_... --actor human --yes` → Release `rel_...`.
6. **Install** — writes the released EXE into the daily location. The next
   launch of that location is the new body; a running process is never
   hot-swapped:
   `vivy-studio install --release rel_... --target <daily-dir>` → Install `ins_...`.
7. **Rollback** — restores the previous release's files; the tenant
   Journal is never touched:
   `vivy-studio rollback --target <daily-dir>`.

## Rules

- Release is human-only. Never pass `--actor` other than `human`; never
  omit `--yes` for a release. Rejecting a generation uses `reject`.
- Install/rollback targets must be outside the source tree and `data/`
  (the tool refuses otherwise).
- Inspect the daily location read-only: `vivy-studio inspect --target <dir>`.
- The species-side studio card and `evals/start` RPC are frozen
  (NG-28); do not extend them. Do not add new product semantics to
  `internal/studio` or `internal/storage/sqlite/studio.go`.

## Forbidden

- Editing `internal/runtime/engine.go` to install anything
- Calling `evals/start` or `promotions/promote` on a live species
- Releasing without an explicit human instruction and `--yes`
- Writing production `data/*.db` from this skill

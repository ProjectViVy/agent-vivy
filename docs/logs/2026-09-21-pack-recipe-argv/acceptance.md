# Acceptance — pack walks the v1 recipe contract

## How a human tells it worked

From the repository root (a built tool: `just studio`):

```text
vivy-studio --worktree . pack --recipe recipes/minimal.vivy.yml --out data/studio-home/generations/acc-min-1
```

Expected before this fix: `vivy-sdk pack failed: unknown pack argument
"--with"`. Expected now:

1. The command runs the real compile (minutes; it builds the whole
   `cmd/vivy` generation) and prints a Studio `Generation` JSON whose
   `id` is the sdk's sealed `generationId` (`gen_…`, not the old
   `gen_demo`-style local id), `phase: "built"`, and
   `source_ref: "file:…/vivy.exe"`.
2. `vivy-studio --worktree . list generations` shows that row, and the
   recorded `artifact_sha256` matches `sha256sum
   data/studio-home/generations/acc-min-1/vivy.exe`.
3. `vivy-studio eval --candidate <that id>` boots the candidate itself
   and records an EvalRun — proving the `file:` source ref contract the
   eval launcher consumes survived the rewrite.
4. `vivy-studio pack` with no `--recipe` fails immediately with
   `pack requires --recipe`.

## Explicit non-acceptance

- `data/studio-home/generations/acc-min-1` is scratch; delete it after
  the check.
- Release/install steps are unchanged by this fix and are covered by the
  species-workbench acceptance instead.

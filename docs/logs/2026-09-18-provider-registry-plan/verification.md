# Verification — 2026-09-18 provider registry plan

All commands were run from the lane worktree
`.worktrees/provider-sot` (branch `docs/provider-registry-plan`) unless stated
otherwise.

## Gate selection

This delivery changes documentation only. Per root `AGENTS.md`
("For minor editorial or agent-instruction changes, review the diff and run
`git diff --check`; full product CI is unnecessary"), the full `just ci` gate was
**not** run. The substituted checks are below, plus a structural proof that no
compiled or generated input changed.

## Commands and results

| # | Command | Result |
|---|---|---|
| 1 | `git status --short` after staging the explicit paths | exactly 13 paths: 1 modified (`docs/TODO.md`) and 12 added, all under `docs/` |
| 2 | `git status --short -- internal sdk ui fixtures schemas Dockerfile config.yaml config.example.yaml justfile` | **empty** — no source, generated, or packaging file changed |
| 3 | `git diff --cached --check` | exit 0; no whitespace or line-ending errors |
| 4 | `git diff --cached --stat` | `13 files changed, 2220 insertions(+), 2 deletions(-)`; the two deletions are the two date markers in `docs/TODO.md` |
| 5 | `git diff --cached -- sdk/internal/assembly/conformance_results.json` | **empty** — the `internal` source digest is untouched, which is the expected property of a docs-only change (`internal/sourcehash/tree.go:27-52` hashes only `internal/`) |
| 6 | Referenced-path sweep: every repository path cited by the five plan documents and this log, checked with `Test-Path` | 63 paths checked, all present. Two apparent misses were explained and neither is a document error: `internal/runtime/sidebar.go` was a typo in the check script (the documents cite `internal/rpc/sidebar.go`, which exists), and the root `config.yaml` is gitignored local walkthrough config that a fresh worktree legitimately does not contain (now stated in `MIGRATION.md` §1.3). |
| 7 | `git ls-files --others --exclude-standard` after staging | **empty** — no unstaged new file was left behind |

Git printed `LF will be replaced by CRLF` warnings for the twelve new files. This
is the repository's Windows checkout policy, not a `--check` failure; check #3 is
the authoritative line-ending gate and it is clean.

## Structural assertion (why no product gate was needed)

`docs/` is outside every hashed or compiled input:

- `internal/sourcehash.Tree` hashes regular files under `internal/` only.
- No Go package imports anything under `docs/`.
- The UI build reads `ui/`, not `docs/`.
- The generated Assembly is produced from `internal/` and `sdk/` inputs.

Therefore a `docs/`-only change cannot alter build output, the sealed Generation,
or the conformance digest. Check #5 confirms this empirically.

## Checks deliberately skipped, and why

| Skipped | Reason |
|---|---|
| `just ci` | docs-only delivery; AGENTS.md provides the lighter path, and running it would exercise unrelated lanes (`studio` submodule is dirty in the root tree, and the WF-1 lane is present) |
| `go test ./...` | no Go file changed |
| `pnpm --dir ui test` | no UI file changed |
| Browser smoke at `http://127.0.0.1:3015` | no user-visible behaviour changed; the plan documents the smoke each future phase must run |
| `go run ./sdk/internal/cmd/generate-default` | no Assembly input changed; regenerating would be a no-op at best and a dirty artifact at worst |

## Root-tree conditions observed

The root working tree was not clean at the start of this delivery:

```
 M studio
?? internal/domain/workflow_test_support.go
?? internal/workflow/
```

These belong to other lanes and were left untouched. The delivery was made in an
isolated worktree (`.worktrees/provider-sot`, branch
`docs/provider-registry-plan`), so the root tree's state is unchanged by it.

The untracked `internal/` entries are inside the `sourcehash` input set; the plan
records this as a hazard for the future code phases
(`docs/plans/provider-registry/MIGRATION.md` §5, `PROV-P5` Task 1).

## Evidence checked while authoring

All Eino/EinoExt claims in `EINO-CAPABILITY.md` were read from the module cache at
the pinned versions, not from online documentation:

- `go list -m -versions github.com/cloudwego/eino-ext/components/model/openai`
  → newest tag is `v0.1.13`
- `go list -m github.com/cloudwego/eino-ext/components/model/responses@latest`
  → no matching versions
- `github.com/cloudwego/eino-ext/components/model/agenticopenai@v0.2.2`
  downloaded into a throwaway module under `%TEMP%` and inspected; it is **not**
  added to this repository's `go.mod` or `go.sum`
- `eino@v0.9.13` inspected in the module cache for the two interface generations
  and for the absence of a `Message`↔`AgenticMessage` conversion

## Post-condition

The repository is unchanged except under `docs/`; the branch is local and was not
pushed.

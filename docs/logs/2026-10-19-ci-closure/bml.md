# S4: Cross-platform independent BML CI entry

## Scope and implementation

Baseline: `e46d332362608062b44509810628e66bce97a712`.
Lane: `.worktrees/ci-repair-bml/agent-vivy`, branch `feat/ci-repair-bml`.
Story: `docs/superpowers/plans/ci-closure/S4.md` (R4).

The baseline `bml-ci` recipe embeds PowerShell syntax even though Linux Just uses `sh`. Replace only that recipe body with a Node helper, following `scripts/lib/process.mjs` conventions rather than duplicating OS-specific shell recipes. The helper uses the existing inherited-stdio runner, resolves the BML module from the script's repository root, and executes `go vet -p=1 ./...` before `go test -p=1 -count=1 -timeout=5m ./...`. It stops on vet failure, returns child exit codes, diagnoses signal termination as failure, and preserves spawn errors as error causes. The Just `go` override remains supported as the helper's optional executable argument.

Behavioral tests inject subprocess results for command ordering, cwd/options, exit codes, spawn causes, and signals at both stages. Additional portable Node subprocess tests exercise the actual CLI's missing-executable failure from a different cwd and verify that stdout/stderr are inherited while a failing subprocess remains nonzero. `bootstrap-test` now includes this suite. No source-text-only regression assertions, skips, sleeps, owner-guard changes, dependency changes, or production BML/runtime changes were introduced. No scope expansion was needed.

## Environment and isolation

Created the lane using the owner-specified command from the original repository (no source edits there):

```text
node /home/ubuntu/memory-write-model/agent-vivy/scripts/lane-new.mjs ci-repair-bml --base e46d332362608062b44509810628e66bce97a712
```

The command successfully created this worktree and its own private sibling Laputa. All subsequent changes and validations were confined to this lane and disposable test fixtures. The original checkout and shared Laputa were not modified. No production data roots, journals, or daily Vivy home were used.

All Go and test-fixture validation commands used process-local settings:

```sh
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME='大湿'
export GIT_AUTHOR_EMAIL=1136523089@qq.com
export GIT_COMMITTER_NAME='大湿'
export GIT_COMMITTER_EMAIL=1136523089@qq.com
export GOTOOLCHAIN=go1.26.4
export GOPROXY=https://proxy.golang.org,direct
```

These identity environment variables override fixture-local `git -c user.*` values for author and committer. No Git configuration or remote was edited. Tool versions: Node `v24.9.0`, Go `go1.26.4 linux/amd64`, Just `1.43.1`.

## Red evidence

1. Unmodified baseline, from the lane root:

   ```text
   just bml-ci
   sh: 1: Syntax error: "{" unexpected (expecting "then")
   error: Recipe `bml-ci` failed on line 82 with exit code 2
   ```

   Result: exit 2 before Go starts. The emitted command is the PowerShell `Push-Location ... try { ... } finally { Pop-Location }` body, confirming the shell mismatch at the owning recipe.

2. Before implementing the helper:

   ```text
   node --test scripts/bml-ci.test.mjs
   ```

   Initial result: exit 1 with `ERR_MODULE_NOT_FOUND` for the not-yet-created helper. After adding only an inert exported `runBmlCI` returning 0, reran the same command: 9 tests, 0 passed, 9 failed, exit 1. Assertions detected absent command calls, lost exit codes (`0 !== 23`, `0 !== 29`, `0 !== 31`), missing spawn/signal failures, and the CLI incorrectly succeeding (`0 !== 1`). The inert scaffold was replaced by the implementation; it is not shipped.

## Green evidence

Commands below ran from the lane root except the explicitly identified BML race command. Every listed command exited 0 after implementation.

| Command | Observed result |
| --- | --- |
| `node --test scripts/bml-ci.test.mjs` | 9 passed, 0 failed, 0 skipped |
| `just bml-ci` | Runs `node scripts/bml-ci.mjs "go"`; vet succeeds, then `ok github.com/ProjectViVy/agent-vivy/bml 1.103s` |
| `just bootstrap-test` | 20 passed, 0 failed, 0 skipped, including the existing dependency/bootstrap protections |
| `node --check scripts/bml-ci.mjs && node --check scripts/bml-ci.test.mjs && git diff --check` | No syntax or whitespace errors |
| `just fmt-check` | `fmt-check: 1378 Go files formatted` |
| `go test -race -p=1 -count=1 -timeout=5m ./...` from `bml/` | `ok github.com/ProjectViVy/agent-vivy/bml 6.111s` |

The original failing entry point now runs the real independent Go module on Linux without a shell-dependent directory change. The behavioral tests also prove early stop after vet failure and retention of stderr diagnostics rather than suppressing them.

## Acceptance and untested boundaries

Run `just bml-ci` from a checkout to exercise BML independently, and `just bootstrap-test` to verify the new helper's failure contracts alongside the existing bootstrap guards. The normal `ci` and `backend-ci` recipes still depend on `bml-ci`; no protection was removed.

This is Linux evidence, not native Windows acceptance. Signal behavior is covered with deterministic injected child results, not a native Windows signal experiment. Windows qualification, integrated `just ci`, aggregate compatibility/real PostgreSQL checks, and independent second-pass acceptance belong to S6 and were not run here. This story does not change Go runtime concurrency; the BML race run is additional focused evidence, not a repository-wide race claim.

No UI or embedded-UI compilation was required by these independent-module checks, so no UI rebuild, dependency installation, or asset copy was performed. No server was started or stopped. No delivery commit, push, PR, merge, or deployment was made.

Delivery: an uncommitted binary-capable Git diff exported to `/home/ubuntu/memory-write-model/ci-closure-results/bml.patch`, with explicit intent-to-add only for the two new scripts and this evidence file.

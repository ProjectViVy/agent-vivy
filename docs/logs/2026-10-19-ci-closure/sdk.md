# S2: Separate SDK Go command data from diagnostics

## Scope and result

Story: `docs/superpowers/plans/ci-closure/S2.md` in the integration lane.
Requirement: R2 from `2026-10-19-ci-closure-design.md`.
Baseline: `e46d332362608062b44509810628e66bce97a712`.
Repair lane: `.worktrees/ci-repair-sdk/agent-vivy`, branch `feat/ci-repair-sdk`.

The SDK now parses or hashes stdout only at the five demonstrated affected
sites in `sdk/internal/frontend_v1.go`: `sourceGoBinding`, `goModulePath`,
`mergeSourceModulePins`' JSON reader, `canonicalResolvedModfile`, and
`snapshotDependencyLocks`' readonly build list. Each command captures stderr
separately; a nonzero exit still wraps its original error with `%w` and includes
both stdout and stderr. The importable-package guard is unchanged.

`go_output_test.go` re-executes the Go test binary as `go` through a private
PATH entry. It checks exact argv, supplies stdout and stderr independently,
and can return exit code 7. The fixture uses no shell or POSIX commands, sets
`GOPROXY=off` and `GOSUMDB=off`, and needs no module network. It covers package
binding, main/empty/missing-field/extra-field/multiple-package rejection, module
paths, both modfile JSON reads, canonical JSON, readonly build-list hashing,
and wrapped exit-error/diagnostic preservation for every changed command.

The file audit left the other `CombinedOutput` calls unchanged: their output
is used for failure diagnostics only, not parsed or hashed on success. The
additional four sites are the same demonstrated stdout/stderr pollution root
cause, not an unrelated cleanup. No new production command framework,
provider/port, runtime, owner-guard, Eino, or dependency behavior was added.
This is build-time SDK output classification, so no Eino capability adapter is
needed; runtime imports and adapters remain unchanged.

## Environment and isolation

Commands below ran from the repair lane root unless specified otherwise.
Every Go check and dependency-install subprocess inherited these process-local
values; no Git configuration or stored remote was edited:

```sh
GIT_CONFIG_GLOBAL=/dev/null
GIT_AUTHOR_NAME=大湿
GIT_AUTHOR_EMAIL=1136523089@qq.com
GIT_COMMITTER_NAME=大湿
GIT_COMMITTER_EMAIL=1136523089@qq.com
GOTOOLCHAIN=go1.26.4
GOPROXY=https://proxy.golang.org,direct
```

Lane creation, run from the original repository only as explicitly authorized:

```sh
node /home/ubuntu/memory-write-model/agent-vivy/scripts/lane-new.mjs ci-repair-sdk --base e46d332362608062b44509810628e66bce97a712
```

Result: exit 0; own Vivy worktree and private canonical Laputa clone at the
baseline lock `0e0027c5f60ad67614d3613c2d5e21d567c4b412`.
The original checkout and its sibling Laputa were not edited. No production
`data/`, journals, or `~/.vivy` were used.

For SDK artifact-building tests, `pnpm install --frozen-lockfile` in this
lane's `ui/` completed with exit 0 (pnpm 10.33.0, 408 packages). This lane has
its own `node_modules`; no mutable dependency directory is shared across lanes.
The already-built integration-lane `ui/dist` was copied unchanged into this
lane's `ui/dist` for Go embedding. No checkout-wide UI rebuild was run; SDK
pack tests built their own isolated selected-Assembly UI artifacts as required.

## Red: before production changes

With only the new tests added:

```sh
go test -count=1 -timeout=10m ./sdk/internal -run 'Test(SourceGoBindingOutput|GoModulePathOutput|MergeSourceModulePinsOutput|CanonicalResolvedModfileOutput|SnapshotDependencyLocksOutput|ParsedGoCommandFailuresRetainDiagnostics)$' -v
```

Result: exit 1, package duration 0.535s.

- `TestSourceGoBindingOutput/success_with_diagnostics` rejected valid package
  stdout because the parsed value contained
  `example/module/provider provider\ngo: downloading example/dependency v1.2.3`.
- `TestGoModulePathOutput` returned a module path containing that stderr
  download diagnostic.
- `TestMergeSourceModulePinsOutput` and
  `TestCanonicalResolvedModfileOutput` failed with
  `invalid character 'g' after top-level value`.
- `TestSnapshotDependencyLocksOutput` initially reached that same canonical
  JSON failure; failure-path assertions already passed for all five commands.
- Main, empty, missing-field, extra-field, and multiple-package rejection
  subtests passed, establishing the existing guard behavior.

The build-list regression was then isolated by keeping canonical-modfile
stderr empty while leaving readonly-list stderr nonempty; no production code
had changed:

```sh
go test -count=1 -timeout=10m ./sdk/internal -run '^TestSnapshotDependencyLocksOutput$' -v
```

Result: exit 1, package duration 0.068s. The actual digest was
`ec9632122d2c1f4cd7f0f6a2ae269e814355022981438871e685b50fdf2e31b6`,
while stdout alone required
`98888ba544d15dd6527e5f997881d02edacd4aef037bef444c4adcba93c82c87`.
This independently demonstrated stderr pollution of the sealed build-list
identity, not merely a downstream JSON failure.

## Green and checks

After replacing only those five parsed `CombinedOutput` calls with stdout
`Output` and a separate stderr buffer:

```sh
gofmt -w sdk/internal/frontend_v1.go sdk/internal/go_output_test.go
go test -count=1 -timeout=10m ./sdk/internal -run 'Test(SourceGoBindingOutput|GoModulePathOutput|MergeSourceModulePinsOutput|CanonicalResolvedModfileOutput|SnapshotDependencyLocksOutput|ParsedGoCommandFailuresRetainDiagnostics)$' -v
```

Result: exit 0; all listed tests and subtests passed, package duration 0.564s.

```sh
go vet ./sdk/internal
```

Result: exit 0, no diagnostics.

```sh
go test -race -count=1 -timeout=10m ./sdk/internal -run 'Test(SourceGoBindingOutput|GoModulePathOutput|MergeSourceModulePinsOutput|CanonicalResolvedModfileOutput|SnapshotDependencyLocksOutput|ParsedGoCommandFailuresRetainDiagnostics)$'
```

Result: exit 0, `ok agent-vivy/sdk/internal 17.067s`; no race report.

```sh
go test -count=1 -timeout=35m ./sdk/internal
```

Result: exit 0, `ok agent-vivy/sdk/internal 269.137s`.

```sh
gofmt -l sdk/internal/frontend_v1.go sdk/internal/go_output_test.go
git diff --check
```

Result: exit 0 for both, no output.

## Acceptance and boundaries

The offline regressions show that successful Go stderr cannot alter generated
package bindings, module paths, parsed modfile JSON, or dependency-list digests.
A failing subprocess remains discoverable with `errors.As(err, *exec.ExitError)`
and its exit code, stdout, and stderr are retained. No skip, xfail, guard
weakening, discarded subprocess failure cause, or race-hiding sleep was added.

This is Linux evidence only. The fixture is written without shell-specific
commands, but native Windows was not run or claimed. Aggregate `just ci`,
full-repository race, independent second-pass review, native Windows,
PostgreSQL, and sibling dependency-pin closure belong to S6/other assigned
stories; this SDK-only lane does not qualify those gates. No testing agent,
UI-driven test, deployment, delivery commit, push, PR, or merge was performed.

The delivery target is an uncommitted binary-capable Git patch at
`/home/ubuntu/memory-write-model/ci-closure-results/sdk.patch`, including only
`frontend_v1.go`, `go_output_test.go`, and this evidence record. New owned files
are explicitly marked with `git add -N` before `git diff --binary` export.

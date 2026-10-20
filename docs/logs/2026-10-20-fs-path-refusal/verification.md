# Verification: filesystem path mistakes do not fail the run (2026-10-20)

## Reproduction (before the fix)

The failing dev run is in the isolated scratch home
`.workspace/dev/home-clean` (`run_83e4d830e5a6af53`, session
`sess_edcf4ebb2275dca4`). Its log records the fatal chain:

```text
15:15:13.959 WARN run failed run=run_83e4d830e5a6af53
  err="engine event error: [NodeRunError] failed to stream tool call
  call_00_Le69gsFAxoCbGRFYLJY25850: filesystem: invalid path
  ... node path: [node_1, ToolNode]"
```

The tool arguments were `{"depth":"2","path":"./","recursive":"true"}`.
No production Journal (`data/vivy.db`, `data/demo/`, `data/workspaces/`) was
read. The process was not hung; it failed the run with the generic
`internal_error` message.

## Environment

| Item | Value |
|---|---|
| OS | Windows 11 10.0.26200 (中文版) |
| Go | `go1.26.4 windows/amd64` |
| Node / pnpm | Node `v24.19.0`; pnpm `10.11.0` (CI resolves pnpm 10.22.0) |
| Worktree | `C:\Users\com01\Desktop\agent-vivy-fs-path-refusal` |
| Branch | `fix/fs-path-refusal` from `main` `20f34ac8` |

## Commands run

| Command | Result |
|---|---|
| `gofmt -l internal/runtime/filesystem_backend.go internal/runtime/filesystem_path_test.go` | no output (both formatted) |
| `go test ./internal/runtime -run 'TestSafeWorkspacePath|TestEinoFilesystemBackendListDir|TestEinoFilesystemBackendPathErrors|TestEinoFilesystemBackendMissingPath|TestToolFailureClassification' -count=1 -timeout 10m` | **PASS** (7.116 s) |
| `go test ./internal/tools -count=1 -timeout 5m` | **PASS** (4.456 s) |
| `go test ./internal/runtime ./internal/tools -count=1 -timeout 10m` | **PASS** (`internal/runtime` 308.725 s; `internal/tools` 0.762 s) |
| `go vet ./...` | **PASS** (completed before the test lane ran) |
| `just fmt-check` | **PASS** (part of the combined lane) |
| `just headless-compile` | **PASS** (exit 0) |
| `just plugin-ci` | **PASS** (exit 0) |
| `go run ./sdk/internal/cmd/source-hash internal ""` | `58e65c9389ffe8eda3349e2a02e8004487c7d5e0b8de604af989250fbd96b621` |
| `go test ./sdk/internal/conformance -run '^TestCheckedInProviderConformanceMatchesExecutedSuites$' -count=1 -timeout 10m -v` | digest check **PASS**; the suite then failed only at `std/ui-extension@v1/fixture/full-ui` because its evidence test builds the Web UI and hits the host esbuild error below |
| `git diff --check` | **PASS** |

The five internal-provider `sourceSha256` rows in
`sdk/internal/assembly/conformance_results.json` were updated from
`9ed7deb7…` to `58e65c93…`. The producer test reaches its subtests only after
the digest equality check at `reproduction_test.go:67-69`; it did, so the
re-pin is consistent with the live `internal/` tree.

## CI gate result and the host blocker

`just ci` was attempted twice. Both attempts passed `ensure-laputa`,
`bootstrap-test` (6/6), `fmt-check`, UI typecheck, and all 643 Vitest tests,
then failed in `ui-core` during `vite build`:

```text
✓ 2583 modules transformed.
✗ Build failed in ~20-35s
error during build:
[vite:esbuild-transpile] remove C:\Users\com01\AppData\Local\Temp\esbuild-…: Access is denied.
```

The failure is not caused by this change. It reproduces on the clean `main`
checkout with:

```text
cd ui
pnpm exec vite build --outDir .verify-dist --emptyOutDir
```

with identical output, and also with `TEMP`/`TMP` redirected to a workspace
directory, to `D:\dsh-temp\...`, and with `--minify false`. The artifact can be
deleted by the shell after the failed build, so the lock is transient during
the esbuild service's remove call, not a permanent ACL. The condition is now
recorded as `CI-VITE-ESBUILD-TEMP` in `docs/TODO.md` §0.1.

A full `just fmt-check vet test headless-compile plugin-ci` run was also
attempted. `fmt-check` and `vet` passed; `test` reached the SDK pack lanes and
failed there for the same esbuild reason. `headless-compile` and `plugin-ci`
were then run separately and passed.

The full `go test ./...` lane also reported these failures in untouched code
under package-level load:

- `internal/app`: `TempDir RemoveAll cleanup` could not unlink `garden.db`
  (file still in use) in two shutdown tests.
- `internal/embedded`: `TestPollCollectsRunEvents` exceeded its 15 s action
  deadline.
- `internal/rpc`: a nil-pointer panic in `modelAdapter.Stream` after the test
  model stream was cancelled/closed under load.
- `sdk/codeclient`: the test fake `fake-vivy-code` executable was not found on
  Windows, and the real `vivy-code` case reported “persona is not initialized”.
- `ui`: the embedded handler returned 503 because `ui/dist` contained only a
  partial build.
- `sdk/internal`: every pack test that builds a Web UI failed at the esbuild
  temp-file removal above.

`internal/runtime` (the changed package) and `internal/tools` passed in that
same full run. None of the failing files or assertions is touched by this
change.

## Skipped checks

- Live split-pair browser acceptance was not run: the running dev backend is
  the user's existing instance and was deliberately not restarted. The
  executable behavior is covered by `internal/runtime/filesystem_path_test.go`
  against the real `EinoFilesystemBackend` and the existing
  `classifyToolFailure` allowlist. `acceptance.md` gives the manual steps for
  an existing instance after `just dev` rebuilds it.
- PostgreSQL DSN-backed conformance was not run; `VIVY_POSTGRES_TEST_DSN` is
  unset in this environment and this change has no storage/schema surface.
- Full `just ci` green could not be established on this host because of the
  pre-existing esbuild blocker recorded above.

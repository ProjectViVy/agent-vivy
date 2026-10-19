# Toolkit matrix verification — 2026-10-19

Environment: macOS 15 (Darwin arm64), Go `go1.27.1`, Node `v26.11.0`,
pnpm `12.10.1` (the UI's `packageManager` pin resolves to pnpm 10.33.0 inside
`ui/`), system Chrome through Playwright `channel: 'chrome'`.

Every command below ran in
`.worktrees/l4-toolkit-matrix/agent-vivy` with:

```bash
export GOCACHE=$PWD/.workspace/gocache
export GOFLAGS=-tags=http2legacy   # host Go 1.27 + pinned x/net only
export XDG_STATE_HOME=$PWD/.workspace/pnpm-state
export XDG_DATA_HOME=$PWD/.workspace/pnpm-data
```

The `GOFLAGS` tag is host-specific; `scripts/toolkits-e2e.mjs` merges it
automatically only when the host toolchain reports Go ≥ 1.27. CI uses the
`go.mod` toolchain and does not need it.

## T0: reproducible Toolkit browser gate

### Isolated ports (accepted T0 evidence)

```bash
VIVY_TOOLKITS_E2E_BACKEND_PORT=18887 VIVY_TOOLKITS_E2E_UI_PORT=3115 \
  node scripts/toolkits-e2e.mjs
```

Exit 0. Observed:

```text
toolkits-e2e: backend 127.0.0.1:18887, UI 127.0.0.1:3115
toolkits-e2e: GOFLAGS -tags=http2legacy
toolkits-e2e: generation 9de83d553ac96d483d7bab25cd7bda2ab3816527ecae3734a6284e332179ed95
toolkits-e2e: modules 42
toolkits-e2e: binary .../.workspace/toolkits-e2e/gate/default/vivy
  1 passed (9.0s)   # e2e/toolkits.spec.ts
  3 passed (7.2s)   # e2e/toolkits-accessibility.spec.ts (390px keyboard/populated, 768/1440 overflow)
toolkits-e2e: all Toolkit suites passed
```

No unexpected skip. The launcher rebuilt `vivy-sdk` and `ui/dist`, packed and
inspected the artifact, and started a fresh backend/Vite/data root per suite.
The log shows backend `127.0.0.1:18887`, never a reused server. The same run
was repeated after the launcher-hardening commit: `1 passed (10.3s)` and
`3 passed (7.4s)`.

### Default ports

```bash
node scripts/toolkits-e2e.mjs
```

Exit 1, preparation/pack succeeded, then Playwright refused the pre-bound URL:

```text
Error: http://127.0.0.1:3015 is already used, make sure that nothing is
running on the port/url or set reuseExistingServer:true in config.webServer.
```

The listener was an unrelated Vite process whose cwd was the **root**
checkout's `ui/` (`lsof -p 39360`: cwd `/Users/.../agent-vivy/ui`,
`TCP *:3015`). It was not started by this lane and was not terminated. This is
the expected fail-closed behavior; the isolated run is the accepted evidence
for this lane. A clean default-port run remains outstanding for the host owner.

### Launcher negative checks

| Check | Command | Observed |
| --- | --- | --- |
| Invalid integer | `VIVY_TOOLKITS_E2E_BACKEND_PORT=0 node scripts/toolkits-e2e.mjs` | exit 1 before any build: `must be between 1 and 65535, got 0` |
| Equal ports | `VIVY_TOOLKITS_E2E_BACKEND_PORT=18887 VIVY_TOOLKITS_E2E_UI_PORT=18887 node scripts/toolkits-e2e.mjs` | exit 1: `backend and UI ports must differ` |
| Occupied port | temporary Node listener on `127.0.0.1:19001`, then backend `19001`/UI `19002` | exit 1 before pack: `backend port 127.0.0.1:19001 is unavailable (EADDRINUSE)` |
| Path with spaces + explicit override | copy artifact to `/tmp/vivy toolkit gate/default`, `VIVY_TOOLKITS_E2E_BINARY="/tmp/vivy toolkit gate/default/vivy"` with ports 18887/3115 | exit 0; log names the explicit binary, inspects it, generation stays `9de83d55…`, `1 passed (6.5s)` + `3 passed (6.4s)`; the override path skips the embedded-UI rebuild |

`node --check scripts/toolkits-e2e.mjs` passed before these runs.

## T1–T4 focused verification

Commands and outcomes:

```bash
go test ./internal/bundles \
  -run 'TestToolkit(BeforeCommitFailurePreservesCommittedEpoch|PreviewSourceChangeDoesNotSubstitute|ReinstallDoesNotInheritConsent)' \
  -count=1
# ok agent-vivy/internal/bundles 0.686s (3 tests)

go test ./internal/bundles \
  -run 'TestToolkitCorruptSnapshotIsUnavailableWithoutLatestFallback' -count=1
# ok agent-vivy/internal/bundles 0.431s

go test ./internal/runtime \
  -run 'TestToolkitCapturedHookFailureMatrix' -count=1
# ok agent-vivy/internal/runtime 3.063s (8 subtests: allow, exit-2 deny,
# other-exit, non-JSON, rewrite, invalid updated_input, timeout, missing executable)

cd ui && ./node_modules/.bin/vitest run ../plugins/vivy-toolkits-ui/ui/vivy-toolkits/src
# 5 files / 19 tests passed (ToolkitPage 5, ToolkitUpdate 4,
# ToolkitCandidate 2, toolkit-client 2, toolkit-view 6)

pnpm --dir ui typecheck
# tsc --noEmit exit 0
```

The new tests use the existing real SQLite store, manager publication path,
captured-run context and platform shell. No test-only hook was added to
production code; `testOwners` in `manager_test.go` gained a failure flag used
by the pre-commit test.

## Substitute for `just ci`

`just` is not installed on this host (`just: command not found`), so the
recipe body was executed step by step. This is a substitute, not a claimed
`just ci` pass.

| Step | Command | Result |
| --- | --- | --- |
| bootstrap-test | `node --test scripts/ensure-laputa.test.mjs scripts/dependency-closure.test.mjs` | exit 0 |
| fmt-check | `node scripts/fmt-check.mjs` | exit 0 (1223 Go files) |
| vet | `go vet ./...` | exit 0 |
| plugin-ci | `node scripts/plugin-ci.mjs` | exit 0; every plugin/face module built and tested |
| test | `go test ./...` | exit 1 with two pre-existing failures, see below |
| diva-recall-test | `node scripts/diva-recall.mjs` | exit 1: host Go 1.27 build of gRPC, see below |
| headless-compile | `go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | exit 1: same tag override; merged-tag rerun exit 0 |
| ui-core typecheck | `pnpm --dir ui typecheck` | exit 0 |
| ui-core test | `pnpm --dir ui test` | 39 failed / 666 passed (705); all failures are the existing Node 26 localStorage baseline |
| ui-core build | `pnpm --dir ui build` (inside the gate) | exit 0 |
| i18n | `node scripts/check-i18n-completeness.js`, `node --test scripts/check-i18n-cross-face.test.js`, `node scripts/check-i18n-cross-face.js` | exit 0; 1575 keys per locale, 14 shared units, 8/8 node tests |

### `go test ./...` failures (not caused by this lane)

1. `internal/app` hit the 10-minute package timeout while running
   `TestMemoryLoopReflectionProvenanceAfterRestart`. The test passes alone
   (`go test ./internal/app -run '^TestMemoryLoopReflectionProvenanceAfterRestart$' -count=1`:
   `ok … 11.141s`), and the whole `-run 'TestMemoryLoop'` set times out on the
   **base checkout** `16808b5a` as well (`FAIL agent-vivy/internal/app 90.817s`).
   This is a pre-existing memory-loop suite interaction, not a Toolkit change.
2. `sdk/codeclient` `TestClientAgainstRealVivyCode` fails handshake with
   `codeclient: protocol stream ended`. The identical test fails on the base
   checkout (`FAIL agent-vivy/sdk/codeclient 6.158s`). Pre-existing.

Every other package in the run passed, including `internal/bundles`,
`internal/runtime`, `internal/app/settings`, `sdk/internal`,
`sdk/internal/conformance` and `ui`.

### Other substitute results

- `plugin-ci`: exit 0 for `faces/headless`, `faces/tui` and all
  `plugins/*` modules, including `plugins/vivy-toolkits-ui`.
- `diva-recall.mjs`: exit 1 before running tests. The script passes its own
  `-tags vivy_diva_integration`, which replaces the host `GOFLAGS` tag, so
  gRPC fails to build with `undefined: http2.TrailerPrefix` — the same
  host-only Go 1.27/x-net mismatch documented above. Re-running the headless
  compile with merged tags
  (`-tags 'vivy_headless,http2legacy'`) passes all three packages; the
  DIVA-recall script has no merged-tag switch, so its behavior is recorded as
  a host build blocker rather than a product failure. Its selected tests are
  memory-loop tests that also hit the pre-existing suite hang described above.
- `pnpm --dir ui test`: `9 failed | 85 passed (94)` files and
  `39 failed | 666 passed (705)` tests. The failures are the existing ambient
  Node `v26.11.0` `localStorage` shadowing that breaks happy-dom and leaves
  roots unset (e.g. `root.unmount` of `undefined`); the same count was
  observed before this lane. The Toolkit module's 19 tests pass when run
  directly through the local Vitest binary; no Toolkit failure is hidden by
  that attribution.
- i18n completeness/cross-face scripts: all exit 0.

## Gates that could not run

| Gate | Reason | Impact |
| --- | --- | --- |
| `just ci` | `just` not installed | Full aggregate recipe not executed; substitute steps above are individually recorded. |
| `just test-postgres` | `VIVY_POSTGRES_TEST_DSN` unset and Docker daemon unavailable | A28/SQLite-vs-Postgres parity remains blocked. |
| Native Windows `.exe` venue | No Windows host in this session | `vivy.exe` suffix and Windows shell path are implemented/tested by review and existing unit fixtures, not executed here. |
| `just release-conformance` | Requires an explicit historical release bundle | B10/A28 cross-version snapshot and release evidence remain open. |
| `just toolkits-e2e` literally | Same `just` absence | Recipe body `node scripts/toolkits-e2e.mjs` ran and passed. |

## Working-tree checks

- `git diff --check`: exit 0.
- `node scripts/fmt-check.mjs`: exit 0 (1223 Go files).
- The only generated file changed is `ui/src/generated/assembly.ts`; it was
  regenerated by `stage-ui` because the Toolkit UI module gained a test file
  and its source hash feeds the Assembly. It was not hand-edited.
- No `data/`, `data/studio-home/` or production journal was read or written.

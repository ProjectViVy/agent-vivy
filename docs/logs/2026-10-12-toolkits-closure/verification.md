# Toolkits closure verification — 2026-10-12

Repository: `agent-vivy`, branch `TOOLKITS`. Commands were run on macOS with `GOFLAGS=-tags=http2legacy`. The Go dependency emits a sonic/ast unsupported-host warning and falls back to `encoding/json`; commands listed as passed exited successfully.

## Product and package tests

- Passed App lifecycle and owner omission:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^TestToolkit(Lifecycle|OwnerOmissionIsReflectedByApp)$' -count=1 -timeout=180s
  ```

  `TestToolkitLifecycle` previewed and installed a native Skill+Mask Toolkit disabled, enabled it, read the captured Skill, disabled and uninstalled it, then reopened the same database and confirmed the orphan/tombstone history remained. `TestToolkitOwnerOmissionIsReflectedByApp` removed Mask/MCP owners from a generated Assembly, checked the reported capabilities, and verified previews containing either unavailable kind were rejected. The latter test first failed because Manager validation hard-coded all owners as available; it passed after `Manager.staticCapabilities` was corrected.

- Passed Toolkit packages after the owner-capability fix:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/bundles ./internal/modules/toolkits ./internal/modules/masks ./internal/actionhost -count=1 -timeout=180s
  GOFLAGS=-tags=http2legacy go test ./internal/runtime -run 'TestToolkit|TestBundle(Hook|MCP|Skill)|TestReportWorkflowAdmissionStoresToolkitSnapshot' -count=1 -timeout=180s
  ```

- The previously slow App test `TestMemoryLoopMissionChangeFencesPendingEffects` passed alone:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^TestMemoryLoopMissionChangeFencesPendingEffects$' -count=1 -v -timeout=75s
  ```

  Result: passed in 33.9 seconds.

- A clean full App-package rerun was attempted:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/app -count=1 -timeout=10m
  ```

  It was interrupted after 186 seconds to collect a goroutine dump. The active test was `TestMemoryLoopCrashC05EffectsDoneBeforeWatermark`, blocked in `memoryLoopFixture.crashAndRestart` while waiting for the fixture-owned SQLite lease timer (`internal/app/memory_loop_crash_test.go:88`). This is an App-wide memory-loop crash/restart test, not a Toolkit test. The complete App package result is inconclusive; the isolated Toolkit lifecycle/owner-omission tests pass.

## Storage and UI

- Passed all Storage packages against SQLite and the disposable local PostgreSQL container:

  ```sh
  VIVY_POSTGRES_TEST_DSN="$VIVY_POSTGRES_TEST_DSN" GOFLAGS=-tags=http2legacy go test -p 1 ./internal/storage/... -count=1 -timeout=10m
  ```

  `VIVY_POSTGRES_TEST_DSN` targeted the local test database at `127.0.0.1:55432/vivy_test`; no production service or data was used.

- Passed full UI tests: `NODE_OPTIONS=--localstorage-file=/tmp/vivy-toolkits-localstorage GOFLAGS=-tags=http2legacy pnpm --dir ui test` — 86 test files, 639 tests.
- Passed UI TypeScript check: `GOFLAGS=-tags=http2legacy pnpm --dir ui run typecheck`.
- Passed production UI build: `GOFLAGS=-tags=http2legacy pnpm --dir ui run build` (run serially after UI staging; Vite reports a large-chunk warning).
- Passed SDK verification:

  ```sh
  GOFLAGS=-tags=http2legacy go run ./sdk verify plugins/vivy-toolkits-ui
  GOFLAGS=-tags=http2legacy go run ./sdk verify plugins/vivy-masks-ui
  ```

- Passed packaged default-binary Playwright flow:

  ```sh
  GOFLAGS=-tags=http2legacy VIVY_TOOLKITS_E2E_BINARY=/tmp/vivy-toolkits-closure-20261012/vivy \
    pnpm --dir ui exec playwright test --config playwright.toolkits.config.ts
  ```

  Result: 1 real-browser test passed. The configuration starts the packaged backend and a separate Vite server with a fresh temporary SQLite directory. The E2E verifies that installing a Toolkit does not eagerly connect to its MCP endpoint.

## SDK artifacts and omission fixtures

All artifact commands succeeded with `GOFLAGS=-tags=http2legacy`; each artifact was written to a fresh `/tmp` directory and inspected with `sdk inspect-artifact`.

| Recipe | Artifact directory | Generation ID | Binary SHA-256 | Inspection |
| --- | --- | --- | --- | --- |
| Default | `/tmp/vivy-toolkits-closure-20261012` | `bf51a3a97b7db3b1bc10035946b2b082ea587f7fb9badcc463e078c749376fb5` | `fc9b4a95f3fb11074b96b66fec7d91acadfe30dd90fc44378ea7d6950be5076f` | 42 Modules; Toolkit/Mask/MCP owners present |
| No Toolkit UI/action owner | `/tmp/vivy-toolkits-no-ui-actions-closure-20261012` | `39876abc55a7502d0a0346c4b9ad834933eb579e7d686367bf603601403e6bc1` | `32d664b7ee6c46800aaed14bc0a239b5be5d0189b07d5a7bdbda36966d35f284` | 40 Modules; Toolkit UI extension and `vivy.bundles.*` actions absent |
| No Mask/MCP owners | `/tmp/vivy-toolkits-no-mask-mcp-closure-20261012` | `976336c5cd974a715fcf569bb56169eb375dd6aa7f6dcb8fa0f228ffae63b57d` | `d20e80b091954a710bf1601923701c80bd8f82847a019274df01f77dd4d563a7` | 39 Modules; Masks, MCP Host and MCP ToolWorld absent; Toolkit UI/actions retained |

The `vivy/protected-tools` export list is identical in all three inspected artifacts (11 protected tools). The no-Toolkit artifact contains no `vivy/toolkits`, `vivy/toolkits-ui`, Toolkit action providers or Toolkit UI extension. The no-Mask/MCP artifact retains Toolkit management but compiles no Mask/MCP owner. The disposable Recipes were generated under `/tmp`; generated Assembly was written through the SDK.

## Full-suite and environment gates

- `just ci` was not runnable: `just`, `pwsh` and `powershell` are not installed in this macOS environment. This remains a full-CI gate, not a Toolkit failure.
- The full App package suite remains incomplete because its memory-loop crash/restart case did not finish within the observed window. It is not counted as a passing gate.
- Keyboard-only navigation and mobile/responsive browser acceptance were not run.
- The local PostgreSQL container is a disposable test dependency and is stopped during cleanup.

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

- Passed isolated keyboard/mobile browser acceptance without an MCP fixture or MCP endpoint:

  ```sh
  VIVY_TOOLKITS_E2E_BINARY=/tmp/vivy-toolkits-closure-20261012/vivy \
    pnpm --dir ui exec playwright test --config playwright.toolkits-accessibility.config.ts
  ```

  Result: 2 real-browser tests passed. At 390px, one opens the mobile navigation, drills into the VIVY section and opens Toolkits using keyboard focus and Enter, then checks for horizontal overflow. The other exercises preview failure and success, installs a native Skill+Mask Toolkit, opens an 1,800-character unbroken Skill body, verifies dialog/content bounds, then closes with Escape and confirms focus returns to the opener. This spec starts only the packaged Vivy backend and Vite UI. It does not create an MCP mock, declare an MCP component, or make an MCP connection.

## Live MCP stdio closure smoke

- Passed a one-off macOS integration smoke using the official [`server-everything`](https://github.com/modelcontextprotocol/servers/blob/main/src/everything/README.md) test server, pinned to `@modelcontextprotocol/server-everything@2026.8.18` and started by `npx` in stdio mode. The upstream describes it as a test server for MCP client builders and documents stdio startup. The package emitted npm's deprecation notice for date-based versions; the pinned release remained installable and completed this run.
- The harness composed the server through Vivy's `MCPBackend` → MCPHost ToolWorld → App ToolHost registry, then ran a local `ScriptedModel` against a temporary SQLite store. No external model service or Windows environment was used. The MCP echo tool was correctly treated as effectful: the run paused for approval, the test explicitly approved it, and the completed run journal contained the exact marker returned by `mcp.everything.echo`.
- Provider shutdown closed the session; a subsequent call through the previously bound ToolHost view failed. The temporary harness source was removed after the smoke; no product code was changed for this check.
- Cold-cache limitation found during the first attempt: ToolHost's default world-discovery deadline is 2 seconds (`internal/toolhost/host.go`), and the first `npx` install/start exceeded it. After the package was present in the local npm cache, the same governed Vivy run passed. Therefore this proves the initialized stdio discovery/call/shutdown path, but it does not prove that a first-time `npx` install can finish within Vivy's current discovery deadline.

- The existing lifecycle Playwright flow passed separately with its own fresh SQLite backend:

  ```sh
  VIVY_TOOLKITS_E2E_BINARY=/tmp/vivy-toolkits-closure-20261012/vivy \
    pnpm --dir ui exec playwright test --config playwright.toolkits.config.ts
  ```

  Result: 1 real-browser test passed. It starts a local MCP mock only to assert that install, binding and consent UI produce zero requests; no MCP transport call or live endpoint was exercised. Running both specs in one backend initially exposed test-state contamination, so the accessibility tests now use a separate config and fresh backend.

## SDK artifacts and omission fixtures

All artifact commands succeeded with `GOFLAGS=-tags=http2legacy`; each artifact was written to a fresh `/tmp` directory and inspected with `sdk inspect-artifact`.

| Recipe | Artifact directory | Generation ID | Binary SHA-256 | Inspection |
| --- | --- | --- | --- | --- |
| Default | `/tmp/vivy-toolkits-closure-20261012` | `bf51a3a97b7db3b1bc10035946b2b082ea587f7fb9badcc463e078c749376fb5` | `fc9b4a95f3fb11074b96b66fec7d91acadfe30dd90fc44378ea7d6950be5076f` | 42 Modules; Toolkit/Mask/MCP owners present |
| No Toolkit UI/action owner | `/tmp/vivy-toolkits-no-ui-actions-closure-20261012` | `39876abc55a7502d0a0346c4b9ad834933eb579e7d686367bf603601403e6bc1` | `32d664b7ee6c46800aaed14bc0a239b5be5d0189b07d5a7bdbda36966d35f284` | 40 Modules; Toolkit UI extension and `vivy.bundles.*` actions absent |
| No Mask/MCP owners | `/tmp/vivy-toolkits-no-mask-mcp-closure-20261012` | `976336c5cd974a715fcf569bb56169eb375dd6aa7f6dcb8fa0f228ffae63b57d` | `d20e80b091954a710bf1601923701c80bd8f82847a019274df01f77dd4d563a7` | 39 Modules; Masks, MCP Host and MCP ToolWorld absent; Toolkit UI/actions retained |

The `vivy/protected-tools` export list is identical in all three inspected artifacts (11 protected tools). The no-Toolkit artifact contains no `vivy/toolkits`, `vivy/toolkits-ui`, Toolkit action providers or Toolkit UI extension. The no-Mask/MCP artifact retains Toolkit management but compiles no Mask/MCP owner. The disposable Recipes were generated under `/tmp`; generated Assembly was written through the SDK.

## Full-suite and environment gates

After the closure commit, a second focused rerun exercised Toolkit paths that do not require an MCP server or transport:

```sh
GOFLAGS=-tags=http2legacy go test ./internal/bundles -run '^(TestToolkitPublishOrder|TestToolkitRestoreRunUsesCapturedReleaseAfterDisable|TestToolkitRollbackCreatesNewReleaseSequenceForRepeatedDigest|TestToolkitUpdateOnlyBlocksRemovedMaskIdentities|TestToolkitGarbageCollectionRespectsInstallRunAndPreviewReferences|TestToolkitRecoveryRestoresCommittedEpochAndRejectsMissingArtifact|TestBundleSkillResources|TestAcquireNativeDirectory|TestAcquireRejectsEscapes|TestObjectRecoveryRejectsTampering|TestGitAcquisitionHasNoExecution)$' -count=1 -timeout=180s
GOFLAGS=-tags=http2legacy go test ./internal/runtime -run '^(TestToolkitRule.*|TestBundleSkill.*|TestHostedBundleSkill.*|TestBundleMaskRecommendationDoesNotGrantSkillActivation|TestScriptHook.*|TestToolHookChain.*|TestToolkitDrain.*|TestToolkitRootAdmissionStoresSnapshotAndMarker|TestToolkitChildAdmissionInheritsParentCapture|TestToolkitRevocationStopsModelAndToolBoundaries|TestReportWorkflowAdmissionStoresToolkitSnapshot)$' -count=1 -timeout=180s
GOFLAGS=-tags=http2legacy go test ./internal/app -run '^(TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp|TestScriptHooksArmingGate)$' -count=1 -timeout=180s
GOFLAGS=-tags=http2legacy go test ./internal/modules/masks -count=1 -timeout=180s
```

All four commands passed. They cover lifecycle publication/recovery/retention, captured Skills, Rules, local Hook script decisions and failure modes, Mask selection/recommendations, admission drain/child/queue/revocation, report snapshots, App lifecycle and Mask APIs. No MCP server was started and no MCP transport call was made. The App omission test only checks that a build without the MCP owner advertises that capability as absent and rejects an MCP component preview; it does not exercise MCP execution or connection setup.

- `just ci` was not runnable: `just`, `pwsh` and `powershell` are not installed in this macOS environment. This remains a full-CI gate, not a Toolkit failure.
- Passed the non-MCP Go package sweep, while omitting `internal/mcphost`, `internal/runtime`, and tests whose names contain MCP or MemoryLoop:

  ```sh
  go list ./... | rg -v '^agent-vivy/internal/(mcphost|runtime)$' | \
    xargs env GOFLAGS=-tags=http2legacy go test -p 1 -timeout 35m \
      -skip '(?i)^Test(MemoryLoop.*|.*MCP.*)$'
  ```

  The complete sweep passed, including App, bundle contract/acquisition/lifecycle, Mask, Toolkit actions, storage, SDK and remaining non-MCP packages. The SDK-internal package took about 105 seconds. An initial sweep found that its legacy-surface scanner also matched the parser test's rejected-format literal; the parser test now constructs that test input from two fragments, preserving runtime rejection coverage while satisfying the physical-removal scan. The static removal test and parser rejection test both pass.
- Passed `GOFLAGS=-tags=http2legacy go vet ./...`.
- Passed headless compile: `go test -run '^$' -tags=http2legacy,vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui`. The first attempt supplied two `-tags` flags, so the later flag replaced `http2legacy` and failed in gRPC; the combined-tag invocation passed.
- Passed dependency-closure tests: `node --test scripts/dependency-closure.test.mjs` (2/2). The combined Laputa bootstrap test command could not run its PowerShell-backed cases: 2 passed and 9 failed to spawn `powershell.exe`, which is absent from this macOS host.
- Passed `GOFLAGS=-tags=http2legacy go run ./sdk verify plugins/vivy-toolkits-ui` after the UI source change.
- The full App package suite was rerun with `GOFLAGS=-tags=http2legacy go test ./internal/app -count=1 -timeout=10m`; it failed at the 10-minute Go timeout during `TestMemoryLoopAutomaticReflection`, with the stack in its `diva.cognitive.status` RPC and roughly 724,000 goroutines present. This is a suite-order/resource leak outside the Toolkit code. The same test passes in isolation in 10.19 seconds, and `TestMemoryLoopCrashC05EffectsDoneBeforeWatermark` passes in isolation in 57.886 seconds, including its observed 29.88-second real SQLite lease expiry. These results do not make the full App package gate pass.
- Broader responsive acceptance outside 390px, populated blocker states, and the remaining UI error/recovery matrix remain untested.
- An early exploratory sweep ran `internal/mcphost` tests using only in-memory fake sessions; no network or stdio MCP transport was opened. The completed sweep above excludes that package entirely.
- The local PostgreSQL container is a disposable test dependency and is stopped during cleanup.

## MCP discovery deadline and status-error follow-up

- Added an optional per-ToolWorld discovery deadline. The generated MCP world receives a 30-second budget; all other worlds continue to inherit ToolHost's existing 2-second default.
- Added an independent MCPHost discovery timeout covering lazy session/process startup, initialization and `tools/list`. MCPHost's ordinary operation timeout remains 8 seconds. Focused ToolHost and MCPHost package tests passed, including the per-world override, default timeout, and discovery-vs-operation timeout cases.
- Passed App composition test `TestGeneratedMCPWorldHasLongerDiscoveryBudgetThanOtherWorlds`, which delays MCP discovery beyond the normal 2-second world deadline.
- Passed a one-off live cold-cache stdio smoke with the official `@modelcontextprotocol/server-everything@2026.8.18`. The test used a fresh temporary npm cache and explicitly mapped `PATH` and `NPM_CONFIG_CACHE` into the stdio process. ToolHost discovery, the `mcp.everything.echo` call and cleanup passed in 10.89 seconds, inside the 30-second discovery budget. The temporary harness and cache were removed. This was a direct ToolHost discovery/call smoke; the separately recorded warm-cache live service test remains the evidence for approval and Journal ordering.
- A broader Runtime MCP selection initially exposed `TestMCPStatusErrorPreservesPathsAndSyntheticEnvironmentValues` requiring resolved credentials in display errors. Replaced that expectation with a regression asserting `EnvFrom` and `AuthEnv` values are redacted while paths remain and control/bidi characters are removed. The new test failed before the sanitizer fix and passed afterward; the full selected Runtime MCP/Toolkit test set then passed.
- No Windows smoke was run. No Eino/EinoExt change was needed: pinned EinoExt MCP `GetTools` provides schema conversion, while process startup and Vivy discovery deadlines are owned by MCPHost/ToolHost.

## Toolkit MCP pre-authorization and Service path

- Added `TestToolkitExecutableConsentCanBeGrantedBeforeInstallIsEnabled`. It first reproduced `toolkits: executable declaration unavailable`: `SetExecutableConsent` only searched the active View, while a newly installed Toolkit is intentionally disabled and absent from that View.
- Updated consent lookup to verify the exact Hook/MCP declaration from the currently installed immutable release, so an installed disabled candidate can receive consent before enable. The operation still checks the whole catalog identity and exact expected release/declaration digests.
- Added an in-transaction config-revision compare to `CommitBundleConsent`. This prevents binding/settings changes between declaration inspection and consent commit from persisting consent for a stale configuration; revocation continues to use its independent monotonic revision.
- Passed focused storage CAS test, the complete `internal/bundles` package, and all `internal/storage/...` package tests on this host:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/storage -run '^TestBundleConsentChecksConfigRevisionInCommitTransaction$' -count=1 -v -timeout=2m
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/storage/... -count=1 -timeout=10m
  ```

- Re-ran the complete storage tree against SQLite and a disposable local PostgreSQL 16 container after adding the consent config-revision CAS. The container used trust auth, bound only to `127.0.0.1:55432`, and was stopped after the run:

  ```sh
  VIVY_POSTGRES_TEST_DSN='postgres://postgres@127.0.0.1:55432/vivy_test?sslmode=disable' GOFLAGS=-tags=http2legacy go test -p 1 ./internal/storage/... -count=1 -timeout=10m
  ```

- Passed App test `TestToolkitMCPConsentCaptureGovernsRealServiceRun`: a local Streamable HTTP MCP endpoint is installed disabled, consented against its exact declaration, enabled and called through MCPBackend, MCPHost, ToolHost and Runtime Service. The test asserts durable consent digest/revision in the run snapshot, effect approval and Journal order, and one remote `tools/call`.

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^TestToolkitMCPConsentCaptureGovernsRealServiceRun$' -count=1 -v -timeout=2m
  ```

- Hardened consent updates to require the live Manager view to match durable catalog state and to reject updates while catalog recovery is required. The regression extends the disabled-install consent flow: after simulating recovery-required state, a revoke attempt returns `catalog_recovery_required` and leaves the durable allowed consent at the same revision.
- Passed the focused regression, complete `internal/bundles` package, selected Runtime MCP/Toolkit cases, and App Toolkit lifecycle/owner-omission/Hook/real-Service consent cases after this guard:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -run '^TestToolkitExecutableConsentCanBeGrantedBeforeInstallIsEnabled$' -count=1 -v -timeout=2m
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/runtime -run '^(Test.*MCP.*|Test.*BundleMCP.*|TestToolkit.*)$' -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^(TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp|TestScriptHooksArmingGate|TestToolkitMCPConsentCaptureGovernsRealServiceRun)$' -count=1 -timeout=180s
  ```

- Unreachable-server behavior and the broader active-run/CAS/schema/revocation matrix remain open; this closes the allowed-consent real-Service path only.

## MCP settings and Toolkit publication race

- Extended `TestToolkitMCPConsentCaptureGovernsRealServiceRun` to reproduce a Settings update that replaces a user MCP config after a Toolkit candidate is prepared but before it is published. Before the fix, the prepared owner restored the stale `user-old` entry; the regression failed with that stale config in the final server list.
- Added an optional `BeforeCommit` owner hook. `Manager.Publish` invokes it after drain/idle and before durable activation commit. The App MCP owner re-merges its candidate against the latest user MCP settings, so a confirmation-time settings change is reflected in the committed owner projection.
- Settings MCP updates now retry the shared exclusive Runtime drain gate while a Toolkit publication is active. Once admitted, they reload settings and pin the current Toolkit view, then replace MCP servers, refresh the generated tool registry and apply the engine reload before releasing the gate. This prevents the losing callback from silently dropping its update and keeps catalog, server and engine projections aligned during live runs.
- Added deterministic helper coverage for gate contention/retry, context cancellation and rebuilding from settings changed before gate acquisition. The publication-order test also asserts `BeforeCommit` runs after `WaitIdle` and before owner publication.
- Passed the following focused verification after the fix:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/runtime -run '^(Test.*MCP.*|Test.*BundleMCP.*|TestToolkit.*)$' -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^(TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp|TestScriptHooksArmingGate|TestToolkitMCPConsentCaptureGovernsRealServiceRun|TestBeginToolkitDrainRetriesWhileAnotherPublicationIsActive|TestReplaceMCPServersWithDrainBuildsLatestSettingsAfterAcquiringGate|TestBeginToolkitDrainStopsRetryWhenContextExpires)$' -count=1 -timeout=180s
  ```

- This closes the reproduced stale-settings overwrite and drain-contention retry cases. It does not yet prove every active-run/update interleaving, durable CAS race or the complete schema/revocation matrix.

## MCP discovery fences and unreachable endpoint

- A real App-side discovery validator now checks the current published Toolkit view against durable catalog state, the exact release/declaration, allowed consent digest/revision and live revocation fence. `bundleGuardedMCPWorld.Discover` checks before discovery, and `MCPHostSessionFactory.Open` checks again immediately before creating a transport for an active Toolkit MCP entry. Runtime run discovery and invocation still require the exact captured consent and live fence.
- Before the status fix, the unreachable endpoint test returned a connection-refused discovery error but `MCPBackend.ServerStatuses` stayed `inactive`, because MCPHost's transport projection omits Toolkit provenance and no longer matched the richer backend config. Status association now compares the exact MCPHost-owned transport projection while retaining the current backend status record. The endpoint then reports `unavailable` and the configured Toolkit declaration remains present.
- Added/updated tests: `TestToolkitMCPDiscoveryRequiresExactCapturedConsentAndLiveFence`, `TestToolkitMCPDiscoveryValidatorRunsBeforeTransportOpen`, and `TestToolkitMCPUnreachableServerKeepsConfiguredDeclaration`. Existing `TestToolkitMCPInvocationRequiresExactCapturedConsentAndLiveFence`, `TestSchemaChangeMarksBindingUnavailable`, `TestProjectedSchemaAndIdentityHashesAreDeterministic`, `TestUnsafeNamespacesFailClosedWithoutSanitizing` and `TestProtectedProjectedIDFailsClosed` provide the focused revoke, schema, identity, invalid-name and protected-name evidence.
- The real App Service consent test still passes with the discovery validator enabled. It installs the local Streamable HTTP server disabled, grants exact consent, enables it, captures the consent provenance, performs one approved remote call and verifies Journal ordering.
- Focused commands passed:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/runtime -run '^(TestToolkitMCP.*|TestSchemaChangeMarksBindingUnavailable|TestProjectedSchemaAndIdentityHashesAreDeterministic)$' -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/mcphost -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^TestToolkitMCPConsentCaptureGovernsRealServiceRun$' -count=1 -timeout=180s
  ```

- These close the targeted discovery, unreachable-status, exact-consent, revoke and schema-change cases. The larger concurrent active-run/catalog-CAS matrix remains open; there is no upstream MCP/Eino change required for the completed cases.

## App MCP publication and startup recovery

- Added `TestToolkitMCPPublicationRefreshesAppToolHostAndRecovery` around the real App, durable Toolkit Manager, MCPBackend, MCPHost and local Streamable HTTP server. It installs the MCP declaration disabled, grants exact consent, enables it, then proves the published App discovers the declared tool. After closing and reopening the same App database, startup recovery performs discovery again and restores the enabled Toolkit MCP config.
- The regression failed before the fix: live activation caused zero `tools/list` calls because replacing MCP server configs did not rebuild the App ToolHost/Engine.
- Added an optional post-publication owner hook. Manager calls it after swapping its current View and releasing its mutex, but before opening the drain gate. App rebuilds MCP ToolHost, updates ContextHost, resolves active tools and synchronously reloads Engine. If discovery of a new MCP config fails, the registry closes the MCP world fail-closed while preserving the declaration as unavailable.
- A post-commit hook failure returns `publication_recovery_required` and leaves the drain active. `TestToolkitPublishOrder` injects that failure and proves `Recover` reruns the owner projection before ending the drain.
- Passed focused checks:

  ```sh
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -run '^TestToolkitPublishOrder$' -count=1 -timeout=3m
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^(TestToolkitMCPPublicationRefreshesAppToolHostAndRecovery|TestToolkitMCPConsentCaptureGovernsRealServiceRun)$' -count=1 -timeout=180s
  GOFLAGS=-tags=http2legacy go test ./internal/bundles -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/runtime -run '^(TestToolkitMCP.*|TestSchemaChangeMarksBindingUnavailable|TestProjectedSchemaAndIdentityHashesAreDeterministic|TestMCPStatusErrorPreservesPathsAndSyntheticEnvironmentValues|TestMCPToolWorldProviderTracksLiveBackendReplacement)$' -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/mcphost -count=1 -timeout=5m
  GOFLAGS=-tags=http2legacy go test ./internal/app -run '^(TestToolkitLifecycle|TestToolkitOwnerOmissionIsReflectedByApp|TestScriptHooksArmingGate|TestToolkitMCPConsentCaptureGovernsRealServiceRun|TestToolkitMCPPublicationRefreshesAppToolHostAndRecovery|TestBeginToolkitDrainRetriesWhileAnotherPublicationIsActive|TestReplaceMCPServersWithDrainBuildsLatestSettingsAfterAcquiringGate|TestBeginToolkitDrainStopsRetryWhenContextExpires)$' -count=1 -timeout=180s
  ```

- An unrelated existing prompt snapshot test was red in isolation because it constructed a selected Mask without the host-generated Frame; the App always supplies this frame. Adding the required frame to the test fixture made `TestBuildBundleMaskSnapshotCarriesImmutableOriginAndRecommendations` pass without changing runtime behavior.
- The edited `internal/` source tree changes the source identity pinned by the SDK conformance artifact. Recomputed the internal source digest as `aea33bd278880e519754504b1d3fe20a7bf645a60d5210396580f4d4e3e57abc` and updated the five corresponding first-party records in `sdk/internal/assembly/conformance_results.json`; the stable-tree suite run below verifies this pin.

## Stable-tree repository verification

- Passed the repository Go suite on the stable final source tree, excluding the separately slow `TestMemoryLoop*` tests:

  ```sh
  GOFLAGS=-tags=http2legacy go test -p 1 -timeout 35m -skip '^TestMemoryLoop' ./...
  ```

  This includes `internal/app`, `internal/runtime`, all storage packages, SDK internals and conformance, TUI and UI packages. The exclusion is explicit; this result is not a claim that the `TestMemoryLoop*` suite passed as part of the repository sweep.
- Passed `GOFLAGS=-tags=http2legacy go vet ./...`.
- Recomputed the SDK internal source hash with `go run ./sdk/internal/cmd/source-hash internal eba914561ec043b74bd56e00236fc7a48df2e20dbc8956fca39cf033288a94fe`; it matches the five updated conformance pins: `aea33bd278880e519754504b1d3fe20a7bf645a60d5210396580f4d4e3e57abc`.
- `git diff --check` and `gofmt -l` over all changed Go files are clean.
- The full `just ci` recipe remains unrun because this host has no `just` or PowerShell, and the recipe invokes `powershell.exe`. Windows smoke is excluded by instruction. The broad crash/CAS/admission matrix and wider responsive UI states listed above also remain open; no upstream architecture change is needed for the fixes in this batch.

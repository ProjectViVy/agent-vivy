# P4 Host Lifecycle and Provider Capabilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This deliverable authorizes planning only.

**Goal:** Resolve H4-H7 and C7-C8 through owned composition, bounded native lifecycle, shared listeners and authoritative capability presentation.

**Architecture:** One owner serializes app composition and lifecycle edits. Reuse Wails election, one event installation Promise, and backend-declared provider capability. Keep caller timeout distinct from completed teardown.

**Tech Stack:** Go contexts/sync/errors, pinned Wails v3 beta.27, Vue/TypeScript/Vitest, existing VIVY SDK host and settings RPC.

**Spec:** [Issue #32 remediation design](../../specs/2026-10-09-issue32-remediation-design.md#p4-host-lifecycle-and-provider-capabilities).

## Global Constraints

- H5/H7 share `app.go`, `runtime_service.go`, and `lifecycle.go`: one write/review lane; no overlapping worktrees edit those files.
- VIVY H4 logger and C7 partial-owner cleanup land first; consume public SDK contracts without DIVA VIVY-internal imports.
- `hostv1.CloseBudget == 5 * time.Second`; never start a fresh five-second budget after producer join.
- `foundation.undefine.diva` remains the singleton identity; second launch never opens the host.
- Backend capability remains authoritative; no new frontend vendor/adapter support registry or browser provider HTTP.
- DIVA's actual pinned transport is Wails; preserve accepted authorized business behavior and existing typed errors.

## Review Focus

- A previous/default/closed logger cannot capture another profile or reopen a closed sink (P4.1a).
- Every later App construction failure closes the bundle exactly once and preserves both causes (P4.1b).
- Pump/speech/host share one deadline; early secondary handoff and failed-startup relaunch preserve ownership (P4.2).
- Concurrent subscription/close/install failure has one native listener, one detach and no unhandled rejection (P4.3).
- Deferred/unknown providers stay nonexecutable while supported unconfigured entries remain configurable (P4.4).

---

### Task P4.1a: Owned logger available before App composition (H4)

**Files:**
- Modify VIVY: `sdk/host/v1/host.go`, `internal/app/app.go`, `internal/logging/logging.go`.
- Test VIVY: `sdk/host/v1/host_test.go`, `internal/logging/logging_test.go`; create `internal/app/logger_ownership_test.go`.

**Interfaces:**
- Consume `logging.Setup(opts Options) (*slog.Logger, Effective, io.Closer, error)` and existing embedded.Options.AppOptions.
- Add `app.WithLogger(logger *slog.Logger) AppOption`; `appOptions.logger *slog.Logger` defaults to slog.Default only if no logger option was supplied.
- Preserve `app.NewWithAssembly(ctx context.Context, cfg config.Config, runtimeAssembly genassembly.RuntimeAssembly, opts ...AppOption) (*App, error)` and public host Open/Close signatures.
- Add permanent `dailyFile.closed bool`; Write after Close returns `io.ErrClosedPipe` and never reopens.

- [ ] **Step 1: Write failing logger regressions.** `TestAppUsesInjectedLoggerAtComposition` installs a previous default capture logger, passes a distinct WithLogger, and observes initialization/component logs only through the owned logger. `TestHostLoggerProfileReopen` opens A, closes it, opens B, then uses a retained A logger: no B event reaches A's files, and late A writes do not reopen a log file. `TestDailyFileCloseIsPermanent` checks errors.Is(io.ErrClosedPipe) and unchanged directory contents. `TestHostFailedOpenRestoresOwnedLogger` injects post-logging startup failure: sink is closed, host slot released, previous default restored only if it still equals the failed host's logger.
- [ ] **Step 2: Verify red.** Run `go test ./internal/app ./internal/logging ./sdk/host/v1 -run 'Test(AppUsesInjectedLoggerAtComposition|HostLoggerProfileReopen|DailyFileCloseIsPermanent|HostFailedOpenRestoresOwnedLogger)' -count=1`. Use existing actual sealed-host fixtures and built embedded UI. Expected: previous global capture or a reopened daily sink violates assertions.
- [ ] **Step 3: Implement explicit logger injection and ownership.** Process AppOptions before selecting its logger. Host passes WithLogger before embedded.Open; any necessary global compatibility setup occurs before runtime composition. Remember previous/owned logger under the host owner, restore only if slog.Default still equals the owned pointer, and finish sink cleanup before releasing the slot. A newer/non-host default must not be overwritten. Close dailyFile permanently and idempotently; return the closed-pipe error on late writes instead of opening another day's file.
- [ ] **Step 4: Verify green.** Re-run the focused command, then `go test -race ./internal/logging ./sdk/host/v1 -count=1` and relevant App composition tests. Check actual profile directories from open -> close -> reopen, not only logger pointer identity. Record shutdown-timeout ownership separately from a fully closed host.
- [ ] **Step 5: Commit.** Stage only the listed logger source/tests and the phase log. Commit `fix(host): inject the owned logger before composition`.

### Task P4.1b: Cognitive bundle cleanup on every failed composition (C7)

**Files:**
- Modify VIVY: `internal/app/app.go`.
- Test VIVY: `internal/app/embedded_cognitive_lifecycle_test.go`, `internal/app/assembly_cognitive_test.go`.

**Interfaces:**
- Consume existing `cognitivecontract.Bundle.Close() error` and the single App construction path.
- Preserve New/NewWithAssembly signatures. A local deferred cleanup owns the opened bundle until successful transfer to `App.cognitive`; no new owner API or generic cleanup framework is added.

- [ ] **Step 1: Write the fault matrix.** `TestCognitiveBundleClosedOnEveryCompositionFailure` uses the existing factory injection seam and faults later primary-admission, binding resolution, Service/control attachment and assembly validation stages. Each failure closes the bundle exactly once, preserves original error, and permits a fresh owner on retry. `TestCognitiveBundleFailurePreservesCleanupError` requires errors.Is for initialization and Close sentinels. `TestCognitiveBundleOwnershipTransfersOnce` succeeds, asserts no construction close, then App.Close closes once.
- [ ] **Step 2: Verify red.** Run `go test ./internal/app -run '^TestCognitiveBundle(ClosedOnEveryCompositionFailure|FailurePreservesCleanupError|OwnershipTransfersOnce)$' -count=1`. Expected: at least one current later return leaks or drops cleanup error. Use fresh temporary Garden/storage owners; never disturb tenant paths.
- [ ] **Step 3: Implement immediate deferred cleanup.** Register bundle failure cleanup as soon as cognitiveBundleForAssembly succeeds. Make every later return use that guard, removing selected branch-specific duplicate closes. Join cleanup failure with the initialization error. Disable the guard only after the final successful App owns the bundle; retain reverse teardown ownership ordering. Keep the change local to composition rather than adding a resource registry.
- [ ] **Step 4: Verify green and reopen.** Run the focused command and `go test ./internal/app -count=1`; on each injected failure retry construction in the same isolated test root and assert no leaked handle/owner. Record any independent existing first-store initialization failure separately; do not hide it by changing this scope.
- [ ] **Step 5: Commit.** Commit `fix(app): release cognitive ownership on failed composition` with exact exercised failure stages.

### Task P4.2: Coordinate bounded teardown and early singleton handoff (H5/H7)

**Files:** Modify `internal/desktop/app.go`, `internal/desktop/runtime_service.go`, `internal/desktop/lifecycle.go`, `internal/speech/service.go`, `cmd/diva/main.go`; tests `internal/desktop/runtime_service_test.go`, `internal/desktop/lifecycle_test.go`, `internal/speech/service_test.go`; create `internal/desktop/app_test.go` for handoff/coordinator tests and native subprocess acceptance fixtures under `scripts/ci/fixtures/desktop-lifecycle/` if needed.

**Interfaces:** Preserve `Compose(context.Context, Config, *log.Logger) (*Desktop, error)`, `(*Desktop).Run() error`, `(*RuntimeService).ServiceShutdown() error`, and `NewRuntimeService(hostAPI, func(string, any)) *RuntimeService`. Proposed internals: `(*RuntimeService).stopPump(ctx context.Context) error`, `(*RuntimeService).shutdown(ctx context.Context) error`, `onShutdown func(context.Context) error`, protected `pumpErr error` and one `shutdownOnce/shutdownDone/shutdownErr`; `(*speech.Service).ShutdownContext(ctx context.Context) ShutdownReport` (existing `Shutdown(grace time.Duration)` delegates to it for compatibility). Lifecycle additions `requestReopen(show func())`, `markReady(show func())`, `failStartup()`, and `recordTeardown(error)` accumulating errors. Keep `Desktop.emit func(string, any)`; recover pump-emitter panics as recorded producer failures rather than inventing a new error-returning public emitter contract.

- [ ] **Step 1: Write failing lifecycle tests.** `TestShutdownBudgetStartsBeforePumpJoin` supplies a gated `Next` ignoring cancellation, invokes `shutdown` with a short injected deadline, asserts `errors.Is(err, context.DeadlineExceeded)`, and proves the observer returns before releasing the gate. `TestShutdownSharesDeadlineWithSpeechAndHost` captures both deadlines and requires equality with the original deadline and decreasing remaining time. `TestShutdownPreservesSpeechAndHostErrors`, `TestLifecycleSuccessfulRecordDoesNotEraseError`, and `TestShutdownIdempotentOutcome` require `errors.Is` for both sentinels, host close exactly once, and the same retained error on repeated calls. `TestPumpCallbackFailureRetained` recovers injected emitter panic and confirms the failure survives successful host close. `TestShutdownDoesNotReportCleanOrReopenWhileDrainPending` checks revoked native capability, no emitted shown event, and lease ownership until safe background completion. `TestSecondLaunchBeforeWindowReadyQueuesOneReopen`, `TestStartupFailureClearsQueuedReopen`, and `TestSecondLaunchDuringShutdownDoesNotReopen` pin one queued show/focus and zero show after failure/admission close.

```go
if !speechDeadline.Equal(hostDeadline) || !hostDeadline.Equal(deadline) { t.Fatal("budget reset") }
if !errors.Is(err, speechErr) || !errors.Is(err, hostErr) { t.Fatal(err) }
if closeCalls != 1 || shownCalls != 0 || lifecycle.admissionOpen.Load() { t.Fatal("unsafe shutdown") }
```

- [ ] **Step 2: Prove failures under the canonical consumer closure.** Since the wrapper stages `git ls-files`, first `git add -N internal/desktop/app_test.go` so the newly created regression file enters the test snapshot without committing implementation. Run `python scripts/build-desktop.py --mode test --test-packages ./internal/desktop ./internal/speech --test-run 'Test(Shutdown|Lifecycle|PumpCallback|SecondLaunch|StartupFailure)'`; expect current unbounded join, fresh deadline, overwritten speech error, or lost early handoff. Use barrier/channel coordination, not timing sleeps, to demonstrate the interleavings.
- [ ] **Step 3: Implement one shared-deadline coordinator.** Establish the deadline at `ServiceShutdown` entry, close admission/revoke capability first, start one owned teardown worker, and bound each caller with that shared deadline. `stopPump(ctx)` records context/pump failure; speech `ShutdownContext` and host `Close(ctx)` consume the same context. Join all errors and retain outcomes; the worker owns resources after caller expiry. On app/run plus teardown failure, return `errors.Join` so neither is lost. Keep speech remaining counts truthful and never reopen after admission closure. Partial `Compose` failures join bounded cleanup errors with their original error.
- [ ] **Step 4: Implement election before runtime composition in the same lane.** Construct the Wails app before `hostv1.Open` with hooks closing over the desktop lifecycle; beta.27 elects in `application.New` and exits the secondary. Register `RuntimeService` via `App.RegisterService` after host/speech success and before `Run`; bind the window capability once. Startup handoff queues one request until a window exists, then fulfills show/focus once. Failure clears the queue and closes partial host ownership; `cmd/diva/main.go` exits after reporting the error so the OS releases the election lock. Do not call pre-Run `App.Quit` as cleanup proof.
- [ ] **Step 5: Verify focused tests and native process behavior.** Re-run Step 2, then canonical full host race tests and speech tests. On both native candidates, hide primary then launch secondary using a different valid config path: primary focuses once; secondary exits and creates no Journal/profile because its host never opens. Launch a deliberately failing primary, let it exit, then launch corrected config: singleton and organism ownership can be acquired. Verify crash restart respects the existing organism lease/TTL contract; graceful quit permits immediate relaunch. Record blocked-pump and pending speech ownership/error observations separately from native clean-quit success.
- [ ] **Step 6: Commit the shared lifecycle patch.** Stage only listed lifecycle/speech/command/tests and `git commit -m "fix(desktop): coordinate startup handoff and bounded shutdown"`.

### Task P4.3: Share native event-listener installation (H6)

**Files:** Modify `agent-diva-gui/src/api/vivy/transport.ts`; create `agent-diva-gui/src/api/vivy/transport.test.ts`; consume unchanged `agent-diva-gui/src/platform/desktop-host.ts` `onVivyEvent` contract.

**Interfaces:** Preserve `createWailsTransport(): VivyTransport`, `onEvent(handler: (event: WireEvent) => void): Promise<() => void>`, and `close(): void`. Internal `ensureListener(): Promise<void>` shares one pending installation Promise. `onVivyEvent(handler)` resolves `Promise<() => void>`; successful native unlisten is consumed exactly once.

- [ ] **Step 1: Write failing transport tests with a manually resolved/rejected installation Promise.** Tests named `shares one pending native listener across concurrent subscribers`, `delivers each event once to each subscribed handler`, `close before install detaches exactly once`, `close after install is idempotent`, `post-close subscription installs nothing`, and `failed shared installation rejects consistently and a later subscription retries`. Assert two simultaneous `onEvent` calls cause native install count `1`, one native event invokes each handler once, each unsubscriber removes only its own handler, pending close causes unlisten count `1` after resolution, repeated close remains `1`, and failed handlers do not survive retry. Await every rejected Promise; capture `unhandledrejection` and require zero events.

```ts
expect(onVivyEvent).toHaveBeenCalledTimes(1)
expect(firstHandler).toHaveBeenCalledTimes(1)
expect(secondHandler).toHaveBeenCalledTimes(1)
expect(nativeUnlisten).toHaveBeenCalledTimes(1)
expect(unhandledRejections).toHaveLength(0)
```

- [ ] **Step 2: Run `pnpm --dir agent-diva-gui test src/api/vivy/transport.test.ts` to prove failure.** Expected current count `2` for concurrent subscription and installation failure not propagated to awaiting subscribers.
- [ ] **Step 3: Implement shared installation state in `createWailsTransport`.** Await `ensureListener` before resolving subscriber registration; clear the shared failed attempt and remove its unsuccessful handlers; do not retain a rejected Promise. Mark close before clearing state, detach eventual installation exactly once, and preserve post-close no-op semantics. A subscriber retry is explicit; no hidden infinite retry or host teardown is added.
- [ ] **Step 4: Re-run focused transport/client tests then frontend build.** `pnpm --dir agent-diva-gui test src/api/vivy/transport.test.ts src/api/vivy/client.test.ts` and `pnpm --dir agent-diva-gui build`; require no duplicate callbacks or unhandled rejection in race cases.
- [ ] **Step 5: Commit `transport.ts` and `transport.test.ts`.** `git commit -m "fix(transport): share pending native event subscription"`.

### Task P4.4: Project authoritative provider capability and guard execution actions (C8)

**Files:** Modify `agent-diva-gui/src/api/settings.ts`, `agent-diva-gui/src/api/settings.test.ts`, `agent-diva-gui/src/components/settings/ProvidersSettings.vue`, `agent-diva-gui/src/components/settings/ProviderListItem.vue`, `agent-diva-gui/src/components/settings/ProviderCard.vue`, `agent-diva-gui/src/components/settings/ProviderWizardModal.vue`, and `agent-diva-gui/src/components/settings/ProvidersSettings.test.ts`; create `agent-diva-gui/src/components/settings/ProviderWizardModal.test.ts`. Add `providers.capabilityUnavailable` to `agent-diva-gui/src/locales/en.ts` and `agent-diva-gui/src/locales/zh.ts` with localized wrapper text and `{state}` interpolation; render the backend capability state verbatim.

**Interfaces:** Existing backend types are `VivyCatalogEndpoint {adapter,base_url,default_model,models,executable,state}`, `VivyProviderProfileStatus {id,adapter_family,endpoint_class,model_ids,state}`, and `VivyProvidersResult.profiles`. Proposed `ProviderCapability { executable: boolean; state: string }`; `resolveProviderCapability(adapter: string, endpoint: VivyCatalogEndpoint | undefined, profiles: readonly VivyProviderProfileStatus[]): ProviderCapability`. Make `ProviderSpec.executable` required and add `capability_state: string`. Existing public settings adapter signatures remain unchanged; guard `saveActiveProvider(selection: ActiveProviderSelection): Promise<void>`, `getProviderModels(...): Promise<ProviderModelCatalog>`, and `testProviderModel(...): Promise<ProviderModelTestResult>` before any execution-related upsert/refresh/select. Keep delete/read paths available.

**Eino capability check:** The existing VIVY `internal/provider/adapters.go` `Adapters()`/`Capabilities()` derive supported completions/messages and deferred responses. Existing quarantined constructors are `github.com/cloudwego/eino-ext/components/model/openai.NewChatModel` at `v0.1.13` and `github.com/cloudwego/eino-ext/components/model/claude.NewChatModel` at `v0.1.25`, with Eino `v0.9.13`. C8 consumes their backend projection; it adds no adapter, provider HTTP path, or alternate capability owner.

- [ ] **Step 1: Write failing adapter and component tests.** `custom deferred provider is never executable despite a stored key` supplies an unmatched `openai-responses` entry plus a `DEFERRED-INDEFINITE` profile; assert executable false, exact state retained, provider ready false, and doctor ready false when it is active. `unknown profile fails closed` supplies an unmatched family with no profile and asserts false/`UNAVAILABLE`. `supported unconfigured provider remains configurable` supplies `COMPILED` or `UNCONFIGURED` profile; assert executable true, configured/ready false until credential configured, and a configuration save still runs. `deferred direct select test and refresh have zero RPC side effects` asserts no `providerUpsert`, `providerRefresh`, or `modelSelect` after each blocked operation; test method returns `ok:false` with exact backend state in its message. `deferred entries remain readable and deletable` verifies load and one explicit delete call. Component tests `disables execution actions and shows deferred reason`, `wizard cannot test or complete a deferred selection`, and `supported configured provider still selects once` assert disabled model-select/test/refresh controls, no saveConfigAction call on programmatic clicks, preserved browsing/deletion, and working supported selection.

```ts
expect(spec.executable).toBe(false)
expect(spec.capability_state).toBe('DEFERRED-INDEFINITE')
expect(providerUpsert).not.toHaveBeenCalled()
expect(providerRefresh).not.toHaveBeenCalled()
expect(modelSelect).not.toHaveBeenCalled()
expect(deleteCall).toHaveBeenCalledTimes(1)
```

- [ ] **Step 2: Run focused adapter/component tests to prove failure.** `pnpm --dir agent-diva-gui test src/api/settings.test.ts src/components/settings/ProvidersSettings.test.ts src/components/settings/ProviderWizardModal.test.ts`; expected current unmatched entry hardcodes `true` and deferred actions invoke RPC.
- [ ] **Step 3: Implement capability derivation and adapter guards.** Match normalized adapter to backend profiles; `COMPILED`, `UNCONFIGURED`, and `READY` are supported capability states; `DEFERRED-INDEFINITE`, `UNAVAILABLE`, unknown states, and absent profiles deny execution. A catalog endpoint `executable:false` denies even with a supported family; retain its state as reason. Use authoritative `prov.profiles` throughout custom projection and `createCustomProvider` result refresh; remove each current unconditional custom `true`. Derive doctor readiness from the active supported profile/configuration, never any unrelated READY profile. Configuration for supported unconfigured entries remains allowed and secrets remain write-only.
- [ ] **Step 4: Implement UI guards using `ProviderSpec` capability.** Show deferred/unavailable reason in provider views; permit row browsing and deletion, but disable model selection/test/refresh/wizard completion and guard the corresponding handlers. Preserve readOnly/frozen restrictions in addition to capability. Do not hide providers, introduce a duplicate backend catalog, or claim all executable adapter families support model-list refresh.
- [ ] **Step 5: Verify focused suites, full frontend tests and build.** Re-run Step 2; `pnpm --dir agent-diva-gui test` and `pnpm --dir agent-diva-gui build`; inspect real backend `settings/providers` → provider list → select/test/refresh for one supported unconfigured, one configured, and one deferred profile on the P7 candidate. Confirm deferred state survives reload without exposing credentials.
- [ ] **Step 6: Commit listed adapter/components/tests/required locale entries.** `git commit -m "fix(settings): honor backend provider capabilities"`.

**P4 exit gate:** All lifecycle race/barrier tests and frontend capability/listener tests pass. Native second-launch, clean quit, and configured provider observations still require P7's exact artifacts.

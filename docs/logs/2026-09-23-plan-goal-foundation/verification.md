# PG-0 implementation and verification record

Date: 2026-09-23

Branch: `feat/issue-47-goal-plan-foundation`

## Implemented contracts

- Plan guidance is applied to the next model request. `submit_plan` durably records the proposal, then suspends the originating Eino run at its exact resume target. A human decision resumes that target once; replaying the same decision does not resume twice.
- The Plan suspension stores its originating run, tool call, resume target and same-batch sibling call IDs. Sibling effects remain fenced after restart and after the review decision. Restart recovery rebuilds only a readable, identity-matched pending review. Cancellation and leaving Plan close the pending run without silently resuming it.
- History messages and Work events use durable sequence anchors so same-timestamp ordering is deterministic across SQLite and PostgreSQL. Fork and rewind preserve the source history and do not refund spent Goal rounds or copy approval authority.
- Human turns are synchronous, process-local admission waiters. Intent registers before waiting on the per-session startup gate; no durable ticket or RunID exists before commit. A committed primary run returns the existing busy conflict. Cancellation before commit writes neither message nor run; after commit, normal detached-run lifetime applies.
- Shared Plan text, Goal objective and round limits are owned by `internal/domain` and consumed by tools, runtime and RPC validation.

## Scenario evidence

| Scenario | Command | Result |
| --- | --- | --- |
| Plan review fences same-batch calls, persists exact Eino resume identity, recovers in a second Service instance, resumes once and accepts replay idempotently | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime -run '^TestPlanGoalProbeSubmissionFencesLaterToolInSameBatch$' -count=1 -timeout 45s` | Passed |
| Human intent registers before the session gate; cancellation before admission leaves no run or message | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime -run 'TestHumanAdmission|TestCancelledHumanAdmission' -count=1 -timeout 30s` | Passed |
| Domain lifecycle, runtime, RPC, SQLite, PostgreSQL adapter and migration regressions | `PATH=/tmp/go/bin:$PATH go test ./internal/runtime ./internal/domain ./internal/storage/sqlite ./internal/storage/postgres ./internal/storage/migrations ./internal/rpc -count=1 -timeout 5m` | Passed |
| History ordering, same-timestamp anchors, fork/rewind and spent-round behavior | Runtime rewind probes and storage conformance tests in the full suite | Passed |
| SDK provider conformance snapshot matches the source tree | `PATH=/tmp/go/bin:$PATH go test -p 4 -timeout 20m ./...` | Passed after updating the five `internal/` digests |

## Repository gates

| Gate | Result |
| --- | --- |
| `gofmt` on changed Go files and tracked-file format scan | Passed |
| `PATH=/tmp/go/bin:$PATH go vet ./...` | Passed |
| `PATH=/tmp/go/bin:$PATH go test -run '^$' -tags vivy_headless ./cmd/vivy ./cmd/vivy-code ./ui` | Passed |
| Full Go suite: `PATH=/tmp/go/bin:$PATH go test -p 4 -timeout 20m ./...` | Passed |
| UI typecheck, 48 test files / 392 tests, production build, catalog completeness and cross-face i18n checks | Passed; Vite reported the >500 KB main-chunk advisory |
| Independent `plugins/` and `faces/` module vet/tests | Passed |
| Live PostgreSQL test | Not run: `VIVY_POSTGRES_TEST_DSN` is unset |
| Browser E2E and real coding walkthrough | Not run in this continuation; no UI source behavior changed |

The earlier container did not have `just`; its `ci` gates were run in recipe order as direct commands. Those results are historical and do not establish a green gate for the later Windows worktree. Live PostgreSQL, browser integration and a real coding walkthrough remain as broader acceptance gates.

## Isolated Windows gate repair (2026-09-23)

Worktree: `codex/issue-47-pg0-gates` at baseline `332e3adc3c658f982c32047b0d2389cd7ae7462b`. Go is `go1.26.4 windows/amd64` from `C:\Program Files\Go\bin`; the commands below put that directory on `PATH`. No runtime behavior was changed. `gofmt -l internal/tools/work_control.go` identified one extra blank line, and `gofmt -w internal/tools/work_control.go` removed it.

| Command | Actual result |
| --- | --- |
| `go run ./sdk/internal/cmd/source-hash internal ''` | Produced `6ba01d9588e44f140e6f13e35ded8343b7465eb3430c1ece134e81c2350ce452`; this is the canonical `internal/` source-tree digest. |
| `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` before refreshing evidence | Failed as expected: computed `6ba01d9588e44f140e6f13e35ded8343b7465eb3430c1ece134e81c2350ce452`, checked-in `7c56dd9930bfb5ee54eee85512f0334597ade16e6aa63bfde80a9e3cdce47818`. |
| Four plugin pressure commands | Exact unescaped commands and the tests actually selected are recorded below. All four final verbose runs passed. The early attempts on the fresh worktree lacked UI dependencies; those attempts are not counted as passing test evidence. |
| `go test ./sdk/internal/conformance -run TestCheckedInProviderConformanceMatchesExecutedSuites -count=1` after refreshing exactly five `internal/` `sourceSha256` values | Passed in 183.230s; the executed provider and host suites match the checked-in result bundle. |
| `just ci` | Exit 1. `fmt-check`, UI typecheck, 49 test files / 400 tests, UI build, i18n checks, and `go vet ./...` passed. Full Go tests failed in untouched `internal/modules/masks` and in `sdk/internal/conformance`. The latter Go package began before the digest refresh and compared the newly computed `6ba01d…` with its already embedded old `7c56dd…`; the separate post-refresh reproduction command above passed. |
| `git diff --check` | Exit 0. |

### Executed plugin pressure matrix

These commands were rerun with `-count=1 -v` after UI dependencies and `ui/dist` were present. The alternation is a plain `|` inside PowerShell single quotes; the earlier log's displayed `\|` was not a reproducible test selector.

```powershell
go test ./sdk/internal/conformance -run 'TestGeneration(FailureMatrixEvidence|RollbackRestoresCatalogAndLocaleIdentity)' -count=1 -v
```

Exit 0, `ok agent-vivy/sdk/internal/conformance 0.336s`. `TestGenerationFailureMatrixEvidence` and `TestGenerationRollbackRestoresCatalogAndLocaleIdentity` both ran and passed.

```powershell
go test ./sdk/internal -run 'Test(GenerationFailureMatrixExecutesEveryCase|MinimalArtifactPhysicallyOmitsOptionalModules)' -count=1 -v
```

Exit 0, `ok agent-vivy/sdk/internal 159.953s`. `TestGenerationFailureMatrixExecutesEveryCase` ran its 25 named cases, including `startup-rollback` and `valid-deterministic-rebuild`; every case passed. `TestMinimalArtifactPhysicallyOmitsOptionalModules` ran and passed.

```powershell
go test ./sdk/internal/assembly -run 'Test(CompilePluginV1GraphFixtures|StartFailureRollsBackEveryConstructedOwner)' -count=1 -v
```

Exit 0, `ok agent-vivy/sdk/internal/assembly 0.427s`. `TestCompilePluginV1GraphFixtures` ran 13 graph cases and all passed. The named `TestStartFailureRollsBackEveryConstructedOwner` is inside generated fixture source, not a registered test of this package, so that selector did not run it directly. The owning integration test was run separately:

```powershell
go test ./sdk/internal/assembly -run '^TestGeneratedBinderCompilesAndRollsBackLifecycle$' -count=1 -v
```

Exit 0, `ok agent-vivy/sdk/internal/assembly 2.843s`; `TestGeneratedBinderCompilesAndRollsBackLifecycle` ran and passed.

```powershell
go test ./internal/toolhost -run 'TestMiddleware(TimeoutFailsClosed|PanicAndInvalidDecisionFailClosed)' -count=1 -v
```

Exit 0, `ok agent-vivy/internal/toolhost 0.041s`. `TestMiddlewareTimeoutFailsClosed` and `TestMiddlewarePanicAndInvalidDecisionFailClosed` both ran and passed.

The six mask failures were `TestMaskCatalogEmbedsCanonicalBuiltIns`, `TestMaskCatalogResolverAndOwnedReturns`, `TestMaskCatalogDuplicateIDsFailClosed`, `TestServiceListMergesBuiltinsAndCustoms`, `TestServiceCaptureResolvesBuiltinsAndOwnsCopies`, and `TestServiceSelectionUsesResolvedCaptureAndRejectsReservedLookup`. Each reported `mask catalog: validate builtin/programmer: definition body is not normalized` (the duplicate-ID assertion instead reported that error where it expected a duplicate-ID error). This task made no mask changes. The first full CI run is not a pass; no later complete CI result is claimed. A second `just ci` was started after the digest refresh, then canceled before completing on supervisor instruction because the focused reproduction had passed and the unrelated mask failure was already recorded.

`VIVY_POSTGRES_TEST_DSN` was unset in this worktree. The `internal/storage/postgres` package's no-service test result is not live PostgreSQL acceptance evidence. Browser E2E and a real coding walkthrough were not run in this gate repair.

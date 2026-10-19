# Verification — provider agentic migration

All commands ran on Windows in the migration worktree
`agent-vivy-agentic-p2` on branch `feat/provider-agentic-p2`, with Go 1.26.4,
`GOPROXY=https://goproxy.cn,direct` and `GOFLAGS=-mod=mod`.

## Required gates

| Command | Result |
| --- | --- |
| `go build ./internal/...` | exit 0 (`ui/dist/.keep` present; the file and directory were deleted before committing) |
| `go vet ./internal/provider/ ./internal/modelhost/ ./internal/runtime/ ./internal/rpc/ ./internal/app/` | exit 0 |
| `go test -count=1 ./internal/provider/ ./internal/modelhost/ ./internal/runtime/ ./internal/rpc/ ./internal/app/` | `provider` ok, `modelhost` ok, `runtime` ok, `rpc` ok, `app` fails three pre-existing tests (below) |
| `gofmt -l` on changed Go files | empty |
| Residue: `rg -n 'ToolCallingChatModel\|DEFERRED-INDEFINITE\|CapabilityDeferredIndefinite' internal/runtime internal/provider` | no hits outside `third_party/` and the (unmodified) spike lane |

`go test` wall times on this host: runtime ≈ 220 s, rpc ≈ 99 s, app ≈ 68 s,
provider ≈ 8 s, modelhost ≈ 0.1 s.

### Pre-existing `internal/app` failures

The full `./internal/app` suite fails three tests on this host **and on the
untouched baseline** — they pass in isolation, so the cause is Windows
SQLite/temp-dir isolation, not the migration:

- `TestPlanGoalIntegratedRoundLimitBlocksDurably`
- `TestAppShutdownStopsGoalAdmissionBeforeChannelDrain`
- `TestAppShutdownBounded`

Evidence: a scratch worktree at the base commit `40723f27` produced exactly the
same three failures (`go test -count=1 ./internal/app/`), and each one passes
alone in both worktrees:

```text
# baseline worktree, full suite
--- FAIL: TestPlanGoalIntegratedRoundLimitBlocksDurably (1.25s)
--- FAIL: TestAppShutdownStopsGoalAdmissionBeforeChannelDrain (2.88s)
--- FAIL: TestAppShutdownBounded (2.90s)
```

## Additional checks

| Check | Result |
| --- | --- |
| `go vet ./internal/...` (whole tree, including test files of every package) | exit 0 |
| `go build ./...` non-UI packages after `go mod tidy` | exit 0 |
| `go test -count=1 ./internal/modules/... ./internal/app/settings/` | ok |
| UI `vitest run src/components/settings/` | 11 files, 101 tests, all passing |
| UI `vitest run` (whole suite) | 69 files / 517 tests pass; 3 suites fail on missing generated plugin sources (`src/generated/assembly.ts` → `./ui/vivy-*/src/index.tsx`), identical on the baseline |
| UI `tsc --noEmit` | 7 errors, all `src/generated/assembly.ts` missing-plugin-source resolution — identical count and text on the baseline |

The three UI suites (`src/plugins/conformance.test.tsx`,
`src/plugins/presentation-host.test.tsx`,
`src/components/chat/ConversationSidebar.test.tsx`) and the seven `tsc` errors
are the same on the untouched base commit; the FACE contract tests
(`src/lib/ui-build-provenance.test.ts`, `src/lib/ui-sdk-face-compat.test.ts`)
were checked specifically because they assert that the UI surface still
satisfies `@vivy/ui-sdk`: they fail until `sdk/ui/src/module.ts` drops the
retired adapter and the capability state, and pass afterwards.

## Targeted tests added or rewritten

- `internal/runtime/checkpoint_test.go`:
  `TestVersionedCheckpointStoreRefusesTheOtherGeneration` writes a synthetic
  pre-migration envelope (no `generation` field, valid checksum) and asserts the
  sentinel, then round-trips a current-generation payload.
- `internal/runtime/recovery_engine_version_test.go`:
  `TestRecoverAcrossModelGenerationCancelsPendingApproval` downgrades a paused
  run's real checkpoint envelope, restarts, and asserts the run is closed with
  the engine-changed message, the approval is cancelled, and a second recovery
  pass is a no-op.
- `internal/runtime/model_stream_observer_test.go`: request-time `model.WithTools`
  reaches the observed `Begin` input (replaces the removed `WithTools` binding
  test).
- `internal/runtime/observability_test.go`, `nudge_*_test.go`,
  `agentic_testhelpers_test.go`: fixtures now build reasoning/tool-call/tool-
  result **blocks**, including `StreamingMeta` on streaming tool-call fragments.
- `internal/provider/{catalog,reconcile,adapters,modelhost_routing,promise,
  resolving_thinking,claude}_test.go`: the retired family fails closed, the
  sealed set is two families, and outbound bodies carry the thinking, sampling
  and cache fields.
- `internal/rpc/{control,project_context}_test.go`: catalog rows no longer carry
  `executable`/`state`; the history snapshot-metadata assertion targets the user
  projection.
- `internal/app/{headless,continuity_e2e,assembly_governance_e2e,model}_test.go`
  and `internal/modules/{model,loop}` tests: agentic scripts and the
  adapter-family constructor.

## Deliberately not run

`just ci`, Docker, Postgres, real-provider smoke and the browser e2e suite were
not run: the task assigned them to the Lead's final gate.

# Verification

Go 1.26.4 was installed by the coordinating lane at
`/workspace/agent-vivy/work/toolchain/go/bin`; commands below prepend that
location to `PATH`. Dependency compilation used the shared module cache and
sibling Laputa checkout prepared by the coordinating lane.

## RED / GREEN

During the first runtime package dependency build, an ignored scratch package
under `work/observer-regression` copied the unchanged production observer file
and extracted the existing recording/fake-model helpers plus the new regression
tests. `go test ./work/observer-regression -count=1` failed in all seven settlement
scenarios: Generate completion returned nil error; streaming completion returned
EOF; provider and Chunk failures lost the End cause. The downstream-close
cleanup guard already passed, preserving existing behavior.

After the minimal production fix, the same scratch test command passed
(`0.106s`), and `go test -race ./work/observer-regression -count=10` passed
(`1.239s`). The scratch sources are not part of the deliverable.

## Runtime package

- `go test ./internal/runtime -run 'TestObserver(GeneratePropagatesSettlementError|StreamSetupJoinsSettlementError|StreamPropagatesSettlementErrorBeforeEOF)$' -count=1`
  passed (`0.108s`).
- `go test ./internal/runtime -run 'TestObserver|TestObservedCall|TestUsageAccumulator' -count=1`
  passed (`2.147s`; final pre-commit run `0.516s`). This covers existing lifecycle, Begin admission, bound-tool
  scope, read/Chunk failures, cancellation, backpressure, live journal ordering,
  quota settlement, budget replay parity, source attribution, and usage tests.
- `go test -race ./internal/runtime -run 'TestObserver(GeneratePropagatesSettlementError|StreamSetupJoinsSettlementError|StreamPropagatesSettlementErrorBeforeEOF|SettlementFailureAfterDownstreamCloseReleasesUpstream)$' -count=10`
  passed (`1.130s`), including all ten settlement/closure scenarios per run.
- `go vet ./internal/runtime` passed (exit 0).
- `gofmt` applied to both changed Go files; `git diff --check` passed.

## Product gate and limitations

The initial `just ci` attempt exited 127 because `just` was not installed yet.
The coordinating lane subsequently installed the required tools and owns the
integrated product CI run after all isolated lanes land; it explicitly directed
this lane not to repeat full product CI. This lane's package checks are not a
claim that integrated `just ci` passed.

No live provider credentials, tenant journal, release deployment, or browser
interaction was required for this persistence-error fix. The regression uses
real observing wrappers and Eino pipes with deterministic failure injection at
the existing observer seam. No release artifact was produced.

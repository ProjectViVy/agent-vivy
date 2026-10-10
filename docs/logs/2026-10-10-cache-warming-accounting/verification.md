# Verification

Toolchain: actual Go 1.26.4 at `/workspace/agent-vivy/work/toolchain/go/bin`; deterministic tests use temporary SQLite Journals and scripted models. No tenant Journal or provider credentials are used.

## RED evidence

- `go test ./internal/config -run TestCacheWarmingRequiresExplicitOptIn -count=1`: failed because the default was `streaming`.
- `go test ./internal/storage -run TestUsageProjectionPreservesMaintenanceCacheWrite -count=1`: failed because cache-write evidence disappeared from projected usage.
- Baseline `go test ./internal/rpc -run TestRowCostCacheWriteWithoutDeclaredRateIsUnknown -count=1`: failed because undeclared creation pricing was silently treated as ordinary input, reporting `$0.0045` as known.
- Baseline `go test ./internal/runtime -run TestCacheWarmStreamingFiresAfterSettle -count=1`: failed with two real requests and zero maintenance requests despite two warm calls.
- `TestTrajectoryMaintenanceDoesNotInterruptChat`: failed because a maintenance request interrupted the main request and, separately, erased the transcript's pending deltas.
- `TestCacheWarmMandatoryPersistenceFailureReachesOwningEnd`: failed for maintenance request, usage, and finish persistence failures because the owning End returned nil.

## GREEN evidence

- Focused accounting/config/trajectory tests passed across runtime, RPC, storage, and config.
- `go test -p 2 ./internal/runtime -run 'TestCacheWarm|TestTrajectoryMaintenanceDoesNotInterruptChat' -count=1`: passed, including exact-prefix miss/write then hit/read, mandatory persistence failure propagation, optional provider failure closure, cancellation before terminal, conservative default, and missing-rate gates.
- `go test -p 2 ./internal/runtime ./internal/rpc ./internal/storage ./internal/config -count=1`: initial complete affected-package suites passed; final result below.

The parent delivery owns the integrated `just ci` gate, including the pinned Laputa source closure and UI/build checks. This lane does not claim standalone `just ci` success or live economic savings.

## Final lane results

- Complete affected-package suites passed on the final implementation: runtime 45.926s, RPC 17.686s, storage 0.003s, config 0.014s.
- Focused `go test -race -p 2 ./internal/runtime -run 'TestCacheWarm|TestTrajectoryMaintenanceDoesNotInterruptChat' -count=1` passed (10.279s).
- Added a synthetic priced-prefix gate check after the complete suites; the focused warming/trajectory suite was rerun and passed. It proves the gate excludes a much larger conversation when a cached rate is explicitly known.
- `go vet -p 2 ./internal/runtime ./internal/rpc ./internal/storage ./internal/config` and `git diff --check`: passed.

Warm fixture accounting: first request reports 1,345 total prompt tokens, including 1,234 creation tokens, and two output tokens. The repeated identical-prefix request reports 1,234 cached-read tokens inside the same 1,345 prompt tokens and two output tokens. Unreported cache-write presence on the hit remains unknown rather than synthesized zero. Prices are undeclared, so the fixture does not establish a dollar amount or profitability.

## Review follow-up verification

- RED `TestCacheWarmAdmissionBudgetDenialDoesNotFailOwningEnd`: both `MaxModelCalls: 1` and exhausted `MaxEvents: 2` returned budget errors from successful owning End.
- GREEN focused budget/persistence checks: optional denial skips, a real single-call Service.Run completes with no paid warm, and paid usage-budget/persistence failures remain mandatory.
- RED `TestRowCostMaintenanceUnreportedCacheWriteIsUnknown`: missing write presence was priced as known `$0.0045` with synthetic declared rates.
- RED `TestRowCostWarmCapableAggregateKeepsUnknownCacheWrite`: source ordering main/maintenance hid missing write evidence and reported known aggregate cost.
- Tests now cover both aggregate orders and explicitly reported zero-write evidence.
- Follow-up complete affected-package suites passed: runtime 48.992s, RPC 14.748s, storage 0.003s, config 0.012s.
- Follow-up focused race checks for optional admission, single-call run, paid usage-budget failure, and mandatory persistence failure passed (6.659s).
- Follow-up vet for the four affected packages and `git diff --check` passed. Integrated `just ci` remains parent-owned.

## Optional diagnostic event-cap follow-up

RED confirmed an event-admission-denied warm still appended an unbudgeted marker, and two economic skips appended two diagnostics at `MaxEvents: 1`. Every diagnostic now reserves event admission, and lack of admission omits the marker without failing the owning call or making provider work.

The event-cap follow-up complete affected-package suites passed: runtime 70.795s, RPC 25.431s, storage 0.012s, config 0.031s. All warming race checks passed (27.475s); affected-package vet and `git diff --check` passed. The parent owns the integrated product gate.

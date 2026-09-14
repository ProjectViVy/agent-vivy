# Verification — P8 review fixes

All commands run from the repository root on branch `fix/p8-review-fixes`.

## Package-level (during development)

```
go test ./internal/observerhost/  -count=1   # ok
go test ./internal/contexthost/   -count=1   # ok
go test ./internal/runtime/       -count=1   # ok (160.8s)
go test ./sdk/internal/assembly/  -count=1   # ok
```

One intermediate failure was found and fixed before CI:
`TestResumeEventMapperRestoresSummaryUsageRoutesFromJournal` panicked because
it builds `Service` via a struct literal (nil `contextViews`); the cache
write now lazily initializes the map (`storeContextView`).

## Full gate

```
just ci    # exit 0 (full kernel + UI + plugin-clone gate)
```

Result recorded in the delivery summary of this iteration: all packages ok,
including `plugins/scx-reference` and the generated-assembly golden tests
updated for the removed `"result"` allowlist field.

## New tests added

- `internal/observerhost`: `TestRetryDelayForDoublesPerAttemptAndCapsAtMax`
  (pure backoff schedule), `TestSCXObserverRetryBackoffBoundsPermanentFailureAttempts`
  (250ms window at 2..8 attempts vs ~50 unthrottled).
- `internal/contexthost`: `TestSCXRequiredExpiredCandidateFailsClosed`
  (`ErrRequiredContextExpired`).
- `internal/runtime`: `TestContextViewRecoveryIsCachedPerRun` (cache hit
  survives a closed backend; miss degrades to empty),
  `TestContextViewRecoveryToleratesIteratorFailure` (iterator error returns
  empty and caches nothing).

## Skipped slices

None. UI smoke not run: no user-visible UI change in this iteration
(kernel host/service behavior only), covered by `just ci`.

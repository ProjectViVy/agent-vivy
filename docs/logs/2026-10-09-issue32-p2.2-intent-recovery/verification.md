# Verification: P2.2 durable cognitive intent

## Toolchain

Used `/tmp/issue32-go/go/bin/go` (Go 1.26.4), downloaded and checksum-verified during P2.1. No toolchain files were added to the repository. Commands that launch `go` as a child process used `PATH=/tmp/issue32-go/go/bin:$PATH` because `/usr/bin/go` is an unrelated game CLI in this environment.

## Red regressions

Before the corresponding implementation fixes, the planned focused command exposed:

- `TestCognitiveIntentCrashMatrix`: `state_schema = 0`; the baseline saved no intent before admission.
- `TestCognitiveLegacyCrashGapFencesUnknown`: `Blocked=""`; legacy state with `SourceHigh=9`, no `PendingThrough`, and `Watermark=0` was replayed as a new workflow.
- `TestCognitiveRetrySafetyRequiresSettlementEvidence`: a failed workflow lookup was classified as retry-safe.

A follow-up assertion also failed because a native failed Run row without a terminal workflow-step projection was treated as settled. These were feature failures, not compile/setup failures.

## Independent review regressions

The first independent review found three additional recovery gaps. Each was reproduced with a regression before the implementation change:

- `TestCognitiveOrphanSupervisorIsAdoptedBeforeAdmission` failed with `storage: active run conflict` when a deterministic supervisor row existed but its ID/sequence had not reached the cognitive snapshot. The reconciler now adopts that active row; terminal spent IDs advance the sequence.
- `TestCognitiveRecoveryClassifiesRunning` observed `Blocked=""`, `ActiveRunID` set and `Phase="running"` for a native active Run whose engine projection was `recovery_required`. Reconciliation now fences it and clears the stale active pointer.
- `TestCognitiveLegacyFailedRunPreservesRetryBudget` recovered operation-key attempt `a2` as attempt 0. Recovery now parses and validates the suffix against the canonical input window and stored legacy counter; the safely failed `a2` run settles at attempt 3 and stays exhausted.
- `TestCognitiveFailedRunWaitsForDurableProjection` first failed because a native failed row with an engine projection still running became a sticky unknown fence. It now keeps the exact intent while projection settlement is pending. The retry regression also seeds that provisional fence and verifies that complete safe-failure evidence for the same Run clears it. Both retry/projection tests passed ten consecutive runs.

An initial complete runtime run also caught the transient-projection timing case in `TestCognitiveFailedRunRetriesWithNewKey`; after the correction, the complete package passed.

## Green verification

```text
go test ./internal/runtime -run '^TestCognitive(IntentCrashMatrix|IntentPreservesLaterInput|LegacyCrashGapFencesUnknown|RetrySafetyRequiresSettlementEvidence)$' -count=1
ok   agent-vivy/internal/runtime  0.172s

go test ./internal/runtime -run '^TestCognitive' -count=1
ok   agent-vivy/internal/runtime  1.295s

go test -race ./internal/runtime -run '^TestCognitive' -count=1
ok   agent-vivy/internal/runtime  24.995s

go test ./internal/storage/sqlite ./internal/storage/postgres -run 'TestBackendConformance/CN-03|TestSnapshotVersions' -count=1 -v
SQLite CN-03 expected-version conflict: PASS
SQLite TestSnapshotVersions: PASS
PostgreSQL: SKIP — VIVY_POSTGRES_TEST_DSN not set

go vet ./internal/runtime
exit code 0

git diff --check
exit code 0
```

The complete affected package was run with the corrected child-process PATH:

```text
PATH=/tmp/issue32-go/go/bin:$PATH /tmp/issue32-go/go/bin/go test ./internal/runtime -count=1
ok   agent-vivy/internal/runtime  38.161s
```

The same full-package command was first run without the PATH correction. It failed only in `TestCommandBackendRunsInsideWorkspaceAndBuildsProposal`, which invoked the unrelated `/usr/bin/go` and received `Go: Unknown option: version`; rerunning with the verified toolchain first on PATH passed.

`gofmt` was applied to all changed Go files.

## Pending gates

- PostgreSQL snapshot/conformance requires a disposable `VIVY_POSTGRES_TEST_DSN`; this environment did not provide one.
- `just ci` was attempted and returned `/bin/bash: just: command not found`. Aggregate CI remains pending in an environment with the repository's required tooling.

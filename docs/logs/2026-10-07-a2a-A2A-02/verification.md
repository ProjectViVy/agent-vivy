# A2A-02 verification

Environment: Go 1.26.8, sqlite + disposable PG14
(`postgres://vivy:vivy@localhost:5432/vivy_test`), laputa pin unchanged
`ff3936f44ff8cf08c12af2cf698c194cfe474fd3`.

## A2A-02.1 storage

```text
GIT_CONFIG_GLOBAL=/dev/null go test ./internal/storage/sqlite -run 'ChannelTask' -count=1 -v
PASS: TestChannelTaskAdmissionFaultMatrix (8 subtests:
      first-admit-commits-atomically, identical-retry-returns-original-receipt,
      foreign-scope-reveals-nothing, unowned-context-reveals-nothing,
      taken-candidate-conflicts, busy-context-rejects-distinct-message,
      failed-commit-leaves-no-orphans, list-paginates-owned-submissions)
      + TestChannelTaskAdmissionTwoHandles
GIT_CONFIG_GLOBAL=/dev/null VIVY_POSTGRES_TEST_DSN=... go test ./internal/storage/postgres -run 'ChannelTask' -count=1 -v
PASS: same matrix on postgres + FOR UPDATE scope/session locks
```

## A2A-02.2 runtime

```text
GIT_CONFIG_GLOBAL=/dev/null go test -race ./internal/runtime -run 'TestChannelTask' -count=1
PASS: TestChannelTaskMissingContextConcurrentRetry,
      TestChannelTaskReplayBeforeBusyGate,
      TestChannelTaskNewSessionPreparation,
      TestChannelTaskRejectsCompositeOptions,
      TestChannelTaskAdmittedPayloadSchema,
      TestChannelTaskOrphanSweep
```

## A2A-02.3 ownership + tombstones

```text
go test ./internal/storage/{sqlite,postgres} -run 'OwnershipAndTombstone' -count=1
PASS both engines: scoped-independence, no guessed-id disclosure,
     exclusion of local/child runs, deletion cannot resurrect, tombstone
     revive-with-new-identity, live-receipt-missing-run = corruption.
go test ./internal/runtime -run 'TestChannelTaskDeletionCannotResurrect' -count=1
PASS: resubmit mints fresh session; tombstoned context id is dead
     address space (ErrNotFound, never adopted).
```

## Story boundary

```text
just ci — see acceptance.md
conformance_results.json internal digest repinned (cee9423d…) LAST,
after every internal/ edit.
```

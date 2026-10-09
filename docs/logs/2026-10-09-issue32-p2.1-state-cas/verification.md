# Verification: P2.1 original-version state CAS

## Toolchain

The `/usr/bin/go` on the initial PATH is the unrelated “Local two-player Go”
game and rejects Go compiler commands. Go 1.26.4 was downloaded to `/tmp` from
the official Go distribution URL. Its SHA256
(`1153d3d50e0ac764b447adfe05c2bcf08e889d42a02e0fe0259bd47f6733ad7f`) matched
the checksum published by the Go download index. No toolchain files were added
to the repository.

## Red regressions

Before implementation, ran:

```text
go test ./internal/runtime -run '^TestCognitive(StateStaleVersionRejected|ConcurrentCapturePolicyAndSettlement|PolicyCASRejectsStaleRevision)$' -count=1
```

All three new tests failed as expected:

- The stale save replaced accepted `SourceHigh=9` with `0`.
- The unsynchronized concurrent state update returned
  `storage: snapshot version conflict`.
- Two policy writes from base revision 0 both succeeded instead of producing
  one success and one `ErrPolicyConflict`.

## Green checks

Using `/tmp/issue32-go/go/bin/go` (Go 1.26.4):

```text
go test ./internal/runtime -run '^TestCognitive(StateStaleVersionRejected|ConcurrentCapturePolicyAndSettlement|PolicyCASRejectsStaleRevision)$' -count=1
ok   agent-vivy/internal/runtime  0.070s

go test -race ./internal/runtime -run '^TestCognitive' -count=1
ok   agent-vivy/internal/runtime  15.872s

go vet ./internal/runtime
exit code 0
```

`gofmt` was applied to all four changed Go files. `gofmt -l` returned no paths,
and `git diff --check` passed.

The concurrency regression also verifies that an intentionally stale attempt
version forces exactly one rebased write after capture and policy updates.

## Aggregate gate

Attempted `just ci`; the shell returned `/bin/bash: just: command not found`.
This environment is Linux while the repository `justfile` selects
`powershell.exe`, so the aggregate Windows-oriented recipe could not be run
here. The runtime-focused tests and vet above passed; aggregate CI remains
pending in a supported environment.

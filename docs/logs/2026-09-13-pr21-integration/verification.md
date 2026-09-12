# PR #21 integration verification

## Initial GitHub failure

PR #21's first Windows backend CI run failed in
`TestHeadlessTurnApprovalFailsLoudly`: the run was cancelled, but one durable
approval remained pending. UI CI passed. The aggregate `just ci` job correctly
reported the backend failure.

## RED

```text
go test ./internal/runtime -run '^TestServiceCancelDuringApprovalPublishClosesDurableApproval$' -count=1 -timeout 2m
```

Result: FAIL as expected. The deterministic sink cancelled at publication and
the pending approval assertion reported the leaked approval ID.

## Focused GREEN

```text
go test ./internal/runtime -run '^TestServiceCancelDuringApprovalPublishClosesDurableApproval$' -count=20 -timeout 5m
go test ./internal/app -run '^TestHeadlessTurnApprovalFailsLoudly$' -count=20 -timeout 5m
go test -race ./internal/runtime -run '^(TestServiceCancelDuringApprovalPublishClosesDurableApproval|TestToolApprovalArgumentsHashCanonicalizesJSON|TestToolApprovalProposalBindingRoundTrip|TestRedactedApprovalArgumentsCopiesNestedValues)$' -count=10 -timeout 10m
go test ./internal/runtime ./internal/app -count=1 -timeout 20m
```

Results: PASS. The broader packages completed in 154.072 seconds and 45.154
seconds respectively.

## Product gate

The first bare `just ci` attempt was invalid because the local shell could not
resolve `gofmt`; it was stopped and is not counted as verification. The gate
was restarted with the installed Go toolchain bound explicitly:

```text
just --set go 'C:\Program Files\Go\bin\go.exe' --set gofmt 'C:\Program Files\Go\bin\gofmt.exe' ci
```

Result: PASS (exit 0).

- Go formatting and vet: PASS.
- Main-module Go tests and headless compile: PASS.
- UI typecheck: PASS.
- UI tests: 35 files, 316 tests, PASS.
- UI production build and i18n gates: PASS.
- All independent `plugins/*` and `faces/*` module vet/test gates: PASS.

The final pushed commit must also pass the repository's required GitHub checks
before merge.

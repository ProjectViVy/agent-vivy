# P5.2 verification

## Baseline red

Before implementation, the focused regressions demonstrated that successful
Generate/Stream output ignored an End error, provider and settlement failures
were not both visible to callers, and the real observer converted a Journal
append failure into a generic error without its storage cause. An overflow-like
provider error joined with a settlement failure also needed an explicit
non-retryable classification.

## Green

Commands run with Go 1.26.4 (`GOTOOLCHAIN=local`):

```text
go test ./internal/runtime -run '^Test(Observed(GenerateSettlementFailure|StreamSettlementFailure|ErrorsJoinSettlement|CloseDuringSettlementDoesNotBlock)|ModelSettlement(JournalFailure|RoutesAndCancellation|FailureIsNotRetried))$' -count=1 PASS
go test ./internal/runtime -run '^TestModelSettlementFailureReachesGenerate$' -count=1 PASS
go test -race ./internal/runtime -run '^Test(Observed(GenerateSettlementFailure|StreamSettlementFailure|ErrorsJoinSettlement|CloseDuringSettlementDoesNotBlock)|ModelSettlement)' -count=1 PASS
go test ./internal/runtime -run 'Test(Observed|Model|Cognitive)' -count=1 PASS
go test ./internal/runtime -run '^TestWorkflowProductCancelRun$' -count=10 PASS
go test ./internal/runtime -count=1 PASS
git diff --check PASS
```

The initial cancellation assertion required both engine cancellation and a
durable native terminal, which is not guaranteed when cancellation interrupts
settlement. The regression now accepts the two existing safe outcomes:
`cancelled` after durable settlement or `recovery_required` with an active
native Run. In both outcomes it confirms the child is cancelled, list/detail
projections agree, and duplicate start is refused.

The final-usage append fault is exercised directly against the real
`runModelCallObserver.End`; mandatory finish append failure is additionally
exercised through the real Generate wrapper and through the actual streaming
overflow run. For the Generate fixture, a provider usage sample is committed
during Chunk; End therefore has no changed usage sample to emit, so the test
faults `model.call.finished` there rather than inventing a duplicate final
usage event.

## Limits

`just` is not installed in this environment, so aggregate `just ci` and SDK
conformance remain pending P7. P5.1 Windows tests cross-compile but were not
executed natively. If the Journal rejects both the mandatory closure and the
best-effort run terminal, storage cannot durably record a finish or terminal;
the implementation returns the original failure and does not replay inference.

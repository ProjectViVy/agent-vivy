# Verification

Baseline: `86215cf20b65d8f45c3481c2bc965b091ea6590b`.
Native isolated checkout:
`C:\Users\Administrator\.codex\worktrees\d14-runtime-residue\agent-vivy`.
Independent Linux-filesystem checkout:
`/home/mastwet/work/vivy-d14-suspension-registration-9214f2/agent-vivy`,
advanced to the baseline above before applying the exact candidate patch.
Both checkouts use their own sibling Laputa pinned to
`9acc999301d5d575da2cea902a1c6d213763f381`; Go is `1.26.4`.

## Reproduction

Independent actual workflow review on the baseline reproduced an approved row
with a nil decision error before private registration, while its required event
was durable and `projectionMu` was free (package failure in 0.113s). The proof
ran the native graph and checkpoint owner, not a fabricated checkpoint payload.

The new committed regression was added before the product correction:

```text
go test -p 1 ./internal/runtime -run 'TestD14OrchestrationResponseWaits' -count=1 -timeout=60s
FAIL agent-vivy/internal/runtime 1.537s
```

Its deny subcase returned nil before registration. After removing the
unsupported exclusion and reporting the existing handoff outcome directly,
the workflow gap and existing workflow cancellation cases passed in 4.957s.

## Final focused verification

Exact shared selector:

```text
TestD14(NativeResponse|CancelDuringNative|Orchestration)|TestServiceApproval(ApproveFlow|DenyFlow|DecisionGuards)|TestServiceCancelDuringApprovalPublish|TestServiceRecoverResumableApproval|TestServiceQuestion(SuspendAnswerAndResume|RecoveryKeepsQuestionDistinct)|TestInactiveNativeChildApproval|TestNativeOneShotChildUsesServiceApprovalResume|TestNativeOneShotChildApprovalRecovers|TestOrchestrationApproval|TestOrchestrationCancellationStopsApprovalSibling|TestRunShellApproval(Approve|Deny)|TestServiceSweepAutoApprovesSmartApprovalOnTimeout
```

- Native Windows: `go test -p 1 ./internal/runtime -run '<selector above>'
  -count=1 -timeout=120s` — exit 0; package passed in **34.475s**.
- Independent Debian WSL checkout, `GOFLAGS=-p=2`: `go test -race
  ./internal/runtime -run '<selector above>' -count=1 -timeout=180s` — exit 0;
  package passed in **106.155s**, no race reports.
- `gofmt` ran on the three changed Go files; `git diff --cached --check` passed.

The final selector includes ordinary approve/deny/question continuation,
unregistered-slot refusal and retry, cancellation, deadline crossing, duplicate
decisions, smart timeout approval, child/restart behavior, direct shell paths,
native workflow approve/deny in the publication gap, workflow gap cancellation,
failed workflow approval handoff and existing workflow cancellation cases.
The workflow assertions verify actual node outcomes, original invocation
handoff result, unchanged completed-sibling transitions and one terminal.

## Review limits

No separate deterministic reproduction was claimed for the consumption-sensitive
pending-map reread. Source inspection established that the response owner can
delete the entry after registration; the direct boolean reports that completed
handoff without depending on the later map state. The tests exercise its
success/cancellation/failure meanings without adding scheduler choreography or
a production hook.

This lane did not run full-product CI, wiring/kernel gates, broad control race,
Postgres, browser smoke, packaging or copied-store upgrade acceptance. The
parent owns final integrated acceptance. Temporary Windows patch/runner files
were removed after Linux verification; disposable test stores were drained and
closed. No production data was accessed.

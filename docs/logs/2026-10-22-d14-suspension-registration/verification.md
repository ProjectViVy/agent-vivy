# Verification

Baseline: `9214f200ec0e5df7b4ffd5231eb4cdd56b27de06`.

Native isolated checkout:
`C:\Users\Administrator\.codex\worktrees\d14-runtime-residue\agent-vivy`.
Independent Linux-filesystem checkout:
`/home/mastwet/work/vivy-d14-suspension-registration-9214f2/agent-vivy`.
Both use their own clean sibling Laputa checkout pinned to
`9acc999301d5d575da2cea902a1c6d213763f381`. Go is `1.26.4` on both hosts.

## Reproduction and test-first evidence

An independent read-only overlay reproduced both published-request gaps on the
actual suspend/settlement owners, with real SQLite admission, immutable prompt,
checkpoint envelope and active Run map. Both public responses returned success,
settled their rows, then left a pending entry with no terminal after release.
That audit's opaque checkpoint bytes deliberately isolated owner ordering;
the committed regression fixture additionally runs actual native Eino
suspension and continuation.

Before the readiness fix:

```text
go test -p 1 ./internal/runtime -run 'TestD14NativeResponse' -count=1 -timeout=90s
FAIL agent-vivy/internal/runtime 2.918s
```

The deny/question boundary cases returned before registration; both missing
registration cases consumed their irreversible slot. After the readiness fix,
the native continuation/slot suite passed in 8.134s.

The deadline audit then found status-only settlement CAS in both storage
dialects. Before adding the post-wait expiry checks:

```text
go test -p 1 ./internal/runtime -run 'TestD14NativeResponseExpiring' -count=1 -timeout=60s
FAIL agent-vivy/internal/runtime 1.587s
```

Both approval and question responses returned nil after their human deadline
crossed while waiting for registration, rather than their existing expired
errors. The final tests cover those refusals and the existing intentional system
auto-approval path.

## Final focused commands

The exact common selector is:

```text
TestD14(NativeResponse|CancelDuringNative)|TestServiceApproval(ApproveFlow|DenyFlow|DecisionGuards)|TestServiceCancelDuringApprovalPublish|TestServiceRecoverResumableApproval|TestServiceQuestion(SuspendAnswerAndResume|RecoveryKeepsQuestionDistinct)|TestInactiveNativeChildApproval|TestNativeOneShotChildUsesServiceApprovalResume|TestNativeOneShotChildApprovalRecovers|TestOrchestrationApprovalResumes|TestRunShellApproval(Approve|Deny)|TestServiceSweepAutoApprovesSmartApprovalOnTimeout
```

- Native: `go test -p 1 ./internal/runtime -run '<selector above>' -count=1
  -timeout=120s` — exit 0, package passed in **30.785s** on final source.
- Debian WSL, Linux filesystem, `GOFLAGS=-p=2`: `go test -race
  ./internal/runtime -run '<selector above>' -count=1 -timeout=180s` — exit 0,
  package passed in **85.138s** on final source, no race reports. An earlier
  pre-deadline-refinement race pass (75.275s) is superseded by this final result.
- `gofmt` ran on the changed Go files; `git diff --cached --check` passed.

Assertions prove real checkpoint continuation, two total model requests, one
tool request/result, one terminal, one approved effect, no denied/cancelled
effect, duplicate refusal, bounded registration failure without CAS, retry of
the preserved slot, cancellation during registration, and deadline crossing.
Existing targeted child/restart/shell/native orchestration and synchronous
cancel-on-publish tests passed alongside them.

## Scoped review and remaining acceptance

The readiness check precedes the decision/answer CAS. Native root approvals
exclude shell/native orchestration; existing child behavior remains. Plan has
its separate unmodified decision owner. Approval expiry permission is explicit
at the private system caller, not inferred from user-controlled actor text.
No additional pending-state owner or checkpoint format was introduced.

No full native CI, wiring/kernel gate, broad control race, Postgres, browser,
pack, copied-store conversion or release gate ran in this scoped lane. Those
belong to the integrated candidate. Temporary patch and runner scripts were
removed from the Windows checkout after Linux verification. Test stores used
owned temporary directories and were closed/drained by fixture cleanup.

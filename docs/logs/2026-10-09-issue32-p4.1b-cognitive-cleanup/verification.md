# P4.1b verification

## Baseline red

Before the cleanup guard, two focused cases leaked the bundle: primary
admission and cognitive action dispatcher validation both returned with
`Close` count zero. ResolveBinding did close the bundle, but discarded the
Close sentinel instead of retaining both causes.

## Green

Commands run with Go 1.26.4 (`GOTOOLCHAIN=local`):

```text
go test ./internal/app -run '^TestCognitiveBundle(ClosedOnEveryCompositionFailure|FailurePreservesCleanupError|OwnershipTransfersOnce)$' -count=1 PASS
go test ./internal/app -count=1 PASS
git diff --check PASS
```

The fault matrix uses the real app composition and real temporary SQLite
backend. Its primary-admission fixture exposes WorkStore, GoalRunStore and
PrimaryRunStore while deliberately omitting RunAdmissionStore; it verifies
Bundle.Close runs while the backend is still open, then confirms the backend
closes and can be reopened for the second attempt. Other cases fail after
bundle creation at observer-host construction, ResolveBinding, AttachRuntime,
and action dispatcher validation. The cleanup-error test verifies both
sentinels with `errors.Is`.

As with P4.1a, Go's embedded UI package needed a temporary `ui/dist/.keep` to
compile the gateway-less App tests. The marker was removed afterward. A real
frontend build remains unavailable because UI dependencies are absent and the
configured package registry is unreachable.

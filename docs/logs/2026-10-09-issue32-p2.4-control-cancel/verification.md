# Verification

## Red evidence

Before implementation, the runtime regression package failed to compile because `CognitiveControlState`, `CancelCognitiveRun` and `ErrCognitiveRunMismatch` were undefined. This exposed the absent service seams. DIVA's existing dispatcher already copied pending/phase/block values when a ControlPort supplied them.

## Green evidence

- Independent review found a medium race: `emitTerminal` could persist a terminal Run while its active handle remained registered, allowing cancellation to return true from a stale status check.
- Added a deterministic second-read regression. Before the fix, it failed because cancellation returned `true` after the atomic status recheck was scripted to observe a completed Run.
- `cancelRunBeforeTerminal` re-reads Run status under `projectionMu`, which `emitTerminal` holds through terminal persistence and active-handle cleanup, and signals the cancellation context before releasing the lock. It does not invoke the generic cancellation path while holding the lock because pending cancellation can emit terminal synchronously.
- `go test ./internal/runtime ./internal/modules/diva-cognitive -count=1` — passed after the review fix (`internal/runtime` 39.229s; module 0.083s).
- `go test ./internal/app -count=1` — passed after the review fix (6.438s); a temporary `ui/dist/.keep` satisfied the embed pattern and was removed afterward.
- Focused status projection, terminal-wins/active cancellation, foreign/stale/settled IDs, app ControlPort and action JSON tests — passed.
- `go test -race ./internal/runtime -run 'TestCognitive(ControlStateOneSnapshot|CancelScopeMatrix)' -count=1` — passed after the review fix (2.491s).
- `go vet ./internal/runtime ./internal/modules/diva-cognitive ./internal/app` and `git diff --check` — passed after the review fix.

The pre-existing DIVA `EvolutionView.vue` already renders all recovery fields. No frontend build was required for this phase because no UI source changed. The independent review finding is fixed; commit is pending.

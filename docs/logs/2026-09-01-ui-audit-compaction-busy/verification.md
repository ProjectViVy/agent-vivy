# Verification

## Gates

- `just ci` — passed (exit 0): Go fmt/vet/test, headless compile, six plugin-ci
  modules, and UI install + `tsc --noEmit` + `vitest run` + `vite build` all
  green (`runActive` export, card subscription, and busyHint key pass type and
  lint checks).
- `just ui-e2e` — passed (exit 0, 10 passed / 1 skipped): real browser + real
  control plane; `compaction-setting.spec.ts` asserts labels only (it does not
  click the button), so button-disabling logic does not affect existing
  assertions.

## Smoke notes

- There is no component-specific spec for the busy-state browser assertion
  (start a turn → the Settings button disables → the run ends → it recovers);
  following the CH-C1-N3 precedent, the full e2e suite is the smoke substitute,
  and the behavior path is available for manual review in acceptance.md.
- The 409 fallback path is unchanged (the backend and `compactNow` catch remain
  as-is), so race behavior does not regress.

## Static review evidence

- `internal/runtime/compaction_service.go:123-127`: busy =
  `len(s.active) > 0 || len(s.pending) > 0` (global to the engine) → 409.
- `ui/src/lib/store.ts`: exports the `runActive` predicate; `currentRun` is
  updated in real time by subscription events (`run.started` → active,
  `run.completed/failed/cancelled` → terminal); `backgroundRuns` is maintained
  by init and `loadBackgroundRuns()`.
- The card's disabling condition `runActive(currentRun) ||
  backgroundRuns.some(runActive)` covers both observable sources; the blind spot
  for cross-client foreground runs is recorded in summary.md.

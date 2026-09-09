# UI-AUDIT-COMPACTION-BUSY — Busy-state precheck for Compact now

## Problem (audit item)

The "Compact now" action in the Settings → Context compaction card could be
clicked while the engine was busy, so the user received 409
`ErrCompactionBusy` only after clicking. Backend busy state is **global to the
engine**: `CompactSession` in `compaction_service.go` rejects when
`len(s.active) > 0 || len(s.pending) > 0`—activity or queued runs in any session
block manual compaction (the run itself compacts inside the run).

## Fix (UI precheck + 409 fallback)

`CompactionSettingsCard` now subscribes to the store's source of truth for run
state:

- `runActive(currentRun)` — the in-flight turn in the currently attached session
  (updated in real time by subscription events)
- `backgroundRuns.some(runActive)` — non-terminal runs in the background
  registry (anything other than `completed/failed/cancelled`)

Either match disables "Compact now" and displays the amber
`settings.compaction.busyHint` (en/zh): "A run is in flight; compaction runs
inside it; wait for the run to finish." The "Refresh usage" button now also
calls `loadBackgroundRuns()` to refetch the background registry, so the user
can manually recheck the busy state.

`store.runActive` (the terminal-state predicate and single source of truth) is
now exported for reuse instead of remaining module-private.

## Remaining races (recorded, not fixed)

- A new run can start between the precheck and the click—409 remains the
  fallback, and `compactNow`'s catch already displays the error in the feedback
  area.
- A **foreground** run attached by another client (such as another browser tab)
  is invisible to this tab's store if it is not in the background registry; only
  the 409 fallback can handle this cross-client busy state.
- `store.ts` and `DashboardView.tsx` each have a terminal-state set literal. The
  card now uses `runActive`, while Dashboard's count predicate has a different
  shape (filter count), so they are not forcibly unified yet.

## Change list

- `ui/src/lib/store.ts`: add `export` to `runActive`.
- `ui/src/components/settings/CompactionSettingsCard.tsx`: busy subscription +
  button disabling + busyHint + refresh coupling to `loadBackgroundRuns`; update
  the doc comment.
- `ui/src/i18n/en.ts` / `zh.ts`: `settings.compaction.busyHint`.
